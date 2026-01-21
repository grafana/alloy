package receiver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/go-kit/log"
	"github.com/go-sourcemap/sourcemap"
	"github.com/grafana/alloy/internal/component/faro/receiver/internal/payload"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/internal/util/wildcard"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/vincent-petithory/dataurl"
)

// sourceMapsStore is an interface for a sourcemap service capable of
// transforming minified source locations to the original source location.
type sourceMapsStore interface {
	GetSourceMap(sourceURL string, release string) (*sourcemap.Consumer, error)
	Start()
	Stop()
}

// Stub interfaces for easier mocking.
type (
	httpClient interface {
		Get(url string) (*http.Response, error)
	}

	fileService interface {
		Stat(name string) (fs.FileInfo, error)
		ReadFile(name string) ([]byte, error)
		ValidateFilePath(name string) (string, error)
	}
)

type osFileService struct{}

func (fs osFileService) ValidateFilePath(name string) (string, error) {
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid file name: %s", name)
	}
	return name, nil
}

func (fs osFileService) Stat(name string) (fs.FileInfo, error) {
	if _, err := fs.ValidateFilePath(name); err != nil {
		return nil, err
	}
	return os.Stat(name)
}

func (fs osFileService) ReadFile(name string) ([]byte, error) {
	if _, err := fs.ValidateFilePath(name); err != nil {
		return nil, err
	}
	return os.ReadFile(name)
}

type sourceMapMetrics struct {
	cacheSize *prometheus.GaugeVec
	downloads *prometheus.CounterVec
	fileReads *prometheus.CounterVec
}

func newSourceMapMetrics(reg prometheus.Registerer) *sourceMapMetrics {
	m := &sourceMapMetrics{
		cacheSize: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "faro_receiver_sourcemap_cache_size",
			Help: "number of items in source map cache, per origin",
		}, []string{"origin"}),
		downloads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "faro_receiver_sourcemap_downloads_total",
			Help: "downloads by the source map service",
		}, []string{"origin", "http_status"}),
		fileReads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "faro_receiver_sourcemap_file_reads_total",
			Help: "source map file reads from file system, by origin and status",
		}, []string{"origin", "status"}),
	}

	m.cacheSize = util.MustRegisterOrGet(reg, m.cacheSize).(*prometheus.GaugeVec)
	m.downloads = util.MustRegisterOrGet(reg, m.downloads).(*prometheus.CounterVec)
	m.fileReads = util.MustRegisterOrGet(reg, m.fileReads).(*prometheus.CounterVec)
	return m
}

type sourcemapFileLocation struct {
	LocationArguments
	pathTemplate *template.Template
}

type timeSource interface {
	Now() time.Time
}

type realTimeSource struct{}

func (realTimeSource) Now() time.Time {
	return time.Now()
}

type sourceMapsStoreImpl struct {
	log     log.Logger
	cli     httpClient
	fs      fileService
	args    SourceMapsArguments
	metrics *sourceMapMetrics
	locs    []*sourcemapFileLocation

	cacheMut      sync.Mutex
	cache         map[string]*cachedSourceMap
	timeSource    timeSource
	cleanupCtx    context.Context
	cleanupCancel context.CancelFunc
	cleanupWg     sync.WaitGroup
	isStarted     bool
}

type cachedSourceMap struct {
	consumer *sourcemap.Consumer
	lastUsed time.Time
}

// newSourceMapStore creates an implementation of sourceMapsStore. The returned
// implementation is not dynamically updatable; create a new sourceMapsStore
// implementation if arguments change.
func newSourceMapsStore(log log.Logger, args SourceMapsArguments, metrics *sourceMapMetrics, cli httpClient, fs fileService) *sourceMapsStoreImpl {
	// TODO(rfratto): it would be nice for this to be dynamically updatable, but
	// that will require swapping out the http client (when the timeout changes)
	// or to find a way to inject a download timeout without modifying the http
	// client.

	if cli == nil {
		cli = &http.Client{Timeout: args.DownloadTimeout}
	}
	if fs == nil {
		fs = osFileService{}
	}

	locs := []*sourcemapFileLocation{}
	for _, loc := range args.Locations {
		tpl, err := template.New(loc.Path).Parse(loc.Path)
		if err != nil {
			panic(err) // TODO(rfratto): why is this set to panic?
		}

		locs = append(locs, &sourcemapFileLocation{
			LocationArguments: loc,
			pathTemplate:      tpl,
		})
	}

	return &sourceMapsStoreImpl{
		log:        log,
		cli:        cli,
		fs:         fs,
		args:       args,
		cache:      make(map[string]*cachedSourceMap),
		metrics:    metrics,
		locs:       locs,
		timeSource: realTimeSource{},
	}
}

func (store *sourceMapsStoreImpl) GetSourceMap(sourceURL string, release string) (*sourcemap.Consumer, error) {
	store.cacheMut.Lock()
	defer store.cacheMut.Unlock()

	cacheKey := fmt.Sprintf("%s__%s", sourceURL, release)
	if cached, ok := store.cache[cacheKey]; ok {
		if cached != nil {
			cached.lastUsed = store.timeSource.Now()
			return cached.consumer, nil
		}
		return nil, nil
	}

	content, sourceMapURL, err := store.getSourceMapContent(sourceURL, release)
	if err != nil || content == nil {
		store.cache[cacheKey] = nil
		return nil, err
	}

	consumer, err := sourcemap.Parse(sourceMapURL, content)
	if err != nil {
		store.cache[cacheKey] = nil
		level.Debug(store.log).Log("msg", "failed to parse source map", "url", sourceMapURL, "release", release, "err", err)
		return nil, err
	}
	level.Info(store.log).Log("msg", "successfully parsed source map", "url", sourceMapURL, "release", release)
	store.cache[cacheKey] = &cachedSourceMap{
		consumer: consumer,
		lastUsed: store.timeSource.Now(),
	}
	store.metrics.cacheSize.WithLabelValues(getOrigin(sourceURL)).Inc()
	return consumer, nil
}

func (store *sourceMapsStoreImpl) CleanOldCacheEntries() {
	store.cacheMut.Lock()
	defer store.cacheMut.Unlock()

	ttl := store.args.Cache.TTL
	for key, cached := range store.cache {
		if cached != nil && cached.lastUsed.Before(store.timeSource.Now().Add(-ttl)) {
			srcUrl := strings.SplitN(key, "__", 2)[0]
			origin := getOrigin(srcUrl)
			store.metrics.cacheSize.WithLabelValues(origin).Dec()
			delete(store.cache, key)
		}
	}
}

func (store *sourceMapsStoreImpl) CleanCachedErrors() {
	store.cacheMut.Lock()
	defer store.cacheMut.Unlock()

	for key, cached := range store.cache {
		if cached == nil {
			delete(store.cache, key)
		}
	}
}

// Start begins the cleanup routines based on configured cache intervals.
func (store *sourceMapsStoreImpl) Start() {
	store.cacheMut.Lock()
	defer store.cacheMut.Unlock()

	if store.isStarted {
		return
	}
	store.isStarted = true

	cacheConfig := store.args.Cache
	if cacheConfig == nil {
		return
	}

	store.cleanupCtx, store.cleanupCancel = context.WithCancel(context.Background())

	if d := cacheConfig.CleanupCheckInterval; d > 0 {
		store.cleanupWg.Add(1)
		go func(interval time.Duration) {
			defer store.cleanupWg.Done()
			store.CleanOldCacheEntries()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-store.cleanupCtx.Done():
					return
				case <-ticker.C:
					store.CleanOldCacheEntries()
				}
			}
		}(d)
	}

	if d := cacheConfig.ErrorCleanupInterval; d > 0 {
		store.cleanupWg.Add(1)
		go func(interval time.Duration) {
			defer store.cleanupWg.Done()
			store.CleanCachedErrors()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-store.cleanupCtx.Done():
					return
				case <-ticker.C:
					store.CleanCachedErrors()
				}
			}
		}(d)
	}
}

// Stop terminates all cleanup goroutines and waits for them to finish.
func (store *sourceMapsStoreImpl) Stop() {
	store.cacheMut.Lock()
	defer store.cacheMut.Unlock()

	if !store.isStarted {
		return
	}
	store.isStarted = false

	if store.cleanupCancel != nil {
		store.cleanupCancel()
		store.cleanupCancel = nil
	}

	store.cleanupWg.Wait()
	store.cleanupCtx = nil
}

func (store *sourceMapsStoreImpl) getSourceMapContent(sourceURL string, release string) (content []byte, sourceMapURL string, err error) {
	// Attempt to find the source map in the filesystem first.
	for _, loc := range store.locs {
		content, sourceMapURL, err = store.getSourceMapFromFileSystem(sourceURL, release, loc)
		if content != nil || err != nil {
			return content, sourceMapURL, err
		}
	}

	// Attempt to download the sourcemap if enabled.
	if strings.HasPrefix(sourceURL, "http") && urlMatchesOrigins(sourceURL, store.args.DownloadFromOrigins) && store.args.Download {
		return store.downloadSourceMapContent(sourceURL)
	}
	return nil, "", nil
}

func (store *sourceMapsStoreImpl) getSourceMapFromFileSystem(sourceURL string, release string, loc *sourcemapFileLocation) (content []byte, sourceMapURL string, err error) {
	if len(sourceURL) == 0 || !strings.HasPrefix(sourceURL, loc.MinifiedPathPrefix) || strings.HasSuffix(sourceURL, "/") {
		return nil, "", nil
	}

	var rootPath bytes.Buffer

	err = loc.pathTemplate.Execute(&rootPath, struct{ Release string }{Release: cleanFilePathPart(release)})
	if err != nil {
		return nil, "", err
	}

	pathParts := []string{rootPath.String()}
	for _, part := range strings.Split(strings.TrimPrefix(strings.Split(sourceURL, "?")[0], loc.MinifiedPathPrefix), "/") {
		if len(part) > 0 && part != "." && part != ".." {
			pathParts = append(pathParts, part)
		}
	}
	mapFilePath := filepath.Join(pathParts...) + ".map"

	validMapFilePath, err := store.fs.ValidateFilePath(mapFilePath)
	if err != nil {
		store.metrics.fileReads.WithLabelValues(getOrigin(sourceURL), "invalid_path").Inc()
		level.Debug(store.log).Log("msg", "source map path contains invalid characters", "url", sourceURL, "file_path", mapFilePath)
		return nil, "", err
	}

	if _, err := store.fs.Stat(validMapFilePath); err != nil {
		store.metrics.fileReads.WithLabelValues(getOrigin(sourceURL), "not_found").Inc()
		level.Debug(store.log).Log("msg", "source map not found on filesystem", "url", sourceURL, "file_path", validMapFilePath)
		return nil, "", nil
	}
	level.Debug(store.log).Log("msg", "source map found on filesystem", "url", sourceURL, "file_path", validMapFilePath)

	content, err = store.fs.ReadFile(validMapFilePath)
	if err != nil {
		store.metrics.fileReads.WithLabelValues(getOrigin(sourceURL), "error").Inc()
	} else {
		store.metrics.fileReads.WithLabelValues(getOrigin(sourceURL), "ok").Inc()
	}

	return content, sourceURL, err
}

func (store *sourceMapsStoreImpl) downloadSourceMapContent(sourceURL string) (content []byte, resolvedSourceMapURL string, err error) {
	level.Debug(store.log).Log("msg", "attempting to download source file", "url", sourceURL)

	result, err := store.downloadFileContents(sourceURL)
	if err != nil {
		level.Debug(store.log).Log("msg", "failed to download source file", "url", sourceURL, "err", err)
		return nil, "", err
	}

	match := reSourceMap.FindAllStringSubmatch(string(result), -1)
	if len(match) == 0 {
		level.Debug(store.log).Log("msg", "no source map url found in source", "url", sourceURL)
		return nil, "", nil
	}
	sourceMapURL := match[len(match)-1][2]

	// Inline sourcemap
	if strings.HasPrefix(sourceMapURL, "data:") {
		dataURL, err := dataurl.DecodeString(sourceMapURL)
		if err != nil {
			level.Debug(store.log).Log("msg", "failed to parse inline source map data url", "url", sourceURL, "err", err)
			return nil, "", err
		}

		level.Info(store.log).Log("msg", "successfully parsed inline source map data url", "url", sourceURL)
		return dataURL.Data, sourceURL + ".map", nil
	}
	// Remote sourcemap
	resolvedSourceMapURL = sourceMapURL

	// If the URL is relative, we need to attempt to resolve the absolute URL.
	if !strings.HasPrefix(resolvedSourceMapURL, "http") {
		base, err := url.Parse(sourceURL)
		if err != nil {
			level.Debug(store.log).Log("msg", "failed to parse source URL", "url", sourceURL, "err", err)
			return nil, "", err
		}
		relative, err := url.Parse(sourceMapURL)
		if err != nil {
			level.Debug(store.log).Log("msg", "failed to parse source map URL", "url", sourceURL, "sourceMapURL", sourceMapURL, "err", err)
			return nil, "", err
		}

		resolvedSourceMapURL = base.ResolveReference(relative).String()
		level.Debug(store.log).Log("msg", "resolved absolute source map URL", "url", sourceURL, "sourceMapURL", sourceMapURL)
	}

	level.Debug(store.log).Log("msg", "attempting to download source map file", "url", resolvedSourceMapURL)
	result, err = store.downloadFileContents(resolvedSourceMapURL)
	if err != nil {
		level.Debug(store.log).Log("msg", "failed to download source map file", "url", resolvedSourceMapURL, "err", err)
		return nil, "", err
	}

	return result, resolvedSourceMapURL, nil
}

func (store *sourceMapsStoreImpl) downloadFileContents(url string) ([]byte, error) {
	resp, err := store.cli.Get(url)
	if err != nil {
		store.metrics.downloads.WithLabelValues(getOrigin(url), "?").Inc()
		return nil, err
	}
	defer resp.Body.Close()

	store.metrics.downloads.WithLabelValues(getOrigin(url), fmt.Sprint(resp.StatusCode)).Inc()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

var reSourceMap = regexp.MustCompile("//[#@]\\s(source(?:Mapping)?URL)=\\s*(?P<url>\\S+)\r?\n?$")

func getOrigin(URL string) string {
	// TODO(rfratto): why are we parsing this every time? Let's parse it once.

	parsed, err := url.Parse(URL)
	if err != nil {
		return "?" // TODO(rfratto): should invalid URLs be permitted?
	}
	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
}

// urlMatchesOrigins returns true if URL matches at least one of origin prefix. Wildcard '*' and '?' supported
func urlMatchesOrigins(URL string, origins []string) bool {
	for _, origin := range origins {
		if origin == "*" || wildcard.Match(origin+"*", URL) {
			return true
		}
	}
	return false
}

func cleanFilePathPart(x string) string {
	return strings.TrimLeft(strings.ReplaceAll(strings.ReplaceAll(x, "\\", ""), "/", ""), ".")
}

func transformException(log log.Logger, store sourceMapsStore, ex *payload.Exception, release string) *payload.Exception {
	if ex.Stacktrace == nil {
		return ex
	}

	var frames []payload.Frame
	for _, frame := range ex.Stacktrace.Frames {
		mappedFrame, err := resolveSourceLocation(store, &frame, release)
		if err != nil {
			level.Error(log).Log("msg", "Error resolving stack trace frame source location", "err", err)
			frames = append(frames, frame)
		} else if mappedFrame != nil {
			frames = append(frames, *mappedFrame)
		} else {
			frames = append(frames, frame)
		}
	}

	return &payload.Exception{
		Type:       ex.Type,
		Value:      ex.Value,
		Stacktrace: &payload.Stacktrace{Frames: frames},
		Timestamp:  ex.Timestamp,
		Context:    ex.Context,
		Trace:      ex.Trace,
	}
}

func resolveSourceLocation(store sourceMapsStore, frame *payload.Frame, release string) (*payload.Frame, error) {
	smap, err := store.GetSourceMap(frame.Filename, release)
	if err != nil {
		return nil, err
	}
	if smap == nil {
		return nil, nil
	}

	file, function, line, col, ok := smap.Source(frame.Lineno, frame.Colno)
	if !ok {
		return nil, nil
	}
	// unfortunately in many cases go-sourcemap fails to determine the original function name.
	// not a big issue as long as file, line and column are correct
	if len(function) == 0 {
		function = "?"
	}
	return &payload.Frame{
		Filename: file,
		Lineno:   line,
		Colno:    col,
		Function: function,
	}, nil
}

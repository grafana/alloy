package receiver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/go-sourcemap/sourcemap"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/vincent-petithory/dataurl"

	"github.com/grafana/alloy/internal/component/faro/receiver/internal/payload"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/internal/util/wildcard"
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
	log         *slog.Logger
	cli         httpClient
	downloadCli httpClient // Fetches URLs taken from client telemetry.
	fs          fileService
	args        SourceMapsArguments
	origins     originPatterns
	metrics     *sourceMapMetrics
	locs        []*sourcemapFileLocation

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
//
// cli fetches sourcemaps from operator-configured location blocks. downloadCli
// fetches URLs taken from client telemetry and must only follow redirects that
// match download_from_origins. A nil client uses the default.
func newSourceMapsStore(log *slog.Logger, args SourceMapsArguments, metrics *sourceMapMetrics, cli, downloadCli httpClient, fs fileService) *sourceMapsStoreImpl {
	// TODO(rfratto): it would be nice for this to be dynamically updatable, but
	// that will require swapping out the http client (when the timeout changes)
	// or to find a way to inject a download timeout without modifying the http
	// client.

	origins := parseOriginPatterns(args.DownloadFromOrigins)
	for _, origin := range origins.invalid {
		log.Warn("ignoring invalid download_from_origins entry", "origin", origin)
	}
	if cli == nil {
		cli = &http.Client{Timeout: args.DownloadTimeout}
	}
	if downloadCli == nil {
		downloadCli = newDownloadClient(args.DownloadTimeout, origins)
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
		log:         log,
		cli:         cli,
		downloadCli: downloadCli,
		fs:          fs,
		args:        args,
		origins:     origins,
		cache:       make(map[string]*cachedSourceMap),
		metrics:     metrics,
		locs:        locs,
		timeSource:  realTimeSource{},
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
		store.log.Debug("failed to parse source map", "url", sourceMapURL, "release", release, "err", err)
		return nil, err
	}
	store.log.Info("successfully parsed source map", "url", sourceMapURL, "release", release)
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
		if hasHttpPrefix(loc.Path) {
			continue
		}

		content, sourceMapURL, err = store.getSourceMapFromFileSystem(sourceURL, release, loc)
		if content != nil || err != nil {
			return content, sourceMapURL, err
		}
	}

	// Attempt to find the source map in the remote locations.
	for _, loc := range store.locs {
		if !(hasHttpPrefix(loc.Path)) {
			continue
		}

		content, sourceMapURL, err = store.getSourceMapFromRemote(sourceURL, release, loc)
		if content != nil || err != nil {
			return content, sourceMapURL, err
		}
	}

	// Attempt to download the sourcemap if enabled.
	if store.args.Download && hasHttpPrefix(sourceURL) && store.origins.matches(sourceURL) {
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
		store.log.Debug("source map path contains invalid characters", "url", sourceURL, "file_path", mapFilePath)
		return nil, "", err
	}

	if _, err := store.fs.Stat(validMapFilePath); err != nil {
		store.metrics.fileReads.WithLabelValues(getOrigin(sourceURL), "not_found").Inc()
		store.log.Debug("source map not found on filesystem", "url", sourceURL, "file_path", validMapFilePath)
		return nil, "", nil
	}
	store.log.Debug("source map found on filesystem", "url", sourceURL, "file_path", validMapFilePath)

	content, err = store.fs.ReadFile(validMapFilePath)
	if err != nil {
		store.metrics.fileReads.WithLabelValues(getOrigin(sourceURL), "error").Inc()
	} else {
		store.metrics.fileReads.WithLabelValues(getOrigin(sourceURL), "ok").Inc()
	}

	return content, sourceURL, err
}

func (store *sourceMapsStoreImpl) getSourceMapFromRemote(sourceURL string, release string, loc *sourcemapFileLocation) (content []byte, sourceMapURL string, err error) {
	if len(sourceURL) == 0 || !strings.HasPrefix(sourceURL, loc.MinifiedPathPrefix) || strings.HasSuffix(sourceURL, "/") {
		return nil, "", nil
	}

	var rootPath bytes.Buffer

	err = loc.pathTemplate.Execute(&rootPath, struct{ Release string }{Release: cleanFilePathPart(release)})
	if err != nil {
		return nil, "", err
	}

	subPath := strings.TrimPrefix(strings.Split(sourceURL, "?")[0], loc.MinifiedPathPrefix) + ".map"
	mapURL, err := url.JoinPath(rootPath.String(), subPath)
	if err != nil {
		store.log.Debug("failed to construct sourcemap url for remote location", "base_path", rootPath, "sub_path", subPath, "err", err)
		return nil, "", err
	}

	content, err = store.downloadFileContents(store.cli, mapURL)
	if err != nil {
		store.log.Debug("failed to download sourcemap file from remote location", "url", mapURL, "err", err)
		return nil, "", err
	}

	return content, sourceURL, err
}

func (store *sourceMapsStoreImpl) downloadSourceMapContent(sourceURL string) (content []byte, resolvedSourceMapURL string, err error) {
	store.log.Debug("attempting to download source file", "url", sourceURL)

	result, err := store.downloadFileContents(store.downloadCli, sourceURL)
	if err != nil {
		store.log.Debug("failed to download source file", "url", sourceURL, "err", err)
		return nil, "", err
	}

	match := reSourceMap.FindAllStringSubmatch(string(result), -1)
	if len(match) == 0 {
		store.log.Debug("no source map url found in source", "url", sourceURL)
		return nil, "", nil
	}
	sourceMapURL := match[len(match)-1][2]

	// Inline sourcemap
	if strings.HasPrefix(sourceMapURL, "data:") {
		dataURL, err := dataurl.DecodeString(sourceMapURL)
		if err != nil {
			store.log.Debug("failed to parse inline source map data url", "url", sourceURL, "err", err)
			return nil, "", err
		}

		store.log.Info("successfully parsed inline source map data url", "url", sourceURL)
		return dataURL.Data, sourceURL + ".map", nil
	}
	// Remote sourcemap
	resolvedSourceMapURL = sourceMapURL

	// If the URL is relative, we need to attempt to resolve the absolute URL.
	if !strings.HasPrefix(resolvedSourceMapURL, "http") {
		base, err := url.Parse(sourceURL)
		if err != nil {
			store.log.Debug("failed to parse source URL", "url", sourceURL, "err", err)
			return nil, "", err
		}
		relative, err := url.Parse(sourceMapURL)
		if err != nil {
			store.log.Debug("failed to parse source map URL", "url", sourceURL, "sourceMapURL", sourceMapURL, "err", err)
			return nil, "", err
		}

		resolvedSourceMapURL = base.ResolveReference(relative).String()
		store.log.Debug("resolved absolute source map URL", "url", sourceURL, "sourceMapURL", sourceMapURL)
	}

	if !hasHttpPrefix(resolvedSourceMapURL) || !store.origins.matches(resolvedSourceMapURL) {
		store.log.Debug("source map url not allowed by download_from_origins", "url", sourceURL, "sourceMapURL", resolvedSourceMapURL)
		return nil, "", fmt.Errorf("source map url not allowed by download_from_origins: %s", resolvedSourceMapURL)
	}

	store.log.Debug("attempting to download source map file", "url", resolvedSourceMapURL)
	result, err = store.downloadFileContents(store.downloadCli, resolvedSourceMapURL)
	if err != nil {
		store.log.Debug("failed to download source map file", "url", resolvedSourceMapURL, "err", err)
		return nil, "", err
	}

	return result, resolvedSourceMapURL, nil
}

func (store *sourceMapsStoreImpl) downloadFileContents(cli httpClient, url string) ([]byte, error) {
	resp, err := cli.Get(url)
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

// Setting CheckRedirect replaces net/http's default limit, so keep its value.
const maxDownloadRedirects = 10

// newDownloadClient returns a client for URLs taken from client telemetry, which
// only follows redirects that match origins.
func newDownloadClient(timeout time.Duration, origins originPatterns) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxDownloadRedirects {
				return fmt.Errorf("stopped after %d redirects", maxDownloadRedirects)
			}
			if target := req.URL.String(); !hasHttpPrefix(target) || !origins.matches(target) {
				return fmt.Errorf("redirect to %s not allowed by download_from_origins", target)
			}
			return nil
		},
	}
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

// originPatterns is a parsed download_from_origins list.
type originPatterns struct {
	any      bool
	patterns []originPattern
	invalid  []string
}

func parseOriginPatterns(origins []string) originPatterns {
	var ps originPatterns
	for _, origin := range origins {
		if origin == "*" {
			ps.any = true
		} else if p, ok := parseOriginPattern(origin); ok {
			ps.patterns = append(ps.patterns, p)
		} else {
			ps.invalid = append(ps.invalid, origin)
		}
	}
	return ps
}

// matches returns true if URL matches at least one origin of the form
// [scheme://]host[:port][/path]. Each part is matched separately, '*' and '?'
// are wildcards, and the path matches as a prefix.
func (ps originPatterns) matches(URL string) bool {
	u, ok := parseOriginURL(URL)
	if !ok {
		return false
	}
	if ps.any {
		return true
	}
	for _, p := range ps.patterns {
		if p.matches(u) {
			return true
		}
	}
	return false
}

type originURL struct {
	scheme, host, port, path string
}

func parseOriginURL(raw string) (originURL, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return originURL{}, false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	// An IPv6 zone like [::1%.example.com] would match host wildcards, but the zone is ignored when dialing.
	if strings.Contains(host, "%") {
		return originURL{}, false
	}
	// net/http converts non-ASCII hosts with IDNA before dialing, which turns characters like U+3002 into dots.
	for i := 0; i < len(host); i++ {
		if host[i] >= 0x80 {
			return originURL{}, false
		}
	}
	port := u.Port()
	if port == "" {
		port = defaultPort(u.Scheme)
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	return originURL{scheme: u.Scheme, host: host, port: port, path: path}, true
}

func defaultPort(scheme string) string {
	if scheme == "https" {
		return "443"
	}
	return "80"
}

type originPattern struct {
	scheme, host, port, path string
}

func parseOriginPattern(origin string) (originPattern, bool) {
	var p originPattern
	rest := origin
	if scheme, after, ok := strings.Cut(rest, "://"); ok {
		p.scheme, rest = strings.ToLower(scheme), after
	}
	authority, path, hasPath := strings.Cut(rest, "/")
	if hasPath {
		p.path = "/" + path
	}

	if strings.HasPrefix(authority, "[") {
		end := strings.Index(authority, "]")
		if end < 0 {
			return originPattern{}, false
		}
		p.host, authority = authority[1:end], authority[end+1:]
		if authority != "" {
			port, ok := strings.CutPrefix(authority, ":")
			if !ok {
				return originPattern{}, false
			}
			p.port = port
		}
	} else if host, port, ok := strings.Cut(authority, ":"); ok {
		p.host, p.port = host, port
	} else {
		p.host = authority
	}
	p.host = strings.TrimSuffix(strings.ToLower(p.host), ".")
	if p.host == "" {
		return originPattern{}, false
	}
	return p, true
}

func (p originPattern) matches(u originURL) bool {
	if p.scheme != "" && !wildcard.Match(p.scheme, u.scheme) {
		return false
	}
	port := p.port
	if port == "" {
		port = defaultPort(u.scheme)
	}
	if !wildcard.Match(port, u.port) {
		return false
	}
	return hostMatches(p.host, u.host) && p.pathMatches(u.path)
}

// hostMatches matches label by label, so a wildcard like 10.0.0.* can't match a
// longer name like 10.0.0.1.example.net. A bare "*" matches any host and a
// leading "*." matches one or more labels.
func hostMatches(pattern, host string) bool {
	if pattern == "*" {
		return true
	}
	hostLabels := strings.Split(host, ".")
	if rest, ok := strings.CutPrefix(pattern, "*."); ok {
		patternLabels := strings.Split(rest, ".")
		if len(hostLabels) <= len(patternLabels) {
			return false
		}
		return labelsMatch(patternLabels, hostLabels[len(hostLabels)-len(patternLabels):])
	}
	return labelsMatch(strings.Split(pattern, "."), hostLabels)
}

func labelsMatch(patterns, labels []string) bool {
	if len(patterns) != len(labels) {
		return false
	}
	for i := range patterns {
		if !wildcard.Match(patterns[i], labels[i]) {
			return false
		}
	}
	return true
}

func (p originPattern) pathMatches(path string) bool {
	return wildcard.Match(p.path+"*", path)
}

func hasHttpPrefix(URL string) bool {
	return strings.HasPrefix(URL, "http://") || strings.HasPrefix(URL, "https://")
}

func cleanFilePathPart(x string) string {
	return strings.TrimLeft(strings.ReplaceAll(strings.ReplaceAll(x, "\\", ""), "/", ""), ".")
}

func transformException(log *slog.Logger, store sourceMapsStore, ex *payload.Exception, release string) *payload.Exception {
	if ex.Stacktrace == nil {
		return ex
	}

	var frames []payload.Frame
	for _, frame := range ex.Stacktrace.Frames {
		mappedFrame, err := resolveSourceLocation(store, &frame, release)
		if err != nil {
			log.Error("Error resolving stack trace frame source location", "err", err)
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

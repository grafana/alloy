package app_agent_receiver

import (
	"bytes"
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

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/go-sourcemap/sourcemap"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/vincent-petithory/dataurl"
)

// SourceMapStore is interface for a sourcemap service capable of transforming
// minified source locations to original source location
type SourceMapStore interface {
	GetSourceMap(sourceURL string, release string) (*SourceMap, error)
}

type httpClient interface {
	Get(url string) (resp *http.Response, err error)
}

// FileService is interface for a service that can be used to load source maps
// from file system
type fileService interface {
	Stat(name string) (fs.FileInfo, error)
	ReadFile(name string) ([]byte, error)
}

type osFileService struct{}

func (s *osFileService) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

func (s *osFileService) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

var reSourceMap = "//[#@]\\s(source(?:Mapping)?URL)=\\s*(?P<url>\\S+)\r?\n?$"

// SourceMap is a wrapper for go-sourcemap consumer
type SourceMap struct {
	consumer *sourcemap.Consumer
}

type sourceMapMetrics struct {
	cacheSize *prometheus.CounterVec
	downloads *prometheus.CounterVec
	fileReads *prometheus.CounterVec
}

type sourcemapFileLocation struct {
	SourceMapFileLocation
	pathTemplate *template.Template
}

// RealSourceMapStore is an implementation of SourceMapStore
// that can download source maps or read them from file system
type RealSourceMapStore struct {
	sync.Mutex
	l             log.Logger
	httpClient    httpClient
	fileService   fileService
	config        SourceMapConfig
	cache         map[string]*SourceMap
	fileLocations []*sourcemapFileLocation
	metrics       *sourceMapMetrics
}

// NewSourceMapStore creates an instance of SourceMapStore.
// httpClient and fileService will be instantiated to defaults if nil is provided
func NewSourceMapStore(l log.Logger, config SourceMapConfig, reg prometheus.Registerer, httpClient httpClient, fileService fileService) SourceMapStore {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: config.DownloadTimeout,
		}
	}

	if fileService == nil {
		fileService = &osFileService{}
	}

	metrics := &sourceMapMetrics{
		cacheSize: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "app_agent_receiver_sourcemap_cache_size",
			Help: "number of items in source map cache, per origin",
		}, []string{"origin"}),
		downloads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "app_agent_receiver_sourcemap_downloads_total",
			Help: "downloads by the source map service",
		}, []string{"origin", "http_status"}),
		fileReads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "app_agent_receiver_sourcemap_file_reads_total",
			Help: "source map file reads from file system, by origin and status",
		}, []string{"origin", "status"}),
	}
	reg.MustRegister(metrics.cacheSize, metrics.downloads, metrics.fileReads)

	fileLocations := []*sourcemapFileLocation{}

	for _, configLocation := range config.FileSystem {
		tpl, err := template.New(configLocation.Path).Parse(configLocation.Path)
		if err != nil {
			panic(err)
		}

		fileLocations = append(fileLocations, &sourcemapFileLocation{
			SourceMapFileLocation: configLocation,
			pathTemplate:          tpl,
		})
	}

	return &RealSourceMapStore{
		l:             l,
		httpClient:    httpClient,
		fileService:   fileService,
		config:        config,
		cache:         make(map[string]*SourceMap),
		metrics:       metrics,
		fileLocations: fileLocations,
	}
}

func (store *RealSourceMapStore) downloadFileContents(url string) ([]byte, error) {
	resp, err := store.httpClient.Get(url)
	if err != nil {
		store.metrics.downloads.WithLabelValues(getOrigin(url), "?").Inc()
		return nil, err
	}
	defer resp.Body.Close()
	store.metrics.downloads.WithLabelValues(getOrigin(url), fmt.Sprint(resp.StatusCode)).Inc()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %v", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (store *RealSourceMapStore) downloadSourceMapContent(sourceURL string) (content []byte, resolvedSourceMapURL string, err error) {
	level.Debug(store.l).Log("msg", "attempting to download source file", "url", sourceURL)

	result, err := store.downloadFileContents(sourceURL)
	if err != nil {
		level.Debug(store.l).Log("msg", "failed to download source file", "url", sourceURL, "err", err)
		return nil, "", err
	}
	r := regexp.MustCompile(reSourceMap)
	match := r.FindAllStringSubmatch(string(result), -1)
	if len(match) == 0 {
		level.Debug(store.l).Log("msg", "no source map url found in source", "url", sourceURL)
		return nil, "", nil
	}
	sourceMapURL := match[len(match)-1][2]

	// inline sourcemap
	if strings.HasPrefix(sourceMapURL, "data:") {
		dataURL, err := dataurl.DecodeString(sourceMapURL)
		if err != nil {
			level.Debug(store.l).Log("msg", "failed to parse inline source map data url", "url", sourceURL, "err", err)
			return nil, "", err
		}

		level.Info(store.l).Log("msg", "successfully parsed inline source map data url", "url", sourceURL)
		return dataURL.Data, sourceURL + ".map", nil
	}
	// remote sourcemap
	resolvedSourceMapURL = sourceMapURL

	// if url is relative, attempt to resolve absolute
	if !strings.HasPrefix(resolvedSourceMapURL, "http") {
		base, err := url.Parse(sourceURL)
		if err != nil {
			level.Debug(store.l).Log("msg", "failed to parse source url", "url", sourceURL, "err", err)
			return nil, "", err
		}
		relative, err := url.Parse(sourceMapURL)
		if err != nil {
			level.Debug(store.l).Log("msg", "failed to parse source map url", "url", sourceURL, "sourceMapURL", sourceMapURL, "err", err)
			return nil, "", err
		}
		resolvedSourceMapURL = base.ResolveReference(relative).String()
		level.Debug(store.l).Log("msg", "resolved absolute source map url", "url", sourceURL, "sourceMapURL", resolvedSourceMapURL)
	}
	level.Debug(store.l).Log("msg", "attempting to download source map file", "url", resolvedSourceMapURL)
	result, err = store.downloadFileContents(resolvedSourceMapURL)
	if err != nil {
		level.Debug(store.l).Log("failed to download source map file", "url", resolvedSourceMapURL, "err", err)
		return nil, "", err
	}
	return result, resolvedSourceMapURL, nil
}

func (store *RealSourceMapStore) getSourceMapFromFileSystem(sourceURL string, release string, fileconf *sourcemapFileLocation) (content []byte, sourceMapURL string, err error) {
	if len(sourceURL) == 0 || !strings.HasPrefix(sourceURL, fileconf.MinifiedPathPrefix) || strings.HasSuffix(sourceURL, "/") {
		return nil, "", nil
	}

	var rootPath bytes.Buffer

	err = fileconf.pathTemplate.Execute(&rootPath, struct{ Release string }{Release: cleanFilePathPart(release)})
	if err != nil {
		return nil, "", err
	}

	pathParts := []string{rootPath.String()}
	for _, part := range strings.Split(strings.TrimPrefix(strings.Split(sourceURL, "?")[0], fileconf.MinifiedPathPrefix), "/") {
		if len(part) > 0 && part != "." && part != ".." {
			pathParts = append(pathParts, part)
		}
	}
	mapFilePath := filepath.Join(pathParts...) + ".map"

	if _, err := store.fileService.Stat(mapFilePath); err != nil {
		store.metrics.fileReads.WithLabelValues(getOrigin(sourceURL), "not_found").Inc()
		level.Debug(store.l).Log("msg", "source map not found on filesystem", "url", sourceURL, "file_path", mapFilePath)
		return nil, "", nil
	}
	level.Debug(store.l).Log("msg", "source map found on filesystem", "url", mapFilePath, "file_path", mapFilePath)

	content, err = store.fileService.ReadFile(mapFilePath)
	if err != nil {
		store.metrics.fileReads.WithLabelValues(getOrigin(sourceURL), "error").Inc()
	} else {
		store.metrics.fileReads.WithLabelValues(getOrigin(sourceURL), "ok").Inc()
	}
	return content, sourceURL, err
}

func (store *RealSourceMapStore) getSourceMapContent(sourceURL string, release string) (content []byte, sourceMapURL string, err error) {
	//attempt to find in fs
	for _, fileconf := range store.fileLocations {
		content, sourceMapURL, err = store.getSourceMapFromFileSystem(sourceURL, release, fileconf)
		if content != nil || err != nil {
			return content, sourceMapURL, err
		}
	}

	//attempt to download
	if strings.HasPrefix(sourceURL, "http") && urlMatchesOrigins(sourceURL, store.config.DownloadFromOrigins) {
		return store.downloadSourceMapContent(sourceURL)
	}
	return nil, "", nil
}

// GetSourceMap returns sourcemap for a given source url
func (store *RealSourceMapStore) GetSourceMap(sourceURL string, release string) (*SourceMap, error) {
	store.Lock()
	defer store.Unlock()

	cacheKey := fmt.Sprintf("%s__%s", sourceURL, release)

	if smap, ok := store.cache[cacheKey]; ok {
		return smap, nil
	}
	content, sourceMapURL, err := store.getSourceMapContent(sourceURL, release)
	if err != nil || content == nil {
		store.cache[cacheKey] = nil
		return nil, err
	}
	if content != nil {
		consumer, err := sourcemap.Parse(sourceMapURL, content)
		if err != nil {
			store.cache[cacheKey] = nil
			level.Debug(store.l).Log("msg", "failed to parse source map", "url", sourceMapURL, "release", release, "err", err)
			return nil, err
		}
		level.Info(store.l).Log("msg", "successfully parsed source map", "url", sourceMapURL, "release", release)
		smap := &SourceMap{
			consumer: consumer,
		}
		store.cache[cacheKey] = smap
		store.metrics.cacheSize.WithLabelValues(getOrigin(sourceURL)).Inc()
		return smap, nil
	}
	return nil, nil
}

// ResolveSourceLocation resolves minified source location to original source location
func ResolveSourceLocation(store SourceMapStore, frame *Frame, release string) (*Frame, error) {
	smap, err := store.GetSourceMap(frame.Filename, release)
	if err != nil {
		return nil, err
	}
	if smap == nil {
		return nil, nil
	}

	file, function, line, col, ok := smap.consumer.Source(frame.Lineno, frame.Colno)
	if !ok {
		return nil, nil
	}
	// unfortunately in many cases go-sourcemap fails to determine the original function name.
	// not a big issue as long as file, line and column are correct
	if len(function) == 0 {
		function = "?"
	}
	return &Frame{
		Filename: file,
		Lineno:   line,
		Colno:    col,
		Function: function,
	}, nil
}

// TransformException will attempt to resolve all minified source locations in the stacktrace with original source locations
func TransformException(store SourceMapStore, log log.Logger, ex *Exception, release string) *Exception {
	if ex.Stacktrace == nil {
		return ex
	}
	frames := []Frame{}

	for _, frame := range ex.Stacktrace.Frames {
		mappedFrame, err := ResolveSourceLocation(store, &frame, release)
		if err != nil {
			level.Error(log).Log("msg", "Error resolving stack trace frame source location", "err", err)
			frames = append(frames, frame)
		} else if mappedFrame != nil {
			frames = append(frames, *mappedFrame)
		} else {
			frames = append(frames, frame)
		}
	}

	return &Exception{
		Type:       ex.Type,
		Value:      ex.Value,
		Stacktrace: &Stacktrace{Frames: frames},
		Timestamp:  ex.Timestamp,
	}
}

func cleanFilePathPart(x string) string {
	return strings.TrimLeft(strings.ReplaceAll(strings.ReplaceAll(x, "\\", ""), "/", ""), ".")
}

func getOrigin(URL string) string {
	parsed, err := url.Parse(URL)
	if err != nil {
		return "?"
	}
	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
}

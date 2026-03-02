package receiver

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/faro/receiver/internal/payload"
	alloyutil "github.com/grafana/alloy/internal/util"
	"github.com/grafana/pyroscope/ebpf/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

// mockTimeSource is a test helper for controlling time.
type mockTimeSource struct {
	now time.Time
}

func (m *mockTimeSource) Now() time.Time {
	return m.now
}

func Test_traceContextKeptWhenStacktraceDefined(t *testing.T) {
	input := &payload.Exception{
		Stacktrace: &payload.Stacktrace{},
		Trace: payload.TraceContext{
			TraceID: "0000000000000",
			SpanID:  "000000",
		},
	}

	expect := input
	actual := transformException(nil, nil, input, "123")
	require.Equal(t, expect, actual)
}

func Test_traceContextKeptWhenStacktraceNotDefined(t *testing.T) {
	input := &payload.Exception{
		Trace: payload.TraceContext{
			TraceID: "0000000000000",
			SpanID:  "000000",
		},
	}

	expect := input
	actual := transformException(nil, nil, input, "123")
	require.Equal(t, expect, actual)
}

func Test_sourceMapsStoreImpl_DownloadSuccess(t *testing.T) {
	var (
		logger = alloyutil.TestLogger(t)

		httpClient = &mockHTTPClient{
			responses: []struct {
				*http.Response
				error
			}{
				{newResponseFromTestData(t, "foo.js"), nil},
				{newResponseFromTestData(t, "foo.js.map"), nil},
			},
		}

		store = newSourceMapsStore(
			logger,
			SourceMapsArguments{
				Download:            true,
				DownloadFromOrigins: []string{"*"},
			},
			newSourceMapMetrics(prometheus.NewRegistry()),
			httpClient,
			newTestFileService(),
		)
	)

	expect := &payload.Exception{
		Stacktrace: &payload.Stacktrace{
			Frames: []payload.Frame{
				{
					Colno:    37,
					Filename: "/__parcel_source_root/demo/src/actions.ts",
					Function: "?",
					Lineno:   6,
				},
				{
					Colno:    2,
					Filename: "/__parcel_source_root/demo/src/actions.ts",
					Function: "?",
					Lineno:   7,
				},
			},
		},
	}

	actual := transformException(logger, store, mockException(), "123")
	require.Equal(t, []string{"http://localhost:1234/foo.js", "http://localhost:1234/foo.js.map"}, httpClient.requests)
	require.Equal(t, expect, actual)
}

func Test_sourceMapsStoreImpl_DownloadError(t *testing.T) {
	var (
		logger = alloyutil.TestLogger(t)

		httpClient = &mockHTTPClient{
			responses: []struct {
				*http.Response
				error
			}{
				{
					&http.Response{StatusCode: 500, Body: io.NopCloser(bytes.NewReader(nil))},
					nil,
				},
			},
		}

		store = newSourceMapsStore(
			logger,
			SourceMapsArguments{
				Download:            true,
				DownloadFromOrigins: []string{"*"},
			},
			newSourceMapMetrics(prometheus.NewRegistry()),
			httpClient,
			newTestFileService(),
		)
	)

	expect := mockException()
	actual := transformException(logger, store, expect, "123")
	require.Equal(t, []string{"http://localhost:1234/foo.js"}, httpClient.requests)
	require.Equal(t, expect, actual)
}

func Test_sourceMapsStoreImpl_DownloadHTTPOriginFiltering(t *testing.T) {
	var (
		logger = alloyutil.TestLogger(t)

		httpClient = &mockHTTPClient{
			responses: []struct {
				*http.Response
				error
			}{
				{newResponseFromTestData(t, "foo.js"), nil},
				{newResponseFromTestData(t, "foo.js.map"), nil},
			},
		}

		store = newSourceMapsStore(
			logger,
			SourceMapsArguments{
				Download:            true,
				DownloadFromOrigins: []string{"http://bar.com/"},
			},
			newSourceMapMetrics(prometheus.NewRegistry()),
			httpClient,
			newTestFileService(),
		)
	)

	expect := &payload.Exception{
		Stacktrace: &payload.Stacktrace{
			Frames: []payload.Frame{
				{
					Colno:    6,
					Filename: "http://foo.com/foo.js",
					Function: "eval",
					Lineno:   5,
				},
				{
					Colno:    2,
					Filename: "/__parcel_source_root/demo/src/actions.ts",
					Function: "?",
					Lineno:   7,
				},
			},
		},
	}

	actual := transformException(logger, store, &payload.Exception{
		Stacktrace: &payload.Stacktrace{
			Frames: []payload.Frame{
				{
					Colno:    6,
					Filename: "http://foo.com/foo.js",
					Function: "eval",
					Lineno:   5,
				},
				{
					Colno:    5,
					Filename: "http://bar.com/foo.js",
					Function: "callUndefined",
					Lineno:   6,
				},
			},
		},
	}, "123")

	require.Equal(t, []string{"http://bar.com/foo.js", "http://bar.com/foo.js.map"}, httpClient.requests)
	require.Equal(t, expect, actual)
}

func Test_sourceMapsStoreImpl_ReadFromFileSystem(t *testing.T) {
	var (
		logger = alloyutil.TestLogger(t)

		httpClient = &mockHTTPClient{}

		fileService = newTestFileService()
	)
	fileService.files = map[string][]byte{
		filepath.FromSlash("/var/build/latest/foo.js.map"): loadTestData(t, "foo.js.map"),
		filepath.FromSlash("/var/build/123/foo.js.map"):    loadTestData(t, "foo.js.map"),
	}

	var store = newSourceMapsStore(
		logger,
		SourceMapsArguments{
			Download: false,
			Locations: []LocationArguments{
				{
					MinifiedPathPrefix: "http://foo.com/",
					Path:               filepath.FromSlash("/var/build/latest/"),
				},
				{
					MinifiedPathPrefix: "http://bar.com/",
					Path:               filepath.FromSlash("/var/build/{{ .Release }}/"),
				},
			},
		},
		newSourceMapMetrics(prometheus.NewRegistry()),
		httpClient,
		fileService,
	)

	expect := &payload.Exception{
		Stacktrace: &payload.Stacktrace{
			Frames: []payload.Frame{
				{
					Colno:    37,
					Filename: "/__parcel_source_root/demo/src/actions.ts",
					Function: "?",
					Lineno:   6,
				},
				{
					Colno:    6,
					Filename: "http://foo.com/bar.js",
					Function: "eval",
					Lineno:   5,
				},
				{
					Colno:    2,
					Filename: "/__parcel_source_root/demo/src/actions.ts",
					Function: "?",
					Lineno:   7,
				},
				{
					Colno:    5,
					Filename: "http://baz.com/foo.js",
					Function: "callUndefined",
					Lineno:   6,
				},
			},
		},
		Context: payload.ExceptionContext{"ReactError": "Annoying Error", "component": "ReactErrorBoundary"},
	}

	actual := transformException(logger, store, &payload.Exception{
		Stacktrace: &payload.Stacktrace{
			Frames: []payload.Frame{
				{
					Colno:    6,
					Filename: "http://foo.com/foo.js",
					Function: "eval",
					Lineno:   5,
				},
				{
					Colno:    6,
					Filename: "http://foo.com/bar.js",
					Function: "eval",
					Lineno:   5,
				},
				{
					Colno:    5,
					Filename: "http://bar.com/foo.js",
					Function: "callUndefined",
					Lineno:   6,
				},
				{
					Colno:    5,
					Filename: "http://baz.com/foo.js",
					Function: "callUndefined",
					Lineno:   6,
				},
			},
		},
		Context: payload.ExceptionContext{"ReactError": "Annoying Error", "component": "ReactErrorBoundary"},
	}, "123")

	require.Equal(t, expect, actual)
	require.EqualValues(t, payload.ExceptionContext{"ReactError": "Annoying Error", "component": "ReactErrorBoundary"}, actual.Context)
}

func Test_sourceMapsStoreImpl_ReadFromFileSystemAndDownload(t *testing.T) {
	var (
		logger = alloyutil.TestLogger(t)

		httpClient = &mockHTTPClient{
			responses: []struct {
				*http.Response
				error
			}{
				{newResponseFromTestData(t, "foo.js"), nil},
				{newResponseFromTestData(t, "foo.js.map"), nil},
			},
		}

		fileService = newTestFileService()
	)
	fileService.files = map[string][]byte{
		filepath.FromSlash("/var/build/latest/foo.js.map"): loadTestData(t, "foo.js.map"),
	}

	var store = newSourceMapsStore(
		logger,
		SourceMapsArguments{
			Download:            true,
			DownloadFromOrigins: []string{"*"},
			Locations: []LocationArguments{
				{
					MinifiedPathPrefix: "http://foo.com/",
					Path:               filepath.FromSlash("/var/build/latest/"),
				},
			},
		},
		newSourceMapMetrics(prometheus.NewRegistry()),
		httpClient,
		fileService,
	)

	expect := &payload.Exception{
		Stacktrace: &payload.Stacktrace{
			Frames: []payload.Frame{
				{
					Colno:    37,
					Filename: "/__parcel_source_root/demo/src/actions.ts",
					Function: "?",
					Lineno:   6,
				},
				{
					Colno:    2,
					Filename: "/__parcel_source_root/demo/src/actions.ts",
					Function: "?",
					Lineno:   7,
				},
			},
		},
	}

	actual := transformException(logger, store, &payload.Exception{
		Stacktrace: &payload.Stacktrace{
			Frames: []payload.Frame{
				{
					Colno:    6,
					Filename: "http://foo.com/foo.js",
					Function: "eval",
					Lineno:   5,
				},
				{
					Colno:    5,
					Filename: "http://bar.com/foo.js",
					Function: "callUndefined",
					Lineno:   6,
				},
			},
		},
	}, "123")

	require.Equal(t, []string{filepath.FromSlash("/var/build/latest/foo.js.map")}, fileService.stats)
	require.Equal(t, []string{filepath.FromSlash("/var/build/latest/foo.js.map")}, fileService.reads)
	require.Equal(t, []string{"http://bar.com/foo.js", "http://bar.com/foo.js.map"}, httpClient.requests)
	require.Equal(t, expect, actual)
}

func Test_sourceMapsStoreImpl_ReadFromFileSystemAndNotDownloadIfDisabled(t *testing.T) {
	var (
		logger = alloyutil.TestLogger(t)

		httpClient = &mockHTTPClient{
			responses: []struct {
				*http.Response
				error
			}{
				{newResponseFromTestData(t, "foo.js"), nil},
				{newResponseFromTestData(t, "foo.js.map"), nil},
			},
		}

		fileService = newTestFileService()
	)
	fileService.files = map[string][]byte{
		filepath.FromSlash("/var/build/latest/foo.js.map"): loadTestData(t, "foo.js.map"),
	}

	var store = newSourceMapsStore(
		logger,
		SourceMapsArguments{
			Download:            false,
			DownloadFromOrigins: []string{"*"},
			Locations: []LocationArguments{
				{
					MinifiedPathPrefix: "http://foo.com/",
					Path:               filepath.FromSlash("/var/build/latest/"),
				},
			},
		},
		newSourceMapMetrics(prometheus.NewRegistry()),
		httpClient,
		fileService,
	)

	expect := &payload.Exception{
		Stacktrace: &payload.Stacktrace{
			Frames: []payload.Frame{
				{
					Colno:    37,
					Filename: "/__parcel_source_root/demo/src/actions.ts",
					Function: "?",
					Lineno:   6,
				},
				{
					Colno:    5,
					Filename: "http://bar.com/foo.js",
					Function: "callUndefined",
					Lineno:   6,
				},
			},
		},
	}

	actual := transformException(logger, store, &payload.Exception{
		Stacktrace: &payload.Stacktrace{
			Frames: []payload.Frame{
				{
					Colno:    6,
					Filename: "http://foo.com/foo.js",
					Function: "eval",
					Lineno:   5,
				},
				{
					Colno:    5,
					Filename: "http://bar.com/foo.js",
					Function: "callUndefined",
					Lineno:   6,
				},
			},
		},
	}, "123")

	require.Equal(t, []string{filepath.FromSlash("/var/build/latest/foo.js.map")}, fileService.stats)
	require.Equal(t, []string{filepath.FromSlash("/var/build/latest/foo.js.map")}, fileService.reads)
	require.Nil(t, httpClient.requests)
	require.Equal(t, expect, actual)
}

func Test_sourceMapsStoreImpl_FilepathSanitized(t *testing.T) {
	var (
		logger = alloyutil.TestLogger(t)

		httpClient  = &mockHTTPClient{}
		fileService = newTestFileService()

		store = newSourceMapsStore(
			logger,
			SourceMapsArguments{
				Download: false,
				Locations: []LocationArguments{
					{
						MinifiedPathPrefix: "http://foo.com/",
						Path:               filepath.FromSlash("/var/build/latest/"),
					},
				},
			},
			newSourceMapMetrics(prometheus.NewRegistry()),
			httpClient,
			fileService,
		)
	)

	input := &payload.Exception{
		Stacktrace: &payload.Stacktrace{
			Frames: []payload.Frame{
				{
					Colno:    6,
					Filename: "http://foo.com/../../../etc/passwd",
					Function: "eval",
					Lineno:   5,
				},
			},
		},
	}

	actual := transformException(logger, store, input, "123")
	require.Equal(t, input, actual)
}

func Test_urlMatchesOrigins(t *testing.T) {
	tt := []struct {
		name        string
		url         string
		origins     []string
		shouldMatch bool
	}{
		{
			name:        "wildcard always matches",
			url:         "https://example.com/static/foo.js",
			origins:     []string{"https://foo.com/", "*"},
			shouldMatch: true,
		},
		{
			name:        "exact matches",
			url:         "http://example.com/static/foo.js",
			origins:     []string{"https://foo.com/", "http://example.com/"},
			shouldMatch: true,
		},
		{
			name:        "matches with subdomain wildcard",
			url:         "http://foo.bar.com/static/foo.js",
			origins:     []string{"https://foo.com/", "http://*.bar.com/"},
			shouldMatch: true,
		},
		{
			name:        "no exact match",
			url:         "http://example.com/static/foo.js",
			origins:     []string{"https://foo.com/", "http://test.com/"},
			shouldMatch: false,
		},
		{
			name:        "no exact match with subdomain wildcard",
			url:         "http://foo.bar.com/static/foo.js",
			origins:     []string{"https://foo.com/", "http://*.baz.com/"},
			shouldMatch: false,
		},
		{
			name:        "matches with wildcard without protocol",
			url:         "http://foo.bar.com/static/foo.js",
			origins:     []string{"https://foo.com/", "*.bar.com/"},
			shouldMatch: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual := urlMatchesOrigins(tc.url, tc.origins)

			if tc.shouldMatch {
				require.True(t, actual, "expected %v to be matched from origin set %v", tc.url, tc.origins)
			} else {
				require.False(t, actual, "expected %v to not be matched from origin set %v", tc.url, tc.origins)
			}
		})
	}
}

func TestOsFileService_RejectsInvalidPaths(t *testing.T) {
	fs := osFileService{}

	invalidPaths := []string{
		"file/with/slash",
		"file\\with\\backslash",
		"file/../with/parent/dir",
		"../parent/dir",
		"./current/dir",
		"file..with..dots",
	}

	for _, path := range invalidPaths {
		_, errStat := fs.Stat(path)
		if errStat == nil {
			t.Errorf("Expected error for path %q containing illegal characters, but got nil", path)
		}
		_, errReadFile := fs.ReadFile(path)
		if errReadFile == nil {
			t.Errorf("Expected error for path %q containing illegal characters, but got nil", path)
		}
	}

	validPath := "validfilename.txt"
	_, errStat := fs.Stat(validPath)
	if errStat != nil && errStat.Error() == "invalid file name: "+validPath {
		t.Errorf("Expected valid path %q to not trigger invalid file name error", validPath)
	}
	_, errReadFile := fs.ReadFile(validPath)
	if errReadFile != nil && errReadFile.Error() == "invalid file name: "+validPath {
		t.Errorf("Expected valid path %q to not trigger invalid file name error", validPath)
	}
}

func Test_sourceMapsStoreImpl_RealWorldPathValidation(t *testing.T) {
	var (
		logger      = alloyutil.TestLogger(t)
		fileService = &testFileService{}
		store       = newSourceMapsStore(
			logger,
			SourceMapsArguments{
				Download: false,
				Locations: []LocationArguments{
					{
						MinifiedPathPrefix: "https://example.com/",
						Path:               "/foo/bar/baz/qux",
					},
				},
			},
			newSourceMapMetrics(prometheus.NewRegistry()),
			nil,
			fileService,
		)
	)

	input := &payload.Exception{
		Stacktrace: &payload.Stacktrace{
			Frames: []payload.Frame{
				{
					Colno:    6,
					Filename: "https://example.com/folder/file.js",
					Function: "eval",
					Lineno:   5,
				},
			},
		},
	}

	actual := transformException(logger, store, input, "123")
	require.Equal(t, input, actual)
	require.Equal(t, []string{filepath.FromSlash("/foo/bar/baz/qux/folder/file.js.map")}, fileService.stats)
	require.Empty(t, fileService.reads, "should not read file when stat fails")
}

func TestSourceMapsStoreImpl_CleanCachedErrors(t *testing.T) {
	tt := []struct {
		name              string
		cache             map[string]*cachedSourceMap
		expectedCacheSize int
	}{
		{
			name: "should remove cached error",
			cache: map[string]*cachedSourceMap{
				"http://shouldRemoveCachedErrors.com__v1": nil,
			},
			expectedCacheSize: 0,
		},
		{
			name: "should not remove from map if no errors",
			cache: map[string]*cachedSourceMap{
				"http://shouldNotRemoveFromCache.com__v2": {},
			},
			expectedCacheSize: 1,
		},
		{
			name: "should not remove from map if no errors",
			cache: map[string]*cachedSourceMap{
				"http://shouldNotRemoveFromCache.com__v1": {},
				"http://shouldNotRemoveFromCache.com__v2": {},
			},
			expectedCacheSize: 2,
		},
		{
			name: "should remove only cached errors",
			cache: map[string]*cachedSourceMap{
				"http://shouldNotRemoveFromCache.com__v1": nil,
				"http://shouldNotRemoveFromCache.com__v2": {},
			},
			expectedCacheSize: 1,
		},
	}

	logger := util.TestLogger(t)

	for _, tc := range tt {
		reg := prometheus.NewRegistry()
		metrics := newSourceMapMetrics(reg)

		store := &sourceMapsStoreImpl{
			log:        logger,
			args:       SourceMapsArguments{Cache: &CacheArguments{TTL: 5 * time.Minute}},
			metrics:    metrics,
			cli:        &mockHTTPClient{},
			fs:         newTestFileService(),
			cache:      tc.cache,
			timeSource: &mockTimeSource{now: time.Now()},
		}

		t.Run(tc.name, func(t *testing.T) {
			store.CleanCachedErrors()
			require.Equal(t, tc.expectedCacheSize, len(store.cache))
		})
	}
}

func TestSourceMapsStoreImpl_CleanOldCachedEntries(t *testing.T) {
	now := time.Now()

	tt := []struct {
		name              string
		cache             map[string]*cachedSourceMap
		timeSource        *mockTimeSource
		cacheTimeout      time.Duration
		expectedCacheSize int
	}{
		{
			name: "should clear entry from cache if too old",
			cache: map[string]*cachedSourceMap{
				"http://shouldRemoveCachedErrors.com__v1": {lastUsed: now},
			},
			timeSource:        &mockTimeSource{now: now.Add(6 * time.Minute)},
			cacheTimeout:      5 * time.Minute,
			expectedCacheSize: 0,
		},
		{
			name: "should not clear entry from cache if not too old",
			cache: map[string]*cachedSourceMap{
				"http://shouldRemoveCachedErrors.com__v1": {lastUsed: now},
			},
			timeSource:        &mockTimeSource{now: now.Add(3 * time.Minute)},
			cacheTimeout:      5 * time.Minute,
			expectedCacheSize: 1,
		},
		{
			name: "should clear only old entries from cache",
			cache: map[string]*cachedSourceMap{
				"http://shouldRemoveCachedErrors.com__v1": {lastUsed: now},
				"http://shouldRemoveCachedErrors.com__v2": {lastUsed: now.Add(-6 * time.Minute)},
			},
			timeSource:        &mockTimeSource{now: now},
			cacheTimeout:      5 * time.Minute,
			expectedCacheSize: 1,
		},
		{
			name: "should not clear multiple entries",
			cache: map[string]*cachedSourceMap{
				"http://shouldRemoveCachedErrors.com__v1": {lastUsed: now.Add(3 * time.Minute)},
				"http://shouldRemoveCachedErrors.com__v2": {lastUsed: now.Add(4 * time.Minute)},
			},
			timeSource:        &mockTimeSource{now: now},
			cacheTimeout:      5 * time.Minute,
			expectedCacheSize: 2,
		},
		{
			name: "should clear multiple old entries from cache",
			cache: map[string]*cachedSourceMap{
				"http://shouldRemoveCachedErrors.com__v1": {lastUsed: now.Add(-10 * time.Minute)},
				"http://shouldRemoveCachedErrors.com__v2": {lastUsed: now.Add(-7 * time.Minute)},
			},
			timeSource:        &mockTimeSource{now: now},
			cacheTimeout:      5 * time.Minute,
			expectedCacheSize: 0,
		},
	}

	logger := util.TestLogger(t)

	for _, tc := range tt {
		reg := prometheus.NewRegistry()
		metrics := newSourceMapMetrics(reg)

		store := &sourceMapsStoreImpl{
			log:        logger,
			args:       SourceMapsArguments{Cache: &CacheArguments{TTL: tc.cacheTimeout}},
			metrics:    metrics,
			cli:        &mockHTTPClient{},
			fs:         newTestFileService(),
			cache:      tc.cache,
			timeSource: tc.timeSource,
		}

		t.Run(tc.name, func(t *testing.T) {
			store.CleanOldCacheEntries()
			require.Equal(t, tc.expectedCacheSize, len(store.cache))
		})
	}
}

type mockHTTPClient struct {
	responses []struct {
		*http.Response
		error
	}
	requests []string
}

func (cl *mockHTTPClient) Get(url string) (resp *http.Response, err error) {
	if len(cl.responses) > len(cl.requests) {
		r := cl.responses[len(cl.requests)]
		cl.requests = append(cl.requests, url)
		return r.Response, r.error
	}
	return nil, errors.New("mockHTTPClient got more requests than expected")
}

type testFileService struct {
	files map[string][]byte
	osFileService
	stats []string
	reads []string
}

func (s *testFileService) Stat(name string) (fs.FileInfo, error) {
	s.stats = append(s.stats, name)
	_, ok := s.files[name]
	if !ok {
		return nil, errors.New("file not found")
	}
	return nil, nil
}

func (s *testFileService) ReadFile(name string) ([]byte, error) {
	s.reads = append(s.reads, name)
	content, ok := s.files[name]
	if ok {
		return content, nil
	}
	return nil, errors.New("file not found")
}

func (s *testFileService) ValidateFilePath(name string) (string, error) {
	return s.osFileService.ValidateFilePath(name)
}

func newResponseFromTestData(t *testing.T, file string) *http.Response {
	return &http.Response{
		Body:       io.NopCloser(bytes.NewReader(loadTestData(t, file))),
		StatusCode: 200,
	}
}

func mockException() *payload.Exception {
	return &payload.Exception{
		Stacktrace: &payload.Stacktrace{
			Frames: []payload.Frame{
				{
					Colno:    6,
					Filename: "http://localhost:1234/foo.js",
					Function: "eval",
					Lineno:   5,
				},
				{
					Colno:    5,
					Filename: "http://localhost:1234/foo.js",
					Function: "callUndefined",
					Lineno:   6,
				},
			},
		},
	}
}

func loadTestData(t *testing.T, file string) []byte {
	t.Helper()
	// Safe to disable, this is a test.
	// nolint:gosec
	content, err := os.ReadFile(filepath.Join("testdata", file))
	require.NoError(t, err, "expected to be able to read file")
	require.True(t, len(content) > 0)
	return content
}

func newTestFileService() *testFileService {
	return &testFileService{
		files:         make(map[string][]byte),
		osFileService: osFileService{},
		stats:         make([]string, 0),
		reads:         make([]string, 0),
	}
}

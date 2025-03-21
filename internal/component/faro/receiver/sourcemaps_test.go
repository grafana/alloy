package receiver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/faro/receiver/internal/payload"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_sourceMapsStoreImpl_ReadFromFileSystem(t *testing.T) {
	var (
		logger = util.TestLogger(t)

		httpClient = &mockHTTPClient{}

		fileService = &mockFileService{
			files: map[string][]byte{
				filepath.FromSlash("/var/build/latest/foo.js.map"): loadTestData(t, "foo.js.map"),
				filepath.FromSlash("/var/build/123/foo.js.map"):    loadTestData(t, "foo.js.map"),
			},
		}

		store = newSourceMapsStore(
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

func Test_sourceMapsStoreImpl_ReadFromFileSystemAndNotDownloadIfDisabled(t *testing.T) {
	var (
		logger = util.TestLogger(t)

		httpClient = &mockHTTPClient{
			responses: []struct {
				*http.Response
				error
			}{
				{newResponseFromTestData(t, "foo.js"), nil},
				{newResponseFromTestData(t, "foo.js.map"), nil},
			},
		}

		fileService = &mockFileService{
			files: map[string][]byte{
				filepath.FromSlash("/var/build/latest/foo.js.map"): loadTestData(t, "foo.js.map"),
			},
		}

		store = newSourceMapsStore(
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

// setupSourceMapMetrics creates metrics for testing
func setupSourceMapMetrics() *sourceMapMetrics {
	return &sourceMapMetrics{
		cacheSize: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_sourcemap_cache_size",
				Help: "test metric",
			},
			[]string{"origin"},
		),
		downloads: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_sourcemap_downloads",
				Help: "test metric",
			},
			[]string{"origin", "status"},
		),
		fileReads: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_sourcemap_file_reads",
				Help: "test metric",
			},
			[]string{"origin", "status"},
		),
	}
}

// createTempDir creates a temporary directory with proper cleanup
func createTempDir(t *testing.T) string {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "sourcemaps-test")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	tempDir, err = filepath.Abs(tempDir)
	require.NoError(t, err)

	return tempDir
}

func TestSecurePathAndPathTraversal(t *testing.T) {
	tempDir := createTempDir(t)

	sourcemapsDir := filepath.Join(tempDir, "sourcemaps")
	err := os.MkdirAll(sourcemapsDir, 0755)
	require.NoError(t, err)

	validFile := filepath.Join(tempDir, "valid.js.map")
	err = os.WriteFile(validFile, []byte(`{"version":3,"file":"valid.js","sources":["src/valid.js"],"names":[],"mappings":"AAAA"}`), 0644)
	require.NoError(t, err)

	appFile := filepath.Join(sourcemapsDir, "app.js.map")
	err = os.WriteFile(appFile, []byte(`{"version":3,"file":"app.js","sources":["src/app.js"],"names":[],"mappings":"..."}`), 0644)
	require.NoError(t, err)

	fs := newOsFileService(tempDir)

	t.Run("Basic file service tests", func(t *testing.T) {
		// Test valid file access
		fileInfo, err := fs.Stat("valid.js.map")
		require.NoError(t, err, "Secure file service should find the file")
		assert.NotNil(t, fileInfo)

		// Test path traversal attempt
		_, err = fs.Stat("../../../etc/passwd")
		require.Error(t, err, "Path traversal attempt should fail")
		assert.Contains(t, err.Error(), "path traversal")
	})

	t.Run("Sourcemap store path traversal tests", func(t *testing.T) {
		var logBuffer bytes.Buffer
		testLogger := &bufLogger{w: &logBuffer}

		// Setup location
		loc := LocationArguments{
			MinifiedPathPrefix: "http://example.com/",
			Path:               sourcemapsDir,
		}

		pathTemplate, err := template.New("path").Parse(loc.Path)
		require.NoError(t, err)

		// Create store
		store := &sourceMapsStoreImpl{
			log:     testLogger,
			fs:      fs,
			metrics: setupSourceMapMetrics(),
			args: SourceMapsArguments{
				Download:           false,
				BaseSourceMapsPath: tempDir,
				Locations:          []LocationArguments{loc},
			},
			locs: []*sourcemapFileLocation{
				{
					LocationArguments: loc,
					pathTemplate:      pathTemplate,
				},
			},
		}

		// Test cases
		testCases := []struct {
			name       string
			url        string
			shouldFind bool
		}{
			{
				name:       "path traversal attempt",
				url:        "http://example.com/../../../../etc/passwd",
				shouldFind: false,
			},
			{
				name:       "valid path",
				url:        "http://example.com/app.js",
				shouldFind: true,
			},
			{
				name:       "non-existent file",
				url:        "http://example.com/nonexistent.js",
				shouldFind: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				content, sourceURL, err := store.getSourceMapFromFileSystem(tc.url, "release", store.locs[0])

				assert.Nil(t, err, "Should not return an error")

				if tc.shouldFind {
					assert.NotNil(t, content, "Should find content")
					assert.NotEmpty(t, sourceURL, "Should return a source URL")
				} else {
					assert.Nil(t, content, "Should not find content")
					assert.Empty(t, sourceURL, "Should return empty source URL")
				}
			})
		}
	})
}

func Test_sourceMapsStoreImpl_FilepathSanitized(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sourcemaps-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tempDir, err = filepath.Abs(tempDir)
	require.NoError(t, err)

	var logBuffer bytes.Buffer
	testLogger := &bufLogger{w: &logBuffer}

	fileService := newOsFileService(tempDir)
	fmt.Printf("File service basePath: %s\n", fileService.basePath)

	sourcemapPath := filepath.Join(tempDir, "valid.js.map")
	fmt.Printf("Creating sourcemap at: %s\n", sourcemapPath)

	validSourcemapContent := []byte(`{"version":3,"file":"valid.js","sources":["src/valid.js"],"names":[],"mappings":"AAAA"}`)
	err = os.WriteFile(sourcemapPath, validSourcemapContent, 0644)
	require.NoError(t, err)

	_, err = os.Stat(sourcemapPath)
	require.NoError(t, err, "File should exist at the expected path")

	location := LocationArguments{
		MinifiedPathPrefix: "http://foo.com/",
		Path:               tempDir,
	}

	store := newSourceMapsStore(
		testLogger,
		SourceMapsArguments{
			Download:  false,
			Locations: []LocationArguments{location},
		},
		newSourceMapMetrics(prometheus.NewRegistry()),
		&mockHTTPClient{},
		fileService,
	)

	input := &payload.Exception{
		Stacktrace: &payload.Stacktrace{
			Frames: []payload.Frame{
				{
					Colno:    6,
					Filename: "http://foo.com/valid.js",
					Function: "eval",
					Lineno:   5,
				},
			},
		},
	}

	transformException(testLogger, store, input, "123")

	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "source map found") {
		t.Error("Sourcemap file wasn't found")
	}
}

type bufLogger struct {
	w io.Writer
}

func (l *bufLogger) Log(keyvals ...any) error {
	var sb strings.Builder
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			sb.WriteString(fmt.Sprintf("%v=%v ", keyvals[i], keyvals[i+1]))
		}
	}
	sb.WriteString("\n")

	_, err := l.w.Write([]byte(sb.String()))
	return err
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

func Test_newSourceMapsStore_BasePathInitialization(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sourcemaps-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	expectedBasePath := tempDir
	if !strings.HasSuffix(expectedBasePath, string(filepath.Separator)) {
		expectedBasePath = expectedBasePath + string(filepath.Separator)
	}

	tests := []struct {
		name             string
		basePath         string
		expectedBasePath string
		expectWarning    bool
	}{
		{
			name:             "with base path",
			basePath:         tempDir,
			expectedBasePath: expectedBasePath,
			expectWarning:    false,
		},
		{
			name:             "empty base path",
			basePath:         "",
			expectedBasePath: "",
			expectWarning:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logOutput strings.Builder
			logger := log.NewLogfmtLogger(&logOutput)

			args := SourceMapsArguments{
				BaseSourceMapsPath: tt.basePath,
			}

			store := newSourceMapsStore(logger, args, nil, nil, nil)

			fileService, ok := store.fs.(osFileService)
			require.True(t, ok, "Expected store.fs to be osFileService")

			assert.Equal(t, tt.expectedBasePath, fileService.basePath)

			if tt.expectWarning {
				assert.Contains(t, logOutput.String(), "No base path specified for source maps")
			}
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

type mockFileService struct {
	files map[string][]byte
	stats []string
	reads []string
}

func (s *mockFileService) Stat(name string) (fs.FileInfo, error) {
	s.stats = append(s.stats, name)
	_, ok := s.files[name]
	if !ok {
		return nil, errors.New("file not found")
	}
	return nil, nil
}

func (s *mockFileService) ReadFile(name string) ([]byte, error) {
	s.reads = append(s.reads, name)
	content, ok := s.files[name]
	if ok {
		return content, nil
	}
	return nil, errors.New("file not found")
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

// Helper function to create a mock exception with a specific source URL
func createMockExceptionWithURL(sourceURL string) *payload.Exception {
	return &payload.Exception{
		Stacktrace: &payload.Stacktrace{
			Frames: []payload.Frame{
				{
					Colno:    6,
					Filename: sourceURL,
					Function: "eval",
					Lineno:   5,
				},
			},
		},
	}
}

func TestSourceMapsDownload(t *testing.T) {
	tests := []struct {
		name            string
		downloadEnabled bool
		allowedOrigins  []string
		sourceURL       string
		setupResponses  []struct {
			*http.Response
			error
		}
		expectedRequests  []string
		expectedTransform bool
	}{
		{
			name:            "successful download",
			downloadEnabled: true,
			allowedOrigins:  []string{"*"},
			sourceURL:       "http://localhost:1234/foo.js",
			setupResponses: []struct {
				*http.Response
				error
			}{
				{newResponseFromTestData(t, "foo.js"), nil},
				{newResponseFromTestData(t, "foo.js.map"), nil},
			},
			expectedRequests:  []string{"http://localhost:1234/foo.js", "http://localhost:1234/foo.js.map"},
			expectedTransform: true,
		},
		{
			name:            "download error",
			downloadEnabled: true,
			allowedOrigins:  []string{"*"},
			sourceURL:       "http://localhost:1234/foo.js",
			setupResponses: []struct {
				*http.Response
				error
			}{
				{
					&http.Response{StatusCode: 500, Body: io.NopCloser(bytes.NewReader(nil))},
					nil,
				},
			},
			expectedRequests:  []string{"http://localhost:1234/foo.js"},
			expectedTransform: false,
		},
		{
			name:            "origin filtering",
			downloadEnabled: true,
			allowedOrigins:  []string{"http://bar.com/"},
			sourceURL:       "http://bar.com/foo.js",
			setupResponses: []struct {
				*http.Response
				error
			}{
				{newResponseFromTestData(t, "foo.js"), nil},
				{newResponseFromTestData(t, "foo.js.map"), nil},
			},
			expectedRequests:  []string{"http://bar.com/foo.js", "http://bar.com/foo.js.map"},
			expectedTransform: true,
		},
		{
			name:              "disallowed origin",
			downloadEnabled:   true,
			allowedOrigins:    []string{"http://bar.com/"},
			sourceURL:         "http://other-domain.com/foo.js",
			setupResponses:    nil,
			expectedRequests:  nil,
			expectedTransform: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := util.TestLogger(t)

			httpClient := &mockHTTPClient{
				responses: tt.setupResponses,
			}

			store := newSourceMapsStore(
				logger,
				SourceMapsArguments{
					Download:            tt.downloadEnabled,
					DownloadFromOrigins: tt.allowedOrigins,
				},
				setupSourceMapMetrics(),
				httpClient,
				&mockFileService{},
			)

			// Create a mock exception with a frame pointing to the source URL
			exception := createMockExceptionWithURL(tt.sourceURL)

			// Transform the exception
			transformed := transformException(logger, store, exception, "123")

			// Verify requests
			if tt.expectedRequests != nil {
				require.Equal(t, tt.expectedRequests, httpClient.requests)
			} else {
				assert.Empty(t, httpClient.requests)
			}

			// Verify transformation
			if tt.expectedTransform {
				// The first frame should be transformed if expectedTransform is true
				assert.NotEqual(t, exception.Stacktrace.Frames[0].Filename, transformed.Stacktrace.Frames[0].Filename,
					"Frame should be transformed")
			} else {
				// The frames should remain unchanged
				assert.Equal(t, exception.Stacktrace.Frames[0].Filename, transformed.Stacktrace.Frames[0].Filename,
					"Frame should not be transformed")
			}
		})
	}
}

func TestSourceMapsFileSystem(t *testing.T) {
	tests := []struct {
		name             string
		downloadEnabled  bool
		allowedOrigins   []string
		stackFrames      []payload.Frame
		fileServiceFiles map[string][]byte
		expectedReads    []string
		expectedStats    []string
		expectedRequests []string
	}{
		{
			name:            "read from filesystem only",
			downloadEnabled: false,
			stackFrames: []payload.Frame{
				{
					Colno:    6,
					Filename: "http://foo.com/foo.js",
					Function: "eval",
					Lineno:   5,
				},
			},
			fileServiceFiles: map[string][]byte{
				filepath.FromSlash("/var/build/latest/foo.js.map"): loadTestData(t, "foo.js.map"),
			},
			expectedReads:    []string{filepath.FromSlash("/var/build/latest/foo.js.map")},
			expectedStats:    []string{filepath.FromSlash("/var/build/latest/foo.js.map")},
			expectedRequests: nil,
		},
		{
			name:            "read from filesystem then download",
			downloadEnabled: true,
			allowedOrigins:  []string{"*"},
			stackFrames: []payload.Frame{
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
			fileServiceFiles: map[string][]byte{
				filepath.FromSlash("/var/build/latest/foo.js.map"): loadTestData(t, "foo.js.map"),
			},
			expectedReads:    []string{filepath.FromSlash("/var/build/latest/foo.js.map")},
			expectedStats:    []string{filepath.FromSlash("/var/build/latest/foo.js.map")},
			expectedRequests: []string{"http://bar.com/foo.js", "http://bar.com/foo.js.map"},
		},
		{
			name:            "read without download if disabled",
			downloadEnabled: false,
			allowedOrigins:  []string{"*"},
			stackFrames: []payload.Frame{
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
			fileServiceFiles: map[string][]byte{
				filepath.FromSlash("/var/build/latest/foo.js.map"): loadTestData(t, "foo.js.map"),
			},
			expectedReads:    []string{filepath.FromSlash("/var/build/latest/foo.js.map")},
			expectedStats:    []string{filepath.FromSlash("/var/build/latest/foo.js.map")},
			expectedRequests: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := util.TestLogger(t)

			httpClient := &mockHTTPClient{
				responses: []struct {
					*http.Response
					error
				}{
					{newResponseFromTestData(t, "foo.js"), nil},
					{newResponseFromTestData(t, "foo.js.map"), nil},
				},
			}

			fileService := &mockFileService{
				files: tt.fileServiceFiles,
			}

			store := newSourceMapsStore(
				logger,
				SourceMapsArguments{
					Download:            tt.downloadEnabled,
					DownloadFromOrigins: tt.allowedOrigins,
					Locations: []LocationArguments{
						{
							MinifiedPathPrefix: "http://foo.com/",
							Path:               filepath.FromSlash("/var/build/latest/"),
						},
					},
				},
				setupSourceMapMetrics(),
				httpClient,
				fileService,
			)

			// Create a mock exception with the test stack frames
			exception := &payload.Exception{
				Stacktrace: &payload.Stacktrace{
					Frames: tt.stackFrames,
				},
			}

			// Transform the exception
			transformException(logger, store, exception, "123")

			// Verify file service interactions
			require.Equal(t, tt.expectedStats, fileService.stats)
			require.Equal(t, tt.expectedReads, fileService.reads)

			// Verify HTTP requests
			if tt.expectedRequests != nil {
				require.Equal(t, tt.expectedRequests, httpClient.requests)
			} else {
				assert.Empty(t, httpClient.requests)
			}
		})
	}
}

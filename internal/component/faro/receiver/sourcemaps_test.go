package receiver

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/faro/receiver/internal/payload"
	alloyutil "github.com/grafana/alloy/internal/util"
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
		logger = alloyutil.TestAlloyLogger(t).Slog()

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
		logger = alloyutil.TestAlloyLogger(t).Slog()

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
		logger = alloyutil.TestAlloyLogger(t).Slog()

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
		logger = alloyutil.TestAlloyLogger(t).Slog()

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
		logger = alloyutil.TestAlloyLogger(t).Slog()

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
		logger = alloyutil.TestAlloyLogger(t).Slog()

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

func Test_sourceMapsStoreImpl_ReadFromRemoteLocation(t *testing.T) {
	var (
		logger = alloyutil.TestAlloyLogger(t).Slog()

		httpClient = &mockHTTPClient{
			responses: []struct {
				*http.Response
				error
			}{
				{newResponseFromTestData(t, "foo.js.map"), nil},
			},
		}

		fileService = newTestFileService()
	)

	var store = newSourceMapsStore(
		logger,
		SourceMapsArguments{
			Download:            false,
			DownloadFromOrigins: []string{"*"},
			Locations: []LocationArguments{
				{
					MinifiedPathPrefix: "http://foo.com/",
					Path:               "http://baz.com/baz",
				},
			},
		},
		newSourceMapMetrics(prometheus.NewRegistry()),
		httpClient,
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

	require.Equal(t, []string{}, fileService.stats)
	require.Equal(t, []string{}, fileService.reads)
	require.Equal(t, []string{"http://baz.com/baz/foo.js.map"}, httpClient.requests)
	require.Equal(t, expect, actual)
}

func Test_sourceMapsStoreImpl_ReadFromFileSystemIfBothLocalAndRemoteLocation(t *testing.T) {
	var (
		logger = alloyutil.TestAlloyLogger(t).Slog()

		httpClient = &mockHTTPClient{}

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
					Path:               "http://baz.com/baz/",
				},
				{
					MinifiedPathPrefix: "http://foo.com/",
					Path:               filepath.FromSlash("/var/build/latest/"),
				},
			},
		},
		newSourceMapMetrics(prometheus.NewRegistry()),
		httpClient,
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

func Test_sourceMapsStoreImpl_ReadFromRemoteLocationIfBothDownloadAndLocationIsSet(t *testing.T) {
	var (
		logger = alloyutil.TestAlloyLogger(t).Slog()

		httpClient = &mockHTTPClient{
			responses: []struct {
				*http.Response
				error
			}{
				{newResponseFromTestData(t, "foo.js.map"), nil},
			},
		}

		fileService = newTestFileService()
	)

	var store = newSourceMapsStore(
		logger,
		SourceMapsArguments{
			Download:            true,
			DownloadFromOrigins: []string{"*"},
			Locations: []LocationArguments{
				{
					MinifiedPathPrefix: "http://foo.com/",
					Path:               "http://baz.com/baz/",
				},
			},
		},
		newSourceMapMetrics(prometheus.NewRegistry()),
		httpClient,
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

	require.Equal(t, []string{}, fileService.stats)
	require.Equal(t, []string{}, fileService.reads)
	require.Equal(t, []string{"http://baz.com/baz/foo.js.map"}, httpClient.requests)
	require.Equal(t, expect, actual)
}

func Test_sourceMapsStoreImpl_FilepathSanitized(t *testing.T) {
	var (
		logger = alloyutil.TestAlloyLogger(t).Slog()

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
			actual := parseOriginPatterns(tc.origins).matches(tc.url)

			if tc.shouldMatch {
				require.True(t, actual, "expected %v to be matched from origin set %v", tc.url, tc.origins)
			} else {
				require.False(t, actual, "expected %v to not be matched from origin set %v", tc.url, tc.origins)
			}
		})
	}
}

func Test_urlMatchesOrigins_URLParts(t *testing.T) {
	tt := []struct {
		name    string
		origin  string
		match   []string
		noMatch []string
	}{
		// Host
		{
			name:   "exact host",
			origin: "https://app.example.com/",
			match: []string{
				"https://APP.Example.COM/foo.js",
				"https://app.example.com:443/foo.js",
			},
			noMatch: []string{
				"http://app.example.com/foo.js",
				"https://app.example.com:8443/foo.js",
				"https://app.example.com.example.net/foo.js",
			},
		},
		{
			name:   "origin without trailing slash doesn't match a longer host or userinfo",
			origin: "https://app.example.com",
			match:  []string{"https://app.example.com/static/foo.js"},
			noMatch: []string{
				"https://app.example.com.other.example.net/foo.js",
				"https://app.example.com@other.example.net/foo.js",
				"https://app.example.com%40other.example.net/foo.js",
				"https://app.example.com\\@other.example.net/foo.js",
			},
		},
		{
			name:   "subdomain wildcard",
			origin: "https://*.example.com/",
			match: []string{
				"https://cdn.example.com/foo.js",
				"https://a.cdn.example.com/foo.js",
			},
			noMatch: []string{
				"https://example.com/foo.js",
				"https://example.net/x.example.com/foo.js",
				"https://example.net/?d=x.example.com/foo.js",
			},
		},
		{
			name:    "subdomain wildcard without scheme doesn't match the query",
			origin:  "*.example.com/",
			noMatch: []string{"https://example.net/?d=x.example.com/foo.js"},
		},
		{
			name:    "backslash in host",
			origin:  "https://*.app.example.com/",
			noMatch: []string{"https://other.example.net\\.app.example.com/foo.js"},
		},
		{
			name:    "wildcard inside a part of the host",
			origin:  "https://app-*.example.com/",
			match:   []string{"https://app-eu.example.com/foo.js"},
			noMatch: []string{"https://app-eu.cdn.example.com/foo.js"},
		},
		{
			name:    "leading wildcard without a dot doesn't match subdomains",
			origin:  "https://*example.com/",
			noMatch: []string{"https://a.cdn.example.com/foo.js"},
		},
		{
			name:    "single character wildcard",
			origin:  "https://app?.example.com/",
			match:   []string{"https://app1.example.com/foo.js"},
			noMatch: []string{"https://app12.example.com/foo.js"},
		},
		{
			name:    "wildcard in an IP address",
			origin:  "http://10.0.0.*/",
			match:   []string{"http://10.0.0.1/foo.js"},
			noMatch: []string{"http://10.0.0.1.example.net/foo.js"},
		},
		{
			name:   "IPv6 host",
			origin: "http://[2001:db8::1]:8080/",
			match:  []string{"http://[2001:db8::1]:8080/foo.js"},
		},
		{
			// The zone is ignored when dialing, so it must not satisfy a host wildcard.
			name:    "IPv6 zone",
			origin:  "http://*.example.com:*/",
			noMatch: []string{"http://[::ffff:127.0.0.1%25.example.com]:8080/foo.js"},
		},
		{
			// IDNA turns these into dots when dialing, so they must not hide a label from a host wildcard.
			name:   "non-ASCII dots",
			origin: "http://192.168.*.*/",
			noMatch: []string{
				"http://192.168.0。example.net/foo.js",
				"http://192.168.0．example.net/foo.js",
				"http://192.168.0｡example.net/foo.js",
			},
		},

		// Port
		{
			name:    "explicit default port",
			origin:  "https://app.example.com:443/",
			noMatch: []string{"https://app.example.com:8443/foo.js"},
		},
		{
			name:   "explicit port",
			origin: "http://app.example.com:8080/",
			match:  []string{"http://app.example.com:8080/foo.js"},
		},
		{
			name:   "port wildcard",
			origin: "http://localhost:*/",
			match:  []string{"http://localhost:3000/foo.js"},
		},
		{
			name:   "any host on a port",
			origin: "http://*:8080/",
			match:  []string{"http://cdn.example.com:8080/foo.js"},
		},

		// Path
		{
			name:   "path prefix",
			origin: "https://app.example.com/maps",
			match: []string{
				"https://app.example.com/maps/foo.js",
				"https://app.example.com/maps-other/foo.js",
			},
		},
		{
			name:   "path prefix with trailing slash",
			origin: "https://app.example.com/maps/",
			match:  []string{"https://app.example.com/maps/foo.js"},
		},
		{
			name:   "path wildcard",
			origin: "https://app.example.com/*/static/",
			match:  []string{"https://app.example.com/v2/static/foo.js"},
		},

		// Scheme
		{
			name:    "non-http scheme",
			origin:  "file:///",
			noMatch: []string{"file:///etc/passwd"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			for _, u := range tc.match {
				require.True(t, parseOriginPatterns([]string{tc.origin}).matches(u), "expected %s to match %s", u, tc.origin)
			}
			for _, u := range tc.noMatch {
				require.False(t, parseOriginPatterns([]string{tc.origin}).matches(u), "expected %s not to match %s", u, tc.origin)
			}
		})
	}
}

func Test_newSourceMapsStore_WarnsOnInvalidOrigins(t *testing.T) {
	var buf bytes.Buffer
	newSourceMapsStore(
		slog.New(slog.NewTextHandler(&buf, nil)),
		SourceMapsArguments{Download: true, DownloadFromOrigins: []string{"https://app.example.com/", "http://"}},
		newSourceMapMetrics(prometheus.NewRegistry()),
		nil,
		nil,
		newTestFileService(),
	)
	require.Contains(t, buf.String(), "level=WARN")
	require.Contains(t, buf.String(), "origin=http://")
	require.NotContains(t, buf.String(), "app.example.com")
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
		logger      = alloyutil.TestAlloyLogger(t).Slog()
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

	logger := alloyutil.TestAlloyLogger(t)

	for _, tc := range tt {
		reg := prometheus.NewRegistry()
		metrics := newSourceMapMetrics(reg)

		store := &sourceMapsStoreImpl{
			log:        logger.Slog(),
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

	logger := alloyutil.TestAlloyLogger(t)

	for _, tc := range tt {
		reg := prometheus.NewRegistry()
		metrics := newSourceMapMetrics(reg)

		store := &sourceMapsStoreImpl{
			log:        logger.Slog(),
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

func Test_sourceMapsStoreImpl_DownloadRedirects(t *testing.T) {
	var otherHits atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherHits.Add(1)
		_, _ = w.Write(loadTestData(t, "foo.js"))
	}))
	defer other.Close()

	var loopHits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/same/foo.js", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/foo.js", http.StatusFound)
	})
	mux.HandleFunc("/other/foo.js", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/foo.js", http.StatusFound)
	})
	mux.HandleFunc("/loop/foo.js", func(w http.ResponseWriter, r *http.Request) {
		loopHits.Add(1)
		http.Redirect(w, r, "/loop/foo.js", http.StatusFound)
	})
	mux.HandleFunc("/foo.js", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(loadTestData(t, "foo.js"))
	})
	mux.HandleFunc("/same/foo.js.map", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(loadTestData(t, "foo.js.map"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	origins := []string{srv.URL + "/"}
	newStore := func() *sourceMapsStoreImpl {
		return newSourceMapsStore(
			alloyutil.TestAlloyLogger(t).Slog(),
			SourceMapsArguments{Download: true, DownloadFromOrigins: origins},
			newSourceMapMetrics(prometheus.NewRegistry()),
			nil,
			newDownloadClient(time.Second, parseOriginPatterns(origins)),
			newTestFileService(),
		)
	}

	t.Run("same origin redirect is followed", func(t *testing.T) {
		smap, err := newStore().GetSourceMap(srv.URL+"/same/foo.js", "123")
		require.NoError(t, err)
		require.NotNil(t, smap)
	})

	t.Run("redirect to other origin is refused", func(t *testing.T) {
		smap, err := newStore().GetSourceMap(srv.URL+"/other/foo.js", "123")
		require.ErrorContains(t, err, "not allowed by download_from_origins")
		require.Nil(t, smap)
		require.Zero(t, otherHits.Load())
	})

	t.Run("redirect chain is capped", func(t *testing.T) {
		smap, err := newStore().GetSourceMap(srv.URL+"/loop/foo.js", "123")
		require.ErrorContains(t, err, "stopped after")
		require.Nil(t, smap)
		require.Equal(t, int32(maxDownloadRedirects), loopHits.Load())
	})
}

func Test_sourceMapsStoreImpl_DownloadRefusesSourceMapURLFromOtherOrigin(t *testing.T) {
	httpClient := &mockHTTPClient{
		responses: []struct {
			*http.Response
			error
		}{
			{&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("console.log(1)\n//# sourceMappingURL=http://other.example.net/foo.js.map\n")),
			}, nil},
			{newResponseFromTestData(t, "foo.js.map"), nil},
		},
	}

	store := newSourceMapsStore(
		alloyutil.TestAlloyLogger(t).Slog(),
		SourceMapsArguments{Download: true, DownloadFromOrigins: []string{"http://app.example.com/"}},
		newSourceMapMetrics(prometheus.NewRegistry()),
		httpClient,
		httpClient,
		newTestFileService(),
	)

	smap, err := store.GetSourceMap("http://app.example.com/foo.js", "123")
	require.ErrorContains(t, err, "not allowed by download_from_origins")
	require.Nil(t, smap)
	require.Equal(t, []string{"http://app.example.com/foo.js"}, httpClient.requests)
}

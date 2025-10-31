package mimirclient

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildURL(t *testing.T) {
	tc := []struct {
		name      string
		path      string
		method    string
		url       string
		resultURL string
	}{
		{
			name:      "builds the correct URL with a trailing slash",
			path:      "/prometheus/config/v1/rules",
			method:    http.MethodPost,
			url:       "http://mimir.local/",
			resultURL: "http://mimir.local/prometheus/config/v1/rules",
		},
		{
			name:      "builds the correct URL without a trailing slash",
			path:      "/prometheus/config/v1/rules",
			method:    http.MethodPost,
			url:       "http://mimir.local",
			resultURL: "http://mimir.local/prometheus/config/v1/rules",
		},
		{
			name:      "builds the correct URL when the base url has a path",
			path:      "/prometheus/config/v1/rules",
			method:    http.MethodPost,
			url:       "http://mimir.local/apathto",
			resultURL: "http://mimir.local/apathto/prometheus/config/v1/rules",
		},
		{
			name:      "builds the correct URL when the base url has a path with trailing slash",
			path:      "/prometheus/config/v1/rules",
			method:    http.MethodPost,
			url:       "http://mimir.local/apathto/",
			resultURL: "http://mimir.local/apathto/prometheus/config/v1/rules",
		},
		{
			name:      "builds the correct URL with a trailing slash and the target path contains special characters",
			path:      "/prometheus/config/v1/rules/%20%2Fspace%F0%9F%8D%BB",
			method:    http.MethodPost,
			url:       "http://mimir.local/",
			resultURL: "http://mimir.local/prometheus/config/v1/rules/%20%2Fspace%F0%9F%8D%BB",
		},
		{
			name:      "builds the correct URL without a trailing slash and the target path contains special characters",
			path:      "/prometheus/config/v1/rules/%20%2Fspace%F0%9F%8D%BB",
			method:    http.MethodPost,
			url:       "http://mimir.local",
			resultURL: "http://mimir.local/prometheus/config/v1/rules/%20%2Fspace%F0%9F%8D%BB",
		},
		{
			name:      "builds the correct URL when the base url has a path and the target path contains special characters",
			path:      "/prometheus/config/v1/rules/%20%2Fspace%F0%9F%8D%BB",
			method:    http.MethodPost,
			url:       "http://mimir.local/apathto",
			resultURL: "http://mimir.local/apathto/prometheus/config/v1/rules/%20%2Fspace%F0%9F%8D%BB",
		},
		{
			name:      "builds the correct URL when the base url has a path and the target path starts with a escaped slash",
			path:      "/prometheus/config/v1/rules/%2F-first-char-slash",
			method:    http.MethodPost,
			url:       "http://mimir.local/apathto",
			resultURL: "http://mimir.local/apathto/prometheus/config/v1/rules/%2F-first-char-slash",
		},
		{
			name:      "builds the correct URL when the base url has a path and the target path ends with a escaped slash",
			path:      "/prometheus/config/v1/rules/last-char-slash%2F",
			method:    http.MethodPost,
			url:       "http://mimir.local/apathto",
			resultURL: "http://mimir.local/apathto/prometheus/config/v1/rules/last-char-slash%2F",
		},
		{
			name:      "builds the correct URL with a customized prometheus_http_prefix",
			path:      "/mimir/config/v1/rules",
			method:    http.MethodPost,
			url:       "http://mimir.local/",
			resultURL: "http://mimir.local/mimir/config/v1/rules",
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			url, err := url.Parse(tt.url)
			require.NoError(t, err)

			req, err := buildRequest("op", tt.path, tt.method, *url, []byte{})
			require.NoError(t, err)
			require.Equal(t, tt.resultURL, req.URL.String())
		})
	}
}

func TestCheckResponseSuccess(t *testing.T) {
	tc := []struct {
		name       string
		body       string
		statusCode int
	}{
		{
			name:       "returns nil error for 200 response",
			body:       "200 message!",
			statusCode: http.StatusOK,
		},
		{
			name:       "returns nil error for 204 response",
			body:       "",
			statusCode: http.StatusNoContent,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			response := &http.Response{
				Status:     http.StatusText(tt.statusCode),
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}

			err := checkResponse(response)
			require.NoError(t, err)
		})
	}
}

func TestCheckResponseErrors(t *testing.T) {
	tc := []struct {
		name       string
		body       string
		statusCode int
		canRetry   bool
	}{
		{
			name:       "returns correct error for 400 response",
			body:       "400 message!",
			statusCode: http.StatusBadRequest,
			canRetry:   false,
		},
		{
			name:       "returns correct error for 404 response",
			body:       "404 message!",
			statusCode: 404,
			canRetry:   false,
		},
		{
			name:       "returns correct error for 429 response",
			body:       "429 message!",
			statusCode: http.StatusTooManyRequests,
			canRetry:   true,
		},
		{
			name:       "returns correct error for 500 response",
			body:       "500 message!",
			statusCode: http.StatusInternalServerError,
			canRetry:   true,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			response := &http.Response{
				Status:     http.StatusText(tt.statusCode),
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}

			err := checkResponse(response)
			require.Error(t, err)
			require.Equal(t, tt.canRetry, IsRecoverable(err), "%+v is not expected recoverable/unrecoverable", err)
		})
	}
}

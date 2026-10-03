package source

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func serve(h http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(h)
}

func doFetch(ctx context.Context, url string, maxSize int64, header http.Header) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	return fetch(http.DefaultClient, req, maxSize)
}

func gzipBytes(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(b); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestFetchOK(t *testing.T) {
	srv := serve(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("\xef\xbb\xbf[1]")) })
	defer srv.Close()

	body, err := doFetch(t.Context(), srv.URL, 1024, nil)
	require.NoError(t, err)
	require.Equal(t, "[1]", string(body))
}

func TestFetchStatus(t *testing.T) {
	srv := serve(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
	defer srv.Close()

	_, err := doFetch(t.Context(), srv.URL, 1024, nil)
	require.Equal(t, reasonStatus, reasonOf(err))
	require.ErrorContains(t, err, "status 503")
}

func TestFetchEmptyBody(t *testing.T) {
	srv := serve(func(w http.ResponseWriter, _ *http.Request) {})
	defer srv.Close()

	_, err := doFetch(t.Context(), srv.URL, 1024, nil)
	require.Equal(t, reasonParse, reasonOf(err))
}

func TestFetchTooLarge(t *testing.T) {
	srv := serve(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(bytes.Repeat([]byte("a"), 2048)) })
	defer srv.Close()

	_, err := doFetch(t.Context(), srv.URL, 1024, nil)
	require.Equal(t, reasonTooLarge, reasonOf(err))
}

func TestFetchGzip(t *testing.T) {
	gz, err := gzipBytes([]byte(`{"a":1}`))
	require.NoError(t, err)
	srv := serve(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(gz)
	})
	defer srv.Close()

	// An explicit Accept-Encoding stops the transport from decoding, so fetch must do it.
	body, err := doFetch(t.Context(), srv.URL, 1024, http.Header{"Accept-Encoding": {"gzip"}})
	require.NoError(t, err)
	require.Equal(t, `{"a":1}`, string(body))
}

func TestFetchGzipBomb(t *testing.T) {
	gz, err := gzipBytes(make([]byte, 20<<20))
	require.NoError(t, err)
	require.Less(t, len(gz), 1<<20, "the compressed body must be under the limit")
	srv := serve(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(gz)
	})
	defer srv.Close()

	_, err = doFetch(t.Context(), srv.URL, 10<<20, http.Header{"Accept-Encoding": {"gzip"}})
	require.Equal(t, reasonTooLarge, reasonOf(err))
}

func TestFetchTimeout(t *testing.T) {
	srv := serve(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
	defer srv.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	_, err := doFetch(ctx, srv.URL, 1024, nil)
	require.Equal(t, reasonTimeout, reasonOf(err))
}

func TestFetchRedactsURL(t *testing.T) {
	srv := serve(func(http.ResponseWriter, *http.Request) {})
	url := srv.URL + "/api?api_key=supersecret"
	srv.Close()

	_, err := doFetch(t.Context(), url, 1024, nil)
	require.Error(t, err)
	require.Equal(t, reasonRequest, reasonOf(err))
	require.NotContains(t, err.Error(), "supersecret")
}

// TestFetchFrameRedactsBuildError checks that an error from building the
// request does not show query params.
func TestFetchFrameRedactsBuildError(t *testing.T) {
	s := querySpec{name: "q", qtype: "json", source: sourceURL, method: http.MethodGet, url: "http://x/%zz?api_key=supersecret"}
	_, err := fetchFrame(t.Context(), &generation{client: http.DefaultClient, timeout: time.Second, maxSize: 1024}, s, atomic.NewInt32(0), buildFrame)
	require.Error(t, err)
	require.Equal(t, reasonRequest, reasonOf(err))
	require.NotContains(t, err.Error(), "supersecret")
}

// secretRedirects are bad redirect Location values. net/http writes each
// one into its error text.
var secretRedirects = map[string]string{
	"query with scheme":    "https://files.example/%zz?token=supersecret",
	"query without scheme": "/%zz?token=supersecret",
	"userinfo":             "https://user:supersecret@files.example/%zz",
	"query with a space":   "https://files.example/%zz?token=prefix supersecret",
	// A control character makes a malformed header line, which net/http
	// reports with the whole line.
	"malformed line with a backslash": "https://files.example/x?token=prefix\\supersecret\x01",
	"malformed line with a space":     "https://files.example/x?token=prefix supersecret\x01",
	"space before the query":          "https://files.example/%zz b?token=supersecret",
	"relative space before the query": "/rel/%zz b?token=supersecret",
	"relative token":                  "x/%zz?supersecret more",
	"password with a space":           "https://user:supersecret x@files.example/%zz",
	"user with a space":               "https://us er:supersecret@files.example/%zz",
}

func redirectTo(location string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", location)
		w.WriteHeader(http.StatusFound)
	}
}

// TestFetchRedactsRedirectURL checks bad redirect Location values. net/http
// puts them inside the error text, below the outer *url.Error.
func TestFetchRedactsRedirectURL(t *testing.T) {
	for name, location := range secretRedirects {
		t.Run(name, func(t *testing.T) {
			srv := serve(redirectTo(location))
			defer srv.Close()

			_, err := doFetch(t.Context(), srv.URL, 1024, nil)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "supersecret")
		})
	}
}

func TestScrubURLs(t *testing.T) {
	in := `Get "http://a.example/x?key=abc": failed to parse Location header "https://files.example/%zz?token=supersecret": bad`
	want := `Get "http://a.example/x?key=REDACTED": failed to parse Location header "https://files.example/%zz?REDACTED": bad`
	require.Equal(t, want, scrubURLs(in))

	in = `parse "/%zz?token=s1": bad; parse "https://user:pw@files.example/%zz": bad`
	want = `parse "/%zz?REDACTED": bad; parse "https://files.example/%zz": bad`
	require.Equal(t, want, scrubURLs(in))
}

func TestScrubURLsQuotedTokens(t *testing.T) {
	tests := map[string]string{
		// Ordinary error text stays as it is.
		`invalid value "what?" for field x`: `invalid value "what?" for field x`,
		`json: unknown field "a?b"`:         `json: unknown field "a?b"`,
		`unexpected token "?"`:              `unexpected token "?"`,
		`unexpected token "what? is this"`:  `unexpected token "what? is this"`,
		// URL-like tokens lose their query.
		`parse "/%zz?token=supersecret": bad`:            `parse "/%zz?REDACTED": bad`,
		`parse "files/x?token=supersecret"`:              `parse "files/x?REDACTED"`,
		`parse "https://user:pw@files.example/%zz": bad`: `parse "https://files.example/%zz": bad`,
	}
	for in, want := range tests {
		require.Equal(t, want, scrubURLs(in), "input %q", in)
	}
}

// TestScrubURLsQuotedQueryWithSpace checks a quoted URL whose query has a
// space or an escape. The URL pass stops there, so the quoted pass must
// redact the rest.
func TestScrubURLsQuotedQueryWithSpace(t *testing.T) {
	tests := map[string]string{
		"space":                   `failed to parse Location header "https://files.example/%zz?token=prefix supersecret": bad`,
		"escaped quote":           `failed to parse Location header "https://files.example/%zz?token=prefix\"supersecret": bad`,
		"escaped quote and space": `parse "https://files.example/%zz?token=prefix\" supersecret": bad`,
		"percent-encoded quote":   `parse "https://files.example/%zz?token=prefix%22 supersecret": bad`,
		"tab":                     `parse "https://files.example/%zz?token=prefix\tsupersecret": bad`,
		"raw tab":                 "parse \"https://files.example/%zz?token=prefix\tsupersecret\": bad",
		"no scheme":               `parse "/%zz?token=prefix\" supersecret": bad`,
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			out := scrubURLs(in)
			require.NotContains(t, out, "supersecret")
			require.NotContains(t, out, "prefix")
			require.Contains(t, out, `?REDACTED": bad`)
		})
	}
}

// TestScrubURLsRemainingGaps checks text that the URL pass alone does not
// fully redact.
func TestScrubURLsRemainingGaps(t *testing.T) {
	tests := map[string]string{
		"unquoted backslash":          `malformed MIME header line: Location: https://files.example/x?token=prefix\supersecret`,
		"unquoted space":              `malformed MIME header line: Location: https://files.example/x?token=prefix supersecret`,
		"quoted line":                 `malformed MIME header line: "Location: https://files.example/x?token=prefix\\supersecret\x01"`,
		"space before query":          `parse "https://files.example/%zz b?token=supersecret": bad`,
		"relative space before query": `parse "/rel/%zz b?token=supersecret": bad`,
		"relative token":              `parse "x/%zz?supersecret more": bad`,
		"password with a space":       `parse "https://user:pa supersecret@files.example/x": bad`,
		"user with a space":           `parse "https://us er:supersecret@files.example/x": bad`,
		"unquoted password":           `bad URL https://us er:supersecret@files.example/x`,
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			require.NotContains(t, scrubURLs(in), "supersecret")
		})
	}
}

// TestScrubURLsUnquotedKeepsNextLine checks that the redaction of an
// unquoted URL ends at the end of its line.
func TestScrubURLsUnquotedKeepsNextLine(t *testing.T) {
	in := "bad line: https://files.example/x?token=a supersecret\nnext line"
	out := scrubURLs(in)
	require.NotContains(t, out, "supersecret")
	require.Contains(t, out, "\nnext line")
}

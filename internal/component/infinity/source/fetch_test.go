package source

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/common/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/service/cluster"
)

func serve(h http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(h)
}

// testFetchTimeout is the timeout that doFetch puts in error messages.
const testFetchTimeout = 50 * time.Millisecond

func doFetch(ctx context.Context, url string, maxSize int64, header http.Header) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	return fetch(http.DefaultClient, req, requestTarget{rawURL: url, timeout: testFetchTimeout}, maxSize)
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
	_, err := doFetch(ctx, srv.URL+"/api?api_key=supersecret", 1024, nil)
	require.Equal(t, reasonTimeout, reasonOf(err))
	require.EqualError(t, err, "request to "+srv.URL+"/api?api_key=REDACTED timed out after 50ms")
}

func TestFetchBodyReadFails(t *testing.T) {
	srv := serve(func(w http.ResponseWriter, _ *http.Request) {
		// The body is shorter than Content-Length, so the read fails.
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("[1"))
	})
	defer srv.Close()

	_, err := doFetch(t.Context(), srv.URL+"/api?api_key=supersecret", 1024, nil)
	require.Equal(t, reasonRequest, reasonOf(err))
	require.EqualError(t, err, "reading the response from "+srv.URL+"/api?api_key=REDACTED failed")
}

func TestFetchCanceled(t *testing.T) {
	srv := serve(func(http.ResponseWriter, *http.Request) {})
	defer srv.Close()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := doFetch(ctx, srv.URL+"/api?api_key=supersecret", 1024, nil)
	require.Equal(t, reasonRequest, reasonOf(err))
	require.EqualError(t, err, "request to "+srv.URL+"/api?api_key=REDACTED was canceled")
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
	require.EqualError(t, err, "could not build the request for <invalid url>")
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

// moreSecretRedirects are Location values that the text scrubbing in
// scrubURLs cannot fully redact. Health and warn logs must still hide them,
// because those messages hold no server text.
var moreSecretRedirects = map[string]string{
	"at sign in the password": "https://user:pa@supersecret@files.example/%zz",
	"query without a name":    "https://h.example/x?supersecret",
	"secret in the fragment":  "https://h.example/%zz#access_token=supersecret",
	"relative without slash":  "rel%zz?supersecret",
}

// allSecretRedirects returns secretRedirects and moreSecretRedirects in one map.
func allSecretRedirects() map[string]string {
	all := make(map[string]string, len(secretRedirects)+len(moreSecretRedirects))
	for k, v := range secretRedirects {
		all[k] = v
	}
	for k, v := range moreSecretRedirects {
		all[k] = v
	}
	return all
}

// forbiddenParts returns the text that a safe message must not contain for
// a redirect to location: the secret markers and each word of location.
func forbiddenParts(location string) []string {
	parts := []string{"supersecret", "pa@", "access_token"}
	for _, w := range strings.FieldsFunc(location, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}) {
		if len(w) >= 3 {
			parts = append(parts, w)
		}
	}
	return parts
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
	for name, location := range allSecretRedirects() {
		t.Run(name, func(t *testing.T) {
			srv := serve(redirectTo(location))
			defer srv.Close()

			_, err := doFetch(t.Context(), srv.URL, 1024, nil)
			require.Error(t, err)
			for _, part := range forbiddenParts(location) {
				require.NotContains(t, err.Error(), part)
			}
		})
	}
}

// TestFetchRedirectDetailIsScrubbed checks the debug detail of each bad
// redirect. It keeps the net/http text, with the secret scrubbed.
func TestFetchRedirectDetailIsScrubbed(t *testing.T) {
	for name, location := range secretRedirects {
		t.Run(name, func(t *testing.T) {
			srv := serve(redirectTo(location))
			defer srv.Close()

			_, err := doFetch(t.Context(), srv.URL, 1024, nil)
			var pe *pollError
			require.ErrorAs(t, err, &pe)
			require.Contains(t, pe.detail(), "Location")
			require.NotContains(t, pe.detail(), "supersecret")
		})
	}
}

func TestLibErrorIsScrubbedAndCapped(t *testing.T) {
	text := `jq: error at "https://user:pw@files.example/x?token=supersecret": ` + strings.Repeat("a", 300)
	err := newLibError(reasonParse, errors.New(text))
	require.Equal(t, reasonParse, reasonOf(err))
	require.NotContains(t, err.Error(), "supersecret")
	require.NotContains(t, err.Error(), "pw@")
	require.True(t, strings.HasPrefix(err.Error(), `jq: error at "https://files.example/x?token=REDACTED": aaa`), err.Error())
	require.Equal(t, maxLibErrorLen+len("..."), len([]rune(err.Error())))
	// The debug detail keeps the whole text, scrubbed.
	var pe *pollError
	require.ErrorAs(t, err, &pe)
	require.NotContains(t, pe.detail(), "supersecret")
	require.Contains(t, pe.detail(), strings.Repeat("a", 300))
}

func TestEmitErrorHidesReceiverText(t *testing.T) {
	pe := newEmitError(errors.New(`remote: Post "https://r.example/push?key=supersecret": 401 denied`))
	require.Equal(t, reasonEmit, reasonOf(pe))
	require.EqualError(t, pe, "could not send the result to the outputs")
	require.Contains(t, pe.detail(), "401 denied")
	require.NotContains(t, pe.detail(), "supersecret")
}

// closedAddr returns a local address where nothing listens.
func closedAddr() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := l.Addr().String()
	return addr, l.Close()
}

// oauthClient returns a client that gets its token from tokenURL.
func oauthClient(tokenURL string) (*http.Client, error) {
	args, err := parse(fmt.Sprintf(`
		client {
			oauth2 {
				client_id     = "id"
				client_secret = "secret"
				token_url     = %q
			}
		}
		query "q" {
			url = "http://example.com"
		}`, tokenURL))
	if err != nil {
		return nil, err
	}
	return config.NewClientFromConfig(*args.Client.Convert(), "test")
}

// errorCase builds a failing request. It returns the URL to request, the
// client to use and the safe message that the error must have.
type errorCase func(t *testing.T) (url string, client *http.Client, want string, err error)

// redirectCase returns an errorCase for a server that redirects to location.
func redirectCase(location string) errorCase {
	return func(t *testing.T) (string, *http.Client, string, error) {
		srv := serve(redirectTo(location))
		t.Cleanup(srv.Close)
		u := srv.URL + "/api?api_key=supersecret"
		return u, http.DefaultClient, "invalid redirect from " + srv.URL + "/api?api_key=REDACTED", nil
	}
}

// TestFetchFrameErrorMessages checks the safe message of each kind of
// request error. Each case makes a real error of that kind.
func TestFetchFrameErrorMessages(t *testing.T) {
	tests := map[string]errorCase{
		"connection refused": func(*testing.T) (string, *http.Client, string, error) {
			addr, err := closedAddr()
			return "http://" + addr + "/api?api_key=supersecret", http.DefaultClient,
				"connection to http://" + addr + "/api?api_key=REDACTED was refused", err
		},
		"configured param without a value": func(*testing.T) (string, *http.Client, string, error) {
			addr, err := closedAddr()
			return "http://" + addr + "/api?supersecret", http.DefaultClient,
				"connection to http://" + addr + "/api?REDACTED was refused", err
		},
		"configured fragment": func(*testing.T) (string, *http.Client, string, error) {
			addr, err := closedAddr()
			return "http://" + addr + "/api#access_token=supersecret", http.DefaultClient,
				"connection to http://" + addr + "/api was refused", err
		},
		"malformed header that is not Location": func(t *testing.T) (string, *http.Client, string, error) {
			srv := serve(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-Bad", "a\x01b")
				w.WriteHeader(http.StatusOK)
			})
			t.Cleanup(srv.Close)
			return srv.URL + "/api?api_key=supersecret", http.DefaultClient,
				"request to " + srv.URL + "/api?api_key=REDACTED failed", nil
		},
		"OAuth2 error with a success status": func(t *testing.T) (string, *http.Client, string, error) {
			tokenSrv := serve(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"supersecret"}`))
			})
			t.Cleanup(tokenSrv.Close)
			client, err := oauthClient(tokenSrv.URL + "/token")
			return "http://example.com/api?api_key=supersecret", client, "OAuth2 token request failed", err
		},
		"DNS lookup": func(*testing.T) (string, *http.Client, string, error) {
			return "http://user:supersecret@nothing.invalid/api?api_key=supersecret", http.DefaultClient,
				"DNS lookup for nothing.invalid failed", nil
		},
		"untrusted certificate": func(t *testing.T) (string, *http.Client, string, error) {
			srv := httptest.NewTLSServer(http.NotFoundHandler())
			t.Cleanup(srv.Close)
			return srv.URL + "/api?api_key=supersecret", http.DefaultClient,
				"TLS handshake with " + srv.URL + "/api?api_key=REDACTED failed", nil
		},
		"redirect loop": func(t *testing.T) (string, *http.Client, string, error) {
			srv := serve(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/loop?token=supersecret", http.StatusFound)
			})
			t.Cleanup(srv.Close)
			return srv.URL + "/api?api_key=supersecret", http.DefaultClient,
				"invalid redirect from " + srv.URL + "/api?api_key=REDACTED", nil
		},
		"unparsable Location":   redirectCase("/%zz?token=supersecret"),
		"malformed header line": redirectCase("https://files.example/x?token=prefix supersecret\x01"),
		"network error after a redirect": func(t *testing.T) (string, *http.Client, string, error) {
			srv := serve(redirectTo("http://nothing.invalid/x?token=supersecret"))
			t.Cleanup(srv.Close)
			return srv.URL + "/api?api_key=supersecret", http.DefaultClient,
				"request to " + srv.URL + "/api?api_key=REDACTED failed after a redirect", nil
		},
		"OAuth2 status": func(t *testing.T) (string, *http.Client, string, error) {
			tokenSrv := serve(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"supersecret"}`))
			})
			t.Cleanup(tokenSrv.Close)
			client, err := oauthClient(tokenSrv.URL + "/token?k=supersecret")
			return "http://example.com/api?api_key=supersecret", client,
				"OAuth2 token request failed with status 401", err
		},
		"OAuth2 token server down": func(*testing.T) (string, *http.Client, string, error) {
			addr, err := closedAddr()
			if err != nil {
				return "", nil, "", err
			}
			client, err := oauthClient("http://" + addr + "/token?k=supersecret")
			return "http://example.com/api?api_key=supersecret", client, "OAuth2 token request failed", err
		},
		"connection closed": func(t *testing.T) (string, *http.Client, string, error) {
			srv := serve(func(w http.ResponseWriter, _ *http.Request) {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err == nil {
					_ = conn.Close()
				}
			})
			t.Cleanup(srv.Close)
			return srv.URL + "/api?api_key=supersecret", http.DefaultClient,
				"request to " + srv.URL + "/api?api_key=REDACTED failed", nil
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			url, client, want, err := tc(t)
			require.NoError(t, err)
			s := querySpec{name: "q", qtype: "json", source: sourceURL, method: http.MethodGet, url: url}
			g := &generation{client: client, timeout: 5 * time.Second, maxSize: 1024}
			_, err = fetchFrame(t.Context(), g, s, atomic.NewInt32(0), buildFrame)
			require.Error(t, err)
			require.Equal(t, want, err.Error())
			require.Equal(t, reasonRequest, reasonOf(err))
		})
	}
}

// TestProxyErrorMessages checks that a failed proxy connection names the
// proxy, not the API host.
func TestProxyErrorMessages(t *testing.T) {
	refusing, err := closedAddr()
	require.NoError(t, err)
	proxies := map[string]string{
		"unresolvable proxy": "http://user:supersecret@proxy.invalid:3128",
		"refusing proxy":     "http://" + refusing,
	}
	for name, proxy := range proxies {
		for _, scheme := range []string{"http", "https"} {
			t.Run(name+" "+scheme, func(t *testing.T) {
				args, err := parse(fmt.Sprintf(`
					interval = "1s"
					timeout  = "500ms"
					client {
						proxy_url = %q
					}
					query "q" {
						url = %q
					}`, proxy, scheme+"://api.example/x?api_key=supersecret"))
				require.NoError(t, err)
				args.Output.Metrics = []otelcol.Consumer{&testConsumer{}}
				logs := newLogRecorder()
				opts := testOptions(t, cluster.Mock())
				opts.Logger = slog.New(logs)
				c, err := New(opts, args)
				require.NoError(t, err)
				cancel, _ := runComponent(t.Context(), c)
				defer cancel()

				want := `query "q" failed: connection to the proxy ` + redactURL(proxy) + ` failed`
				require.EventuallyWithT(t, func(ct *assert.CollectT) {
					assert.Equal(ct, want, c.CurrentHealth().Message)
				}, 3*time.Second, 20*time.Millisecond)
				for _, line := range logs.at(slog.LevelWarn) {
					require.NotContains(t, line, "supersecret")
				}
			})
		}
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

// TestScrubURLsAtSignInPassword checks user info whose password holds an
// "@". The scrubber must remove all of it, not only the text up to the
// first "@".
func TestScrubURLsAtSignInPassword(t *testing.T) {
	tests := map[string]string{
		`parse "https://user:part@secret@files.example/%zz": bad`: `parse "https://files.example/%zz": bad`,
		`bad URL https://user:part@secret@files.example/x`:        `bad URL https://files.example/x`,
	}
	for in, want := range tests {
		require.Equal(t, want, scrubURLs(in), "input %q", in)
	}

	err := newLibError(reasonParse, errors.New(`jq: error at "https://user:part@secret@files.example/x": bad`))
	require.Equal(t, `jq: error at "https://files.example/x": bad`, err.Error())
	var pe *pollError
	require.ErrorAs(t, err, &pe)
	require.NotContains(t, pe.detail(), "part@secret@")
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

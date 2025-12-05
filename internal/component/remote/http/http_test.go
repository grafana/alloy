package http_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	http_component "github.com/grafana/alloy/internal/component/remote/http"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	ctx := componenttest.TestContext(t)

	var handler lazyHandler
	srv := httptest.NewServer(&handler)
	defer srv.Close()

	handler.SetHandler(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, string(b), "hello there!")
		fmt.Fprintln(w, "Hello, world!")
	})

	ctrl, err := componenttest.NewControllerFromID(util.TestLogger(t), "remote.http")
	require.NoError(t, err)

	cfg := fmt.Sprintf(`
		url = "%s"
		method = "%s"
        headers = {
            "x-custom" = "value",
			"User-Agent" = "custom_useragent",
        }
		body = "%s"

		poll_frequency = "50ms"
		poll_timeout   = "25ms"
	`, srv.URL, http.MethodPut, "hello there!")
	var args http_component.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
	require.NoError(t, ctrl.WaitExports(time.Second), "component never exported anything")

	requireExports := func(expect http_component.Exports) {
		eventually(t, 10*time.Millisecond, 100*time.Millisecond, 5, func() error {
			actual := ctrl.Exports().(http_component.Exports)
			if expect != actual {
				return fmt.Errorf("expected %#v, got %#v", expect, actual)
			}
			return nil
		})
	}

	requireExports(http_component.Exports{
		Content: alloytypes.OptionalSecret{
			IsSecret: false,
			Value:    "Hello, world!",
		},
	})

	// Change the content to ensure new exports get produced.
	handler.SetHandler(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Testing!")
		fmt.Fprintf(w, "Method: %s\n", r.Method)
		fmt.Fprintf(w, "Header: %s\n", r.Header.Get("x-custom"))

		require.Equal(t, "custom_useragent", r.Header.Get("User-Agent"))
	})
	require.NoError(t, ctrl.WaitExports(time.Second), "component didn't update exports")
	requireExports(http_component.Exports{
		Content: alloytypes.OptionalSecret{
			IsSecret: false,
			Value:    "Testing!\nMethod: PUT\nHeader: value",
		},
	})
}

func TestUnmarshalValidation(t *testing.T) {
	var tests = []struct {
		testname      string
		cfg           string
		expectedError string
	}{
		{
			"Missing url",
			`
			poll_frequency = "0"
			`,
			`missing required attribute "url"`,
		},
		{
			"Invalid URL",
			`
			url = "://example.com"
			`,
			`parse "://example.com": missing protocol scheme`,
		},
		{
			"Invalid poll_frequency",
			`
			url = "http://example.com"
			poll_frequency = "0"
			`,
			`poll_frequency must be greater than 0`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			var args http_component.Arguments
			require.EqualError(t, syntax.Unmarshal([]byte(tt.cfg), &args), tt.expectedError)
		})
	}
}

func eventually(t *testing.T, min, max time.Duration, retries int, f func() error) {
	t.Helper()

	l := util.TestLogger(t)

	bo := backoff.New(t.Context(), backoff.Config{
		MinBackoff: min,
		MaxBackoff: max,
		MaxRetries: retries,
	})
	for bo.Ongoing() {
		err := f()
		if err == nil {
			return
		}

		level.Error(l).Log("msg", "condition failed", "err", err)
		bo.Wait()
		continue
	}

	require.NoError(t, bo.Err(), "condition failed")
}

type lazyHandler struct {
	mut   sync.Mutex
	inner http.HandlerFunc
}

func (lh *lazyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	lh.mut.Lock()
	defer lh.mut.Unlock()

	if lh.inner == nil {
		http.NotFound(w, r)
		return
	}
	lh.inner.ServeHTTP(w, r)
}

func (lh *lazyHandler) SetHandler(h http.HandlerFunc) {
	lh.mut.Lock()
	defer lh.mut.Unlock()

	lh.inner = h
}

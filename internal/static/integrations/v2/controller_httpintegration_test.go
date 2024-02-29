package integrations

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/agent/internal/util"
	"github.com/stretchr/testify/require"
)

//
// Tests for controller's utilization of the HTTPIntegration interface.
//

func Test_controller_Handler_Sync(t *testing.T) {
	httpConfigFromID := func(t *testing.T, name, identifier string) Config {
		t.Helper()

		cfg := mockConfigNameTuple(t, name, identifier)
		cfg.NewIntegrationFunc = func(log.Logger, Globals) (Integration, error) {
			i := mockHTTPIntegration{
				Integration: NoOpIntegration,
				HandlerFunc: func(prefix string) (http.Handler, error) {
					return http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
						// We should never reach here since we don't run the integrations.
						rw.WriteHeader(http.StatusBadRequest)
					}), nil
				},
			}
			return i, nil
		}

		return cfg
	}

	cfg := controllerConfig{httpConfigFromID(t, "foo", "bar")}
	ctrl, err := newController(util.TestLogger(t), cfg, Globals{})
	require.NoError(t, err)

	handler, err := ctrl.Handler("/integrations/")
	require.NoError(t, err)

	srv := httptest.NewServer(handler)

	resp, err := srv.Client().Get(srv.URL + "/integrations/foo/bar")
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

// Test_controller_HTTPIntegration_Prefixes ensures that the controller will assign
// appropriate prefixes to HTTPIntegrations.
func Test_controller_HTTPIntegration_Prefixes(t *testing.T) {
	httpConfigFromID := func(t *testing.T, prefixes *[]string, name, identifier string) Config {
		t.Helper()

		cfg := mockConfigNameTuple(t, name, identifier)
		cfg.NewIntegrationFunc = func(log.Logger, Globals) (Integration, error) {
			i := mockHTTPIntegration{
				Integration: NoOpIntegration,
				HandlerFunc: func(prefix string) (http.Handler, error) {
					*prefixes = append(*prefixes, prefix)
					return http.NotFoundHandler(), nil
				},
			}
			return i, nil
		}

		return cfg
	}

	t.Run("fully unique", func(t *testing.T) {
		var prefixes []string

		ctrl, err := newController(
			util.TestLogger(t),
			controllerConfig{
				httpConfigFromID(t, &prefixes, "foo", "bar"),
				httpConfigFromID(t, &prefixes, "fizz", "buzz"),
				httpConfigFromID(t, &prefixes, "hello", "world"),
			},
			Globals{},
		)
		require.NoError(t, err)
		_ = newSyncController(t, ctrl)

		_, err = ctrl.Handler("/integrations/")
		require.NoError(t, err)

		expect := []string{
			"/integrations/foo/",
			"/integrations/fizz/",
			"/integrations/hello/",
		}
		require.ElementsMatch(t, prefixes, expect)
	})

	t.Run("multiple instances", func(t *testing.T) {
		var prefixes []string

		ctrl, err := newController(
			util.TestLogger(t),
			controllerConfig{
				httpConfigFromID(t, &prefixes, "foo", "bar"),
				httpConfigFromID(t, &prefixes, "foo", "buzz"),
				httpConfigFromID(t, &prefixes, "hello", "world"),
			},
			Globals{},
		)
		require.NoError(t, err)
		_ = newSyncController(t, ctrl)

		_, err = ctrl.Handler("/integrations/")
		require.NoError(t, err)

		expect := []string{
			"/integrations/foo/bar/",
			"/integrations/foo/buzz/",
			"/integrations/hello/",
		}
		require.ElementsMatch(t, prefixes, expect)
	})
}

// Test_controller_HTTPIntegration_Routing ensures that the controller will route
// requests to the appropriate integration.
func Test_controller_HTTPIntegration_Routing(t *testing.T) {
	httpConfigFromID := func(t *testing.T, name, identifier string) Config {
		t.Helper()

		cfg := mockConfigNameTuple(t, name, identifier)
		cfg.NewIntegrationFunc = func(log.Logger, Globals) (Integration, error) {
			i := mockHTTPIntegration{
				Integration: NoOpIntegration,
				HandlerFunc: func(prefix string) (http.Handler, error) {
					return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
						fmt.Fprintf(rw, "prefix=%s, path=%s", prefix, r.URL.Path)
					}), nil
				},
			}
			return i, nil
		}

		return cfg
	}

	ctrl, err := newController(
		util.TestLogger(t),
		controllerConfig{
			httpConfigFromID(t, "foo", "bar"),
			httpConfigFromID(t, "foo", "buzz"),
			httpConfigFromID(t, "hello", "world"),
		},
		Globals{},
	)
	require.NoError(t, err)
	_ = newSyncController(t, ctrl)

	handler, err := ctrl.Handler("/integrations/")
	require.NoError(t, err)

	srv := httptest.NewServer(handler)

	getResponse := func(t *testing.T, path string) string {
		t.Helper()
		resp, err := srv.Client().Get(srv.URL + path)
		require.NoError(t, err)
		defer resp.Body.Close()

		var sb strings.Builder
		_, err = io.Copy(&sb, resp.Body)
		require.NoError(t, err)
		return sb.String()
	}

	tt := []struct {
		path, expect string
	}{
		{"/integrations/foo/bar", "prefix=/integrations/foo/bar/, path=/integrations/foo/bar"},
		{"/integrations/foo/bar/", "prefix=/integrations/foo/bar/, path=/integrations/foo/bar/"},
		{"/integrations/foo/bar/extra", "prefix=/integrations/foo/bar/, path=/integrations/foo/bar/extra"},
	}

	for _, tc := range tt {
		require.Equal(t, tc.expect, getResponse(t, tc.path))
	}
}

// Test_controller_HTTPIntegration_NestedRouting ensures that the controller
// will work with nested routers.
func Test_controller_HTTPIntegration_NestedRouting(t *testing.T) {
	cfg := mockConfigNameTuple(t, "test", "test")
	cfg.NewIntegrationFunc = func(log.Logger, Globals) (Integration, error) {
		i := mockHTTPIntegration{
			Integration: NoOpIntegration,
			HandlerFunc: func(prefix string) (http.Handler, error) {
				r := mux.NewRouter()
				r.StrictSlash(true)

				r.HandleFunc(prefix, func(rw http.ResponseWriter, r *http.Request) {
					fmt.Fprintf(rw, "prefix=%s, path=%s", prefix, r.URL.Path)
				})

				r.HandleFunc(prefix+"greet", func(rw http.ResponseWriter, _ *http.Request) {
					fmt.Fprintf(rw, "Hello, world!")
				})
				return r, nil
			},
		}
		return i, nil
	}

	ctrl, err := newController(util.TestLogger(t), controllerConfig{cfg}, Globals{})
	require.NoError(t, err)
	_ = newSyncController(t, ctrl)

	handler, err := ctrl.Handler("/integrations/")
	require.NoError(t, err)

	srv := httptest.NewServer(handler)

	getResponse := func(t *testing.T, path string) string {
		t.Helper()
		resp, err := srv.Client().Get(srv.URL + path)
		require.NoError(t, err)
		defer resp.Body.Close()

		var sb strings.Builder
		_, err = io.Copy(&sb, resp.Body)
		require.NoError(t, err)
		return sb.String()
	}

	tt := []struct {
		path, expect string
	}{
		{"/integrations/test", "prefix=/integrations/test/, path=/integrations/test/"},
		{"/integrations/test/", "prefix=/integrations/test/, path=/integrations/test/"},
		{"/integrations/test/greet", "Hello, world!"},
	}

	for _, tc := range tt {
		require.Equal(t, tc.expect, getResponse(t, tc.path))
	}
}

type mockHTTPIntegration struct {
	Integration
	HandlerFunc func(prefix string) (http.Handler, error)
}

func (m mockHTTPIntegration) Handler(prefix string) (http.Handler, error) {
	return m.HandlerFunc(prefix)
}

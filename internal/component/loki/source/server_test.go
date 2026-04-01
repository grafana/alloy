package source

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/util"
)

func TestServer(t *testing.T) {
	t.Run("forward entries", func(t *testing.T) {
		recv := loki.NewCollectingBatchReceiver()
		defer recv.Stop()

		srv := newTestServer(
			t,
			recv,
			testServerConfig(time.Second, &LogsConfig{}),
			newTestLogsRoute(func(_ *http.Request, _ *LogsConfig) ([]loki.Entry, int, error) {
				return []loki.Entry{loki.NewEntry(model.LabelSet{"source": "test"}, push.Entry{Line: "hello"})},
					http.StatusAccepted,
					nil
			}),
		)
		defer srv.ForceShutdown()

		resp := doPost(t, srv)
		require.NoError(t, resp.Body.Close())
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		assertReceivedLogs(t, recv, []loki.Entry{
			loki.NewEntry(model.LabelSet{"source": "test"}, push.Entry{Line: "hello"}),
		})
	})

	t.Run("returns error when no entries are produced", func(t *testing.T) {
		recv := loki.NewCollectingBatchReceiver()
		defer recv.Stop()

		srv := newTestServer(
			t,
			recv,
			testServerConfig(time.Second, &LogsConfig{}),
			newTestLogsRoute(func(_ *http.Request, _ *LogsConfig) ([]loki.Entry, int, error) {
				return nil, http.StatusBadRequest, errors.New("bad request")
			}),
		)
		defer srv.ForceShutdown()

		resp := doPost(t, srv)
		require.NoError(t, resp.Body.Close())

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assertReceivedLogs(t, recv, nil)
	})

	t.Run("forwards entries and report partial failure", func(t *testing.T) {
		recv := loki.NewCollectingBatchReceiver()
		defer recv.Stop()

		srv := newTestServer(
			t,
			recv,
			testServerConfig(time.Second, &LogsConfig{}),
			newTestLogsRoute(func(_ *http.Request, _ *LogsConfig) ([]loki.Entry, int, error) {
				return []loki.Entry{loki.NewEntry(model.LabelSet{"source": "test"}, push.Entry{Line: "partial"})},
					http.StatusUnprocessableEntity,
					errors.New("partial failure")
			}),
		)
		defer srv.ForceShutdown()

		resp := doPost(t, srv)
		require.NoError(t, resp.Body.Close())

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

		assertReceivedLogs(t, recv, []loki.Entry{
			loki.NewEntry(model.LabelSet{"source": "test"}, push.Entry{Line: "partial"}),
		})
	})

	t.Run("handles canceled requests", func(t *testing.T) {
		recv := newTestLogsBatchReceiver(1)
		srv := newTestServer(
			t,
			recv,
			testServerConfig(time.Second, &LogsConfig{}),
			newTestLogsRoute(func(_ *http.Request, _ *LogsConfig) ([]loki.Entry, int, error) {
				return []loki.Entry{loki.NewEntry(model.LabelSet{"source": "test"}, push.Entry{Line: "hello"})},
					http.StatusNoContent,
					nil
			}),
		)
		defer srv.ForceShutdown()

		resp, err := doPostWithContext(t, context.Background(), srv)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())

		const numRequest = 5

		var (
			wg   sync.WaitGroup
			errs = make(chan error, numRequest)
		)

		for range numRequest {
			wg.Go(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
				defer cancel()

				_, err := doPostWithContext(t, ctx, srv)
				if !errors.Is(err, context.DeadlineExceeded) {
					errs <- fmt.Errorf("expected context deadline exceeded, got %v", err)
					return
				}
			})
		}

		wg.Wait()

		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
	})
}

func TestServer_Update(t *testing.T) {
	t.Run("update config", func(t *testing.T) {
		recv := loki.NewCollectingBatchReceiver()
		defer recv.Stop()

		srv := newTestServer(
			t,
			recv,
			testServerConfig(time.Second, &LogsConfig{FixedLabels: model.LabelSet{"version": "before"}}),
			newTestLogsRoute(func(_ *http.Request, cfg *LogsConfig) ([]loki.Entry, int, error) {
				return []loki.Entry{loki.NewEntry(cfg.FixedLabels.Clone(), push.Entry{Line: "hello"})}, http.StatusNoContent, nil
			}),
		)
		defer srv.Shutdown()

		resp := doPost(t, srv)
		require.NoError(t, resp.Body.Close())
		assertReceivedLogs(t, recv, []loki.Entry{
			loki.NewEntry(model.LabelSet{"version": "before"}, push.Entry{Line: "hello"}),
		})

		srv.Update(&LogsConfig{
			FixedLabels: model.LabelSet{"version": "after"},
		})
		recv.Clear()

		resp = doPost(t, srv)
		resp.Body.Close()
		assertReceivedLogs(t, recv, []loki.Entry{
			loki.NewEntry(model.LabelSet{"version": "after"}, push.Entry{Line: "hello"}),
		})
	})

	t.Run("needs restart when server config changed", func(t *testing.T) {
		recv := loki.NewCollectingBatchReceiver()
		defer recv.Stop()

		srv := newTestServer(
			t,
			recv,
			testServerConfig(time.Second, &LogsConfig{}),
		)
		defer srv.ForceShutdown()

		assert.False(t, srv.NeedsRestart(srv.netConfig))

		restartedConfig := testServerConfig(time.Second, &LogsConfig{})
		restartedConfig.NetConfig.HTTP.ListenPort = 12345
		assert.True(t, srv.NeedsRestart(restartedConfig.NetConfig))
	})
}

func TestServer_Shutdown(t *testing.T) {
	t.Run("shutdown rejects blocked requests after graceful timeout", func(t *testing.T) {
		requestReceived := make(chan struct{})
		srv := newTestServer(
			t,
			loki.NewLogsBatchReceiver(),
			testServerConfig(25*time.Millisecond, &LogsConfig{}),
			newTestLogsRoute(func(_ *http.Request, _ *LogsConfig) ([]loki.Entry, int, error) {
				close(requestReceived)
				return []loki.Entry{loki.NewEntry(model.LabelSet{}, push.Entry{Line: "blocked"})}, http.StatusNoContent, nil
			}),
		)

		status := make(chan int, 1)
		errs := make(chan error, 1)

		go func() {
			resp, err := http.Post(fmt.Sprintf("http://%s/logs", srv.HTTPAddr()), "text/plain", strings.NewReader("test"))
			if err != nil {
				errs <- err
				return
			}
			defer resp.Body.Close()
			status <- resp.StatusCode
		}()

		select {
		case <-requestReceived:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for request to reach logs route")
		}

		srv.Shutdown()

		select {
		case err := <-errs:
			require.NoError(t, err)
		case code := <-status:
			assert.Equal(t, http.StatusServiceUnavailable, code)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for blocked request to finish")
		}
	})

	t.Run("force shutdown cancels blocked requests", func(t *testing.T) {
		requestReceived := make(chan struct{})
		srv := newTestServer(
			t,
			loki.NewLogsBatchReceiver(),
			testServerConfig(time.Second, &LogsConfig{}),
			newTestLogsRoute(func(_ *http.Request, _ *LogsConfig) ([]loki.Entry, int, error) {
				close(requestReceived)
				return []loki.Entry{loki.NewEntry(model.LabelSet{}, push.Entry{Line: "blocked"})}, http.StatusNoContent, nil
			}),
		)

		status := make(chan int, 1)
		errs := make(chan error, 1)
		go func() {
			resp, err := http.Post(fmt.Sprintf("http://%s/logs", srv.HTTPAddr()), "text/plain", strings.NewReader("test"))
			if err != nil {
				errs <- err
				return
			}
			defer resp.Body.Close()
			status <- resp.StatusCode
		}()

		select {
		case <-requestReceived:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for request to reach logs route")
		}

		srv.ForceShutdown()

		select {
		case err := <-errs:
			require.NoError(t, err)
		case code := <-status:
			assert.Equal(t, http.StatusServiceUnavailable, code)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for blocked request to finish")
		}
	})
}

func newTestLogsRoute(logsFn func(r *http.Request, opts *LogsConfig) ([]loki.Entry, int, error)) *testLogsRoute {
	return &testLogsRoute{
		logsFn: logsFn,
	}
}

type testLogsRoute struct {
	logsFn func(r *http.Request, opts *LogsConfig) ([]loki.Entry, int, error)
}

func (r testLogsRoute) Path() string {
	return "/logs"
}

func (r testLogsRoute) Method() string {
	return http.MethodPost
}

func (r testLogsRoute) Logs(req *http.Request, opts *LogsConfig) ([]loki.Entry, int, error) {
	return r.logsFn(req, opts)
}

type testHandlerRoute struct {
	path    string
	method  string
	handler http.Handler
}

func (r testHandlerRoute) Path() string {
	return r.path
}

func (r testHandlerRoute) Method() string {
	if r.method == "" {
		return http.MethodGet
	}
	return r.method
}

func (r testHandlerRoute) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.handler.ServeHTTP(w, req)
}

func newTestServer(t *testing.T, recv loki.LogsBatchReceiver, cfg ServerConfig, logsRoutes ...LogsRoute) *Server {
	t.Helper()

	srv, err := NewServer(util.TestLogger(t), prometheus.NewRegistry(), recv, cfg)
	require.NoError(t, err)

	err = srv.Run(logsRoutes, []HandlerRoute{testHandlerRoute{
		path: "/ready",
		handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		resp, err := http.Get(fmt.Sprintf("http://%s/ready", srv.HTTPAddr()))
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, time.Second, 10*time.Millisecond)

	return srv
}

func testServerConfig(timeout time.Duration, logsConfig *LogsConfig) ServerConfig {
	return ServerConfig{
		Namespace:  "test",
		LogsConfig: logsConfig,
		NetConfig: &fnet.ServerConfig{
			HTTP: &fnet.HTTPConfig{
				ListenAddress: "127.0.0.1",
				ListenPort:    0,
			},
			GRPC: &fnet.GRPCConfig{
				ListenAddress: "127.0.0.1",
				ListenPort:    0,
			},
			GracefulShutdownTimeout: timeout,
		},
	}
}

func doPost(t *testing.T, srv *Server) *http.Response {
	t.Helper()

	resp, err := doPostWithContext(t, t.Context(), srv)
	require.NoError(t, err)
	return resp
}

func doPostWithContext(t *testing.T, ctx context.Context, srv *Server) (*http.Response, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://%s/logs", srv.HTTPAddr()), nil)
	require.NoError(t, err)

	return http.DefaultClient.Do(req)
}

func assertReceivedLogs(t *testing.T, recv *loki.CollectingBatchReceiver, want []loki.Entry) {
	t.Helper()

	require.Eventually(t, func() bool {
		return len(recv.Received()) == len(want)
	}, time.Second, 10*time.Millisecond)

	got := recv.Received()
	require.Len(t, got, len(want))

	for i := range want {
		assert.Equal(t, want[i].Line, got[i].Line)
		assert.Equal(t, want[i].Labels, got[i].Labels)
	}
}

type testLogsBatchReceiver struct {
	ch chan []loki.Entry
}

func newTestLogsBatchReceiver(size int) *testLogsBatchReceiver {
	return &testLogsBatchReceiver{ch: make(chan []loki.Entry, size)}
}

func (r *testLogsBatchReceiver) Chan() chan []loki.Entry {
	return r.ch
}

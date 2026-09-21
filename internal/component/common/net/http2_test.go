package net

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"

	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func newHTTP2TestServer(t *testing.T, cfg *ServerConfig, handler http.Handler) *TargetServer {
	t.Helper()
	cfg.HTTP.ListenAddress = "127.0.0.1"
	cfg.HTTP.ListenPort = 0
	cfg.GRPC.ListenAddress = "127.0.0.1"
	cfg.GRPC.ListenPort = 0
	ts, err := NewTargetServer(util.TestAlloyLogger(t).Slog(), "test_http2", prometheus.NewRegistry(), cfg)
	require.NoError(t, err)
	require.NoError(t, ts.MountAndRun(func(router *mux.Router) {
		router.Path("/test").Handler(handler)
	}))
	t.Cleanup(ts.StopAndShutdown)
	return ts
}

func TestTargetServer_Protocols(t *testing.T) {
	for _, useTLS := range []bool{false, true} {
		for _, mode := range []string{"absent", "disabled", "enabled"} {
			t.Run(mode+map[bool]string{false: "/plaintext", true: "/tls"}[useTLS], func(t *testing.T) {
				cfg := DefaultServerConfig()
				cfg.HTTP.HTTP2.Enabled = mode == "enabled"
				if mode == "absent" {
					cfg.HTTP.HTTP2 = nil
				}
				var clientTLS *tls.Config
				if useTLS {
					// Reuse httptest's trusted loopback certificate for the dskit server.
					fixture := httptest.NewTLSServer(http.NotFoundHandler())
					t.Cleanup(fixture.Close)
					cert := fixture.TLS.Certificates[0]
					key, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
					require.NoError(t, err)
					cfg.HTTP.TLSConfig = &TLSConfig{
						Cert: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})),
						Key:  alloytypes.Secret(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})),
					}
					clientTLS = fixture.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
				}
				ts := newHTTP2TestServer(t, cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("X-Protocol", r.Proto)
					_, _ = io.Copy(w, r.Body)
				}))
				scheme := "http://"
				if useTLS {
					scheme = "https://"
				}
				for _, major := range []int{1, 2} {
					protocols := new(http.Protocols)
					protocols.SetHTTP1(major == 1)
					protocols.SetHTTP2(major == 2 && useTLS)
					protocols.SetUnencryptedHTTP2(major == 2 && !useTLS)
					transport := &http.Transport{Protocols: protocols, TLSClientConfig: clientTLS}
					t.Cleanup(transport.CloseIdleConnections)
					client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
					res, err := client.Post(scheme+ts.HTTPListenAddr()+"/test", "text/plain", strings.NewReader("payload"))
					if major == 2 && !useTLS && mode != "enabled" {
						require.Error(t, err)
						continue
					}
					require.NoError(t, err)
					body, err := io.ReadAll(res.Body)
					require.NoError(t, res.Body.Close())
					require.NoError(t, err)
					require.Equal(t, http.StatusOK, res.StatusCode)
					require.Equal(t, major, res.ProtoMajor)
					require.Equal(t, res.Proto, res.Header.Get("X-Protocol"))
					require.Equal(t, "payload", string(body))
					if major == 1 && !useTLS && mode == "enabled" {
						// Native HTTP/2 doesn't support the legacy Upgrade handshake.
						req, err := http.NewRequest(http.MethodPost, scheme+ts.HTTPListenAddr()+"/test", strings.NewReader("upgrade payload"))
						require.NoError(t, err)
						req.Header.Set("Connection", "Upgrade, HTTP2-Settings")
						req.Header.Set("Upgrade", "h2c")
						req.Header.Set("HTTP2-Settings", "")
						res, err := client.Do(req)
						require.NoError(t, err)
						body, err := io.ReadAll(res.Body)
						require.NoError(t, res.Body.Close())
						require.NoError(t, err)
						require.Equal(t, http.StatusOK, res.StatusCode)
						require.Equal(t, "HTTP/1.1", res.Header.Get("X-Protocol"))
						require.Equal(t, "upgrade payload", string(body))
					}
				}
			})
		}
	}
}

func TestTargetServer_HTTP2MaxHandlersWarning(t *testing.T) {
	for _, tc := range []struct {
		name         string
		enabled      bool
		maxHandlers  int
		wantWarnings int
	}{
		{"default", true, 0, 0},
		{"nonzero", true, 42, 1},
		{"negative", true, -1, 1},
		{"disabled", false, 42, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			warnings := make(chan struct{}, 10)
			logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
				Level: slog.LevelWarn,
				ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
					if attr.Key == slog.MessageKey && attr.Value.String() == "http2 max_handlers is deprecated and has no effect; remove it from the configuration" {
						warnings <- struct{}{}
					}
					return attr
				},
			}))
			cfg := DefaultServerConfig()
			cfg.HTTP.ListenAddress = "127.0.0.1"
			cfg.HTTP.ListenPort = 0
			cfg.GRPC.ListenAddress = "127.0.0.1"
			cfg.HTTP.HTTP2.Enabled = tc.enabled
			cfg.HTTP.HTTP2.MaxHandlers = tc.maxHandlers
			ts, err := NewTargetServer(logger, "test_http2", prometheus.NewRegistry(), cfg)
			require.NoError(t, err)
			require.NoError(t, ts.MountAndRun(func(_ *mux.Router) {}))
			t.Cleanup(ts.StopAndShutdown)
			require.Len(t, warnings, tc.wantWarnings)
		})
	}
}

func TestTargetServer_HTTP2IdleTimeout(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
		idle    time.Duration
		want    time.Duration
	}{
		{"inherit", true, 0, 2 * time.Minute},
		{"override", true, time.Minute, time.Minute},
		{"no timeout", true, -1, -1},
		{"disabled", false, time.Minute, 2 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultServerConfig()
			cfg.HTTP.HTTP2.Enabled = tc.enabled
			cfg.HTTP.HTTP2.IdleTimeout = tc.idle
			ts := newHTTP2TestServer(t, cfg, http.NotFoundHandler())
			require.Equal(t, tc.want, ts.server.HTTPServer.IdleTimeout)
		})
	}
}

func TestTargetServer_HTTP2Frames(t *testing.T) {
	for _, mode := range []string{"ping", "idle", "shutdown"} {
		t.Run(mode, func(t *testing.T) {
			cfg := DefaultServerConfig()
			cfg.HTTP.HTTP2.Enabled = true
			cfg.HTTP.HTTP2.MaxConcurrentStreams = 7
			cfg.HTTP.HTTP2.MaxReadFrameSize = 32768
			cfg.HTTP.HTTP2.MaxUploadBufferPerStream = 524288
			if mode == "ping" {
				cfg.HTTP.HTTP2.ReadIdleTimeout = 50 * time.Millisecond
				cfg.HTTP.HTTP2.PingTimeout = 50 * time.Millisecond
			}
			if mode == "idle" {
				cfg.HTTP.ServerIdleTimeout = 50 * time.Millisecond
			}
			ts := newHTTP2TestServer(t, cfg, http.NotFoundHandler())
			conn, err := net.DialTimeout("tcp", ts.HTTPListenAddr(), 5*time.Second)
			require.NoError(t, err)
			t.Cleanup(func() { _ = conn.Close() })
			require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
			_, err = io.WriteString(conn, http2.ClientPreface)
			require.NoError(t, err)
			framer := http2.NewFramer(conn, conn)
			require.NoError(t, framer.WriteSettings())
			frame, err := framer.ReadFrame()
			require.NoError(t, err)
			settings, ok := frame.(*http2.SettingsFrame)
			require.True(t, ok)
			for id, want := range map[http2.SettingID]uint32{
				http2.SettingMaxConcurrentStreams: 7,
				http2.SettingMaxFrameSize:         32768,
				http2.SettingInitialWindowSize:    524288,
			} {
				got, ok := settings.Value(id)
				require.True(t, ok)
				require.Equal(t, want, got)
			}
			require.NoError(t, framer.WriteSettingsAck())
			var shutdownDone chan error
			if mode == "shutdown" {
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				shutdownDone = make(chan error, 1)
				go func() { shutdownDone <- ts.server.HTTPServer.Shutdown(ctx) }()
			}
			for {
				frame, err := framer.ReadFrame()
				require.NoError(t, err)
				if ping, ok := frame.(*http2.PingFrame); ok && mode == "ping" {
					require.False(t, ping.IsAck())
					// An unanswered health check must close the connection.
					for err == nil {
						_, err = framer.ReadFrame()
					}
					var netErr net.Error
					require.False(t, errors.As(err, &netErr) && netErr.Timeout(), "connection did not close before the test deadline")
					break
				}
				if goAway, ok := frame.(*http2.GoAwayFrame); ok {
					require.NotEqual(t, "ping", mode)
					require.Equal(t, http2.ErrCodeNo, goAway.ErrCode)
					break
				}
			}
			if shutdownDone != nil {
				require.NoError(t, conn.Close())
				require.NoError(t, <-shutdownDone)
			}
		})
	}
}

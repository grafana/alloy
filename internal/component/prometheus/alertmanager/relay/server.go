package relay

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/grafana/alloy/internal/component"
)

const gracefulShutdownTimeout = 5 * time.Second

func (c *Component) runServer(ctx context.Context, cfg serverConfig) error {
	addr := net.JoinHostPort(cfg.listenAddress, strconv.Itoa(cfg.listenPort))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != cfg.webhookPath {
			http.NotFound(w, r)
			return
		}
		c.handleWebhook(w, r)
	})

	writeTimeout := cfg.requestTimeout + 5*time.Second
	if writeTimeout < 30*time.Second {
		writeTimeout = 30 * time.Second
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(listener)
	}()

	c.opts.Logger.Info("starting Alertmanager webhook relay", "addr", listener.Addr().String(), "path", cfg.webhookPath)
	c.setHealth(component.HealthTypeHealthy, "component is ready to receive Alertmanager webhooks")

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return fmt.Errorf("gracefully shutting down webhook server: %w", err)
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

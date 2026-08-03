//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"syscall"
)

func (c *Component) Handler() http.Handler {
	return http.HandlerFunc(c.serveHTTP)
}

// serveHTTP reverse-proxies a request to the Beyla subprocess, routing /debug/pprof
// to the pprof port when enabled and the rest to the main subprocess port.
func (c *Component) serveHTTP(w http.ResponseWriter, r *http.Request) {
	addr, profilePort, ready := c.subprocess.ProxyTarget()

	if addr == "" {
		http.Error(w, "Beyla subprocess not started", http.StatusServiceUnavailable)
		return
	}

	targetAddr, ok := resolveTargetAddr(addr, profilePort, r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	target, err := url.Parse(targetAddr)
	if err != nil {
		c.opts.Logger.Error("failed to parse subprocess URL", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if errors.Is(err, context.Canceled) {
			c.opts.Logger.Debug("proxy request cancelled", "err", err)
			return
		}
		if !ready {
			c.opts.Logger.Debug("proxy error (subprocess initializing)", "err", err)
			w.Header().Set("Content-Type", "text/plain; version=0.0.4")
			w.WriteHeader(http.StatusOK)
			return
		}
		if isSubprocessGoneErr(err) {
			c.opts.Logger.Warn("subprocess connection unavailable", "err", err)
		} else {
			c.opts.Logger.Error("proxy error", "err", err)
		}
		http.Error(w, "subprocess unavailable", http.StatusBadGateway)
	}

	proxy.ServeHTTP(w, r)
}

func isSubprocessGoneErr(err error) bool {
	return errors.Is(err, io.EOF) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EPIPE)
}

func resolveTargetAddr(addr string, profilePort int, path string) (targetAddr string, ok bool) {
	if strings.HasPrefix(path, "/debug/pprof") {
		if profilePort == 0 {
			return "", false
		}
		return fmt.Sprintf("http://127.0.0.1:%d", profilePort), true
	}

	return addr, true
}

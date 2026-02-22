package alloycli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthcheckRun_Ready(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/-/ready", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Alloy is ready.")
	}))
	defer srv.Close()

	h := &alloyHealthcheck{
		url:     srv.URL + "/-/ready",
		timeout: 5 * time.Second,
	}
	err := h.Run()
	require.NoError(t, err)
}

func TestHealthcheckRun_NotReady(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(w, "Alloy is not ready.")
	}))
	defer srv.Close()

	h := &alloyHealthcheck{
		url:     srv.URL + "/-/ready",
		timeout: 5 * time.Second,
	}
	err := h.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestHealthcheckRun_UnhealthyComponents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/-/healthy", r.URL.Path)
		http.Error(w, "unhealthy components: comp1, comp2", http.StatusInternalServerError)
	}))
	defer srv.Close()

	h := &alloyHealthcheck{
		url:     srv.URL + "/-/healthy",
		timeout: 5 * time.Second,
	}
	err := h.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestHealthcheckRun_ConnectionRefused(t *testing.T) {
	h := &alloyHealthcheck{
		addr:    "127.0.0.1:0",
		path:    "/-/ready",
		timeout: 1 * time.Second,
	}
	err := h.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request failed")
}

func TestHealthcheckRun_URLOverridesAddr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	}))
	defer srv.Close()

	h := &alloyHealthcheck{
		url:     srv.URL + "/-/ready",
		addr:    "should-not-be-used:9999",
		path:    "/should-not-be-used",
		timeout: 5 * time.Second,
	}
	err := h.Run()
	require.NoError(t, err)
}

func TestHealthcheckRun_URLWithoutScheme(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/-/ready", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Alloy is ready.")
	}))
	defer srv.Close()

	// Strip the "http://" prefix to simulate a user passing a URL without scheme.
	urlWithoutScheme := strings.TrimPrefix(srv.URL, "http://") + "/-/ready"

	h := &alloyHealthcheck{
		url:     urlWithoutScheme,
		timeout: 5 * time.Second,
	}
	err := h.Run()
	require.NoError(t, err)
}

func TestHealthcheckRun_AddrAndPathConstruction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/-/healthy", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "All Alloy components are healthy.")
	}))
	defer srv.Close()

	h := &alloyHealthcheck{
		addr:    srv.Listener.Addr().String(),
		path:    "/-/healthy",
		timeout: 5 * time.Second,
	}
	err := h.Run()
	require.NoError(t, err)
}

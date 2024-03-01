package instance

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/dskit/backoff"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

var slowBackoff = backoff.Config{
	MinBackoff: 1 * time.Second,
	MaxBackoff: 1 * time.Minute,
	MaxRetries: 10,
}

// TestInstance_Update performs a full integration test by doing the following:
//
//  1. Launching an HTTP server which can be scraped and also mocks the remote_write
//     endpoint.
//  2. Creating an instance config with no scrape_configs or remote_write configs.
//  3. Updates the instance with a scrape_config and remote_write.
//  4. Validates that after 15 seconds, the scrape endpoint and remote_write
//     endpoint has been called.
func TestInstance_Update(t *testing.T) {
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))

	walDir := t.TempDir()

	var (
		scraped = atomic.NewBool(false)
		pushed  = atomic.NewBool(false)
	)

	r := mux.NewRouter()
	r.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		scraped.Store(true)
		promhttp.Handler().ServeHTTP(w, r)
	})
	r.HandleFunc("/push", func(w http.ResponseWriter, r *http.Request) {
		pushed.Store(true)
		// We don't particularly care what was pushed to us, so we'll ignore
		// everything here; we just want to make sure the endpoint was invoked.
	})

	// Start a server for exposing the router.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	go func() {
		_ = http.Serve(l, r)
	}()

	// Create a new instance where it's not scraping or writing anything by default.
	initialConfig := loadConfig(t, `
name: integration_test
scrape_configs: []
remote_write: []
`)
	inst, err := New(prometheus.NewRegistry(), initialConfig, walDir, logger)
	require.NoError(t, err)

	instCtx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := inst.Run(instCtx)
		require.NoError(t, err)
	}()

	// Update the config with a single scrape_config and remote_write.
	newConfig := loadConfig(t, fmt.Sprintf(`
name: integration_test
scrape_configs:
  - job_name: test_scrape
    scrape_interval: 5s
    static_configs:
      - targets: ['%[1]s']
remote_write:
  - url: http://%[1]s/push
`, l.Addr()))

	// Wait for the instance to be ready before updating.
	util.EventuallyWithBackoff(t, func(t require.TestingT) {
		require.True(t, inst.Ready())
	}, slowBackoff)

	// Wait for the instance to update (it might not be ready yet and would
	// return an error until everything is initialized), and then wait again for
	// the configs to apply and set the scraped and pushed atomic variables,
	// indicating that the Prometheus components successfully updated.
	util.EventuallyWithBackoff(t, func(t require.TestingT) {
		err := inst.Update(newConfig)
		if err != nil {
			logger.Log("msg", "failed to update instance", "err", err)
		}
		require.NoError(t, err)
	}, slowBackoff)

	util.EventuallyWithBackoff(t, func(t require.TestingT) {
		require.True(t, scraped.Load() && pushed.Load())
	}, slowBackoff)
}

func TestInstance_Update_Failed(t *testing.T) {
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))

	walDir := t.TempDir()

	r := mux.NewRouter()
	r.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		promhttp.Handler().ServeHTTP(w, r)
	})
	r.HandleFunc("/push", func(w http.ResponseWriter, r *http.Request) {})

	// Start a server for exposing the router.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	go func() {
		_ = http.Serve(l, r)
	}()

	// Create a new instance where it's not scraping or writing anything by default.
	initialConfig := loadConfig(t, `
name: integration_test
scrape_configs: []
remote_write: []
`)
	inst, err := New(prometheus.NewRegistry(), initialConfig, walDir, logger)
	require.NoError(t, err)

	instCtx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := inst.Run(instCtx)
		require.NoError(t, err)
	}()

	// Create a new config to use for updating
	newConfig := loadConfig(t, fmt.Sprintf(`
name: integration_test
scrape_configs:
  - job_name: test_scrape
    scrape_interval: 5s
    static_configs:
      - targets: ['%[1]s']
remote_write:
  - url: http://%[1]s/push
`, l.Addr()))

	// Make sure the instance can successfully update first
	util.Eventually(t, func(t require.TestingT) {
		err := inst.Update(newConfig)
		if err != nil {
			logger.Log("msg", "failed to update instance", "err", err)
		}
		require.NoError(t, err)
	})

	// Now force an update back to the original config to fail
	inst.readyScrapeManager.Set(nil)
	require.NotNil(t, inst.Update(initialConfig), "update should have failed")
	require.Equal(t, newConfig, inst.cfg, "config did not roll back")
}

// TestInstance_Update_InvalidChanges runs an instance with a blank initial
// config and performs various unacceptable updates that should return an
// error.
func TestInstance_Update_InvalidChanges(t *testing.T) {
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))

	walDir := t.TempDir()

	// Create a new instance where it's not scraping or writing anything by default.
	initialConfig := loadConfig(t, `
name: integration_test
scrape_configs: []
remote_write: []
`)
	inst, err := New(prometheus.NewRegistry(), initialConfig, walDir, logger)
	require.NoError(t, err)

	instCtx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := inst.Run(instCtx)
		require.NoError(t, err)
	}()

	// Do a no-op update that succeeds to ensure that the instance is running.
	util.Eventually(t, func(t require.TestingT) {
		err := inst.Update(initialConfig)
		if err != nil {
			logger.Log("msg", "failed to update instance", "err", err)
		}
		require.NoError(t, err)
	})

	tt := []struct {
		name   string
		mut    func(c *Config)
		expect string
	}{
		{
			name:   "name changed",
			mut:    func(c *Config) { c.Name = "changed name" },
			expect: "name cannot be changed dynamically",
		},
		{
			name:   "host_filter changed",
			mut:    func(c *Config) { c.HostFilter = true },
			expect: "host_filter cannot be changed dynamically",
		},
		{
			name:   "wal_truncate_frequency changed",
			mut:    func(c *Config) { c.WALTruncateFrequency *= 2 },
			expect: "wal_truncate_frequency cannot be changed dynamically",
		},
		{
			name:   "remote_flush_deadline changed",
			mut:    func(c *Config) { c.RemoteFlushDeadline *= 2 },
			expect: "remote_flush_deadline cannot be changed dynamically",
		},
		{
			name:   "write_stale_on_shutdown changed",
			mut:    func(c *Config) { c.WriteStaleOnShutdown = true },
			expect: "write_stale_on_shutdown cannot be changed dynamically",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			mutatedConfig := initialConfig
			tc.mut(&mutatedConfig)

			err := inst.Update(mutatedConfig)
			require.EqualError(t, err, tc.expect)
		})
	}
}

func loadConfig(t *testing.T, s string) Config {
	cfg, err := UnmarshalConfig(strings.NewReader(s))
	require.NoError(t, err)
	require.NoError(t, cfg.ApplyDefaults(DefaultGlobalConfig))
	return *cfg
}

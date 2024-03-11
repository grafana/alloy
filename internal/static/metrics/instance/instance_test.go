package instance

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/agent/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
)

func TestConfig_Unmarshal_Defaults(t *testing.T) {
	global := DefaultGlobalConfig
	cfgText := `name: test
scrape_configs:
  - job_name: local_scrape
    static_configs:
      - targets: ['127.0.0.1:12345']
        labels:
          cluster: 'localhost'
remote_write:
  - url: http://localhost:9009/api/prom/push`

	cfg, err := UnmarshalConfig(strings.NewReader(cfgText))
	require.NoError(t, err)

	err = cfg.ApplyDefaults(global)
	require.NoError(t, err)

	require.Equal(t, DefaultConfig.HostFilter, cfg.HostFilter)
	require.Equal(t, DefaultConfig.WALTruncateFrequency, cfg.WALTruncateFrequency)
	require.Equal(t, DefaultConfig.RemoteFlushDeadline, cfg.RemoteFlushDeadline)
	require.Equal(t, DefaultConfig.WriteStaleOnShutdown, cfg.WriteStaleOnShutdown)

	for _, sc := range cfg.ScrapeConfigs {
		require.Equal(t, sc.ScrapeInterval, global.Prometheus.ScrapeInterval)
		require.Equal(t, sc.ScrapeTimeout, global.Prometheus.ScrapeTimeout)
	}
}

func TestConfig_ApplyDefaults_Validations(t *testing.T) {
	global := DefaultGlobalConfig
	cfg := DefaultConfig
	cfg.Name = "instance"
	cfg.ScrapeConfigs = []*config.ScrapeConfig{{
		JobName: "scrape",
		ServiceDiscoveryConfigs: discovery.Configs{
			discovery.StaticConfig{{
				Targets: []model.LabelSet{{
					model.AddressLabel: model.LabelValue("127.0.0.1:12345"),
				}},
				Labels: model.LabelSet{"cluster": "localhost"},
			}},
		},
	}}
	cfg.RemoteWrite = []*config.RemoteWriteConfig{{Name: "write"}}

	tt := []struct {
		name     string
		mutation func(c *Config)
		err      error
	}{
		{
			"valid config",
			nil,
			nil,
		},
		{
			"requires name",
			func(c *Config) { c.Name = "" },
			fmt.Errorf("missing instance name"),
		},
		{
			"missing scrape",
			func(c *Config) { c.ScrapeConfigs[0] = nil },
			fmt.Errorf("empty or null scrape config section"),
		},
		{
			"missing wal truncate frequency",
			func(c *Config) { c.WALTruncateFrequency = 0 },
			fmt.Errorf("wal_truncate_frequency must be greater than 0s"),
		},
		{
			"missing remote flush deadline",
			func(c *Config) { c.RemoteFlushDeadline = 0 },
			fmt.Errorf("remote_flush_deadline must be greater than 0s"),
		},
		{
			"scrape timeout too high",
			func(c *Config) { c.ScrapeConfigs[0].ScrapeTimeout = global.Prometheus.ScrapeInterval + 1 },
			fmt.Errorf("scrape timeout greater than scrape interval for scrape config with job name \"scrape\""),
		},
		{
			"scrape interval greater than truncate frequency",
			func(c *Config) { c.ScrapeConfigs[0].ScrapeInterval = model.Duration(c.WALTruncateFrequency + 1) },
			fmt.Errorf("scrape interval greater than wal_truncate_frequency for scrape config with job name \"scrape\""),
		},
		{
			"multiple scrape configs with same name",
			func(c *Config) {
				c.ScrapeConfigs = append(c.ScrapeConfigs, &config.ScrapeConfig{
					JobName: "scrape",
				})
			},
			fmt.Errorf("found multiple scrape configs with job name \"scrape\""),
		},
		{
			"empty remote write",
			func(c *Config) { c.RemoteWrite = append(c.RemoteWrite, nil) },
			fmt.Errorf("empty or null remote write config section"),
		},
		{
			"multiple remote writes with same name",
			func(c *Config) {
				c.RemoteWrite = []*config.RemoteWriteConfig{
					{Name: "foo"},
					{Name: "foo"},
				}
			},
			fmt.Errorf("found duplicate remote write configs with name \"foo\""),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Copy the input and all of its slices
			input := cfg

			var scrapeConfigs []*config.ScrapeConfig
			for _, sc := range input.ScrapeConfigs {
				scCopy := *sc
				scrapeConfigs = append(scrapeConfigs, &scCopy)
			}
			input.ScrapeConfigs = scrapeConfigs

			var remoteWrites []*config.RemoteWriteConfig
			for _, rw := range input.RemoteWrite {
				rwCopy := *rw
				remoteWrites = append(remoteWrites, &rwCopy)
			}
			input.RemoteWrite = remoteWrites

			if tc.mutation != nil {
				tc.mutation(&input)
			}

			err := input.ApplyDefaults(global)
			if tc.err == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.err.Error())
			}
		})
	}
}

func TestConfig_ApplyDefaults_HashedName(t *testing.T) {
	cfgText := `
name: default
host_filter: false
remote_write:
- url: http://localhost:9009/api/prom/push
  sigv4: {}`

	cfg, err := UnmarshalConfig(strings.NewReader(cfgText))
	require.NoError(t, err)
	require.NoError(t, cfg.ApplyDefaults(DefaultGlobalConfig))
	require.NotEmpty(t, cfg.RemoteWrite[0].Name)
}

func TestInstance_Path(t *testing.T) {
	scrapeAddr, closeSrv := getTestServer(t)
	defer closeSrv()

	walDir := t.TempDir()

	globalConfig := getTestGlobalConfig(t)

	cfg := getTestConfig(t, &globalConfig, scrapeAddr)
	cfg.WALTruncateFrequency = time.Hour
	cfg.RemoteFlushDeadline = time.Hour

	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	inst, err := New(prometheus.NewRegistry(), cfg, walDir, logger)
	require.NoError(t, err)
	runInstance(t, inst)

	// <walDir>/<inst.name> path should exist for WAL
	util.Eventually(t, func(t require.TestingT) {
		_, err := os.Stat(path.Join(walDir, "test"))
		require.NoError(t, err)
	})
}

// TestInstance tests that discovery and scraping are working by using a mock
// instance of the WAL storage and testing that samples get written to it.
// This test touches most of Instance and is enough for a basic integration test.
func TestInstance(t *testing.T) {
	scrapeAddr, closeSrv := getTestServer(t)
	defer closeSrv()

	walDir := t.TempDir()

	globalConfig := getTestGlobalConfig(t)
	cfg := getTestConfig(t, &globalConfig, scrapeAddr)
	cfg.WALTruncateFrequency = time.Hour
	cfg.RemoteFlushDeadline = time.Hour

	mockStorage := mockWalStorage{
		series:    make(map[storage.SeriesRef]int),
		directory: walDir,
	}
	newWal := func(_ prometheus.Registerer) (walStorage, error) { return &mockStorage, nil }

	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	inst, err := newInstance(cfg, nil, logger, newWal)
	require.NoError(t, err)
	runInstance(t, inst)

	// Wait until mockWalStorage has had a series added to it.
	util.EventuallyWithBackoff(t, func(t require.TestingT) {
		mockStorage.mut.Lock()
		defer mockStorage.mut.Unlock()
		require.True(t, len(mockStorage.series) > 0)
	}, slowBackoff)
}

// TestInstance_Recreate ensures that creating an instance with the same name twice
// does not cause any duplicate metrics registration that leads to a panic.
func TestInstance_Recreate(t *testing.T) {
	scrapeAddr, closeSrv := getTestServer(t)
	defer closeSrv()

	walDir := t.TempDir()

	globalConfig := getTestGlobalConfig(t)

	cfg := getTestConfig(t, &globalConfig, scrapeAddr)
	cfg.Name = "recreate_test"
	cfg.WALTruncateFrequency = time.Hour
	cfg.RemoteFlushDeadline = time.Hour

	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	currentReg := prometheus.NewRegistry()
	inst, err := New(currentReg, cfg, walDir, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	exited := make(chan bool)
	go func() {
		err := inst.Run(ctx)
		close(exited)

		if err != nil {
			require.Equal(t, context.Canceled, err)
		}
	}()

	time.Sleep(1 * time.Second)
	cancel()
	<-exited

	// Recreate the instance, no panic should happen.
	require.NotPanics(t, func() {
		inst, err := New(currentReg, cfg, walDir, logger)
		require.NoError(t, err)
		runInstance(t, inst)

		time.Sleep(1 * time.Second)
	})
}

func getTestServer(t *testing.T) (addr string, closeFunc func()) {
	t.Helper()

	reg := prometheus.NewRegistry()

	testCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_metric_total",
	})
	testCounter.Inc()
	reg.MustRegister(testCounter)

	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	httpSrv := httptest.NewServer(handler)
	return httpSrv.Listener.Addr().String(), httpSrv.Close
}

func getTestGlobalConfig(t *testing.T) GlobalConfig {
	t.Helper()

	return GlobalConfig{
		Prometheus: config.GlobalConfig{
			ScrapeInterval:     model.Duration(time.Millisecond * 50),
			ScrapeTimeout:      model.Duration(time.Millisecond * 25),
			EvaluationInterval: model.Duration(time.Hour),
		},
	}
}

func getTestConfig(t *testing.T, global *GlobalConfig, scrapeAddr string) Config {
	t.Helper()

	scrapeCfg := config.DefaultScrapeConfig
	scrapeCfg.JobName = "test"
	scrapeCfg.ScrapeInterval = global.Prometheus.ScrapeInterval
	scrapeCfg.ScrapeTimeout = global.Prometheus.ScrapeTimeout
	scrapeCfg.ServiceDiscoveryConfigs = discovery.Configs{
		discovery.StaticConfig{{
			Targets: []model.LabelSet{{
				model.AddressLabel: model.LabelValue(scrapeAddr),
			}},
			Labels: model.LabelSet{},
		}},
	}

	cfg := DefaultConfig
	cfg.Name = "test"
	cfg.ScrapeConfigs = []*config.ScrapeConfig{&scrapeCfg}
	cfg.global = *global

	return cfg
}

type mockWalStorage struct {
	storage.Queryable
	storage.ChunkQueryable

	directory string
	mut       sync.Mutex
	series    map[storage.SeriesRef]int
}

func (s *mockWalStorage) Directory() string                          { return s.directory }
func (s *mockWalStorage) StartTime() (int64, error)                  { return 0, nil }
func (s *mockWalStorage) WriteStalenessMarkers(f func() int64) error { return nil }
func (s *mockWalStorage) Close() error                               { return nil }
func (s *mockWalStorage) Truncate(mint int64) error                  { return nil }

func (s *mockWalStorage) Appender(context.Context) storage.Appender {
	return &mockAppender{s: s}
}

type mockAppender struct {
	s *mockWalStorage
}

func (a *mockAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	if ref == 0 {
		return a.Add(l, t, v)
	}
	return ref, a.AddFast(ref, t, v)
}

// Add adds a new series and sets its written count to 1.
func (a *mockAppender) Add(l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	a.s.mut.Lock()
	defer a.s.mut.Unlock()

	hash := l.Hash()
	a.s.series[storage.SeriesRef(hash)] = 1
	return storage.SeriesRef(hash), nil
}

// AddFast increments the number of writes to an existing series.
func (a *mockAppender) AddFast(ref storage.SeriesRef, t int64, v float64) error {
	a.s.mut.Lock()
	defer a.s.mut.Unlock()
	_, ok := a.s.series[ref]
	if !ok {
		return storage.ErrNotFound
	}

	a.s.series[ref]++
	return nil
}

func (a *mockAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	return 0, nil
}

func (a *mockAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	return 0, nil
}

func (a *mockAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return 0, nil
}

func (a *mockAppender) Commit() error {
	return nil
}

func (a *mockAppender) Rollback() error {
	return nil
}

func runInstance(t *testing.T, i *Instance) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel() })
	go require.NotPanics(t, func() {
		_ = i.Run(ctx)
	})
}

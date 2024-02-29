package metrics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/agent/internal/static/metrics/instance"
	"github.com/grafana/agent/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"gopkg.in/yaml.v2"
)

func TestConfig_Validate(t *testing.T) {
	valid := Config{
		WALDir: "/tmp/data",
		Configs: []instance.Config{
			makeInstanceConfig("instance"),
		},
		InstanceMode: instance.DefaultMode,
	}

	tt := []struct {
		name    string
		mutator func(c *Config)
		expect  error
	}{
		{
			name:    "complete config should be valid",
			mutator: func(c *Config) {},
			expect:  nil,
		},
		{
			name:    "no wal dir",
			mutator: func(c *Config) { c.WALDir = "" },
			expect:  errors.New("no wal_directory configured"),
		},
		{
			name:    "missing instance name",
			mutator: func(c *Config) { c.Configs[0].Name = "" },
			expect:  errors.New("error validating instance at index 0: missing instance name"),
		},
		{
			name: "duplicate config name",
			mutator: func(c *Config) {
				c.Configs = append(c.Configs,
					makeInstanceConfig("newinstance"),
					makeInstanceConfig("instance"),
				)
			},
			expect: errors.New("prometheus instance names must be unique. found multiple instances with name instance"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg := copyConfig(t, valid)
			tc.mutator(&cfg)

			err := cfg.ApplyDefaults()
			if tc.expect == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expect.Error())
			}
		})
	}
}

func copyConfig(t *testing.T, c Config) Config {
	t.Helper()

	bb, err := yaml.Marshal(c)
	require.NoError(t, err)

	var cp Config
	err = yaml.Unmarshal(bb, &cp)
	require.NoError(t, err)
	return cp
}

func TestConfigNonzeroDefaultScrapeInterval(t *testing.T) {
	cfgText := `
wal_directory: ./wal
configs:
  - name: testconfig
    scrape_configs:
      - job_name: 'node'
        static_configs:
          - targets: ['localhost:9100']
`

	var cfg Config

	err := yaml.Unmarshal([]byte(cfgText), &cfg)
	require.NoError(t, err)
	err = cfg.ApplyDefaults()
	require.NoError(t, err)

	require.Equal(t, len(cfg.Configs), 1)
	instanceConfig := cfg.Configs[0]
	require.Equal(t, len(instanceConfig.ScrapeConfigs), 1)
	scrapeConfig := instanceConfig.ScrapeConfigs[0]
	require.Greater(t, int64(scrapeConfig.ScrapeInterval), int64(0))
}

func TestAgent(t *testing.T) {
	// Launch two instances
	cfg := Config{
		WALDir: "/tmp/wal",
		Configs: []instance.Config{
			makeInstanceConfig("instance_a"),
			makeInstanceConfig("instance_b"),
		},
		InstanceRestartBackoff: time.Duration(0),
		InstanceMode:           instance.ModeDistinct,
	}

	fact := newFakeInstanceFactory()

	a, err := newAgent(prometheus.NewRegistry(), cfg, log.NewNopLogger(), fact.factory)
	require.NoError(t, err)

	util.Eventually(t, func(t require.TestingT) {
		require.NotNil(t, fact.created)
		require.Equal(t, 2, int(fact.created.Load()))
		require.Equal(t, 2, len(a.mm.ListInstances()))
	})

	t.Run("instances should be running", func(t *testing.T) {
		for _, mi := range fact.Mocks() {
			// Each instance should have wait called on it
			util.Eventually(t, func(t require.TestingT) {
				require.True(t, mi.running.Load())
			})
		}
	})

	t.Run("instances should be restarted when stopped", func(t *testing.T) {
		for _, mi := range fact.Mocks() {
			util.Eventually(t, func(t require.TestingT) {
				require.Equal(t, 1, int(mi.startedCount.Load()))
			})
		}

		for _, mi := range fact.Mocks() {
			mi.err <- fmt.Errorf("really bad error")
		}

		for _, mi := range fact.Mocks() {
			util.Eventually(t, func(t require.TestingT) {
				require.Equal(t, 2, int(mi.startedCount.Load()))
			})
		}
	})
}

func TestAgent_NormalInstanceExits(t *testing.T) {
	tt := []struct {
		name          string
		simulateError error
	}{
		{"no error", nil},
		{"context cancelled", context.Canceled},
	}

	cfg := Config{
		WALDir: "/tmp/wal",
		Configs: []instance.Config{
			makeInstanceConfig("instance_a"),
			makeInstanceConfig("instance_b"),
		},
		InstanceRestartBackoff: time.Duration(0),
		InstanceMode:           instance.ModeDistinct,
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			fact := newFakeInstanceFactory()

			a, err := newAgent(prometheus.NewRegistry(), cfg, log.NewNopLogger(), fact.factory)
			require.NoError(t, err)

			util.Eventually(t, func(t require.TestingT) {
				require.NotNil(t, fact.created)
				require.Equal(t, 2, int(fact.created.Load()))
				require.Equal(t, 2, len(a.mm.ListInstances()))
			})
			for _, mi := range fact.Mocks() {
				mi.err <- tc.simulateError
			}

			time.Sleep(time.Millisecond * 100)

			// Get the new total amount of instances starts; value should
			// be unchanged.
			var startedCount int64
			for _, i := range fact.Mocks() {
				startedCount += i.startedCount.Load()
			}

			// There should only be two instances that started. If there's more, something
			// restarted despite our error.
			require.Equal(t, int64(2), startedCount, "instances should not have restarted")
		})
	}
}

func TestAgent_Stop(t *testing.T) {
	// Launch two instances
	cfg := Config{
		WALDir: "/tmp/wal",
		Configs: []instance.Config{
			makeInstanceConfig("instance_a"),
			makeInstanceConfig("instance_b"),
		},
		InstanceRestartBackoff: time.Duration(0),
		InstanceMode:           instance.ModeDistinct,
	}

	fact := newFakeInstanceFactory()

	a, err := newAgent(prometheus.NewRegistry(), cfg, log.NewNopLogger(), fact.factory)
	require.NoError(t, err)

	util.Eventually(t, func(t require.TestingT) {
		require.NotNil(t, fact.created)
		require.Equal(t, 2, int(fact.created.Load()))
		require.Equal(t, 2, len(a.mm.ListInstances()))
	})

	a.Stop()

	time.Sleep(time.Millisecond * 100)

	for _, mi := range fact.Mocks() {
		require.False(t, mi.running.Load(), "instance should not have been restarted")
	}
}

type fakeInstance struct {
	cfg instance.Config

	err          chan error
	startedCount *atomic.Int64
	running      *atomic.Bool
}

func (i *fakeInstance) Run(ctx context.Context) error {
	i.startedCount.Inc()
	i.running.Store(true)
	defer i.running.Store(false)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-i.err:
		return err
	}
}

func (i *fakeInstance) Ready() bool {
	return true
}

func (i *fakeInstance) Update(_ instance.Config) error {
	return instance.ErrInvalidUpdate{
		Inner: fmt.Errorf("can't dynamically update fakeInstance"),
	}
}

func (i *fakeInstance) TargetsActive() map[string][]*scrape.Target {
	return nil
}

func (i *fakeInstance) StorageDirectory() string {
	return ""
}

func (i *fakeInstance) WriteHandler() http.Handler {
	return nil
}

func (i *fakeInstance) Appender(ctx context.Context) storage.Appender {
	return nil
}

type fakeInstanceFactory struct {
	mut   sync.Mutex
	mocks []*fakeInstance

	created *atomic.Int64
}

func newFakeInstanceFactory() *fakeInstanceFactory {
	return &fakeInstanceFactory{created: atomic.NewInt64(0)}
}

func (f *fakeInstanceFactory) Mocks() []*fakeInstance {
	f.mut.Lock()
	defer f.mut.Unlock()
	return f.mocks
}

func (f *fakeInstanceFactory) factory(_ prometheus.Registerer, cfg instance.Config, _ string, _ log.Logger) (instance.ManagedInstance, error) {
	f.created.Add(1)

	f.mut.Lock()
	defer f.mut.Unlock()

	inst := &fakeInstance{
		cfg:          cfg,
		running:      atomic.NewBool(false),
		startedCount: atomic.NewInt64(0),
		err:          make(chan error),
	}

	f.mocks = append(f.mocks, inst)
	return inst, nil
}

func makeInstanceConfig(name string) instance.Config {
	cfg := instance.DefaultConfig
	cfg.Name = name
	return cfg
}

func TestAgent_MarshalYAMLOmitDefaultConfigFields(t *testing.T) {
	cfg := DefaultConfig
	yml, err := yaml.Marshal(&cfg)
	require.NoError(t, err)
	require.NotContains(t, string(yml), "scraping_service_client")
	require.NotContains(t, string(yml), "scraping_service")
	require.NotContains(t, string(yml), "global")
}

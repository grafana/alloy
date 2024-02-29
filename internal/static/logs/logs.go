// Package logs implements logs support for the Grafana Agent.
package logs

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	_ "time/tzdata" // embed timezone data

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/agent/internal/agentseed"
	"github.com/grafana/agent/internal/useragent"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/loki/clients/pkg/promtail"
	"github.com/grafana/loki/clients/pkg/promtail/api"
	"github.com/grafana/loki/clients/pkg/promtail/client"
	"github.com/grafana/loki/clients/pkg/promtail/config"
	"github.com/grafana/loki/clients/pkg/promtail/server"
	"github.com/grafana/loki/clients/pkg/promtail/targets/file"
	"github.com/grafana/loki/clients/pkg/promtail/wal"
	"github.com/grafana/loki/pkg/tracing"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	client.UserAgent = useragent.Get()
}

// Logs is a Logs log collection. It uses multiple distinct sets of Logs
// Promtail agents to collect logs and send them to a Logs server.
type Logs struct {
	mut sync.Mutex

	reg       prometheus.Registerer
	l         log.Logger
	instances map[string]*Instance
}

// New creates and starts Loki log collection.
func New(reg prometheus.Registerer, c *Config, l log.Logger, dryRun bool) (*Logs, error) {
	logs := &Logs{
		instances: make(map[string]*Instance),
		reg:       reg,
		l:         log.With(l, "component", "logs"),
	}
	if err := logs.ApplyConfig(c, dryRun); err != nil {
		return nil, err
	}
	return logs, nil
}

// ApplyConfig updates Logs with a new Config.
func (l *Logs) ApplyConfig(c *Config, dryRun bool) error {
	l.mut.Lock()
	defer l.mut.Unlock()

	if c == nil {
		c = &Config{}
	}

	newInstances := make(map[string]*Instance, len(c.Configs))

	for _, ic := range c.Configs {
		// If an old instance existed, update it and move it to the new map.
		if old, ok := l.instances[ic.Name]; ok {
			err := old.ApplyConfig(ic, c.Global, dryRun)
			if err != nil {
				return err
			}

			newInstances[ic.Name] = old
			continue
		}

		inst, err := NewInstance(l.reg, ic, c.Global, l.l, dryRun)
		if err != nil {
			return fmt.Errorf("unable to apply config for %s: %w", ic.Name, err)
		}
		newInstances[ic.Name] = inst
	}

	// Any promtail in l.instances that isn't in newInstances has been removed
	// from the config. Stop them before replacing the map.
	for key, i := range l.instances {
		if _, exist := newInstances[key]; exist {
			continue
		}
		i.Stop()
	}
	l.instances = newInstances

	return nil
}

// Stop stops the log collector.
func (l *Logs) Stop() {
	l.mut.Lock()
	defer l.mut.Unlock()

	for _, i := range l.instances {
		i.Stop()
	}
}

// Instance is used to retrieve a named Logs instance
func (l *Logs) Instance(name string) *Instance {
	l.mut.Lock()
	defer l.mut.Unlock()

	return l.instances[name]
}

// Instance is an individual Logs instance.
type Instance struct {
	mut sync.Mutex

	cfg *InstanceConfig
	log log.Logger
	reg *util.Unregisterer

	promtail *promtail.Promtail
}

// NewInstance creates and starts a Logs instance.
func NewInstance(reg prometheus.Registerer, c *InstanceConfig, g GlobalConfig, l log.Logger, dryRun bool) (*Instance, error) {
	instReg := prometheus.WrapRegistererWith(prometheus.Labels{"logs_config": c.Name}, reg)

	inst := Instance{
		reg: util.WrapWithUnregisterer(instReg),
		log: log.With(l, "logs_config", c.Name),
	}
	if err := inst.ApplyConfig(c, g, dryRun); err != nil {
		return nil, err
	}
	return &inst, nil
}

// DefaultConfig returns a default config for a Logs instance.
func DefaultConfig() config.Config {
	return config.Config{
		ServerConfig: server.Config{Disable: true},
		Tracing:      tracing.Config{Enabled: false},
		WAL:          wal.Config{Enabled: false},
	}
}

// ApplyConfig will apply a new InstanceConfig. If the config hasn't changed,
// then nothing will happen, otherwise the old Promtail will be stopped and
// then replaced with a new one.
func (i *Instance) ApplyConfig(c *InstanceConfig, g GlobalConfig, dryRun bool) error {
	i.mut.Lock()
	defer i.mut.Unlock()

	// No-op if the configs haven't changed.
	if util.CompareYAML(c, i.cfg) {
		level.Debug(i.log).Log("msg", "instance config hasn't changed, not recreating Promtail")
		return nil
	}
	i.cfg = c

	positionsDir := filepath.Dir(c.PositionsConfig.PositionsFile)
	err := os.MkdirAll(positionsDir, 0775)
	if err != nil {
		level.Warn(i.log).Log("msg", "failed to create the positions directory. logs may be unable to save their position", "path", positionsDir, "err", err)
	}

	if i.promtail != nil {
		i.promtail.Shutdown()
		i.promtail = nil
	}

	// Unregister all existing metrics before trying to create a new instance.
	if !i.reg.UnregisterAll() {
		// If UnregisterAll fails, we need to abort, otherwise the new promtail
		// would try to re-register an existing metric and might panic.
		return fmt.Errorf("failed to unregister all metrics from previous promtail. THIS IS A BUG")
	}

	if len(c.ClientConfigs) == 0 {
		level.Debug(i.log).Log("msg", "skipping creation of a promtail because no client_configs are present")
		return nil
	}

	uid := agentseed.Get().UID
	for i := range c.ClientConfigs {
		// ClientConfigs is a slice of struct, so we set values with the index
		if c.ClientConfigs[i].Headers == nil {
			c.ClientConfigs[i].Headers = map[string]string{}
		}
		c.ClientConfigs[i].Headers[agentseed.HeaderName] = uid
	}

	clientMetrics := client.NewMetrics(i.reg)
	cfg := DefaultConfig()
	cfg.Global = config.GlobalConfig{
		FileWatch: file.WatchConfig{
			MinPollFrequency: g.FileWatch.MinPollFrequency,
			MaxPollFrequency: g.FileWatch.MaxPollFrequency,
		},
	}
	cfg.ClientConfigs = c.ClientConfigs
	cfg.PositionsConfig = c.PositionsConfig
	cfg.ScrapeConfig = c.ScrapeConfig
	cfg.TargetConfig = c.TargetConfig
	cfg.LimitsConfig = c.LimitsConfig

	p, err := promtail.New(cfg, nil, clientMetrics, dryRun, promtail.WithLogger(i.log), promtail.WithRegisterer(i.reg))
	if err != nil {
		return fmt.Errorf("unable to create logs instance: %w", err)
	}

	i.promtail = p
	return nil
}

// SendEntry passes an entry to the internal promtail client and returns true if successfully sent. It is
// best effort and not guaranteed to succeed.
func (i *Instance) SendEntry(entry api.Entry, dur time.Duration) bool {
	i.mut.Lock()
	defer i.mut.Unlock()

	// promtail is nil it has been stopped
	if i.promtail != nil {
		// send non blocking so we don't block the mutex. this is best effort
		select {
		case i.promtail.Client().Chan() <- entry:
			return true
		case <-time.After(dur):
		}
	}

	return false
}

// Stop stops the Promtail instance.
func (i *Instance) Stop() {
	i.mut.Lock()
	defer i.mut.Unlock()

	if i.promtail != nil {
		i.promtail.Shutdown()
		i.promtail = nil
	}
	i.reg.UnregisterAll()
}

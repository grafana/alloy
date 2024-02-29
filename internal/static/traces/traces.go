package traces

import (
	"fmt"
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/agent/internal/static/logs"
	"github.com/grafana/agent/internal/static/metrics/instance"
	"github.com/grafana/agent/internal/util/zapadapter"
	prom_client "github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// Traces wraps the OpenTelemetry collector to enable tracing pipelines
type Traces struct {
	mut       sync.Mutex
	instances map[string]*Instance

	logger *zap.Logger
	reg    prom_client.Registerer

	promInstanceManager instance.Manager
}

// New creates and starts trace collection.
func New(logsSubsystem *logs.Logs, promInstanceManager instance.Manager, reg prom_client.Registerer, cfg Config, l log.Logger) (*Traces, error) {
	traces := &Traces{
		instances:           make(map[string]*Instance),
		logger:              newLogger(l),
		reg:                 reg,
		promInstanceManager: promInstanceManager,
	}
	if err := traces.ApplyConfig(logsSubsystem, promInstanceManager, cfg); err != nil {
		return nil, err
	}
	return traces, nil
}

// Instance is used to retrieve a named Traces instance
func (t *Traces) Instance(name string) *Instance {
	t.mut.Lock()
	defer t.mut.Unlock()

	return t.instances[name]
}

// ApplyConfig updates Traces with a new Config.
func (t *Traces) ApplyConfig(logsSubsystem *logs.Logs, promInstanceManager instance.Manager, cfg Config) error {
	t.mut.Lock()
	defer t.mut.Unlock()

	newInstances := make(map[string]*Instance, len(cfg.Configs))

	for _, c := range cfg.Configs {
		var (
			instReg = prom_client.WrapRegistererWith(prom_client.Labels{"traces_config": c.Name}, t.reg)
		)

		// If an old instance exists, update it and move it to the new map.
		if old, ok := t.instances[c.Name]; ok {
			err := old.ApplyConfig(logsSubsystem, promInstanceManager, instReg, c)
			if err != nil {
				return err
			}

			newInstances[c.Name] = old
			continue
		}

		var (
			instLogger = t.logger.With(zap.String("traces_config", c.Name))
		)

		inst, err := NewInstance(logsSubsystem, instReg, c, instLogger, t.promInstanceManager)
		if err != nil {
			return fmt.Errorf("failed to create tracing instance %s: %w", c.Name, err)
		}
		newInstances[c.Name] = inst
	}

	// Any instance in l.instances that isn't in newInstances has been removed
	// from the config. Stop them before replacing the map.
	for key, i := range t.instances {
		if _, exist := newInstances[key]; exist {
			continue
		}
		i.Stop()
	}
	t.instances = newInstances

	return nil
}

// Stop stops the OpenTelemetry collector subsystem
func (t *Traces) Stop() {
	t.mut.Lock()
	defer t.mut.Unlock()

	for _, i := range t.instances {
		i.Stop()
	}
}

func newLogger(l log.Logger) *zap.Logger {
	logger := zapadapter.New(l)
	logger = logger.With(zap.String("component", "traces"))
	logger.Info("Traces Logger Initialized")

	return logger
}

//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/errgroup" //nolint:depguard

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/health"
	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/subprocess"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "beyla.ebpf",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Component struct {
	opts component.Options

	argsUpdate     chan Arguments
	subprocessExit chan error

	// owned by Run() (no synchronisation)
	args             Arguments
	subprocessCancel context.CancelFunc
	subprocessGroup  *errgroup.Group
	restartTimer     *time.Timer
	otlpReceiverAddr string
	otlpServer       *http.Server
	otlpQueue        chan otlpItem
	otlpWorkerCancel context.CancelFunc
	otlpWorkersWG    sync.WaitGroup

	subprocess *subprocess.Handle
	health     *health.Reporter
}

var _ component.HealthComponent = (*Component)(nil)

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:           opts,
		args:           args,
		argsUpdate:     make(chan Arguments, 1),
		subprocessExit: make(chan error),
		subprocess:     subprocess.New(),
		health:         health.New(),
	}

	if err := c.registerMetrics(opts.Registerer); err != nil {
		return nil, err
	}

	if err := c.publishExports(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	c.logDeprecationWarnings()

	c.restartTimer = time.NewTimer(0)
	defer c.restartTimer.Stop()

	for {
		if c.applyPendingArgsUpdate() {
			continue
		}

		select {
		case <-ctx.Done():
			c.stopSubprocess()
			return nil

		case newArgs := <-c.argsUpdate:
			c.handleArgsUpdate(newArgs)

		case err := <-c.subprocessExit:
			c.handleSubprocessExit(err)

		case <-c.restartTimer.C:
			c.handleSubprocessStart(ctx)
		}
	}
}

func (c *Component) Update(args component.Arguments) error {
	if err := c.publishExports(); err != nil {
		return err
	}

	c.argsUpdate <- args.(Arguments)

	return nil
}

func (c *Component) CurrentHealth() component.Health {
	return c.health.Current()
}

func (c *Component) applyPendingArgsUpdate() bool {
	select {
	case newArgs := <-c.argsUpdate:
		c.handleArgsUpdate(newArgs)
		return true
	default:
		return false
	}
}

func (c *Component) handleArgsUpdate(newArgs Arguments) {
	c.args = getLatestArgsFromChannel(c.argsUpdate, newArgs)

	c.stopSubprocess()

	c.subprocess.ResetRestartTracking()
	c.restartTimer.Reset(0)
}

func getLatestArgsFromChannel[A any](ch chan A, current A) A {
	for {
		select {
		case x := <-ch:
			current = x
		default:
			return current
		}
	}
}

func (c *Component) logDeprecationWarnings() {
	if c.args.Port != "" { //nolint:staticcheck // intentionally reads the deprecated field to warn
		c.opts.Logger.Warn("The 'open_port' field is deprecated. Use 'discovery.services' instead.")
	}

	if c.args.ExecutableName != "" { //nolint:staticcheck // intentionally reads the deprecated field to warn
		c.opts.Logger.Warn("The 'executable_name' field is deprecated. Use 'discovery.services' instead.")
	}

	if c.args.Debug {
		c.opts.Logger.Warn("The 'debug' field is deprecated. Use 'log_level = \"debug\"' instead.")
	}

	if len(c.args.Discovery.Services) > 0 {
		c.opts.Logger.Warn("discovery.services is deprecated, use discovery.instrument instead")
	}

	if len(c.args.Discovery.ExcludeServices) > 0 {
		c.opts.Logger.Warn("discovery.exclude_services is deprecated, use discovery.exclude_instrument instead")
	}

	if len(c.args.Discovery.DefaultExcludeServices) > 0 {
		c.opts.Logger.Warn("discovery.default_exclude_services is deprecated, use discovery.default_exclude_instrument instead")
	}
}

// Package kubernetes_events implements the loki.source.kubernetes_events
// component.
package kubernetes_events

import (
	"context"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/go-kit/log"
	"k8s.io/client-go/rest"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/featuregate"
)

// Generous timeout period for configuring informers
const informerSyncTimeout = 10 * time.Minute

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.kubernetes_events",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the
// loki.source.kubernetes_events component.
type Arguments struct {
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`

	JobName    string   `alloy:"job_name,attr,optional"`
	Namespaces []string `alloy:"namespaces,attr,optional"`
	LogFormat  string   `alloy:"log_format,attr,optional"`

	// Client settings to connect to Kubernetes.
	Client kubernetes.ClientArguments `alloy:"client,block,optional"`
}

// DefaultArguments holds default settings for loki.source.kubernetes_events.
var DefaultArguments = Arguments{
	JobName:   "loki.source.kubernetes_events",
	LogFormat: logFormatFmt,

	Client: kubernetes.DefaultClientArguments,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.JobName == "" {
		return fmt.Errorf("job_name must not be an empty string")
	}
	if args.LogFormat != logFormatFmt && args.LogFormat != logFormatJson {
		return fmt.Errorf("supported values of log_format are %s and %s", logFormatFmt, logFormatJson)
	}
	return nil
}

// Component implements the loki.source.kubernetes_events component, which
// watches events from Kubernetes and forwards received events to other Loki
// components.
type Component struct {
	log       log.Logger
	opts      component.Options
	positions positions.Positions
	handler   loki.LogsReceiver

	mut        sync.RWMutex
	args       Arguments
	restConfig *rest.Config
	scheduler  *source.Scheduler[string]

	fanout *loki.Fanout
}

var (
	_ component.Component      = (*Component)(nil)
	_ component.DebugComponent = (*Component)(nil)
)

// New creates a new loki.source.kubernetes_events component.
func New(o component.Options, args Arguments) (*Component, error) {
	err := os.MkdirAll(o.DataPath, 0750)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	positionsFile, err := positions.New(o.Logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: filepath.Join(o.DataPath, "positions.yml"),
	})
	if err != nil {
		return nil, err
	}

	c := &Component{
		log:       o.Logger,
		opts:      o,
		positions: positionsFile,
		handler:   loki.NewLogsReceiver(),
		scheduler: source.NewScheduler[string](),
		fanout:    loki.NewFanout(args.ForwardTo, o.Registerer),
	}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.positions.Stop()
		source.Drain(c.handler, func() {
			c.mut.Lock()
			defer c.mut.Unlock()
			c.scheduler.Stop()
		})
	}()

	source.Consume(ctx, c.handler, c.fanout)
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)

	c.fanout.UpdateChildren(newArgs.ForwardTo)

	restConfig := c.restConfig

	// Create a new restConfig if we don't have one or if our arguments changed.
	if restConfig == nil || !reflect.DeepEqual(c.args.Client, newArgs.Client) {
		var err error
		restConfig, err = newArgs.Client.BuildRESTConfig(c.log)
		if err != nil {
			return fmt.Errorf("building Kubernetes client config: %w", err)
		}

		// When restConfig changes we need to restart all scheduled sources.
		c.scheduler.Reset()
	}

	source.Reconcile(
		c.opts.Logger,
		c.scheduler,
		getNamespaces(newArgs),
		func(namespace string) string { return namespace },
		func(_ string, namespace string) (source.Source[string], error) {
			return newEventController(eventControllerOptions{
				Log:          c.log,
				Config:       restConfig,
				Namespace:    namespace,
				JobName:      newArgs.JobName,
				InstanceName: c.opts.ID,
				Receiver:     c.handler,
				Positions:    c.positions,
				LogFormat:    newArgs.LogFormat,
			}), nil
		},
	)

	c.args = newArgs
	return nil
}

// getNamespaces returns a iterator of namespaces to watch from the arguments. If the
// list of namespaces is empty, returns a iterator to watch all namespaces.
func getNamespaces(args Arguments) iter.Seq[string] {
	return func(yield func(string) bool) {
		if len(args.Namespaces) == 0 {
			// Empty string means to watch all namespaces
			yield("")
			return
		}

		for _, namespace := range args.Namespaces {
			if !yield(namespace) {
				return
			}
		}
	}
}

// DebugInfo implements [component.DebugComponent].
func (c *Component) DebugInfo() any {
	c.mut.RLock()
	defer c.mut.RUnlock()

	type Info struct {
		Controllers []controllerInfo `alloy:"event_controller,block,optional"`
	}

	var info Info
	for s := range c.scheduler.Sources() {
		info.Controllers = append(info.Controllers, s.(*eventController).DebugInfo())
	}

	return info
}

// Package kubernetes_events implements the loki.source.kubernetes_events
// component.
package kubernetes_events //nolint:golint

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runner"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/oklog/run"
	"k8s.io/client-go/rest"
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
	log        log.Logger
	opts       component.Options
	positions  positions.Positions
	handler    loki.LogsReceiver
	runner     *runner.Runner[eventControllerTask]
	newTasksCh chan struct{}

	mut        sync.Mutex
	args       Arguments
	restConfig *rest.Config

	tasksMut sync.RWMutex
	tasks    []eventControllerTask

	receiversMut sync.RWMutex
	receivers    []loki.LogsReceiver
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
		runner: runner.New(func(t eventControllerTask) runner.Worker {
			return newEventController(t)
		}),
		newTasksCh: make(chan struct{}, 1),
	}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer c.positions.Stop()
	defer c.runner.Stop()

	var rg run.Group

	// Runner to apply tasks.
	rg.Add(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-c.newTasksCh:
				c.tasksMut.RLock()
				tasks := c.tasks
				c.tasksMut.RUnlock()

				if err := c.runner.ApplyTasks(ctx, tasks); err != nil {
					level.Error(c.log).Log("msg", "failed to apply event watchers", "err", err)
				}
			}
		}
	}, func(_ error) {
		cancel()
	})

	// Runner to forward received logs.
	rg.Add(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case entry := <-c.handler.Chan():
				c.receiversMut.RLock()
				receivers := c.receivers
				c.receiversMut.RUnlock()

				for _, receiver := range receivers {
					receiver.Chan() <- entry
				}
			}
		}
	}, func(_ error) {
		cancel()
	})

	return rg.Run()
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)

	c.receiversMut.Lock()
	c.receivers = newArgs.ForwardTo
	c.receiversMut.Unlock()

	restConfig := c.restConfig

	// Create a new restConfig if we don't have one or if our arguments changed.
	if restConfig == nil || !reflect.DeepEqual(c.args.Client, newArgs.Client) {
		var err error
		restConfig, err = newArgs.Client.BuildRESTConfig(c.log)
		if err != nil {
			return fmt.Errorf("building Kubernetes client config: %w", err)
		}
	}

	// Create a task for each defined namespace.
	var newTasks []eventControllerTask
	for _, namespace := range getNamespaces(newArgs) {
		newTasks = append(newTasks, eventControllerTask{
			Log:          c.log,
			Config:       restConfig,
			JobName:      newArgs.JobName,
			InstanceName: c.opts.ID,
			Namespace:    namespace,
			Receiver:     c.handler,
			Positions:    c.positions,
			LogFormat:    newArgs.LogFormat,
		})
	}

	c.tasksMut.Lock()
	c.tasks = newTasks
	c.tasksMut.Unlock()

	select {
	case c.newTasksCh <- struct{}{}:
	default:
		// no-op: task reload already queued.
	}

	c.args = newArgs
	return nil
}

// getNamespaces gets a list of namespaces to watch from the arguments. If the
// list of namespaces is empty, returns a slice to watch all namespaces.
func getNamespaces(args Arguments) []string {
	if len(args.Namespaces) == 0 {
		return []string{""} // Empty string means to watch all namespaces
	}
	return args.Namespaces
}

// DebugInfo implements [component.DebugComponent].
func (c *Component) DebugInfo() interface{} {
	type Info struct {
		Controllers []controllerInfo `alloy:"event_controller,block,optional"`
	}

	var info Info
	for _, worker := range c.runner.Workers() {
		info.Controllers = append(info.Controllers, worker.(*eventController).DebugInfo())
	}
	return info
}

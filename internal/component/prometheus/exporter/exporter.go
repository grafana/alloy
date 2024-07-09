package exporter

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/prometheus/common/model"
)

// Creator is a function provided by an implementation to create a concrete exporter instance.
type Creator func(component.Options, component.Arguments, string) (integrations.Integration, string, error)

// Exports are simply a list of targets for a scraper to consume.
type Exports struct {
	Targets []discovery.Target `alloy:"targets,attr"`
}

type Component struct {
	opts component.Options

	mut sync.Mutex

	reload chan struct{}

	creator           Creator
	targetBuilderFunc func(discovery.Target, component.Arguments) []discovery.Target
	baseTarget        discovery.Target

	exporter       integrations.Integration
	metricsHandler http.Handler
}

// New creates a new exporter component.
func New(creator Creator, name string) func(component.Options, component.Arguments) (component.Component, error) {
	return newExporter(creator, name, nil)
}

// NewWithTargetBuilder creates a new exporter component with a custom target builder function. It can be used to expand
// a set of targets from a single one, or to customize the labels of the targets.
func NewWithTargetBuilder(creator Creator, name string, targetBuilderFunc func(discovery.Target, component.Arguments) []discovery.Target) func(component.Options, component.Arguments) (component.Component, error) {
	return newExporter(creator, name, targetBuilderFunc)
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	var cancel context.CancelFunc
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.reload:
			// cancel any previously running exporter
			if cancel != nil {
				cancel()
			}
			// create new context so we can cancel it if we get any future updates
			// since it is derived from the main run context, it only needs to be
			// canceled directly if we receive new updates
			newCtx, cancelFunc := context.WithCancel(ctx)
			cancel = cancelFunc

			// finally create and run new exporter
			c.mut.Lock()
			exporter := c.exporter
			c.metricsHandler = c.getHttpHandler(exporter)
			c.mut.Unlock()
			go func() {
				if err := exporter.Run(newCtx); err != nil && err != context.Canceled {
					level.Error(c.opts.Logger).Log("msg", "error running exporter", "err", err)
				}
			}()
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	exporter, instanceKey, err := c.creator(c.opts, args, defaultInstance())
	if err != nil {
		return err
	}
	c.mut.Lock()
	c.exporter = exporter
	if instanceKey != "" {
		c.baseTarget["instance"] = instanceKey
	}

	var targets []discovery.Target
	if c.targetBuilderFunc == nil {
		targets = []discovery.Target{c.baseTarget}
	} else {
		targets = c.targetBuilderFunc(c.baseTarget, args)
	}

	c.opts.OnStateChange(Exports{
		Targets: targets,
	})
	c.mut.Unlock()
	select {
	case c.reload <- struct{}{}:
	default:
	}
	return err
}

// Handler serves metrics endpoint from the integration implementation.
func (c *Component) Handler() http.Handler {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.metricsHandler
}

func newExporter(creator Creator, name string, targetBuilderFunc func(discovery.Target, component.Arguments) []discovery.Target) func(component.Options, component.Arguments) (component.Component, error) {
	return func(opts component.Options, args component.Arguments) (component.Component, error) {
		c := &Component{
			opts:              opts,
			reload:            make(chan struct{}, 1),
			creator:           creator,
			targetBuilderFunc: targetBuilderFunc,
		}
		jobName := fmt.Sprintf("integrations/%s", name)
		instance := defaultInstance()

		data, err := opts.GetServiceData(http_service.ServiceName)
		if err != nil {
			return nil, fmt.Errorf("failed to get HTTP information: %w", err)
		}
		httpData := data.(http_service.Data)

		componentName := opts.ID[:strings.LastIndex(opts.ID, ".")]
		if opts.ID == "prometheus.exporter.unix" {
			componentName = opts.ID
		}

		c.baseTarget = discovery.Target{
			model.AddressLabel:      httpData.MemoryListenAddr,
			model.SchemeLabel:       "http",
			model.MetricsPathLabel:  path.Join(httpData.HTTPPathForComponent(opts.ID), "metrics"),
			"instance":              instance,
			"job":                   jobName,
			"__meta_component_name": componentName,
			"__meta_component_id":   opts.ID,
		}

		// Call to Update() to set the output once at the start.
		if err := c.Update(args); err != nil {
			return nil, err
		}

		return c, nil
	}
}

// get the http handler once and save it, so we don't create extra garbage
func (c *Component) getHttpHandler(integration integrations.Integration) http.Handler {
	h, err := integration.MetricsHandler()
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to creating metrics handler", "err", err)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
	}
	return h
}

// defaultInstance retrieves the hostname identifying the machine the process is
// running on. It will return the value of $HOSTNAME, if defined, and fall
// back to Go's os.Hostname. If that fails, it will return "unknown".
func defaultInstance() string {
	hostname := os.Getenv("HOSTNAME")
	if hostname != "" {
		return hostname
	}

	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

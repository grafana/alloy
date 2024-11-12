package dbo11y

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "grafanacloud.dbo11y",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	DataSourceName alloytypes.Secret   `alloy:"data_source_name,attr,optional"`
	SomeArg        string              `alloy:"somearg,attr,optional"`
	ForwardTo      []loki.LogsReceiver `alloy:"forward_to,attr"`
}

type Exports struct {
	Targets []discovery.Target `alloy:"targets,attr"`
}

type Component struct {
	opts       component.Options
	log        log.Logger
	mut        sync.RWMutex
	receivers  []loki.LogsReceiver
	handler    loki.LogsReceiver
	registry   *prometheus.Registry
	baseTarget discovery.Target
}

var testCounter = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "test_counter",
	Help: "This is a test counter",
})

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:      opts,
		receivers: args.ForwardTo,
		handler:   loki.NewLogsReceiver(),
		registry:  prometheus.NewRegistry(),
	}

	c.registry.MustRegister(testCounter)
	baseTarget, err := c.getBaseTarget()
	if err != nil {
		return nil, err
	}
	c.baseTarget = baseTarget

	// Call to Update() to start readers and set receivers once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.handler.Chan():
			c.mut.RLock()
			for _, receiver := range c.receivers {
				receiver.Chan() <- entry
			}
			c.mut.RUnlock()
		}
	}
}

func (c *Component) getBaseTarget() (discovery.Target, error) {
	data, err := c.opts.GetServiceData(http_service.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get HTTP information: %w", err)
	}
	httpData := data.(http_service.Data)

	return discovery.Target{
		model.AddressLabel:     httpData.MemoryListenAddr,
		model.SchemeLabel:      "http",
		model.MetricsPathLabel: path.Join(httpData.HTTPPathForComponent(c.opts.ID), "metrics"),
		"instance":             "todo",
		"job":                  "dbo11y",
	}, nil
}

func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.opts.OnStateChange(Exports{
		Targets: []discovery.Target{c.baseTarget},
	})

	go func() {
		for {
			testCounter.Add(1)

			entryHandler := loki.NewEntryHandler(c.handler.Chan(), func() {})
			entryHandler.Chan() <- loki.Entry{
				Labels: model.LabelSet{"lbl": "val"},
				Entry: logproto.Entry{
					Timestamp: time.Unix(0, time.Now().UnixNano()),
					Line:      args.(Arguments).SomeArg,
				},
			}

			time.Sleep(10 * time.Second)
		}
	}()

	return nil
}

func (c *Component) Handler() http.Handler {
	return promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{})
}

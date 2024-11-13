package dbo11y

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/dbo11y/collector"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
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
	DataSourceName alloytypes.Secret   `alloy:"data_source_name,attr"`
	ScrapeInterval time.Duration       `alloy:"scrape_interval,attr,optional"`
	ForwardTo      []loki.LogsReceiver `alloy:"forward_to,attr"`
}

var DefaultArguments = Arguments{
	ScrapeInterval: 10 * time.Second,
}

func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) Validate() error {
	_, err := mysql.ParseDSN(string(a.DataSourceName))
	if err != nil {
		return err
	}
	return nil
}

type Exports struct {
	Targets []discovery.Target `alloy:"targets,attr"`
}

type Component struct {
	opts       component.Options
	mut        sync.RWMutex
	receivers  []loki.LogsReceiver
	handler    loki.LogsReceiver
	registry   *prometheus.Registry
	baseTarget discovery.Target
	collectors []collector.Collector
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

	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	defer func() {
		level.Info(c.opts.Logger).Log("msg", "grafanacloud.dbo11y component shutting down, stopping collectors")
		c.mut.RLock()
		for _, collector := range c.collectors {
			collector.Stop()
		}
		c.mut.RUnlock()
	}()

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

	for _, collector := range c.collectors {
		collector.Stop()
	}
	c.collectors = nil

	newArgs := args.(Arguments)
	entryHandler := loki.NewEntryHandler(c.handler.Chan(), func() {})

	qsCollector, err := collector.NewQuerySample(collector.QuerySampleArguments{
		DSN:            string(newArgs.DataSourceName),
		ScrapeInterval: newArgs.ScrapeInterval,
		EntryHandler:   entryHandler,
		Logger:         c.opts.Logger,
	})
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to create QuerySample collector", "err", err)
		return err
	}
	if err := qsCollector.Run(context.Background()); err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to run QuerySample collector", "err", err)
		return err
	}
	c.collectors = append(c.collectors, qsCollector)

	stCollector, err := collector.NewSchemaTable(collector.SchemaTableArguments{
		DSN:            string(newArgs.DataSourceName),
		ScrapeInterval: newArgs.ScrapeInterval,
		EntryHandler:   entryHandler,
		Logger:         c.opts.Logger,
	})
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to create SchemaTable collector", "err", err)
		return err
	}
	if err := stCollector.Run(context.Background()); err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to run SchemaTable collector", "err", err)
		return err
	}
	c.collectors = append(c.collectors, stCollector)

	go func() {
		for {
			testCounter.Add(1)
			time.Sleep(10 * time.Second)
		}
	}()

	return nil
}

func (c *Component) Handler() http.Handler {
	return promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{})
}

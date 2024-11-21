package mysql

import (
	"context"
	"database/sql"
	"errors"
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
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
)

const name = "grafanacloud.database_observability.mysql"

func init() {
	component.Register(component.Registration{
		Name:      name,
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

var (
	_ syntax.Defaulter = (*Arguments)(nil)
	_ syntax.Validator = (*Arguments)(nil)
)

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

var (
	_ component.Component    = (*Component)(nil)
	_ http_service.Component = (*Component)(nil)
)

type Collector interface {
	Run(context.Context) error
	Stop()
}

type Component struct {
	opts       component.Options
	args       Arguments
	mut        sync.RWMutex
	receivers  []loki.LogsReceiver
	handler    loki.LogsReceiver
	registry   *prometheus.Registry
	baseTarget discovery.Target
	collectors []Collector
}

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:      opts,
		args:      args,
		receivers: args.ForwardTo,
		handler:   loki.NewLogsReceiver(),
		registry:  prometheus.NewRegistry(),
	}

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
		level.Info(c.opts.Logger).Log("msg", name+" component shutting down, stopping collectors")
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
		"instance":             c.instanceKey(),
		"job":                  "integrations/db-o11y",
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
	c.args = newArgs

	// TODO(cristian): verify before appending parameter
	dbConnection, err := sql.Open("mysql", string(newArgs.DataSourceName)+"?parseTime=true")
	if err != nil {
		return err
	}

	if dbConnection == nil {
		return errors.New("nil DB connection")
	}

	if err = dbConnection.Ping(); err != nil {
		return err
	}

	entryHandler := loki.NewEntryHandler(c.handler.Chan(), func() {})

	// TODO(cristian)
	// dbConnection.Close()
	// entryHandler.Stop()

	qsCollector, err := collector.NewQuerySample(collector.QuerySampleArguments{
		DB:             dbConnection,
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
		DB:             dbConnection,
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

	ciCollector, err := collector.NewConnectionInfo(collector.ConnectionInfoArguments{
		DSN:      string(newArgs.DataSourceName),
		Registry: c.registry,
	})
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to create ConnectionInfo collector", "err", err)
		return err
	}
	if err := ciCollector.Run(context.Background()); err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to run ConnectionInfo collector", "err", err)
		return err
	}
	c.collectors = append(c.collectors, ciCollector)

	return nil
}

func (c *Component) Handler() http.Handler {
	return promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{})
}

// instanceKey returns network(hostname:port)/dbname of the MySQL server.
// This is the same key as used by the mysqld_exporter integration.
func (c *Component) instanceKey() string {
	m, _ := mysql.ParseDSN(string(c.args.DataSourceName))

	if m.Addr == "" {
		m.Addr = "localhost:3306"
	}
	if m.Net == "" {
		m.Net = "tcp"
	}

	return fmt.Sprintf("%s(%s)/%s", m.Net, m.Addr, m.DBName)
}

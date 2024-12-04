package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
)

const name = "database_observability.mysql"

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

// TODO(cristian) consider using something like "enabled_collectors"
// to allow users to enable/disable collectors.
type Arguments struct {
	DataSourceName      alloytypes.Secret   `alloy:"data_source_name,attr"`
	CollectInterval     time.Duration       `alloy:"collect_interval,attr,optional"`
	QuerySamplesEnabled bool                `alloy:"query_samples_enabled,attr,optional"`
	ForwardTo           []loki.LogsReceiver `alloy:"forward_to,attr"`
}

var DefaultArguments = Arguments{
	CollectInterval:     10 * time.Second,
	QuerySamplesEnabled: true,
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
	Start(context.Context) error
	Stop()
}

type Component struct {
	opts         component.Options
	args         Arguments
	mut          sync.RWMutex
	receivers    []loki.LogsReceiver
	handler      loki.LogsReceiver
	registry     *prometheus.Registry
	baseTarget   discovery.Target
	collectors   []Collector
	dbConnection *sql.DB
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
		if c.dbConnection != nil {
			c.dbConnection.Close()
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
		"job":                  database_observability.JobName,
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

	if c.dbConnection != nil {
		c.dbConnection.Close()
	}

	c.args = args.(Arguments)

	dbConnection, err := sql.Open("mysql", formatDSN(string(c.args.DataSourceName), "parseTime=true"))
	if err != nil {
		return err
	}

	if dbConnection == nil {
		return errors.New("nil DB connection")
	}
	if err = dbConnection.Ping(); err != nil {
		return err
	}
	c.dbConnection = dbConnection

	entryHandler := loki.NewEntryHandler(c.handler.Chan(), func() {})

	if c.args.QuerySamplesEnabled {
		qsCollector, err := collector.NewQuerySample(collector.QuerySampleArguments{
			DB:              dbConnection,
			CollectInterval: c.args.CollectInterval,
			EntryHandler:    entryHandler,
			Logger:          c.opts.Logger,
		})
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to create QuerySample collector", "err", err)
			return err
		}
		if err := qsCollector.Start(context.Background()); err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to start QuerySample collector", "err", err)
			return err
		}
		c.collectors = append(c.collectors, qsCollector)
	}

	stCollector, err := collector.NewSchemaTable(collector.SchemaTableArguments{
		DB:              dbConnection,
		CollectInterval: c.args.CollectInterval,
		EntryHandler:    entryHandler,
		Logger:          c.opts.Logger,
	})
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to create SchemaTable collector", "err", err)
		return err
	}
	if err := stCollector.Start(context.Background()); err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to start SchemaTable collector", "err", err)
		return err
	}
	c.collectors = append(c.collectors, stCollector)

	ciCollector, err := collector.NewConnectionInfo(collector.ConnectionInfoArguments{
		DSN:      string(c.args.DataSourceName),
		Registry: c.registry,
	})
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to create ConnectionInfo collector", "err", err)
		return err
	}
	if err := ciCollector.Start(context.Background()); err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to start ConnectionInfo collector", "err", err)
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

// formatDSN appends the given parameters to the DSN.
// parameters are expected to be in the form of "key=value".
func formatDSN(dsn string, params ...string) string {
	if len(params) == 0 {
		return dsn
	}

	if strings.Contains(dsn, "?") {
		dsn = dsn + "&"
	} else {
		dsn = dsn + "?"
	}
	return dsn + strings.Join(params, "&")
}

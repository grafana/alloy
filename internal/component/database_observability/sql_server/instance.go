package sql_server

import (
	"database/sql"
	"fmt"
	"path"

	"github.com/microsoft/go-mssqldb/msdsn"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/discovery"
	http_service "github.com/grafana/alloy/internal/service/http"
)

// dbInstance holds the per-instance state of the component: the identity,
// connection, collectors, and metrics registry of a single monitored SQL
// Server instance.
type dbInstance struct {
	instanceKey  string
	baseTarget   discovery.Target
	registry     *prometheus.Registry
	collectors   []Collector
	dbConnection *sql.DB
	healthErr    *atomic.String
}

func newDBInstance(opts component.Options, dsn string) (*dbInstance, error) {
	key, err := instanceKey(dsn)
	if err != nil {
		return nil, err
	}

	baseTarget, err := getBaseTarget(opts, key)
	if err != nil {
		return nil, err
	}

	return &dbInstance{
		instanceKey: key,
		baseTarget:  baseTarget,
		registry:    prometheus.NewRegistry(),
		healthErr:   atomic.NewString(""),
	}, nil
}

// getBaseTarget returns the scrape target of the component's own metrics
// endpoint for a database instance.
func getBaseTarget(opts component.Options, instanceKey string) (discovery.Target, error) {
	data, err := opts.GetServiceData(http_service.ServiceName)
	if err != nil {
		return discovery.EmptyTarget, fmt.Errorf("failed to get HTTP information: %w", err)
	}
	httpData := data.(http_service.Data)

	return discovery.NewTargetFromMap(map[string]string{
		model.AddressLabel:     httpData.MemoryListenAddr,
		model.SchemeLabel:      "http",
		model.MetricsPathLabel: path.Join(httpData.HTTPPathForComponent(opts.ID), "metrics"),
		"instance":             instanceKey,
		"job":                  database_observability.JobName,
	}), nil
}

// instanceKey returns a connection-string-derived identifier for the SQL Server
// instance, in the form "host:port/database". This mirrors what mysql/postgres
// components use to label metrics and logs.
func instanceKey(dsn string) (string, error) {
	cfg, err := msdsn.Parse(dsn)
	if err != nil {
		return "", err
	}

	host := cfg.Host
	if host == "" {
		host = "localhost"
	}

	port := cfg.Port
	if port == 0 {
		port = 1433
	}

	return fmt.Sprintf("%s:%d/%s", host, port, cfg.Database), nil
}

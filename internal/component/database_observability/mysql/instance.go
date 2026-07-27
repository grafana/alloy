package mysql

import (
	"database/sql"
	"fmt"
	"path"

	"github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/discovery"
	http_service "github.com/grafana/alloy/internal/service/http"
)

// dbInstance holds the per-database state of the component: the identity,
// connection, collectors, and metrics registry of a single monitored MySQL
// server.
type dbInstance struct {
	instanceKey       string
	baseTarget        discovery.Target
	registry          *prometheus.Registry
	collectors        []Collector
	dbConnection      *sql.DB
	healthErr         *atomic.String
	exporterCollector prometheus.Collector
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

// instanceKey returns network(hostname:port)/dbname of the MySQL server.
// This is the same key as used by the mysqld_exporter integration.
func instanceKey(dsn string) (string, error) {
	m, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", err
	}

	if m.Addr == "" {
		m.Addr = "localhost:3306"
	}
	if m.Net == "" {
		m.Net = "tcp"
	}

	return fmt.Sprintf("%s(%s)/%s", m.Net, m.Addr, m.DBName), nil
}

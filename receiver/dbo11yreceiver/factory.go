package dbo11yreceiver

import (
	"context"
	"database/sql"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

var (
	typeStr   = component.MustNewType("dbo11y")
	stability = component.StabilityLevelDevelopment
)

// NewFactory creates a factory for the db-o11y receiver.
func NewFactory() receiver.Factory {
	return newFactory(sql.Open)
}

func newFactory(openDB func(driver, dsn string) (*sql.DB, error)) receiver.Factory {
	f := &factory{openDB: openDB, receivers: make(map[*Config]*dbo11yReceiver)}
	return receiver.NewFactory(
		typeStr,
		func() component.Config { return createDefaultConfig() },
		receiver.WithMetrics(f.createMetrics, stability),
		receiver.WithLogs(f.createLogs, stability),
	)
}

// factory hands out one dbo11yReceiver per config, so a receiver ID on a
// metrics and a logs pipeline shares one instance per database, one
// connection, and one server_id. It's the sharedcomponent pattern otlpreceiver
// uses; that package is internal to the collector module.
type factory struct {
	openDB func(driver, dsn string) (*sql.DB, error)

	mu        sync.Mutex
	receivers map[*Config]*dbo11yReceiver
}

func (f *factory) get(cfg *Config, set receiver.Settings) *dbo11yReceiver {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.receivers[cfg]; ok {
		return r
	}
	r := newReceiver(cfg, set, f.openDB, func() {
		f.mu.Lock()
		delete(f.receivers, cfg)
		f.mu.Unlock()
	})
	f.receivers[cfg] = r
	return r
}

func (f *factory) createMetrics(_ context.Context, set receiver.Settings, cfg component.Config, next consumer.Metrics) (receiver.Metrics, error) {
	r := f.get(cfg.(*Config), set)
	r.setMetricsConsumer(next)
	return r, nil
}

func (f *factory) createLogs(_ context.Context, set receiver.Settings, cfg component.Config, next consumer.Logs) (receiver.Logs, error) {
	r := f.get(cfg.(*Config), set)
	r.setLogsConsumer(next)
	return r, nil
}

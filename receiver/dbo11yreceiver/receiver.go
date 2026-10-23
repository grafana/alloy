package dbo11yreceiver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	_ "github.com/lib/pq" // the "postgres" driver dbinstance opens
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/dbinstance"
)

// maxLogBatch caps how many queued entries go into one ConsumeLogs call.
const maxLogBatch = 100

// dbo11yReceiver runs every configured database for both signals. One
// goroutine per database owns its dbinstance.Instance, which isn't safe for
// concurrent use.
type dbo11yReceiver struct {
	cfg      *Config
	set      receiver.Settings
	logger   *slog.Logger
	openDB   func(driver, dsn string) (*sql.DB, error)
	onRemove func()

	metricsConsumer consumer.Metrics
	logsConsumer    consumer.Logs

	startOnce    sync.Once
	startErr     error
	shutdownOnce sync.Once
	shutdownErr  error

	cancel   context.CancelFunc
	entries  chan loki.Entry
	dbs      []*database
	runWG    sync.WaitGroup
	pumpDone chan struct{}

	statusMu   sync.Mutex
	host       component.Host
	dbErrs     map[int]string
	lastStatus string

	connectErrors metric.Int64Counter
	gatherErrors  metric.Int64Counter
	consumeErrors metric.Int64Counter
}

type database struct {
	inst *dbinstance.Instance
	// connectedAt is when Connect last got as far as computing server_id.
	connectedAt time.Time
	// targetLabels are job, instance and the MetricRules output, nil before
	// Connect computes server_id.
	targetLabels map[string]string
	// connectErr is the last Connect's error, "" after a clean one.
	connectErr string
}

func newReceiver(cfg *Config, set receiver.Settings, openDB func(string, string) (*sql.DB, error), onRemove func()) *dbo11yReceiver {
	r := &dbo11yReceiver{
		cfg:      cfg,
		set:      set,
		logger:   slog.New(newZapHandler(set.Logger)),
		openDB:   openDB,
		onRemove: onRemove,
		entries:  make(chan loki.Entry),
		dbErrs:   make(map[int]string),
	}
	meter := set.MeterProvider.Meter(scopeName)
	r.connectErrors, _ = meter.Int64Counter("dbo11y_receiver_connect_errors",
		metric.WithDescription("Failed database connection attempts."))
	r.gatherErrors, _ = meter.Int64Counter("dbo11y_receiver_gather_errors",
		metric.WithDescription("Registry gathers that returned an error."))
	r.consumeErrors, _ = meter.Int64Counter("dbo11y_receiver_consume_errors",
		metric.WithDescription("Batches the next consumer rejected."))
	return r
}

func (r *dbo11yReceiver) setMetricsConsumer(c consumer.Metrics) { r.metricsConsumer = c }
func (r *dbo11yReceiver) setLogsConsumer(c consumer.Logs)       { r.logsConsumer = c }

// Start starts every database once, however many pipelines share the
// receiver. An unreachable database doesn't fail Start: it's retried every
// ReconnectInterval and reported as a recoverable error.
func (r *dbo11yReceiver) Start(_ context.Context, host component.Host) error {
	r.startOnce.Do(func() { r.startErr = r.start(host) })
	return r.startErr
}

func (r *dbo11yReceiver) start(host component.Host) error {
	r.host = host
	for _, dbCfg := range r.cfg.Databases {
		inst, err := dbinstance.New(r.cfg.instanceConfig(dbCfg), dbinstance.Deps{
			Logger:  r.logger,
			Entries: loki.NewEntryHandler(r.entries, func() {}),
			OpenDB:  r.openDB,
		})
		if err != nil {
			for _, d := range r.dbs {
				d.inst.Stop()
			}
			r.dbs = nil
			return err
		}
		r.dbs = append(r.dbs, &database{inst: inst})
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.pumpDone = make(chan struct{})
	go r.pumpLogs(ctx)
	for i, d := range r.dbs {
		r.runWG.Go(func() { r.runDatabase(ctx, i, d) })
	}
	return nil
}

func (r *dbo11yReceiver) runDatabase(ctx context.Context, idx int, d *database) {
	if r.connect(ctx, idx, d) {
		r.scrape(ctx, d)
	}

	scrape := time.NewTicker(r.cfg.ScrapeInterval)
	defer scrape.Stop()
	reconnect := time.NewTicker(r.cfg.ReconnectInterval)
	defer reconnect.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reconnect.C:
			if !d.inst.Started() && r.connect(ctx, idx, d) {
				r.scrape(ctx, d)
			}
		case <-scrape.C:
			r.scrape(ctx, d)
			// Like the component, a connect error sticks until a clean Connect.
			if d.connectErr != "" {
				continue
			}
			if ok, msg := d.inst.Health(); !ok {
				r.setDBStatus(idx, "collectors stopped: "+msg)
			} else {
				r.setDBStatus(idx, "")
			}
		}
	}
}

// connect runs Connect and reports whether target labels are known, which
// they are once Connect got as far as server_id, even if collectors then
// failed to start. That matches when the Alloy component exports targets.
func (r *dbo11yReceiver) connect(ctx context.Context, idx int, d *database) bool {
	err := d.inst.Connect(ctx)
	if ctx.Err() != nil {
		return false
	}
	if rules := d.inst.MetricRules(); rules != nil {
		// POC-HACK: no per-database rule blocks in the receiver config yet
		// (relabel.Config has no mapstructure decoding), so none are applied.
		d.targetLabels = targetLabels(d.inst.InstanceKey(), rules, nil)
		d.connectedAt = time.Now()
	}
	d.connectErr = ""
	if err != nil {
		r.connectErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("instance", d.inst.InstanceKey())))
		r.logger.Error("failed to connect", "instance", d.inst.InstanceKey(), "err", err)
		d.connectErr = fmt.Sprintf("%s: %v", d.inst.InstanceKey(), err)
	}
	r.setDBStatus(idx, d.connectErr)
	return d.targetLabels != nil
}

// targetLabels is the label set the Alloy component's scrape target carries:
// job and instance, then the server_id and cloud provider rules from
// MetricRules, then the per-database rules. server_id comes only from
// MetricRules, which dbinstance.Connect computes.
func targetLabels(instanceKey string, builtin, rules []*relabel.Config) map[string]string {
	lb := mapBuilder{
		labelJob:      database_observability.JobName,
		labelInstance: instanceKey,
	}
	if !relabel.ProcessBuilder(lb, builtin...) || !database_observability.ApplyRules(lb, rules) {
		return nil
	}
	return lb
}

func (r *dbo11yReceiver) scrape(ctx context.Context, d *database) {
	if r.metricsConsumer == nil || d.targetLabels == nil {
		return
	}
	families, err := d.inst.Registry().Gather()
	if err != nil {
		// Gather returns what it could alongside the error, so keep going.
		r.gatherErrors.Add(ctx, 1)
		r.logger.Warn("gathering metrics failed", "instance", d.inst.InstanceKey(), "err", err)
	}
	if len(families) == 0 {
		return
	}
	md := pmetric.NewMetrics()
	appendMetrics(md, families, d.targetLabels, time.Now(), d.connectedAt)
	if err := r.metricsConsumer.ConsumeMetrics(ctx, md); err != nil {
		r.consumeErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("signal", "metrics")))
		r.logger.Warn("next consumer rejected metrics", "err", err)
	}
}

// pumpLogs drains every database's entries into the logs consumer, or drops
// them on a metrics-only receiver.
func (r *dbo11yReceiver) pumpLogs(ctx context.Context) {
	defer close(r.pumpDone)
	for {
		var batch []loki.Entry
		select {
		case <-ctx.Done():
			return
		case e := <-r.entries:
			batch = append(batch, e)
		}
	fill:
		for len(batch) < maxLogBatch {
			select {
			case e := <-r.entries:
				batch = append(batch, e)
			default:
				break fill
			}
		}
		if r.logsConsumer == nil {
			continue
		}
		if err := r.logsConsumer.ConsumeLogs(ctx, entriesToLogs(batch, time.Now())); err != nil {
			r.consumeErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("signal", "logs")))
			r.logger.Warn("next consumer rejected logs", "err", err)
		}
	}
}

// Shutdown stops every database and returns by ctx's deadline even when the
// logs consumer never returns. In that case the pump goroutine exits once
// the consumer does.
func (r *dbo11yReceiver) Shutdown(ctx context.Context) error {
	r.shutdownOnce.Do(func() {
		r.shutdownErr = r.shutdown(ctx)
		r.onRemove()
	})
	return r.shutdownErr
}

func (r *dbo11yReceiver) shutdown(ctx context.Context) error {
	if r.cancel == nil {
		return nil
	}
	r.cancel()

	// Collectors send entries without a timeout, so keep the channel drained
	// while they stop, even if the pump is stuck in ConsumeLogs.
	drainStop := make(chan struct{})
	go func() {
		for {
			select {
			case <-r.entries:
			case <-drainStop:
				return
			}
		}
	}()

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		defer close(drainStop)
		r.runWG.Wait()
		for _, d := range r.dbs {
			d.inst.Stop()
		}
	}()

	var errs []error
	select {
	case <-stopped:
	case <-ctx.Done():
		errs = append(errs, fmt.Errorf("databases didn't stop in time: %w", ctx.Err()))
	}
	select {
	case <-r.pumpDone:
	case <-ctx.Done():
		errs = append(errs, fmt.Errorf("logs consumer didn't return in time: %w", ctx.Err()))
	}
	return errors.Join(errs...)
}

// setDBStatus records one database's problem ("" for none) and reports the
// receiver's status to the host when it changes: a recoverable error while
// any database is unreachable or has stopped collectors, OK otherwise.
func (r *dbo11yReceiver) setDBStatus(idx int, problem string) {
	r.statusMu.Lock()
	defer r.statusMu.Unlock()
	if problem == "" {
		delete(r.dbErrs, idx)
	} else {
		r.dbErrs[idx] = problem
	}

	var errs []error
	for i := range r.dbs {
		if p, ok := r.dbErrs[i]; ok {
			errs = append(errs, errors.New(p))
		}
	}
	err := errors.Join(errs...)
	status := ""
	if err != nil {
		status = err.Error()
	}
	if status == r.lastStatus {
		return
	}
	r.lastStatus = status
	if err != nil {
		componentstatus.ReportStatus(r.host, componentstatus.NewRecoverableErrorEvent(err))
	} else {
		componentstatus.ReportStatus(r.host, componentstatus.NewEvent(componentstatus.StatusOK))
	}
}

// mapBuilder is a relabel.LabelBuilder over a plain map.
type mapBuilder map[string]string

func (b mapBuilder) Get(label string) string { return b[label] }

func (b mapBuilder) Range(f func(label, value string)) {
	for k, v := range b {
		f(k, v)
	}
}

func (b mapBuilder) Set(label, val string) {
	if val == "" {
		delete(b, label)
		return
	}
	b[label] = val
}

func (b mapBuilder) Del(labels ...string) {
	for _, l := range labels {
		delete(b, l)
	}
}

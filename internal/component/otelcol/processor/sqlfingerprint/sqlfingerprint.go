// Package sqlfingerprint implements the otelcol.processor.sql_fingerprint component.
package sqlfingerprint

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fanoutconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/interceptconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazyconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/livedebuggingpublisher"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/livedebugging"
	queryfingerprint "github.com/grafana/alloy/sqlfingerprint"
	"github.com/grafana/alloy/syntax"
)

const fingerprintAttribute = "db.query.fingerprint"

// init registers the component and its stable consumer export with Alloy.
func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.sql_fingerprint",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},
		Build: func(o component.Options, a component.Arguments) (component.Component, error) {
			return New(o, a.(Arguments))
		},
	})
}

// Arguments configures the offline normalizer and the downstream trace pipeline.
type Arguments struct {
	PostgreSQLVersion            int                        `alloy:"postgresql_version,attr,optional"`
	PostgreSQLBackslashEscapes   bool                       `alloy:"postgresql_backslash_escapes,attr,optional"`
	MySQLANSIQuotes              bool                       `alloy:"mysql_ansi_quotes,attr,optional"`
	MySQLNoBackslashEscapes      bool                       `alloy:"mysql_no_backslash_escapes,attr,optional"`
	SQLServerQuotedIdentifierOff bool                       `alloy:"sql_server_quoted_identifier_off,attr,optional"`
	MaxQueryBytes                int                        `alloy:"max_query_bytes,attr,optional"`
	Output                       *otelcol.ConsumerArguments `alloy:"output,block"`
}

var (
	_ syntax.Defaulter = (*Arguments)(nil)
	_ syntax.Validator = (*Arguments)(nil)
)

// SetToDefault selects standard SQL modes and PostgreSQL 18 list normalization.
func (a *Arguments) SetToDefault() {
	*a = Arguments{PostgreSQLVersion: 18, MaxQueryBytes: 1 << 20}
}

// Validate rejects unsupported output signals and invalid normalizer options.
func (a *Arguments) Validate() error {
	if a.PostgreSQLVersion < 16 || a.PostgreSQLVersion > 18 {
		return fmt.Errorf("postgresql_version must be 16, 17, or 18")
	}
	if a.MaxQueryBytes < 1 {
		return fmt.Errorf("max_query_bytes must be positive")
	}
	if a.Output == nil {
		return fmt.Errorf("output is required")
	}
	if len(a.Output.Metrics) != 0 || len(a.Output.Logs) != 0 {
		return fmt.Errorf("only traces are supported in output")
	}
	_, err := queryfingerprint.New(a.options())
	return err
}

// options translates Alloy syntax into the exact public backend library options.
func (a *Arguments) options() queryfingerprint.Options {
	return queryfingerprint.Options{
		PostgreSQLVersion:            a.PostgreSQLVersion,
		PostgreSQLBackslashEscapes:   a.PostgreSQLBackslashEscapes,
		MySQLANSIQuotes:              a.MySQLANSIQuotes,
		MySQLNoBackslashEscapes:      a.MySQLNoBackslashEscapes,
		SQLServerQuotedIdentifierOff: a.SQLServerQuotedIdentifierOff,
		MaxQueryBytes:                a.MaxQueryBytes,
	}
}

// Component owns a stable input and counters. Reloads atomically replace the
// consumer closure, so each request uses one complete configuration snapshot.
type Component struct {
	input     *lazyconsumer.Consumer
	opts      component.Options
	publisher livedebugging.DebugDataPublisher
	spans     *prometheus.CounterVec
	failures  *prometheus.CounterVec
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// New validates configuration before registering metrics or publishing input.
func New(o component.Options, args Arguments) (*Component, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}
	data, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}
	publisher, ok := data.(livedebugging.DebugDataPublisher)
	if !ok {
		return nil, fmt.Errorf("invalid live debugging service")
	}
	c := &Component{input: lazyconsumer.New(context.Background(), o.ID), opts: o, publisher: publisher}
	if err := c.registerMetrics(); err != nil {
		return nil, err
	}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	o.OnStateChange(otelcol.ConsumerExports{Input: c.input})
	return c, nil
}

// registerMetrics installs bounded labels. SQL, fingerprints, and database names
// never become metric labels, which avoids both leakage and unbounded cardinality.
func (c *Component) registerMetrics() error {
	c.spans = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "otelcol_processor_sql_fingerprint_spans_total",
		Help: "Spans with SQL text processed, by database system and outcome.",
	}, []string{"db_system", "outcome"})
	c.failures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "otelcol_processor_sql_fingerprint_failures_total",
		Help: "SQL fingerprint failures by database system and reason.",
	}, []string{"db_system", "reason"})
	if err := c.opts.Registerer.Register(c.spans); err != nil {
		return err
	}
	if err := c.opts.Registerer.Register(c.failures); err != nil {
		c.opts.Registerer.Unregister(c.spans)
		return err
	}
	return nil
}

// Update swaps consumers only after constructing a valid immutable normalizer.
// Mutating capability causes lazyconsumer to clone shared upstream data first.
func (c *Component) Update(newConfig component.Arguments) error {
	args := newConfig.(Arguments)
	if err := args.Validate(); err != nil {
		return err
	}
	f, err := queryfingerprint.New(args.options())
	if err != nil {
		return err
	}
	next := fanoutconsumer.Traces(args.Output.Traces)
	metadata := otelcol.GetComponentMetadata(args.Output.Traces)
	consumer := interceptconsumer.TracesMutating(next, func(ctx context.Context, traces ptrace.Traces) error {
		c.processTraces(f, traces)
		livedebuggingpublisher.PublishTracesIfActive(c.publisher, c.opts.ID, traces, metadata)
		return next.ConsumeTraces(ctx, traces)
	})
	c.input.SetConsumers(consumer, nil, nil)
	return nil
}

// processTraces visits spans without changing resource attributes or timing.
func (c *Component) processTraces(f *queryfingerprint.Fingerprinter, traces ptrace.Traces) {
	for _, resource := range traces.ResourceSpans().All() {
		for _, scope := range resource.ScopeSpans().All() {
			for _, span := range scope.Spans().All() {
				c.processSpan(f, span.Attributes())
			}
		}
	}
}

// processSpan owns db.query.fingerprint for recognized SQL spans. Failed
// statements cannot retain a stale hash from an earlier pass through this pipeline.
// Every span is forwarded, including spans for which no fingerprint can be made.
func (c *Component) processSpan(f *queryfingerprint.Fingerprinter, attributes pcommon.Map) {
	query := stringAttribute(attributes, "db.query.text", "db.statement")
	if query == "" {
		return
	}
	system := stringAttribute(attributes, "db.system.name", "db.system")
	dialect, ok := queryfingerprint.ParseDialect(system)
	if !ok {
		c.spans.WithLabelValues("unknown", "skipped").Inc()
		c.failures.WithLabelValues("unknown", "unsupported_db_system").Inc()
		return
	}
	attributes.Remove(fingerprintAttribute)
	result := f.Fingerprint(dialect, query)
	for _, failure := range result.Failures {
		c.failures.WithLabelValues(string(dialect), string(failure.Reason)).Inc()
	}
	if len(result.Fingerprints) == 0 {
		c.spans.WithLabelValues(string(dialect), "skipped").Inc()
		return
	}
	values := attributes.PutEmptySlice(fingerprintAttribute)
	for _, fingerprint := range result.Fingerprints {
		values.AppendEmpty().SetStr(fingerprint)
	}
	c.spans.WithLabelValues(string(dialect), "fingerprinted").Inc()
}

// stringAttribute prefers a nonempty current semantic-convention attribute,
// falling back to the legacy spelling. Non-string values are never coerced to SQL.
func stringAttribute(attributes pcommon.Map, current, legacy string) string {
	for _, key := range []string{current, legacy} {
		if v, ok := attributes.Get(key); ok && v.Type() == pcommon.ValueTypeStr && v.Str() != "" {
			return v.Str()
		}
	}
	return ""
}

// Run has no background work or database connections; consumption is synchronous.
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// LiveDebugging advertises the processed trace stream to Alloy's debug service.
func (c *Component) LiveDebugging() {}

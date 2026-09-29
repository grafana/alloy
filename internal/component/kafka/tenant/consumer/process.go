package consumer

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/user"
	"github.com/klauspost/compress/zstd"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"

	"github.com/grafana/alloy/internal/component/kafka/tenant/kafkaclient"
	lokisource "github.com/grafana/alloy/internal/component/loki/source"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/pyroscope"
	pyroreceive "github.com/grafana/alloy/internal/component/pyroscope/receive_http"
	pyrowrite "github.com/grafana/alloy/internal/component/pyroscope/write"
)

// errDecode marks errors that retrying cannot fix. Records failing with it
// are dropped so they never block a partition.
var errDecode = errors.New("decode error")

func decodeErr(err error) error { return fmt.Errorf("%w: %w", errDecode, err) }

// errNoDownstream is returned when no component is configured to receive a
// record's signal.
var errNoDownstream = errors.New("no downstream configured")

// processRecord forwards one record downstream, retrying downstream errors
// with bounded backoff. It returns false if ctx was canceled before the
// record was finished, in which case its offset must not be committed.
func (c *Component) processRecord(ctx context.Context, args Arguments, r *kgo.Record) bool {
	var (
		signal = kafkaclient.Header(r, kafkaclient.HeaderSignal)
		format = kafkaclient.Header(r, kafkaclient.HeaderFormat)
		start  = time.Now()
	)
	defer func() { c.metrics.processDuration.WithLabelValues(format).Observe(time.Since(start).Seconds()) }()

	tenant, reason := c.guard(r)
	if reason != "" {
		c.drop(r, reason, format, nil)
		return true
	}

	bo := backoff.New(ctx, backoff.Config{
		MinBackoff: args.MinBackoff,
		MaxBackoff: args.MaxBackoff,
		MaxRetries: args.MaxRetries + 1, // The first attempt counts as a retry.
	})
	for {
		err := c.forward(ctx, args, tenant, r)
		switch {
		case err == nil:
			c.metrics.recordsConsumed.WithLabelValues(signal, format).Inc()
			return true
		case errors.Is(err, errDecode):
			c.drop(r, "decode_error", format, err)
			return true
		case errors.Is(err, errNoDownstream):
			c.drop(r, "no_downstream", format, err)
			return true
		case ctx.Err() != nil:
			return false
		}

		// Only this worker's partition waits while retrying.
		bo.Wait()
		if !bo.Ongoing() {
			if ctx.Err() != nil {
				return false
			}
			c.drop(r, "downstream_error", format, err)
			return true
		}
		c.opts.Logger.Debug("retrying record", "partition", r.Partition, "offset", r.Offset, "err", err)
	}
}

// guard returns the tenant owning r's partition, or a drop reason if r must
// not be processed.
func (c *Component) guard(r *kgo.Record) (tenant, reason string) {
	reg := c.registry.Load()
	expected, ok := reg.TenantFor(r.Partition)
	if !ok {
		return "", "unassigned_partition"
	}
	if kafkaclient.Header(r, kafkaclient.HeaderTenantID) != expected {
		return "", "tenant_mismatch"
	}
	if topic, ok := reg.TopicFor(kafkaclient.Header(r, kafkaclient.HeaderSignal)); !ok || topic != r.Topic {
		return "", "topic_mismatch"
	}
	if v := kafkaclient.Header(r, kafkaclient.HeaderSchemaVersion); v != kafkaclient.SchemaVersion {
		return "", "unsupported_schema_version"
	}
	return expected, ""
}

func (c *Component) drop(r *kgo.Record, reason, format string, err error) {
	c.metrics.dropped.WithLabelValues(reason, format).Inc()
	// Never log the payload: it belongs to a tenant.
	c.opts.Logger.Warn("dropping record", "reason", reason, "format", format,
		"partition", r.Partition, "offset", r.Offset, "err", err)
}

func (c *Component) forward(ctx context.Context, args Arguments, tenant string, r *kgo.Record) error {
	ctx = user.InjectOrgID(ctx, tenant)

	switch format := kafkaclient.Header(r, kafkaclient.HeaderFormat); format {
	case kafkaclient.FormatPromRWv1:
		if len(args.MetricsForwardTo) == 0 {
			return errNoDownstream
		}
		return c.forwardPromRW(ctx, r)
	case kafkaclient.FormatLokiPush:
		if len(args.LogsForwardTo) == 0 {
			return errNoDownstream
		}
		return c.forwardLoki(ctx, tenant, r)
	case kafkaclient.FormatPyroscopeIngest:
		if len(args.ProfilesForwardTo) == 0 {
			return errNoDownstream
		}
		return c.forwardPyroscope(ctx, r)
	case kafkaclient.FormatOTLP:
		return forwardOTLP(ctx, args.Output, tenant, r)
	default:
		return decodeErr(fmt.Errorf("unknown format %q", format))
	}
}

// replayRequest rebuilds the original ingest request from r.
func replayRequest(ctx context.Context, r *kgo.Record) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kafkaclient.Header(r, kafkaclient.HeaderURL), bytes.NewReader(r.Value))
	if err != nil {
		return nil, decodeErr(err)
	}
	req.ContentLength = int64(len(r.Value))
	if v := kafkaclient.Header(r, kafkaclient.HeaderContentType); v != "" {
		req.Header.Set("Content-Type", v)
	}
	if v := kafkaclient.Header(r, kafkaclient.HeaderContentEncoding); v != "" {
		req.Header.Set("Content-Encoding", v)
	}
	return req, nil
}

// forwardPromRW decodes the record with the upstream remote-write handler, as
// prometheus.receive_http does. The handler appends with the request context,
// so the tenant reaches every appender.
func (c *Component) forwardPromRW(ctx context.Context, r *kgo.Record) error {
	req, err := replayRequest(ctx, r)
	if err != nil {
		return err
	}
	rec := httptest.NewRecorder()
	c.promHandler.ServeHTTP(rec, req)

	switch rec.Code / 100 {
	case 2:
		return nil
	case 4:
		return decodeErr(fmt.Errorf("remote write handler returned %d: %s", rec.Code, strings.TrimSpace(rec.Body.String())))
	default:
		return fmt.Errorf("remote write handler returned %d: %s", rec.Code, strings.TrimSpace(rec.Body.String()))
	}
}

// forwardLoki decodes the record with the loki.source.api push parser, which
// sets __tenant_id__ from the request's X-Scope-OrgID header.
func (c *Component) forwardLoki(ctx context.Context, tenant string, r *kgo.Record) error {
	req, err := replayRequest(ctx, r)
	if err != nil {
		return err
	}
	req.Header.Set(user.OrgIDHeaderName, tenant)

	entries, _, err := c.lokiRoute.Logs(req, &lokisource.LogsConfig{UseIncomingTimestamp: true})
	if err != nil {
		if len(entries) == 0 {
			return decodeErr(err)
		}
		// Forward what could be parsed; the rest is lost.
		c.metrics.dropped.WithLabelValues("decode_error", kafkaclient.FormatLokiPush).Inc()
		c.opts.Logger.Warn("partially failed to decode loki push record", "partition", r.Partition, "offset", r.Offset, "err", err)
	}
	return c.lokiFanout.SendBatch(ctx, entries)
}

// forwardPyroscope builds the profile the same way pyroscope.receive_http does
// for /ingest.
func (c *Component) forwardPyroscope(ctx context.Context, r *kgo.Record) error {
	u, err := url.ParseRequestURI(kafkaclient.Header(r, kafkaclient.HeaderURL))
	if err != nil {
		return decodeErr(err)
	}
	lbls, err := pyroreceive.LabelsFromIngestURL(u)
	if err != nil {
		c.opts.Logger.Warn("failed to parse labels from name parameter", "partition", r.Partition, "offset", r.Offset, "err", err)
	}

	profile := &pyroscope.IncomingProfile{
		RawBody: r.Value,
		URL:     u,
		Labels:  lbls,
	}
	if ct := kafkaclient.Header(r, kafkaclient.HeaderContentType); ct != "" {
		profile.ContentType = []string{ct}
	}

	err = c.pyroFanout.Appender().AppendIngest(ctx, profile)
	writeErr, ok := errors.AsType[*pyrowrite.PyroscopeWriteError](err)
	if ok && writeErr.StatusCode/100 == 4 && writeErr.StatusCode != http.StatusTooManyRequests {
		return decodeErr(err)
	}
	return err
}

// forwardOTLP decodes an OTLP/HTTP request body and passes it to the output
// consumers with X-Scope-OrgID in the client metadata, which
// otelcol.processor.batch (metadata_keys) and otelcol.auth.headers
// (from_context) understand.
func forwardOTLP(ctx context.Context, output *otelcol.ConsumerArguments, tenant string, r *kgo.Record) error {
	body, err := decompress(kafkaclient.Header(r, kafkaclient.HeaderContentEncoding), r.Value)
	if err != nil {
		return decodeErr(err)
	}
	isJSON := strings.HasPrefix(kafkaclient.Header(r, kafkaclient.HeaderContentType), "application/json")
	unmarshal := func(m otlpUnmarshaler) error {
		if isJSON {
			return m.UnmarshalJSON(body)
		}
		return m.UnmarshalProto(body)
	}

	ctx = client.NewContext(ctx, client.Info{
		Metadata: client.NewMetadata(map[string][]string{user.OrgIDHeaderName: {tenant}}),
	})
	if output == nil {
		output = &otelcol.ConsumerArguments{}
	}

	switch signal := kafkaclient.Header(r, kafkaclient.HeaderSignal); signal {
	case kafkaclient.SignalMetrics:
		req := pmetricotlp.NewExportRequest()
		if err := unmarshal(req); err != nil {
			return decodeErr(err)
		}
		return fanout(output.Metrics, req.Metrics(), func(md pmetric.Metrics) pmetric.Metrics {
			out := pmetric.NewMetrics()
			md.CopyTo(out)
			return out
		}, func(c otelcol.Consumer, md pmetric.Metrics) error { return c.ConsumeMetrics(ctx, md) })
	case kafkaclient.SignalLogs:
		req := plogotlp.NewExportRequest()
		if err := unmarshal(req); err != nil {
			return decodeErr(err)
		}
		return fanout(output.Logs, req.Logs(), func(ld plog.Logs) plog.Logs {
			out := plog.NewLogs()
			ld.CopyTo(out)
			return out
		}, func(c otelcol.Consumer, ld plog.Logs) error { return c.ConsumeLogs(ctx, ld) })
	case kafkaclient.SignalTraces:
		req := ptraceotlp.NewExportRequest()
		if err := unmarshal(req); err != nil {
			return decodeErr(err)
		}
		return fanout(output.Traces, req.Traces(), func(td ptrace.Traces) ptrace.Traces {
			out := ptrace.NewTraces()
			td.CopyTo(out)
			return out
		}, func(c otelcol.Consumer, td ptrace.Traces) error { return c.ConsumeTraces(ctx, td) })
	default:
		return decodeErr(fmt.Errorf("unknown OTLP signal %q", signal))
	}
}

type otlpUnmarshaler interface {
	UnmarshalProto([]byte) error
	UnmarshalJSON([]byte) error
}

// fanout passes data to every consumer, giving each but the last its own copy
// since consumers may mutate it.
func fanout[T any](consumers []otelcol.Consumer, data T, clone func(T) T, consume func(otelcol.Consumer, T) error) error {
	if len(consumers) == 0 {
		return errNoDownstream
	}
	var errs []error
	for i, c := range consumers {
		d := data
		if i < len(consumers)-1 {
			d = clone(data)
		}
		if err := consume(c, d); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func decompress(encoding string, body []byte) ([]byte, error) {
	var (
		rd  io.Reader
		err error
	)
	switch encoding {
	case "", "identity":
		return body, nil
	case "gzip":
		rd, err = gzip.NewReader(bytes.NewReader(body))
	case "deflate", "zlib":
		rd, err = zlib.NewReader(bytes.NewReader(body))
	case "zstd":
		var zr *zstd.Decoder
		zr, err = zstd.NewReader(bytes.NewReader(body))
		if err == nil {
			defer zr.Close()
			rd = zr
		}
	default:
		return nil, fmt.Errorf("unsupported content encoding %q", encoding)
	}
	if err != nil {
		return nil, err
	}
	return io.ReadAll(rd)
}

// Package loki provides an otelcol.receiver.loki component.
package loki

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/grafana/loki/pkg/push"
	loki_translator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fanoutconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/interceptconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/livedebuggingpublisher"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/service/lokipipelinev2"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.loki",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(o component.Options, a component.Arguments) (component.Component, error) {
			return New(o, a.(Arguments))
		},
	})
}

const hintAttributes = "loki.attribute.labels"

// Arguments configures the otelcol.receiver.loki component.
type Arguments struct {
	// LabelsAsResourceAttributes maps stream labels to resource attributes instead of log attributes.
	LabelsAsResourceAttributes bool `alloy:"labels_as_resource_attributes,attr,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

// Exports holds the receiver that is used to send log entries to the
// loki.write component.
type Exports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr"`
}

// Component is the otelcol.receiver.loki component.
type Component struct {
	opts       component.Options
	pipelineV2 bool

	mut              sync.RWMutex
	stopped          bool
	receiver         loki.LogsReceiver
	logsSink         consumer.Logs
	labelsAsResource bool

	debugDataPublisher livedebugging.DebugDataPublisher
}

var (
	_ component.Component     = (*Component)(nil)
	_ loki.Consumer           = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// New creates a new otelcol.receiver.loki component.
func New(o component.Options, c Arguments) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	lokiSvc, err := o.GetServiceData(lokipipelinev2.ServiceName)
	if err != nil {
		return nil, err
	}

	// TODO(@tpaschalis) Create a metrics struct to count
	// total/successful/errored log entries?
	res := &Component{
		opts:               o,
		pipelineV2:         lokiSvc.(lokipipelinev2.Data).PipelineV2Enabled(),
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	// Create and immediately export the receiver which remains the same for
	// the component's lifetime.
	if res.pipelineV2 {
		res.receiver = loki.NewLogsReceiverV2(res, o.ID)
	} else {
		res.receiver = loki.NewLogsReceiver(loki.WithComponentID(o.ID))
	}
	o.OnStateChange(Exports{Receiver: res.receiver})

	if err := res.Update(c); err != nil {
		return nil, err
	}
	return res, nil
}

// Run implements Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.mut.Lock()
		defer c.mut.Unlock()
		c.stopped = true
	}()

	if c.pipelineV2 {
		<-ctx.Done()
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.receiver.Chan():
			c.mut.RLock()
			if err := c.logsSink.ConsumeLogs(ctx, convertLokiEntryToPlog(entry, c.labelsAsResource)); err != nil {
				c.opts.Logger.Error("failed to consume log entries", "err", err)
			}
			c.mut.RUnlock()
		}
	}
}

// Update implements Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	var (
		newArgs           = args.(Arguments)
		fanout            = fanoutconsumer.Logs(newArgs.Output.Logs)
		componentMetadata = otelcol.GetComponentMetadata(newArgs.Output.Logs)
	)

	c.logsSink = interceptconsumer.Logs(
		fanout,
		func(ctx context.Context, ld plog.Logs) error {
			livedebuggingpublisher.PublishLogsIfActive(c.debugDataPublisher, c.opts.ID, ld, componentMetadata)
			return fanout.ConsumeLogs(ctx, ld)
		},
	)
	c.labelsAsResource = newArgs.LabelsAsResourceAttributes
	return nil
}

// Consume implements loki.Consumer.
func (c *Component) Consume(ctx context.Context, batch loki.Batch) error {
	c.mut.RLock()
	stopped, sink, labelsAsResource := c.stopped, c.logsSink, c.labelsAsResource
	c.mut.RUnlock()

	if stopped {
		return fmt.Errorf("%s: %w", c.opts.ID, loki.ErrConsumerStopped)
	}

	if batch.EntryLen() == 0 {
		return nil
	}

	logs := plog.NewLogs()
	for _, stream := range batch.Streams() {
		appendLogs(logs, stream.Labels, labelsAsResource, stream.Entries...)
	}

	// Not holding c.mut here so Update is never blocked behind a slow downstream.
	return sink.ConsumeLogs(ctx, logs)
}

func (c *Component) String() string {
	return c.opts.ID + ".receiver"
}

// Create a new Otlp Logs entry from a loki.Entry
func convertLokiEntryToPlog(entry loki.Entry, labelsAsResource bool) plog.Logs {
	logs := plog.NewLogs()
	appendLogs(logs, entry.Labels, labelsAsResource, entry.Entry)
	return logs
}

// appendLogs converts each entry in entries, which all have the labels lset, into a LogRecord in logs.
func appendLogs(logs plog.Logs, lset model.LabelSet, labelsAsResource bool, entries ...push.Entry) {
	var (
		logRecords   plog.LogRecordSlice
		recordLabels = lset
		hint         string
	)

	if labelsAsResource {
		// Each set of labels gets its own ResourceLogs since the labels become the resource.
		rl := logs.ResourceLogs().AppendEmpty()
		putResourceAttributes(rl.Resource().Attributes(), lset)
		logRecords = rl.ScopeLogs().AppendEmpty().LogRecords()
		recordLabels = nil
	} else {
		// Every LogRecord shares one ResourceLogs when labels stay on the log record.
		if logs.ResourceLogs().Len() == 0 {
			logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
		}
		logRecords = logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()

		lbls := make([]string, 0, len(lset))
		for key := range lset {
			lbls = append(lbls, string(key))
		}
		slices.Sort(lbls)
		hint = strings.Join(lbls, ",")
	}
	logRecords.EnsureCapacity(logRecords.Len() + len(entries))

	filename, hasFilename := recordLabels["filename"]

	for _, entry := range entries {
		lr := logRecords.AppendEmpty()

		if hasFilename {
			filenameStr := string(filename)
			// The `lokireceiver` from the opentelemetry-collector-contrib
			// repo adds these two labels based on these "semantic conventions
			// for log media".
			// https://opentelemetry.io/docs/reference/specification/logs/semantic_conventions/media/
			// We're keeping them as well, but we're also adding the `filename`
			// attribute so that it can be used from the
			// `loki.attribute.labels` hint for when the opposite OTel -> Loki
			// transformation happens.
			lr.Attributes().PutStr("log.file.path", filenameStr)
			lr.Attributes().PutStr("log.file.name", path.Base(filenameStr))
			// TODO(@tpaschalis) Remove the addition of "log.file.path" and "log.file.name",
			// because the Collector doesn't do it and we would be more in line with it.
		}

		if hint != "" {
			// This hint is defined in the pkg/translator/loki package and the
			// opentelemetry-collector-contrib repo, but is not exported so we
			// re-define it.
			// It is used to detect which attributes should be promoted to labels
			// when transforming back from OTel -> Loki.
			lr.Attributes().PutStr(hintAttributes, hint)
		}

		loki_translator.ConvertEntryToLogRecord(&entry, &lr, recordLabels, true)
	}
}

// putResourceAttributes puts lset into attrs, dropping internal labels like loki.write does.
func putResourceAttributes(attrs pcommon.Map, lset model.LabelSet) {
	attrs.EnsureCapacity(len(lset))
	for name, value := range lset {
		if strings.HasPrefix(string(name), model.ReservedLabelPrefix) {
			continue
		}
		attrs.PutStr(string(name), string(value))
	}
}

func (c *Component) LiveDebugging() {}

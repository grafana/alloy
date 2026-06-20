// Package loki provides an otelcol.receiver.loki component.
package loki

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"

	loki_translator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fanoutconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/interceptconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/livedebuggingpublisher"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/livedebugging"
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

var hintAttributes = "loki.attribute.labels"

// LabelsConfig controls which Loki labels are forwarded as OTel log record
// attributes and allows renaming them during conversion.
type LabelsConfig struct {
	// Include is an allowlist of label names. When set, only these labels are
	// forwarded as attributes; all others are dropped. Mutually exclusive with
	// Exclude.
	Include []string `alloy:"include,attr,optional"`

	// Exclude is a blocklist of label names. When set, these labels are dropped
	// and all others are forwarded. Mutually exclusive with Include.
	Exclude []string `alloy:"exclude,attr,optional"`

	// Rename maps original label names to new attribute names. Applied after
	// include/exclude filtering.
	Rename map[string]string `alloy:"rename,attr,optional"`
}

// Arguments configures the otelcol.receiver.loki component.
type Arguments struct {
	// Labels optionally controls which Loki labels are forwarded and how they
	// are named in the resulting OTel log record attributes.
	Labels *LabelsConfig `alloy:"labels,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

// Validate implements syntax.Validator.
func (a Arguments) Validate() error {
	if a.Labels != nil && len(a.Labels.Include) > 0 && len(a.Labels.Exclude) > 0 {
		return fmt.Errorf("the \"include\" and \"exclude\" attributes inside the \"labels\" block are mutually exclusive")
	}
	return nil
}

// Exports holds the receiver that is used to send log entries to the
// loki.write component.
type Exports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr"`
}

// Component is the otelcol.receiver.loki component.
type Component struct {
	opts component.Options

	mut      sync.RWMutex
	receiver loki.LogsReceiver
	logsSink consumer.Logs

	debugDataPublisher livedebugging.DebugDataPublisher

	args Arguments
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// New creates a new otelcol.receiver.loki component.
func New(o component.Options, c Arguments) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	// TODO(@tpaschalis) Create a metrics struct to count
	// total/successful/errored log entries?
	res := &Component{
		opts:               o,
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	// Create and immediately export the receiver which remains the same for
	// the component's lifetime.
	res.receiver = loki.NewLogsReceiver()
	o.OnStateChange(Exports{Receiver: res.receiver})

	if err := res.Update(c); err != nil {
		return nil, err
	}
	return res, nil
}

// Run implements Component.
func (c *Component) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.receiver.Chan():
			c.mut.RLock()
			labelsCfg := c.args.Labels
			c.mut.RUnlock()

			logs := convertLokiEntryToPlog(entry, labelsCfg)

			// TODO(@tpaschalis) Is there any more handling to be done here?
			err := c.logsSink.ConsumeLogs(ctx, logs)
			if err != nil {
				c.opts.SLogger.Error("failed to consume log entries", "err", err)
			}
		}
	}
}

// Update implements Component.
func (c *Component) Update(newConfig component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.args = newConfig.(Arguments)
	nextLogs := c.args.Output.Logs
	fanout := fanoutconsumer.Logs(nextLogs)
	logsInterceptor := interceptconsumer.Logs(fanout,
		func(ctx context.Context, ld plog.Logs) error {
			livedebuggingpublisher.PublishLogsIfActive(c.debugDataPublisher, c.opts.ID, ld, otelcol.GetComponentMetadata(nextLogs))
			return fanout.ConsumeLogs(ctx, ld)
		},
	)
	c.logsSink = logsInterceptor

	return nil
}

// filterLabels applies the LabelsConfig to a label set, returning a new
// filtered label set. When cfg is nil, the original set is returned as-is.
func filterLabels(labels model.LabelSet, cfg *LabelsConfig) model.LabelSet {
	if cfg == nil {
		return labels
	}

	filtered := make(model.LabelSet, len(labels))

	if len(cfg.Include) > 0 {
		includeSet := make(map[string]struct{}, len(cfg.Include))
		for _, l := range cfg.Include {
			includeSet[l] = struct{}{}
		}
		for k, v := range labels {
			if _, ok := includeSet[string(k)]; ok {
				filtered[k] = v
			}
		}
	} else if len(cfg.Exclude) > 0 {
		excludeSet := make(map[string]struct{}, len(cfg.Exclude))
		for _, l := range cfg.Exclude {
			excludeSet[l] = struct{}{}
		}
		for k, v := range labels {
			if _, ok := excludeSet[string(k)]; !ok {
				filtered[k] = v
			}
		}
	} else {
		// No include/exclude: copy all labels.
		for k, v := range labels {
			filtered[k] = v
		}
	}

	return filtered
}

// renameLabel returns the renamed key for a label if a rename mapping exists,
// otherwise returns the original key.
func renameLabel(key string, cfg *LabelsConfig) string {
	if cfg != nil && len(cfg.Rename) > 0 {
		if newName, ok := cfg.Rename[key]; ok {
			return newName
		}
	}
	return key
}

// convertLokiEntryToPlog creates a new OTel Logs entry from a Loki entry.
// The optional LabelsConfig controls which labels are forwarded and how they
// are named.
func convertLokiEntryToPlog(lokiEntry loki.Entry, labelsCfg *LabelsConfig) plog.Logs {
	logs := plog.NewLogs()

	lr := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	if filename, exists := lokiEntry.Labels["filename"]; exists {
		filenameStr := string(filename)
		// The `promtailreceiver` from the opentelemetry-collector-contrib
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

	// Apply label filtering.
	filteredLabels := filterLabels(lokiEntry.Labels, labelsCfg)

	// Build the hint attribute key list, applying renames.
	var lbls []string
	for key := range filteredLabels {
		keyStr := renameLabel(string(key), labelsCfg)
		lbls = append(lbls, keyStr)
	}
	sort.Strings(lbls)

	if len(lbls) > 0 {
		// This hint is defined in the pkg/translator/loki package and the
		// opentelemetry-collector-contrib repo, but is not exported so we
		// re-define it.
		// It is used to detect which attributes should be promoted to labels
		// when transforming back from OTel -> Loki.
		lr.Attributes().PutStr(hintAttributes, strings.Join(lbls, ","))
	}

	// Let the upstream translator set timestamps and body from the entry,
	// and add the filtered labels as log record attributes.
	loki_translator.ConvertEntryToLogRecord(&lokiEntry.Entry, &lr, filteredLabels, true)

	// Apply renames: overwrite original label keys with new names.
	if labelsCfg != nil && len(labelsCfg.Rename) > 0 {
		for origName, newName := range labelsCfg.Rename {
			if val, ok := lr.Attributes().Get(origName); ok {
				lr.Attributes().PutStr(newName, val.AsString())
				lr.Attributes().Remove(origName)
			}
		}
	}

	return logs
}

func (c *Component) LiveDebugging() {}

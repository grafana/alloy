package app_agent_receiver

import (
	"context"
	"fmt"
	"time"

	kitlog "github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/go-logfmt/logfmt"
	"github.com/grafana/agent/internal/static/logs"
	"github.com/grafana/loki/clients/pkg/promtail/api"
	"github.com/grafana/loki/pkg/logproto"
	prommodel "github.com/prometheus/common/model"
)

// logsInstance is an interface with capability to send log entries
type logsInstance interface {
	SendEntry(entry api.Entry, dur time.Duration) bool
}

// logsInstanceGetter is a function that returns a LogsInstance to send log entries to
type logsInstanceGetter func() (logsInstance, error)

// LogsExporterConfig holds the configuration of the logs exporter
type LogsExporterConfig struct {
	SendEntryTimeout time.Duration
	GetLogsInstance  logsInstanceGetter
	Labels           map[string]string
}

// LogsExporter will send logs & errors to loki
type LogsExporter struct {
	getLogsInstance  logsInstanceGetter
	sendEntryTimeout time.Duration
	logger           kitlog.Logger
	labels           map[string]string
	sourceMapStore   SourceMapStore
}

// NewLogsExporter creates a new logs exporter with the given
// configuration
func NewLogsExporter(logger kitlog.Logger, conf LogsExporterConfig, sourceMapStore SourceMapStore) AppAgentReceiverExporter {
	return &LogsExporter{
		logger:           logger,
		getLogsInstance:  conf.GetLogsInstance,
		sendEntryTimeout: conf.SendEntryTimeout,
		labels:           conf.Labels,
		sourceMapStore:   sourceMapStore,
	}
}

// Name of the exporter, for logging purposes
func (le *LogsExporter) Name() string {
	return "logs exporter"
}

// Export implements the AppDataExporter interface
func (le *LogsExporter) Export(ctx context.Context, payload Payload) error {
	meta := payload.Meta.KeyVal()

	var err error

	// log events
	for _, logItem := range payload.Logs {
		kv := logItem.KeyVal()
		MergeKeyVal(kv, meta)
		err = le.sendKeyValsToLogsPipeline(kv)
	}

	// exceptions
	for _, exception := range payload.Exceptions {
		transformedException := TransformException(le.sourceMapStore, le.logger, &exception, payload.Meta.App.Release)
		kv := transformedException.KeyVal()
		MergeKeyVal(kv, meta)
		err = le.sendKeyValsToLogsPipeline(kv)
	}

	// measurements
	for _, measurement := range payload.Measurements {
		kv := measurement.KeyVal()
		MergeKeyVal(kv, meta)
		err = le.sendKeyValsToLogsPipeline(kv)
	}

	// events
	for _, event := range payload.Events {
		kv := event.KeyVal()
		MergeKeyVal(kv, meta)
		err = le.sendKeyValsToLogsPipeline(kv)
	}

	return err
}

func (le *LogsExporter) sendKeyValsToLogsPipeline(kv *KeyVal) error {
	line, err := logfmt.MarshalKeyvals(KeyValToInterfaceSlice(kv)...)
	if err != nil {
		level.Error(le.logger).Log("msg", "failed to logfmt a frontend log event", "err", err)
		return err
	}
	instance, err := le.getLogsInstance()
	if err != nil {
		return err
	}
	sent := instance.SendEntry(api.Entry{
		Labels: le.labelSet(kv),
		Entry: logproto.Entry{
			Timestamp: time.Now(),
			Line:      string(line),
		},
	}, le.sendEntryTimeout)
	if !sent {
		level.Warn(le.logger).Log("msg", "failed to log frontend log event to logs pipeline")
		return fmt.Errorf("failed to send app event to logs pipeline")
	}
	return nil
}

func (le *LogsExporter) labelSet(kv *KeyVal) prommodel.LabelSet {
	set := make(prommodel.LabelSet, len(le.labels))

	for k, v := range le.labels {
		if len(v) > 0 {
			set[prommodel.LabelName(k)] = prommodel.LabelValue(v)
		} else {
			if val, ok := kv.Get(k); ok {
				set[prommodel.LabelName(k)] = prommodel.LabelValue(fmt.Sprint(val))
			}
		}
	}

	return set
}

// Static typecheck tests
var (
	_ AppAgentReceiverExporter = (*LogsExporter)(nil)
	_ logsInstance             = (*logs.Instance)(nil)
)

package gcplogtarget

// This code is copied from Promtail. The gcplogtarget package is used to
// configure and run the targets that can read log entries from cloud resource
// logs like bucket logs, load balancer logs, and Kubernetes cluster logs
// from GCP.

import (
	"fmt"
	"strings"
	"time"

	"github.com/grafana/loki/pkg/push"
	jsoniter "github.com/json-iterator/go"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// reservedLabelTenantID reserved to override the tenant ID while processing
// pipeline stages
const reservedLabelTenantID = "__tenant_id__"

// LogEntry that will be written to the pubsub topic according to the following spec.
// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry
type LogEntry struct {
	LogName  string `json:"logName"`
	Resource struct {
		Type   string            `json:"type"`
		Labels map[string]string `json:"labels"`
	} `json:"resource"`
	Timestamp string `json:"timestamp"`

	// The time the log entry was received by Logging.
	// Its important that `Timestamp` is optional in GCE log entry.
	ReceiveTimestamp string `json:"receiveTimestamp"`

	// Optional. The severity of the log entry. The default value is DEFAULT.
	// DEFAULT, DEBUG, INFO, NOTICE, WARNING, ERROR, CRITICAL, ALERT, EMERGENCY
	// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#LogSeverity
	Severity string `json:"severity"`

	// Optional. A map of key, value pairs that provides additional information about the log entry.
	// The labels can be user-defined or system-defined.
	Labels map[string]string `json:"labels"`

	TextPayload string `json:"textPayload"`
}

type parseOptions struct {
	useFullLine          bool
	useIncomingTimestamp bool
	fixedLabels          model.LabelSet
}

func parseLogEntry(data []byte, builder *labels.Builder, relabelConfig []*relabel.Config, opts parseOptions) (loki.Entry, error) {
	var entry LogEntry

	if err := jsoniter.Unmarshal(data, &entry); err != nil {
		return loki.Entry{}, err
	}

	// Adding mandatory labels for gcplog.
	builder.Set("__gcp_logname", entry.LogName)
	builder.Set("__gcp_resource_type", entry.Resource.Type)
	builder.Set("__gcp_severity", entry.Severity)

	// Resource labels from log entry, add it as internal labels.
	for k, v := range entry.Resource.Labels {
		builder.Set("__gcp_resource_labels_"+convertToLokiCompatibleLabel(k), v)
	}

	// Labels from log entry, add it as internal labels.
	for k, v := range entry.Labels {
		builder.Set("__gcp_labels_"+convertToLokiCompatibleLabel(k), v)
	}

	var processed labels.Labels

	// Apply relabeling.
	if len(relabelConfig) > 0 {
		processed, _ = relabel.Process(builder.Labels(), relabelConfig...)
	} else {
		processed = builder.Labels()
	}

	lbls := make(model.LabelSet)
	processed.Range(func(lbl labels.Label) {
		if strings.HasPrefix(lbl.Name, "__") && lbl.Name != reservedLabelTenantID {
			return
		}
		// ignore invalid labels
		// TODO: add support for different validation schemes.
		//nolint:staticcheck
		if !model.LabelName(lbl.Name).IsValid() || !model.LabelValue(lbl.Value).IsValid() {
			return
		}
		lbls[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	})

	// Add fixed labels.
	lbls = lbls.Merge(opts.fixedLabels)

	ts := time.Now()
	if opts.useIncomingTimestamp {
		tt := entry.Timestamp
		if tt == "" {
			tt = entry.ReceiveTimestamp
		}
		var err error
		ts, err = time.Parse(time.RFC3339, tt)
		if err != nil {
			return loki.Entry{}, fmt.Errorf("invalid timestamp format: %w", err)
		}

		if ts.IsZero() {
			return loki.Entry{}, fmt.Errorf("no timestamp found in the log entry")
		}
	}

	var line string
	// Use text paylod as log line if configured and not empty.
	if !opts.useFullLine && strings.TrimSpace(entry.TextPayload) != "" {
		line = entry.TextPayload
	} else {
		line = string(data)
	}

	return loki.NewEntry(lbls, push.Entry{Timestamp: ts, Line: line}), nil
}

package gcplogtarget

// This code is copied from Promtail. The gcplogtarget package is used to
// configure and run the targets that can read log entries from cloud resource
// logs like bucket logs, load balancer logs, and Kubernetes cluster logs
// from GCP.

import (
	"fmt"
	"strings"
	"time"

	json "github.com/json-iterator/go"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/loki/pkg/push"
)

// GCPLogEntry that will be written to the pubsub topic according to the following spec.
// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry
type GCPLogEntry struct {
	// why was this here?? nolint:revive
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

	// NOTE(kavi): There are other fields on GCPLogEntry. but we need only need
	// above fields for now anyway we will be sending the entire entry to Loki.
}

func parseGCPLogsEntry(data []byte, other model.LabelSet, otherInternal labels.Labels, useIncomingTimestamp bool, useFullLine bool, relabelConfig []*relabel.Config) (loki.Entry, error) {
	var ge GCPLogEntry

	if err := json.Unmarshal(data, &ge); err != nil {
		return loki.Entry{}, err
	}

	// Adding mandatory labels for gcplog
	lbs := labels.NewBuilder(otherInternal)
	lbs.Set("__gcp_logname", ge.LogName)
	lbs.Set("__gcp_resource_type", ge.Resource.Type)
	lbs.Set("__gcp_severity", ge.Severity)

	// resource labels from gcp log entry. Add it as internal labels
	for k, v := range ge.Resource.Labels {
		lbs.Set("__gcp_resource_labels_"+convertToLokiCompatibleLabel(k), v)
	}

	// labels from gcp log entry. Add it as internal labels
	for k, v := range ge.Labels {
		lbs.Set("__gcp_labels_"+convertToLokiCompatibleLabel(k), v)
	}

	var processed labels.Labels

	// apply relabeling
	if len(relabelConfig) > 0 {
		processed, _ = relabel.Process(lbs.Labels(), relabelConfig...)
	} else {
		processed = lbs.Labels()
	}

	// final labelset that will be sent to loki
	lbls := make(model.LabelSet)
	processed.Range(func(lbl labels.Label) {
		if strings.HasPrefix(lbl.Name, "__") {
			return
		}
		// ignore invalid labels
		//nolint:staticcheck
		if !model.LabelName(lbl.Name).IsValid() || !model.LabelValue(lbl.Value).IsValid() {
			return
		}
		lbls[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	})

	// add labels coming from scrapeconfig
	lbls = lbls.Merge(other)

	ts := time.Now()
	line := string(data)

	if useIncomingTimestamp {
		tt := ge.Timestamp
		if tt == "" {
			tt = ge.ReceiveTimestamp
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

	// Send only `ge.textPayload` as log line if its present and user don't explicitly ask for the whole log.
	if !useFullLine && strings.TrimSpace(ge.TextPayload) != "" {
		line = ge.TextPayload
	}

	return loki.Entry{
		Labels: lbls,
		Entry: push.Entry{
			Timestamp: ts,
			Line:      line,
		},
	}, nil
}

package gcplogtarget

// This code is copied from Promtail. The gcplogtarget package is used to
// configure and run the targets that can read log entries from cloud resource
// logs like bucket logs, load balancer logs, and Kubernetes cluster logs
// from GCP.

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/loki/util"
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

	if err := json.Unmarshal(data, &entry); err != nil {
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

	return loki.Entry{
		Labels: lbls,
		Entry: push.Entry{
			Timestamp: ts,
			Line:      line,
		},
	}, nil
}

// pushMessageBody is the POST body format sent by GCP PubSub push subscriptions.
// See https://cloud.google.com/pubsub/docs/push for details.
type pushMessageBody struct {
	Message      pushMessage `json:"message"`
	Subscription string      `json:"subscription"`
}

type pushMessage struct {
	Attributes          map[string]string `json:"attributes"`
	Data                string            `json:"data"`
	MessageID           string            `json:"messageId"`
	DeprecatedMessageID string            `json:"message_id"`
}

// Validate checks that the required fields of a PushMessage are set.
func (pm pushMessageBody) Validate() error {
	if pm.Message.Data == "" {
		return fmt.Errorf("push message has no data")
	}
	if pm.ID() == "" {
		return fmt.Errorf("push message has no ID")
	}
	if pm.Subscription == "" {
		return fmt.Errorf("push message has no subscription")
	}
	return nil
}

func (pm pushMessageBody) ID() string {
	if pm.Message.MessageID != "" {
		return pm.Message.MessageID
	}
	return pm.Message.DeprecatedMessageID
}

// parsePushMessage converts a PushMessage into a loki.Entry.
func parsePushMessage(m pushMessageBody, relabelConfigs []*relabel.Config, xScopeOrgID string, opts parseOptions) (loki.Entry, error) {
	// Collect all push-specific labels. Every one of them is first configured
	// as optional, and the user can relabel it if needed. The relabeling and
	// internal drop is handled in parseGCPLogsEntry.
	builder := labels.NewBuilder(labels.EmptyLabels())
	builder.Set("__gcp_message_id", m.ID())
	builder.Set("__gcp_subscription_name", m.Subscription)
	for k, v := range m.Message.Attributes {
		builder.Set(fmt.Sprintf("__gcp_attributes_%s", convertToLokiCompatibleLabel(k)), v)
	}

	// If the incoming request carries the tenant id, inject it as the reserved
	// label, so it's used by the remote write client.
	if xScopeOrgID != "" {
		// Expose tenant ID through relabel to use as logs or metrics label.
		builder.Set(reservedLabelTenantID, xScopeOrgID)
	}

	decodedData, err := base64.StdEncoding.DecodeString(m.Message.Data)
	if err != nil {
		return loki.Entry{}, fmt.Errorf("failed to decode data: %w", err)
	}

	entry, err := parseLogEntry(decodedData, builder, relabelConfigs, opts)
	if err != nil {
		return loki.Entry{}, fmt.Errorf("failed to parse logs entry: %w", err)
	}

	return entry, nil
}

var separatorCharacterReplacer = strings.NewReplacer(".", "_", "-", "_", "/", "_")

// convertToLokiCompatibleLabel converts an incoming GCP Push message label to
// a loki compatible format. There are labels such as
// `logging.googleapis.com/timestamp`, which contain non-loki-compatible
// characters, which is just alphanumeric and _. The approach taken is to
// translate every non-alphanumeric separator character to an underscore.
func convertToLokiCompatibleLabel(label string) string {
	return util.SnakeCase(separatorCharacterReplacer.Replace(label))
}

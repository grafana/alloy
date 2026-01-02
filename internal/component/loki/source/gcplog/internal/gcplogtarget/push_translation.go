package gcplogtarget

// This code is copied from Promtail. The gcplogtarget package is used to
// configure and run the targets that can read log entries from cloud resource
// logs like bucket logs, load balancer logs, and Kubernetes cluster logs
// from GCP.

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/loki/util"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// ReservedLabelTenantID reserved to override the tenant ID while processing
// pipeline stages
const ReservedLabelTenantID = "__tenant_id__"

// PushMessageBody is the POST body format sent by GCP PubSub push subscriptions.
// See https://cloud.google.com/pubsub/docs/push for details.
type PushMessageBody struct {
	Message      PushMessage `json:"message"`
	Subscription string      `json:"subscription"`
}

type PushMessage struct {
	Attributes          map[string]string `json:"attributes"`
	Data                string            `json:"data"`
	MessageID           string            `json:"messageId"`
	DeprecatedMessageID string            `json:"message_id"`
}

// Validate checks that the required fields of a PushMessage are set.
func (pm PushMessageBody) Validate() error {
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

func (pm PushMessageBody) ID() string {
	if pm.Message.MessageID != "" {
		return pm.Message.MessageID
	}
	return pm.Message.DeprecatedMessageID
}

// translate converts a GCP PushMessage into a loki.Entry. It parses the
// push-specific labels and delegates the rest to parseGCPLogsEntry.
func translate(m PushMessageBody, other model.LabelSet, useIncomingTimestamp bool, useFullLine bool, relabelConfigs []*relabel.Config, xScopeOrgID string) (loki.Entry, error) {
	// Collect all push-specific labels. Every one of them is first configured
	// as optional, and the user can relabel it if needed. The relabeling and
	// internal drop is handled in parseGCPLogsEntry.
	lbs := labels.NewBuilder(labels.EmptyLabels())
	lbs.Set("__gcp_message_id", m.ID())
	lbs.Set("__gcp_subscription_name", m.Subscription)
	for k, v := range m.Message.Attributes {
		lbs.Set(fmt.Sprintf("__gcp_attributes_%s", convertToLokiCompatibleLabel(k)), v)
	}

	// Add fixed labels coming from the target configuration
	fixedLabels := other.Clone()

	// If the incoming request carries the tenant id, inject it as the reserved
	// label, so it's used by the remote write client.
	if xScopeOrgID != "" {
		// Expose tenant ID through relabel to use as logs or metrics label.
		lbs.Set(ReservedLabelTenantID, xScopeOrgID)
		fixedLabels[ReservedLabelTenantID] = model.LabelValue(xScopeOrgID)
	}

	decodedData, err := base64.StdEncoding.DecodeString(m.Message.Data)
	if err != nil {
		return loki.Entry{}, fmt.Errorf("failed to decode data: %w", err)
	}

	entry, err := parseGCPLogsEntry(decodedData, fixedLabels, lbs.Labels(), useIncomingTimestamp, useFullLine, relabelConfigs)
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

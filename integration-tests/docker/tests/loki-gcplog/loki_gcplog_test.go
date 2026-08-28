//go:build alloyintegrationtests

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

const (
	pushURL = "http://127.0.0.1:1518/gcp/api/v1/push"

	emulatorHost     = "localhost:8681"
	emulatorProject  = "test-project"
	emulatorTopic    = "gcplog-topic"
	emulatorEnvVar   = "PUBSUB_EMULATOR_HOST"
	pushSubscription = "projects/test-project/subscriptions/test-push-sub"

	entriesPerType = 10
	tenantEntries  = 5
	tenantOrgID    = "42"
)

// gcpResource represents a resource entry (parsed from `resource` in a GCP LogEntry).
type gcpResource struct {
	Type   string            `json:"type"`
	Labels map[string]string `json:"labels"`
}

// gcpLogEntry is a minimal subset of the GCP Cloud Logging LogEntry shape used by
// the gcplog parser (formatter.go::LogEntry). Only the fields the test exercises
// are populated.
type gcpLogEntry struct {
	LogName     string      `json:"logName"`
	Resource    gcpResource `json:"resource"`
	Timestamp   string      `json:"timestamp"`
	Severity    string      `json:"severity"`
	TextPayload string      `json:"textPayload,omitempty"`
}

// resourceShape bundles a resource type with the resource.labels values used to
// generate fixtures. The component prefixes each label with __gcp_resource_labels_
// (after sanitization) so the relabel chain in config.alloy can promote them.
type resourceShape struct {
	resourceType  string
	resourceLabel map[string]string
}

var resourceShapes = []resourceShape{
	{
		resourceType: "k8s_cluster",
		resourceLabel: map[string]string{
			"cluster_name": "dev-us-central-42",
			"location":     "us-central1",
			"project_id":   emulatorProject,
		},
	},
	{
		resourceType: "gcs_bucket",
		resourceLabel: map[string]string{
			"bucket_name": "loki-bucket",
			"project_id":  emulatorProject,
		},
	},
	{
		resourceType: "cloud_function",
		resourceLabel: map[string]string{
			"function_name": "loki-fn",
			"region":        "us-central1",
		},
	},
}

func TestLokiGcplog(t *testing.T) {
	ctx := t.Context()

	// Push: per-resource-type batches
	for _, rs := range resourceShapes {
		require.NoError(t, postEnvelopes(ctx, "", buildEntries(rs, entriesPerType, "")))
	}

	// Push: tenant header batch (X-Scope-OrgID -> tenant_id)
	tenantShape := resourceShape{
		resourceType: "k8s_cluster",
		resourceLabel: map[string]string{
			// distinct cluster name so the assertion can target this slice
			// without ambiguity against the regular k8s_cluster batch above.
			"cluster_name": "tenant-cluster",
			"location":     "us-central1",
			"project_id":   emulatorProject,
		},
	}
	require.NoError(t, postEnvelopes(ctx, tenantOrgID, buildEntries(tenantShape, tenantEntries, "tenant ")))

	// Pull: publish identical per-resource-type batches via the emulator
	require.NoError(t, publishViaEmulator(ctx, t, resourceShapes, entriesPerType))

	// Assert
	totalPush := entriesPerType*len(resourceShapes) + tenantEntries
	totalPull := entriesPerType * len(resourceShapes)

	common.AssertLogsPresent(
		t,
		totalPush+totalPull,
		// Push streams.
		common.ExpectedLogResult{
			Labels: map[string]string{
				"mode":         "push",
				"service_name": "k8s_cluster",
				"cluster":      "dev-us-central-42",
			},
			StructuredMetadata: map[string]string{"severity": "INFO"},
			EntryCount:         entriesPerType,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"mode":         "push",
				"service_name": "gcs_bucket",
				"bucket":       "loki-bucket",
			},
			EntryCount: entriesPerType,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"mode":         "push",
				"service_name": "cloud_function",
			},
			EntryCount: entriesPerType,
		},
		// Push tenant slice.
		common.ExpectedLogResult{
			Labels: map[string]string{
				"mode":         "push",
				"service_name": "k8s_cluster",
				"cluster":      "tenant-cluster",
			},
			StructuredMetadata: map[string]string{"tenant_id": tenantOrgID},
			EntryCount:         tenantEntries,
		},
		// Pull streams.
		common.ExpectedLogResult{
			Labels: map[string]string{
				"mode":         "pull",
				"service_name": "k8s_cluster",
				"cluster":      "dev-us-central-42",
			},
			EntryCount: entriesPerType,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"mode":         "pull",
				"service_name": "gcs_bucket",
				"bucket":       "loki-bucket",
			},
			EntryCount: entriesPerType,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"mode":         "pull",
				"service_name": "cloud_function",
			},
			EntryCount: entriesPerType,
		},
	)

	common.AssertLabelsNotIndexed(t, "severity", "log_name", "subscription", "tenant_id")
}

// buildEntries produces n LogEntry values with the given resource shape. The
// optional linePrefix is mixed into textPayload to make per-batch lines unique.
func buildEntries(rs resourceShape, n int, linePrefix string) []gcpLogEntry {
	now := time.Now().UTC()
	entries := make([]gcpLogEntry, 0, n)
	for i := range n {
		entries = append(entries, gcpLogEntry{
			LogName:     fmt.Sprintf("projects/%s/logs/integration-test", emulatorProject),
			Resource:    gcpResource{Type: rs.resourceType, Labels: rs.resourceLabel},
			Timestamp:   now.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano),
			Severity:    "INFO",
			TextPayload: fmt.Sprintf("%s%s message %d", linePrefix, rs.resourceType, i),
		})
	}
	return entries
}

// postEnvelopes wraps each LogEntry in a Pub/Sub push envelope and POSTs it to
// the gcplog push receiver. If tenantID is non-empty it is sent in the
// X-Scope-OrgID header so the receiver promotes it to __tenant_id__.
func postEnvelopes(ctx context.Context, tenantID string, entries []gcpLogEntry) error {
	for i, entry := range entries {
		body, err := buildPushEnvelopeBody(entry, i)
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, pushURL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if tenantID != "" {
			req.Header.Set("X-Scope-OrgID", tenantID)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("push request: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("unexpected push status %d (entry %d, type %s)", resp.StatusCode, i, entry.Resource.Type)
		}
	}
	return nil
}

// buildPushEnvelopeBody marshals a LogEntry into a Pub/Sub push envelope. The
// envelope shape mirrors the GCP Pub/Sub push spec and matches the unit-test
// fixture in push_target_test.go::testPayload.
func buildPushEnvelopeBody(entry gcpLogEntry, idx int) ([]byte, error) {
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("marshal entry: %w", err)
	}

	envelope := struct {
		Message struct {
			Attributes map[string]string `json:"attributes,omitempty"`
			Data       string            `json:"data"`
			MessageID  string            `json:"messageId"`
		} `json:"message"`
		Subscription string `json:"subscription"`
	}{
		Subscription: pushSubscription,
	}
	envelope.Message.Data = base64.StdEncoding.EncodeToString(entryJSON)
	envelope.Message.MessageID = strconv.Itoa(idx)

	return json.Marshal(envelope)
}

// publishViaEmulator publishes per-resource-type batches to the emulator topic.
// Alloy's pull subscriber consumes them through the pre-created subscription.
func publishViaEmulator(ctx context.Context, t *testing.T, shapes []resourceShape, perShape int) error {
	t.Helper()

	t.Setenv(emulatorEnvVar, emulatorHost)
	client, err := newEmulatorClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	publisher := client.Publisher(emulatorTopic)
	defer publisher.Stop()

	results := make([]*pubsub.PublishResult, 0, perShape*len(shapes))
	for _, rs := range shapes {
		for _, entry := range buildEntries(rs, perShape, "") {
			data, err := json.Marshal(entry)
			if err != nil {
				return fmt.Errorf("marshal pull entry: %w", err)
			}
			results = append(results, publisher.Publish(ctx, &pubsub.Message{Data: data}))
		}
	}

	for _, r := range results {
		if _, err := r.Get(ctx); err != nil {
			return fmt.Errorf("publish: %w", err)
		}
	}
	return nil
}

// newEmulatorClient retries until the Pub/Sub emulator accepts a client. The
// emulator's TCP listener may answer before topic provisioning completes, so a
// brief retry window absorbs the gap.
func newEmulatorClient(ctx context.Context) (*pubsub.Client, error) {
	bk := backoff.New(ctx, backoff.Config{
		MinBackoff: 100 * time.Millisecond,
		MaxBackoff: 2 * time.Second,
		MaxRetries: 20,
	})

	var lastErr error
	for bk.Ongoing() {
		client, err := pubsub.NewClient(ctx, emulatorProject)
		if err == nil {
			return client, nil
		}
		lastErr = err
		bk.Wait()
	}

	return nil, fmt.Errorf("connect to pubsub emulator: %w", lastErr)
}

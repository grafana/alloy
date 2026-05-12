package deps

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/internal/lokihttp"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

const (
	// Both must match manifests/loki.yaml.
	lokiSelector = "app=loki"
	lokiHTTPPort = "3100"

	// lokiQueryLimit caps how many entries Loki returns per QueryLogs call.
	// High enough to cover any realistic test.
	lokiQueryLimit = 1000

	lokiImage = "grafana/loki:3.5.5"
)

//go:embed manifests/loki.yaml
var lokiManifest string

// Compile-time check that *Loki satisfies the harness.Dependency interface.
var _ harness.Dependency = (*Loki)(nil)

// Loki runs a single-pod Loki in monolithic mode (filesystem storage,
// in-memory rings). In-cluster URL: http://loki:3100.
type Loki struct {
	opts            LokiOptions
	namespace       string
	localPort       string
	stopPortForward func()
	installed       bool
}

type LokiOptions struct {
	Namespace string
}

func NewLoki(opts LokiOptions) *Loki {
	return &Loki{opts: opts, namespace: opts.Namespace}
}

func (l *Loki) Name() string { return "loki" }

func (l *Loki) Install(ctx *harness.TestContext) error {
	if l.namespace == "" {
		return errors.New("loki namespace is required")
	}

	if err := ensureKindImage(lokiImage); err != nil {
		return fmt.Errorf("faild to load loki image: %w", err)
	}

	if err := util.Step("apply loki manifest", func() error {
		return harness.ApplyManifest(l.namespace, lokiManifest)
	}); err != nil {
		return err
	}
	l.installed = true

	if err := util.Step("wait for loki pod ready", func() error {
		return harness.WaitForReady(l.namespace, lokiSelector)
	}); err != nil {
		return err
	}

	localPort, stop, err := startPortForwardWithRetries(l.namespace, "loki", 5, lokiHTTPPort)
	if err != nil {
		return err
	}
	l.localPort = localPort
	l.stopPortForward = stop
	return nil
}

func (l *Loki) Cleanup() {
	if l.stopPortForward != nil {
		l.stopPortForward()
	}
	if !l.installed || l.namespace == "" {
		return
	}
	_ = harness.DeleteManifest(l.namespace, lokiManifest)
}

func (l *Loki) endpoint(path string) string {
	return "http://localhost:" + l.localPort + path
}

// ExpectedLogResult is the per-assertion shape used by QueryLogs. An entry
// matches when its stream labels are a superset of Labels and its structured
// metadata is a superset of StructuredMetadata.
type ExpectedLogResult struct {
	EntryCount         int
	Labels             map[string]string
	StructuredMetadata map[string]string
}

// QueryLogs polls Loki for entries labelled alloy_test_name=<testName> and
// asserts each ExpectedLogResult provided.
func (l *Loki) QueryLogs(t *testing.T, testName string, expected ...ExpectedLogResult) {
	t.Helper()

	queryURL, err := url.Parse(l.endpoint("/loki/api/v1/query_range"))
	require.NoError(t, err)
	values := queryURL.Query()
	values.Set("query", "{"+testNameLabel+"=\""+testName+"\"}")
	values.Set("limit", strconv.Itoa(lokiQueryLimit))
	queryURL.RawQuery = values.Encode()

	// Without categorize-labels structured metadata would be part of the stream.
	headers := http.Header{
		"X-Loki-Response-Encoding-Flags": []string{"categorize-labels"},
	}

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		resp := curl(c, queryURL.String(), headers)

		var parsed lokihttp.LogResponse
		err := json.Unmarshal([]byte(resp), &parsed)
		require.NoError(c, err, "failed to parse loki response")
		require.Equal(c, "success", parsed.Status, "loki query failed: %s", parsed.Status)

		for _, e := range expected {
			entries := matchingEntries(e.Labels, e.StructuredMetadata, parsed.Data.Result)
			require.NotEmptyf(c, entries, "no entries with labels %v and structured metadata %v", e.Labels, e.StructuredMetadata)
			require.Lenf(c, entries, e.EntryCount, "unexpected entry count for labels %v and structured metadata %v", e.Labels, e.StructuredMetadata)
		}
	}, timeout, retryInterval)
}

// matchingEntries returns all entries who partially matches stream labels and structured metadata.
func matchingEntries(labels, metadata map[string]string, result []lokihttp.LogData) []lokihttp.LogEntry {
	var entries []lokihttp.LogEntry
	for _, r := range result {
		if !mapContains(r.Stream, labels) {
			continue
		}
		for _, v := range r.Values {
			if mapContains(v.Metadata.StructuredMetadata, metadata) {
				entries = append(entries, v)
			}
		}
	}
	return entries
}

func mapContains(have, want map[string]string) bool {
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}

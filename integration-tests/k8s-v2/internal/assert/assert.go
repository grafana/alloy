// Package assert provides the port-forward + eventual-assertion helpers
// used by per-test TestAssertions functions. Backend metadata (namespace,
// port, ...) is sourced from internal/deps so there is one source of
// truth across install-time and assert-time.
package assert

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/deps"
	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/kube"
)

const (
	DefaultTimeout  = 2 * time.Minute
	DefaultInterval = 2 * time.Second
)

// LokiBackend / MimirBackend expose the shared dep Specs under the names
// assert callers are used to. They are not separate values — just aliases
// so per-test code can import a single package.
var (
	LokiBackend  = deps.Loki
	MimirBackend = deps.Mimir
)

// StartBackendPortForward starts a kubectl port-forward to spec and returns
// a base URL + close function.
func StartBackendPortForward(ctx context.Context, kubeconfig string, spec deps.Spec) (string, func(), error) {
	handle, err := kube.StartPortForwardAndWait(ctx, kube.PortForwardConfig{
		Kubeconfig:    kubeconfig,
		Namespace:     spec.Namespace,
		Service:       spec.Service,
		TargetPort:    spec.Port,
		ReadinessPath: spec.ReadinessPath,
		PollInterval:  DefaultInterval,
		ReadyTimeout:  DefaultTimeout,
	})
	if err != nil {
		return "", nil, fmt.Errorf("start %s port-forward: %w", spec.Name, err)
	}
	return handle.BaseURL, func() { _ = handle.Close() }, nil
}

// eventually polls query at baseURL+path, calling check on every non-5xx
// response body. It returns nil as soon as check returns true, or an error
// on DefaultTimeout. matchDesc is used in the timeout error for context.
func eventually(
	ctx context.Context,
	baseURL, path, query, matchDesc string,
	check func(body []byte) bool,
) error {
	deadlineCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	var lastSummary string
	for {
		parsed, err := url.Parse(baseURL)
		if err != nil {
			return fmt.Errorf("query=%q expected=valid_base_url got=%q err=%w", query, baseURL, err)
		}
		parsed.Path = path
		parsed.RawQuery = url.Values{
			"query": []string{query},
			"limit": []string{"50"},
		}.Encode()

		req, err := http.NewRequestWithContext(deadlineCtx, http.MethodGet, parsed.String(), nil)
		if err != nil {
			return fmt.Errorf("query=%q build_request: %w", query, err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp != nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 && check(body) {
				return nil
			}
			lastSummary = fmt.Sprintf("status=%d body=%s", resp.StatusCode, string(body))
		} else {
			lastSummary = fmt.Sprintf("request_error=%v", err)
		}

		select {
		case <-deadlineCtx.Done():
			return fmt.Errorf(
				"query=%q expected=%s timeout=%s last_observed=%s",
				query, matchDesc, DefaultTimeout, lastSummary,
			)
		case <-time.After(DefaultInterval):
		}
	}
}

// EventuallyLokiQueryContainsExactLine polls Loki until the query returns a
// stream with an entry whose value equals expectedLine.
func EventuallyLokiQueryContainsExactLine(ctx context.Context, baseURL, query, expectedLine string) error {
	return eventually(ctx, baseURL, "/loki/api/v1/query_range", query,
		fmt.Sprintf("exact_line(%q)", expectedLine),
		func(body []byte) bool {
			var decoded struct {
				Status string `json:"status"`
				Data   struct {
					Result []struct {
						Values [][]string `json:"values"`
					} `json:"result"`
				} `json:"data"`
			}
			if json.Unmarshal(body, &decoded) != nil || decoded.Status != "success" {
				return false
			}
			for _, stream := range decoded.Data.Result {
				for _, value := range stream.Values {
					if len(value) > 1 && strings.TrimSpace(value[1]) == expectedLine {
						return true
					}
				}
			}
			return false
		},
	)
}

// EventuallyMimirQueryHasSeries polls Mimir until the query returns at
// least one series.
func EventuallyMimirQueryHasSeries(ctx context.Context, baseURL, query string) error {
	return eventually(ctx, baseURL, "/prometheus/api/v1/query", query,
		"at_least_one_series",
		func(body []byte) bool {
			var decoded struct {
				Status string `json:"status"`
				Data   struct {
					Result []json.RawMessage `json:"result"`
				} `json:"data"`
			}
			return json.Unmarshal(body, &decoded) == nil &&
				decoded.Status == "success" &&
				len(decoded.Data.Result) > 0
		},
	)
}

package assert

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	DefaultTimeout  = 2 * time.Minute
	DefaultInterval = 2 * time.Second
)

type promQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []json.RawMessage `json:"result"`
	} `json:"data"`
}

func EventuallyMimirQueryHasSeries(ctx context.Context, baseURL, query string) error {
	deadlineCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	var lastSummary string
	for {
		parsed, err := url.Parse(baseURL)
		if err != nil {
			return fmt.Errorf("mimir query=%q expected=valid_base_url got=%q err=%w", query, baseURL, err)
		}
		parsed.Path = "/prometheus/api/v1/query"
		parsed.RawQuery = url.Values{"query": []string{query}}.Encode()

		req, err := http.NewRequestWithContext(deadlineCtx, http.MethodGet, parsed.String(), nil)
		if err != nil {
			return fmt.Errorf("mimir query=%q expected=request_build_success err=%w", query, err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp != nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var decoded promQueryResponse
				if unmarshalErr := json.Unmarshal(body, &decoded); unmarshalErr == nil {
					hasSeries := decoded.Status == "success" && len(decoded.Data.Result) > 0
					if hasSeries {
						return nil
					}
				}
				lastSummary = fmt.Sprintf("status=%d body=%s", resp.StatusCode, string(body))
			} else {
				lastSummary = fmt.Sprintf("status=%d body=%s", resp.StatusCode, string(body))
			}
		} else {
			lastSummary = fmt.Sprintf("request_error=%v", err)
		}

		select {
		case <-deadlineCtx.Done():
			return fmt.Errorf(
				"mimir query=%q expected=at_least_one_series timeout=%s last_observed=%s",
				query,
				DefaultTimeout,
				lastSummary,
			)
		case <-time.After(DefaultInterval):
		}
	}
}

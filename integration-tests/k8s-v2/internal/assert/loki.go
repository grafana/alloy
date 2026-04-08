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
)

type lokiQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Values [][]string `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

func EventuallyLokiQueryContainsLine(ctx context.Context, baseURL, query, expectedSubstring string) error {
	deadlineCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	var lastSummary string
	for {
		parsed, err := url.Parse(baseURL)
		if err != nil {
			return fmt.Errorf("loki query=%q expected=valid_base_url got=%q err=%w", query, baseURL, err)
		}
		parsed.Path = "/loki/api/v1/query_range"
		parsed.RawQuery = url.Values{
			"query": []string{query},
			"limit": []string{"50"},
		}.Encode()

		req, err := http.NewRequestWithContext(deadlineCtx, http.MethodGet, parsed.String(), nil)
		if err != nil {
			return fmt.Errorf("loki query=%q expected=request_build_success err=%w", query, err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp != nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var decoded lokiQueryResponse
				if unmarshalErr := json.Unmarshal(body, &decoded); unmarshalErr == nil && decoded.Status == "success" {
					for _, stream := range decoded.Data.Result {
						for _, value := range stream.Values {
							if len(value) > 1 && strings.Contains(value[1], expectedSubstring) {
								return nil
							}
						}
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
				"loki query=%q expected=substring(%q) timeout=%s last_observed=%s",
				query,
				expectedSubstring,
				DefaultTimeout,
				lastSummary,
			)
		case <-time.After(DefaultInterval):
		}
	}
}

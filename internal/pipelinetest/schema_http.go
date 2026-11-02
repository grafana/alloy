package pipelinetest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultHTTPInputMethod = http.MethodPost

	// httpInputTimeout bounds each request, so a stalled listener fails the
	// test instead of hanging it.
	httpInputTimeout = 5 * time.Second

	// httpInputDialTimeout is how long to keep retrying while nothing listens
	// on the target port yet. Components start their listeners as they run,
	// which can be shortly after the config finishes loading.
	httpInputDialTimeout = 10 * time.Second
	httpInputDialBackoff = 50 * time.Millisecond
)

// HTTPInputSchema describes one HTTP request sent to a component's own
// listener, such as a push receiver. Requests are sent in order, after any
// Loki inputs.
//
// The URL must name the port the component listens on in the config, for
// example
//
//	url: http://127.0.0.1:1516/heroku/api/v1/drain
//
// for a component configured with listen_port = 1516.
type HTTPInputSchema struct {
	URL string `yaml:"url"`
	// Method defaults to POST.
	Method  string            `yaml:"method,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty"`
	// ExpectedStatus is the status code the response must have. When unset,
	// any 2xx status is accepted.
	ExpectedStatus int `yaml:"expected_status,omitempty"`
}

func validateHTTPInputs(inputs []HTTPInputSchema) error {
	for i, input := range inputs {
		u, err := url.Parse(input.URL)
		if err != nil {
			return fmt.Errorf("http input %d: invalid url: %w", i, err)
		}
		if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("http input %d: url must be an absolute http or https URL, got %q", i, input.URL)
		}
		if input.ExpectedStatus != 0 && (input.ExpectedStatus < 100 || input.ExpectedStatus > 599) {
			return fmt.Errorf("http input %d: expected_status %d is not a valid HTTP status code", i, input.ExpectedStatus)
		}
	}
	return nil
}

func produceHTTPInputs(ctx context.Context, inputs []HTTPInputSchema) error {
	client := &http.Client{Timeout: httpInputTimeout}
	for i, input := range inputs {
		if err := sendHTTPInput(ctx, client, input); err != nil {
			return fmt.Errorf("http input %d: %w", i, err)
		}
	}
	return nil
}

func sendHTTPInput(ctx context.Context, client *http.Client, input HTTPInputSchema) error {
	method := input.Method
	if method == "" {
		method = defaultHTTPInputMethod
	}

	deadline := time.Now().Add(httpInputDialTimeout)
	for {
		req, err := http.NewRequestWithContext(ctx, method, input.URL, strings.NewReader(input.Body))
		if err != nil {
			return err
		}
		for k, v := range input.Headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			// Retry only while the connection cannot be made, meaning the
			// listener is not up yet. Anything else is a real failure.
			var opErr *net.OpError
			if errors.As(err, &opErr) && opErr.Op == "dial" && time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(httpInputDialBackoff):
				}
				continue
			}
			return err
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		_ = resp.Body.Close()

		if input.ExpectedStatus != 0 && resp.StatusCode != input.ExpectedStatus {
			return fmt.Errorf("%s %s: want status %d, got %d: %s", method, input.URL, input.ExpectedStatus, resp.StatusCode, body)
		}
		if input.ExpectedStatus == 0 && (resp.StatusCode < 200 || resp.StatusCode > 299) {
			return fmt.Errorf("%s %s: want a 2xx status, got %d: %s", method, input.URL, resp.StatusCode, body)
		}
		return nil
	}
}

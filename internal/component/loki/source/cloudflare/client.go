package cloudflare

// This file implements a minimal client for the Cloudflare Logpull API.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/dskit/backoff"

	"github.com/grafana/alloy/internal/useragent"
)

const (
	// defaultAPIURL is the Cloudflare API base URL used when api_url is not set.
	defaultAPIURL = "https://api.cloudflare.com/client/v4"

	// maxErrorBodySize bounds how much of an error response body is read.
	maxErrorBodySize = 64 * 1024
)

// requestBackoff retries Logpull requests that fail with a transport error, 429 or 5xx response.
var requestBackoff = backoff.Config{
	MinBackoff: 1 * time.Second,
	MaxBackoff: 30 * time.Second,
	MaxRetries: 4,
}

type client struct {
	cfg        clientConfig
	httpClient *http.Client
}

type clientConfig struct {
	apiURL   string
	apiToken string
	zoneID   string
	fields   []string
	backoff  backoff.Config
}

func newClient(cfg clientConfig) *client {
	return &client{
		cfg:        cfg,
		httpClient: &http.Client{},
	}
}

// LogpullReceived requests the logs received for the zone between start and end.
// API reference: https://developers.cloudflare.com/logs/logpull/requesting-logs
func (c *client) LogpullReceived(ctx context.Context, start, end time.Time) (*logpullIterator, error) {
	v := url.Values{}
	v.Set("start", strconv.FormatInt(start.UnixNano(), 10))
	v.Set("end", strconv.FormatInt(end.UnixNano(), 10))
	if c.cfg.fields != nil {
		v.Set("fields", strings.Join(c.cfg.fields, ","))
	}
	uri := c.cfg.apiURL + "/zones/" + url.PathEscape(c.cfg.zoneID) + "/logs/received?" + v.Encode()

	var (
		boff    = backoff.New(ctx, c.cfg.backoff)
		lastErr error
	)
	for boff.Ongoing() {
		resp, err := c.do(ctx, uri)
		if err != nil {
			lastErr = err
			boff.Wait()
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
			lastErr = responseError(resp)
			boff.Wait()
			continue
		}
		if resp.StatusCode >= http.StatusBadRequest {
			return nil, responseError(resp)
		}

		return &logpullIterator{
			body:   resp.Body,
			reader: bufio.NewReader(resp.Body),
		}, nil
	}

	if lastErr == nil {
		lastErr = boff.Err()
	}

	return nil, lastErr
}

func (c *client) do(ctx context.Context, uri string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.apiToken)
	req.Header.Set("User-Agent", useragent.Get())
	return c.httpClient.Do(req)
}

// responseError reads and closes the body of a failed response and returns an error
// containing the status code and the error messages returned by Cloudflare.
func responseError(resp *http.Response) error {
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
	body = bytes.TrimSpace(body)

	// Logpull errors are usually plain text, but other API errors are JSON.
	if len(body) > 0 && body[0] == '{' {
		var r struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := json.Unmarshal(body, &r); err == nil && len(r.Errors) > 0 {
			msgs := make([]string, 0, len(r.Errors))
			for _, e := range r.Errors {
				msgs = append(msgs, e.Message)
			}
			return fmt.Errorf("HTTP status %d: %s", resp.StatusCode, strings.Join(msgs, ", "))
		}
	}
	if len(body) == 0 {
		return fmt.Errorf("HTTP status %d", resp.StatusCode)
	}
	return fmt.Errorf("HTTP status %d: %s", resp.StatusCode, body)
}

// logpullIterator iterates on NDJSON from LogPull response.
type logpullIterator struct {
	body   io.ReadCloser
	reader *bufio.Reader

	buf []byte
	err error
}

func (it *logpullIterator) Next() bool {
	if it.err != nil {
		return false
	}
	if err := it.readLine(); err != nil {
		if !errors.Is(err, io.EOF) {
			it.err = err
		}
		it.buf = it.buf[:0]
		return false
	}
	return true
}

// readLine reads the next line, including its line ending, into buf. The last
// line is read even if it has no trailing newline.
func (it *logpullIterator) readLine() error {
	it.buf = it.buf[:0]
	for {
		frag, err := it.reader.ReadSlice('\n')
		it.buf = append(it.buf, frag...)
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if err == nil || (errors.Is(err, io.EOF) && len(it.buf) > 0) {
			return nil
		}
		return err
	}
}

func (it *logpullIterator) Err() error {
	return it.err
}

// Line returns the current log line. It is only valid until the next call to Next.
func (it *logpullIterator) Line() []byte {
	b := bytes.TrimSuffix(it.buf, []byte{'\n'})
	return bytes.TrimSuffix(b, []byte{'\r'})
}

func (it *logpullIterator) Close() error {
	return it.body.Close()
}

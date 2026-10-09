package cloudflare

// This file implements a minimal client for the Cloudflare Logpull API. It
// replaces the archived github.com/grafana/cloudflare-go fork, whose line
// scanner failed on log lines larger than 64 KiB.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/dskit/backoff"
	"golang.org/x/time/rate"

	"github.com/grafana/alloy/internal/useragent"
)

const (
	defaultAPIURL = "https://api.cloudflare.com/client/v4"

	// maxLineSize is the maximum size of a single log line returned by the Logpull API.
	// Longer lines are dropped instead of failing the whole pull window.
	maxLineSize = 1024 * 1024

	// maxErrorBodySize bounds how much of an error response body is read.
	maxErrorBodySize = 64 * 1024
)

// requestRateLimit matches Cloudflare's default API rate limit of 1200 requests per 5 minutes.
var requestRateLimit = rate.Limit(4)

// requestBackoff retries Logpull requests that fail with a transport error, 429 or 5xx response.
var requestBackoff = backoff.Config{
	MinBackoff: 1 * time.Second,
	MaxBackoff: 30 * time.Second,
	MaxRetries: 4,
}

// Client is a wrapper around the Cloudflare API that allow for testing and being zone/fields aware.
type Client interface {
	LogpullReceived(ctx context.Context, start, end time.Time) (LogpullReceivedIterator, error)
}

// LogpullReceivedIterator iterates over the log lines returned by the Logpull API.
type LogpullReceivedIterator interface {
	// Next advances the iterator to the next log line and reports whether there is one.
	Next() bool
	// Err returns the error that stopped the iteration, if any.
	Err() error
	// Line returns the current log line. It is only valid until the next call to Next.
	Line() []byte
	// Close closes the underlying response body.
	Close() error
}

type wrappedClient struct {
	httpClient  *http.Client
	rateLimiter *rate.Limiter
	apiURL      string
	apiToken    string
	zoneID      string
	fields      []string
	logger      *slog.Logger
	metrics     *metrics
}

// getClient is overridden in tests.
var getClient = newClient

func newClient(apiKey, zoneID string, fields []string, logger *slog.Logger, metrics *metrics) (Client, error) {
	apiURL := defaultAPIURL
	// Allow override API URL for Loki firehose integration tests.
	if u := os.Getenv("ALLOY_CLOUDFLARE_API_URL"); u != "" {
		apiURL = u
	}

	return &wrappedClient{
		httpClient:  &http.Client{},
		rateLimiter: rate.NewLimiter(requestRateLimit, 1),
		apiURL:      strings.TrimSuffix(apiURL, "/"),
		apiToken:    apiKey,
		zoneID:      zoneID,
		fields:      fields,
		logger:      logger,
		metrics:     metrics,
	}, nil
}

// LogpullReceived requests the logs received for the zone between start and end.
//
// API reference: https://developers.cloudflare.com/logs/logpull/requesting-logs
func (w *wrappedClient) LogpullReceived(ctx context.Context, start, end time.Time) (LogpullReceivedIterator, error) {
	v := url.Values{}
	v.Set("start", strconv.FormatInt(start.UnixNano(), 10))
	v.Set("end", strconv.FormatInt(end.UnixNano(), 10))
	if w.fields != nil {
		v.Set("fields", strings.Join(w.fields, ","))
	}
	uri := w.apiURL + "/zones/" + url.PathEscape(w.zoneID) + "/logs/received?" + v.Encode()

	var (
		boff    = backoff.New(ctx, requestBackoff)
		lastErr error
	)
	for boff.Ongoing() {
		if err := w.rateLimiter.Wait(ctx); err != nil {
			return nil, err
		}
		resp, err := w.do(ctx, uri)
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
			body:    resp.Body,
			reader:  bufio.NewReader(resp.Body),
			onDrop:  w.onLineDropped,
			maxSize: maxLineSize,
		}, nil
	}
	if lastErr == nil {
		lastErr = boff.Err()
	}
	return nil, lastErr
}

func (w *wrappedClient) do(ctx context.Context, uri string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+w.apiToken)
	req.Header.Set("User-Agent", useragent.Get())
	return w.httpClient.Do(req)
}

func (w *wrappedClient) onLineDropped(size int) {
	w.metrics.DroppedLines.Inc()
	w.logger.Warn("dropping Cloudflare log line larger than the maximum line size", "zone_id", w.zoneID, "size_bytes", size, "max_size_bytes", maxLineSize)
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

// logpullIterator reads newline-delimited log lines from a Logpull response.
// Unlike bufio.Scanner, it doesn't fail on lines longer than its buffer: lines
// up to maxSize are returned, and longer ones are dropped and reported to onDrop.
type logpullIterator struct {
	body    io.ReadCloser
	reader  *bufio.Reader
	onDrop  func(size int)
	maxSize int

	buf  []byte
	line []byte
	err  error
}

func (it *logpullIterator) Next() bool {
	for it.err == nil {
		line, size, err := it.readLine()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				it.err = err
			}
			it.line = nil
			return false
		}
		if line == nil {
			it.onDrop(size)
			continue
		}
		it.line = line
		return true
	}
	return false
}

// readLine returns the next line without its line ending. If the line is longer
// than maxSize, it's discarded and readLine returns a nil line with its size.
// The last line is returned even if it has no trailing newline.
func (it *logpullIterator) readLine() (line []byte, size int, err error) {
	it.buf = it.buf[:0]
	tooLong := false
	for {
		frag, err := it.reader.ReadSlice('\n')
		size += len(frag)
		if !tooLong {
			// Allow room for a "\r\n" line ending before deciding the line is too long.
			if len(it.buf)+len(frag) > it.maxSize+2 {
				tooLong = true
				it.buf = it.buf[:0]
			} else {
				it.buf = append(it.buf, frag...)
			}
		}

		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if err != nil && !(errors.Is(err, io.EOF) && size > 0) {
			return nil, 0, err
		}
		break
	}

	if tooLong {
		return nil, size, nil
	}
	line = dropLineEnding(it.buf)
	if len(line) > it.maxSize {
		return nil, size, nil
	}
	if line == nil {
		line = []byte{}
	}
	return line, size, nil
}

func dropLineEnding(b []byte) []byte {
	b = bytes.TrimSuffix(b, []byte{'\n'})
	return bytes.TrimSuffix(b, []byte{'\r'})
}

func (it *logpullIterator) Err() error   { return it.err }
func (it *logpullIterator) Line() []byte { return it.line }
func (it *logpullIterator) Close() error { return it.body.Close() }

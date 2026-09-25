package source

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var utf8BOM = []byte("\xef\xbb\xbf")

// fetch runs req and returns the body. It enforces maxSize on the decoded
// body, so a small gzip response cannot expand past the limit.
func fetch(client *http.Client, req *http.Request, maxSize int64) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, newPollError(reasonTimeout, errors.New("request timed out"))
		}
		return nil, newPollError(reasonRequest, redactErr(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		// Drain a little so the connection can be reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, newPollError(reasonStatus, fmt.Errorf("status %d", resp.StatusCode))
	}

	var r io.Reader = resp.Body
	if !resp.Uncompressed && strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, newPollError(reasonParse, fmt.Errorf("invalid gzip body: %w", err))
		}
		defer gz.Close()
		r = gz
	}

	body, err := io.ReadAll(io.LimitReader(r, maxSize+1))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, newPollError(reasonTimeout, errors.New("reading the response timed out"))
		}
		return nil, newPollError(reasonRequest, fmt.Errorf("reading the response: %w", redactErr(err)))
	}
	if int64(len(body)) > maxSize {
		return nil, newPollError(reasonTooLarge, fmt.Errorf("response body is larger than max_response_size (%d bytes)", maxSize))
	}
	body = bytes.TrimPrefix(body, utf8BOM)
	if len(body) == 0 {
		return nil, newPollError(reasonParse, errors.New("empty response body"))
	}
	return body, nil
}

// redactErr hides query params in the URL that net/http puts in its errors.
func redactErr(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return &url.Error{Op: ue.Op, URL: redactURL(ue.URL), Err: ue.Err}
	}
	return err
}

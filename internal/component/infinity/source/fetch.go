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
	"regexp"
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

// redactErr hides query params in URLs that net/http puts in its errors.
// It redacts each nested *url.Error, and then scrubs the final text,
// because net/http also writes URLs into plain text, for example a bad
// redirect Location.
func redactErr(err error) error {
	if err == nil {
		return nil
	}
	r := redactURLErrors(err)
	return &redactedError{msg: scrubURLs(r.Error()), err: r}
}

func redactURLErrors(err error) error {
	if ue, ok := err.(*url.Error); ok {
		return &url.Error{Op: ue.Op, URL: redactURL(ue.URL), Err: redactURLErrors(ue.Err)}
	}
	return err
}

// urlInTextRE matches a URL with a query in error text. Quotes and spaces
// end the match, because Go error text quotes URLs.
var urlInTextRE = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s"'?#]*\?[^\s"'#]*`)

// scrubURLs redacts each URL with a query in msg. A URL that does not parse
// loses its whole query.
func scrubURLs(msg string) string {
	return urlInTextRE.ReplaceAllStringFunc(msg, func(m string) string {
		u, err := url.Parse(m)
		if err != nil {
			base, _, _ := strings.Cut(m, "?")
			return base + "?REDACTED"
		}
		return redactURL(u.String())
	})
}

// redactedError has a message without secrets. Unwrap keeps errors.Is
// working for callers.
type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string { return e.msg }
func (e *redactedError) Unwrap() error { return e.err }

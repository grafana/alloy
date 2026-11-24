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
// body, so a small gzip response cannot expand past the limit. target names
// the request in error messages.
func fetch(client *http.Client, req *http.Request, target requestTarget, maxSize int64) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, requestError(err, target, req.URL.String())
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
			return nil, newLibError(reasonParse, "the response is not valid gzip", err)
		}
		defer gz.Close()
		r = gz
	}

	body, err := io.ReadAll(io.LimitReader(r, maxSize+1))
	if err != nil {
		u := redactURL(target.rawURL)
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &pollError{reason: reasonTimeout, msg: fmt.Sprintf("reading the response from %s timed out after %s", u, target.timeout), err: redactURLErrors(err)}
		}
		return nil, &pollError{reason: reasonRequest, msg: fmt.Sprintf("reading the response from %s failed", u), err: redactURLErrors(err)}
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

// redactURLErrors redacts the URL of each nested *url.Error, because
// net/http puts the request URL, with its query, into its errors.
func redactURLErrors(err error) error {
	if ue, ok := err.(*url.Error); ok {
		return &url.Error{Op: ue.Op, URL: redactURL(ue.URL), Err: redactURLErrors(ue.Err)}
	}
	return err
}

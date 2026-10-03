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

var (
	// urlInTextRE matches a URL with a query in error text. Quotes and
	// spaces end the match, because Go error text quotes URLs. A backslash
	// before a quote also ends it, so the match never takes the "\" of an
	// escaped quote.
	urlInTextRE = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://(?:[^\s"'?#\\]|\\[^"\s])*\?(?:[^\s"'#\\]|\\[^"\s])*`)
	// quotedQueryRE matches a quoted token with a "?", for example a
	// relative redirect Location that has no scheme. scrubURLs checks that
	// the token looks like a URL before it redacts it. Go error text
	// escapes a quote as \", so an escape does not end the token.
	quotedQueryRE = regexp.MustCompile(`"((?:[^"\\]|\\.)*)\?((?:[^"\\]|\\.)*)"`)
	// userinfoRE matches user info after "//", which can hold a password.
	// A bad URL can have a space in its user info, so only a quote, a line
	// end or a URL delimiter ends the match.
	userinfoRE = regexp.MustCompile(`//[^/?#"'@\n]+@`)
)

// scrubURLs removes user info and redacts each query in msg. A URL that
// parses keeps its param names. Any other query loses its whole text.
func scrubURLs(msg string) string {
	msg = userinfoRE.ReplaceAllString(msg, "//")
	msg = redactURLsInText(msg)
	return quotedQueryRE.ReplaceAllStringFunc(msg, func(m string) string {
		token := m[1 : len(m)-1]
		if loc := urlInTextRE.FindStringIndex(token); loc != nil && loc[1] == len(token) && strings.IndexByte(token, '?') > loc[0] {
			// The URL pass above already redacted all of the query.
			return m
		}
		base, query, _ := strings.Cut(m, "?")
		// Scrub only a token that looks like a URL, a path or a key=value
		// query, so ordinary text such as "what?" stays readable. The URL
		// pass stops at a space or a quote, so the rest of the query can
		// still hold a secret.
		if !strings.Contains(base, "/") && !strings.Contains(query, "=") {
			return m
		}
		return base + `?REDACTED"`
	})
}

// redactURLsInText redacts the query of each URL in msg. The quoted pass
// in scrubURLs covers the rest of a quoted URL. An unquoted URL has no end
// mark, so text after a space can still be part of its query. Thus an
// unquoted URL loses the rest of its line.
func redactURLsInText(msg string) string {
	var b strings.Builder
	last := 0
	for _, loc := range urlInTextRE.FindAllStringIndex(msg, -1) {
		start, end := loc[0], loc[1]
		if start < last {
			continue
		}
		m := msg[start:end]
		b.WriteString(msg[last:start])
		if end < len(msg) && (msg[end] == ' ' || msg[end] == '\t') && !insideQuotes(msg, start) {
			base, _, _ := strings.Cut(m, "?")
			b.WriteString(base + "?REDACTED")
			if nl := strings.IndexByte(msg[end:], '\n'); nl >= 0 {
				end += nl
			} else {
				end = len(msg)
			}
		} else if u, err := url.Parse(m); err != nil {
			base, _, _ := strings.Cut(m, "?")
			b.WriteString(base + "?REDACTED")
		} else {
			b.WriteString(redactURL(u.String()))
		}
		last = end
	}
	b.WriteString(msg[last:])
	return b.String()
}

// insideQuotes tells if pos is inside a Go-quoted string on its line.
func insideQuotes(msg string, pos int) bool {
	in := false
	for i := strings.LastIndexByte(msg[:pos], '\n') + 1; i < pos; i++ {
		switch {
		case in && msg[i] == '\\':
			i++
		case msg[i] == '"':
			in = !in
		}
	}
	return in
}

// redactedError has a message without secrets. Unwrap keeps errors.Is
// working for callers.
type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string { return e.msg }
func (e *redactedError) Unwrap() error { return e.err }

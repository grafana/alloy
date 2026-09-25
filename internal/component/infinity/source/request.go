package source

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// buildRequest builds the HTTP request for a url source.
func buildRequest(ctx context.Context, s querySpec) (*http.Request, error) {
	u, err := url.Parse(s.url)
	if err != nil {
		return nil, err
	}
	if len(s.params) > 0 {
		q := u.Query()
		for _, k := range slices.Sorted(maps.Keys(s.params)) {
			q.Set(k, s.params[k])
		}
		u.RawQuery = q.Encode()
	}

	var (
		body        io.Reader
		contentType string
	)
	switch s.bodyType {
	case "raw":
		if s.body != "" {
			body = strings.NewReader(s.body)
			contentType = s.bodyContentType
		}
	case "x-www-form-urlencoded":
		form := url.Values{}
		for k, v := range s.bodyForm {
			form.Set(k, v)
		}
		body = strings.NewReader(form.Encode())
		contentType = "application/x-www-form-urlencoded"
	case "form-data":
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		for _, k := range slices.Sorted(maps.Keys(s.bodyForm)) {
			if err := w.WriteField(k, s.bodyForm[k]); err != nil {
				return nil, err
			}
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		body = &buf
		contentType = w.FormDataContentType()
	case typeGraphQL:
		payload := map[string]any{"query": s.graphqlQuery}
		if s.graphqlVariables != "" {
			payload["variables"] = json.RawMessage(s.graphqlVariables)
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
		contentType = "application/json"
	}

	req, err := http.NewRequestWithContext(ctx, s.method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

// redactURL removes user info and hides every query param value. Query
// params often hold API keys.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<invalid url>"
	}
	u.User = nil
	if u.RawQuery != "" {
		q := u.Query()
		for k := range q {
			q[k] = []string{"REDACTED"}
		}
		u.RawQuery = q.Encode()
	}
	return u.String()
}

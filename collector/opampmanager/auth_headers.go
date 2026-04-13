package opampmanager

import (
	"net/http"
	"strings"
)

func opampRequestHeaders(cfg Config) http.Header {
	raw := strings.TrimSpace(cfg.BasicAuthToken)
	if raw == "" {
		return nil
	}
	raw = strings.TrimPrefix(raw, "Basic ")
	raw = strings.TrimSpace(raw)
	h := make(http.Header)
	h.Set("Authorization", "Basic "+raw)
	return h
}

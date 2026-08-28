package database_observability

import "strings"

// TryExtractTraceParent attempts to extract a W3C traceparent value added at the end of SQL text as a trailing
// block comment, e.g. "/*traceparent='00-<traceid>-<spanid>-<flags>'*/".
// It returns the traceparent string when matched, otherwise an empty string.
func TryExtractTraceParent(sqlText string) string {
	if strings.HasSuffix(sqlText, "...") {
		return ""
	}

	// Find the last comment: strip out /* and */
	start := strings.LastIndex(sqlText, "/*")
	if start < 0 {
		return ""
	}
	body := sqlText[start+2:]
	end := strings.Index(body, "*/")
	if end < 0 {
		return ""
	}

	body = body[:end]
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	// Split the comment by comma into key value pairs
	pairs := strings.Split(body, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		key, val, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}

		if !strings.EqualFold(strings.TrimSpace(key), "traceparent") {
			continue
		}

		// SQL unescape: trim ' or " at beginning and end of value
		if strings.HasPrefix(val, "'") || strings.HasPrefix(val, `"`) {
			quote := string(val[0])
			val = strings.TrimPrefix(val, quote)
			val = strings.TrimSuffix(val, quote)
		}

		return strings.TrimSpace(val)
	}

	return ""
}

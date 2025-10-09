package postgres

import (
	"net/url"
	"strings"
)

// AppName is the fixed application_name used by Alloy DB observability and related wrappers.
const AppName = "grafana_db011y"

// AugmentPostgresDSN appends application_name. Supports both URL and connstring DSNs.
func AugmentPostgresDSN(dsn string, applicationName string) string {
	if applicationName == "" {
		return dsn
	}
	isURL := strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://")
	if isURL {
		u, err := url.Parse(dsn)
		if err != nil {
			return dsn
		}
		q := u.Query()
		if q.Get("application_name") == "" {
			q.Set("application_name", applicationName)
		}
		u.RawQuery = q.Encode()
		return u.String()
	}
	// connstring: space-separated key=value
	needsSpace := func(s string) string {
		if len(s) > 0 && s[len(s)-1] != ' ' {
			return s + " "
		}
		return s
	}
	if !strings.Contains(dsn, "application_name=") {
		dsn = needsSpace(dsn) + "application_name=" + applicationName
	}
	return dsn
}

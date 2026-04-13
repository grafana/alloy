// Package opampmanager implements an experimental in-process OpAMP client for the
// Alloy OTel engine (POC). Enable with --opamp-managed-config (YAML only).
package opampmanager

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

const EnvGCLOUDBasicAuth = "GCLOUD_BASIC_AUTH"

func ExportGCLOUDBasicAuthEnv(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return os.Unsetenv(EnvGCLOUDBasicAuth)
	}
	return os.Setenv(EnvGCLOUDBasicAuth, token)
}

// Config holds settings for the OpAMP manager POC (from the managed YAML file).
type Config struct {
	Enabled        bool
	ServerURL      string
	EffectivePath  string
	StatePath      string
	HealthCheckURL string
	TLSInsecure    bool
	BasicAuthToken string
}

func validateManagedOpAMPConfig(cfg *Config) error {
	if cfg.ServerURL == "" {
		return fmt.Errorf("opamp server_url is required")
	}
	u, err := url.Parse(cfg.ServerURL)
	if err != nil {
		return fmt.Errorf("server_url: invalid URL: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return fmt.Errorf("server_url must be http or https, got scheme %q", u.Scheme)
	}
	if strings.TrimSpace(cfg.EffectivePath) == "" {
		return fmt.Errorf("effective_config_path is required (must match alloy otel --config=file: path)")
	}
	cfg.EffectivePath = strings.TrimSpace(cfg.EffectivePath)
	if strings.TrimSpace(cfg.StatePath) == "" {
		cfg.StatePath = cfg.EffectivePath + ".opampstate.json"
	} else {
		cfg.StatePath = strings.TrimSpace(cfg.StatePath)
	}
	cfg.ServerURL = strings.TrimSpace(cfg.ServerURL)
	cfg.HealthCheckURL = strings.TrimSpace(cfg.HealthCheckURL)
	cfg.BasicAuthToken = strings.TrimSpace(cfg.BasicAuthToken)
	return nil
}

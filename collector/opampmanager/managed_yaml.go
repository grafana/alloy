package opampmanager

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type managedFileConfig struct {
	ServerURL             string `yaml:"server_url"`
	EffectiveConfigPath   string `yaml:"effective_config_path"`
	StatePath             string `yaml:"state_path,omitempty"`
	HealthCheckURL        string `yaml:"healthcheck_url,omitempty"`
	TLSInsecureSkipVerify bool   `yaml:"tls_insecure_skip_verify,omitempty"`
	InstanceID            string `yaml:"instance_id,omitempty"`
	RWAPIKey              string `yaml:"rw_api_key,omitempty"`
}

func LoadManagedConfigFromPath(path string) (Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Config{}, fmt.Errorf("opampmanager: managed config path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("opampmanager: read managed config %q: %w", path, err)
	}
	var raw managedFileConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("opampmanager: parse managed config YAML: %w", err)
	}
	basicTok, err := basicAuthTokenFromManaged(raw)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Enabled:        true,
		ServerURL:      strings.TrimSpace(raw.ServerURL),
		EffectivePath:  strings.TrimSpace(raw.EffectiveConfigPath),
		StatePath:      strings.TrimSpace(raw.StatePath),
		HealthCheckURL: strings.TrimSpace(raw.HealthCheckURL),
		TLSInsecure:    raw.TLSInsecureSkipVerify,
		BasicAuthToken: basicTok,
	}
	if err := validateManagedOpAMPConfig(&cfg); err != nil {
		return Config{}, err
	}
	if err := ExportGCLOUDBasicAuthEnv(cfg.BasicAuthToken); err != nil {
		return Config{}, fmt.Errorf("opampmanager: export %s: %w", EnvGCLOUDBasicAuth, err)
	}
	return cfg, nil
}

func basicAuthTokenFromManaged(raw managedFileConfig) (string, error) {
	inst := strings.TrimSpace(raw.InstanceID)
	key := strings.TrimSpace(raw.RWAPIKey)
	switch {
	case inst == "" && key == "":
		return "", nil
	case inst == "" || key == "":
		return "", fmt.Errorf("opampmanager: instance_id and rw_api_key must both be set for OpAMP auth")
	default:
		return base64.StdEncoding.EncodeToString([]byte(inst + ":" + key)), nil
	}
}

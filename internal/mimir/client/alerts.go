package client

import (
	"context"

	"github.com/grafana/alloy/internal/runtime/logging/level"
	alertmgr_cfg "github.com/prometheus/alertmanager/config"
	"gopkg.in/yaml.v3"
)

// This struct is what Mimir expects to receive:
// https://github.com/grafana/mimir/blob/8a08500af3c50763d21a78b2a08007a2abcb5af1/pkg/mimirtool/client/alerts.go#L20-L23
// TODO: Export the struct in the Mimir repo, so that we can import it instead of copying it.
type configCompat struct {
	TemplateFiles      map[string]string `yaml:"template_files"`
	AlertmanagerConfig string            `yaml:"alertmanager_config"`
}

func (r *MimirClient) CreateAlertmanagerConfigs(ctx context.Context, conf *alertmgr_cfg.Config, templateFiles map[string]string) error {
	// If we don't set this, secrets will be obfuscated by being set to "<secret>".
	alertmgr_cfg.MarshalSecretValue = true

	payload := configCompat{
		AlertmanagerConfig: conf.String(),
		TemplateFiles:      templateFiles,
	}
	payloadStr, err := yaml.Marshal(&payload)
	if err != nil {
		return err
	}

	level.Debug(r.logger).Log("msg", "sending Alertmanager config to Mimir", "config", payloadStr)

	op := "/api/v1/alerts"
	path := "/api/v1/alerts"

	res, err := r.doRequest(op, path, "POST", payloadStr)
	if err != nil {
		return err
	}

	res.Body.Close()

	return nil
}

package client

import (
	"context"

	alertmgr_cfg "github.com/grafana/alloy/internal/mimir/alertmanager"
	"github.com/grafana/alloy/internal/runtime/logging/level"
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
	confStr, err := conf.String()
	if err != nil {
		return err
	}

	payload := configCompat{
		AlertmanagerConfig: confStr,
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

package client

import (
	"context"
	"io"
	"net/url"

	"github.com/grafana/alloy/internal/runtime/logging/level"
	alertmgr_cfg "github.com/prometheus/alertmanager/config"
	"gopkg.in/yaml.v3"
)

// RemoteWriteConfig is used to specify a remote write endpoint
type RemoteWriteConfig struct {
	URL string `json:"url,omitempty"`
}

// CreateRuleGroup creates a new rule group
func (r *MimirClient) CreateRuleGroup(ctx context.Context, namespace string, rg MimirRuleGroup) error {
	payload, err := yaml.Marshal(&rg)
	if err != nil {
		return err
	}

	escapedNamespace := url.PathEscape(namespace)
	path := r.apiPath + "/" + escapedNamespace
	op := r.apiPath + "/" + "<namespace>"

	res, err := r.doRequest(op, path, "POST", payload)
	if err != nil {
		return err
	}

	res.Body.Close()

	return nil
}

// DeleteRuleGroup deletes a rule group
func (r *MimirClient) DeleteRuleGroup(ctx context.Context, namespace, groupName string) error {
	escapedNamespace := url.PathEscape(namespace)
	escapedGroupName := url.PathEscape(groupName)
	path := r.apiPath + "/" + escapedNamespace + "/" + escapedGroupName
	op := r.apiPath + "/" + "<namespace>" + "/" + "<group_name>"

	res, err := r.doRequest(op, path, "DELETE", nil)
	if err != nil {
		return err
	}

	res.Body.Close()

	return nil
}

// ListRules retrieves a rule group
func (r *MimirClient) ListRules(ctx context.Context, namespace string) (map[string][]MimirRuleGroup, error) {
	path := r.apiPath
	op := r.apiPath
	if namespace != "" {
		path = path + "/" + namespace
		op = op + "/" + "<namespace>"
	}

	res, err := r.doRequest(op, path, "GET", nil)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	ruleSet := map[string][]MimirRuleGroup{}
	err = yaml.Unmarshal(body, &ruleSet)
	if err != nil {
		return nil, err
	}

	return ruleSet, nil
}

// This struct is what Mimir expects to receive:
// https://github.com/grafana/mimir/blob/8a08500af3c50763d21a78b2a08007a2abcb5af1/pkg/mimirtool/client/alerts.go#L20-L23
type configCompat struct {
	TemplateFiles      map[string]string `yaml:"template_files"`
	AlertmanagerConfig string            `yaml:"alertmanager_config"`
}

func (r *MimirClient) CreateAlertmanagerConfigs(ctx context.Context, conf alertmgr_cfg.Config, templateFiles map[string]string) error {
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

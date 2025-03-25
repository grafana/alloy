package client

import (
	"context"
	"io"
	"net/url"

	alertmgr_cfg "github.com/prometheus/alertmanager/config"
	"github.com/prometheus/prometheus/model/rulefmt"
	"gopkg.in/yaml.v3"
)

// RemoteWriteConfig is used to specify a remote write endpoint
type RemoteWriteConfig struct {
	URL string `json:"url,omitempty"`
}

// CreateRuleGroup creates a new rule group
func (r *MimirClient) CreateRuleGroup(ctx context.Context, namespace string, rg rulefmt.RuleGroup) error {
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
func (r *MimirClient) ListRules(ctx context.Context, namespace string) (map[string][]rulefmt.RuleGroup, error) {
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

	ruleSet := map[string][]rulefmt.RuleGroup{}
	err = yaml.Unmarshal(body, &ruleSet)
	if err != nil {
		return nil, err
	}

	return ruleSet, nil
}

func (r *MimirClient) CreateAlertmanagerConfigs(ctx context.Context, conf alertmgr_cfg.Config) error {
	payload, err := yaml.Marshal(&conf)
	if err != nil {
		return err
	}

	// escapedNamespace := url.PathEscape(namespace)
	// TODO: Use the correct API path
	// path := r.apiPath + "/" + escapedNamespace
	// op := r.apiPath + "/" + "<namespace>"
	op := ""
	path := ""

	res, err := r.doRequest(op, path, "POST", payload)
	if err != nil {
		return err
	}

	res.Body.Close()

	return nil
}

// func (r *MimirClient) ListAlertmanagerConfigs(ctx context.Context, namespace string) (map[string][]alertmgr_cfg.Config, error) {
// 	// TODO: Use the correct API path
// 	path := r.apiPath
// 	op := r.apiPath
// 	if namespace != "" {
// 		path = path + "/" + namespace
// 		op = op + "/" + "<namespace>"
// 	}

// 	res, err := r.doRequest(op, path, "GET", nil)
// 	if err != nil {
// 		return nil, err
// 	}

// 	defer res.Body.Close()
// 	body, err := io.ReadAll(res.Body)

// 	if err != nil {
// 		return nil, err
// 	}

// 	configSet := map[string][]alertmgr_cfg.Config{}
// 	err = yaml.Unmarshal(body, &configSet)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return configSet, nil
// }

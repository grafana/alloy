package client

import (
	"bytes"
	"context"
	"io"
	"net/url"

	"go.yaml.in/yaml/v3"
)

// RemoteWriteConfig is used to specify a remote write endpoint
type RemoteWriteConfig struct {
	URL string `json:"url,omitempty"`
}

// CreateRuleGroup creates a new rule group
func (r *MimirClient) CreateRuleGroup(ctx context.Context, namespace string, rg MimirRuleGroup) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	// With the default indent of 4, block scalars inside the rules list get a wrong indentation indicator.
	enc.SetIndent(2)
	if err := enc.Encode(&rg); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	payload := buf.Bytes()

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

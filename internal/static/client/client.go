// Package client provides a client interface to the Agent HTTP
// API.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/grafana/agent/internal/static/metrics/cluster/configapi"
	"github.com/grafana/agent/internal/static/metrics/instance"
	"gopkg.in/yaml.v2"
)

// Client is a collection of all subsystem clients.
type Client struct {
	PrometheusClient
}

// New creates a new Client.
func New(addr string) *Client {
	return &Client{
		PrometheusClient: &prometheusClient{addr: addr},
	}
}

// PrometheusClient is the client interface to the API exposed by the
// Prometheus subsystem of the Grafana Agent.
type PrometheusClient interface {
	// Instances runs the list of currently running instances.
	Instances(ctx context.Context) ([]string, error)

	// The following methods are for the scraping service mode
	// only and will fail when not enabled on the Agent.

	// ListConfigs runs the list of instance configs stored in the config
	// management KV store.
	ListConfigs(ctx context.Context) (*configapi.ListConfigurationsResponse, error)

	// GetConfiguration returns a named configuration from the config
	// management KV store.
	GetConfiguration(ctx context.Context, name string) (*instance.Config, error)

	// PutConfiguration adds or updates a named configuration into the
	// config management KV store.
	PutConfiguration(ctx context.Context, name string, cfg *instance.Config) error

	// DeleteConfiguration removes a named configuration from the config
	// management KV store.
	DeleteConfiguration(ctx context.Context, name string) error
}

type prometheusClient struct {
	addr string
}

func (c *prometheusClient) Instances(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("%s/agent/api/v1/metrics/instances", c.addr)

	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var data []string
	err = unmarshalPrometheusAPIResponse(resp.Body, &data)
	return data, err
}

func (c *prometheusClient) ListConfigs(ctx context.Context) (*configapi.ListConfigurationsResponse, error) {
	url := fmt.Sprintf("%s/agent/api/v1/configs", c.addr)

	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var data configapi.ListConfigurationsResponse
	err = unmarshalPrometheusAPIResponse(resp.Body, &data)
	return &data, err
}

func (c *prometheusClient) GetConfiguration(ctx context.Context, name string) (*instance.Config, error) {
	url := fmt.Sprintf("%s/agent/api/v1/configs/%s", c.addr, name)

	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var data configapi.GetConfigurationResponse
	if err := unmarshalPrometheusAPIResponse(resp.Body, &data); err != nil {
		return nil, err
	}

	var config instance.Config
	err = yaml.NewDecoder(strings.NewReader(data.Value)).Decode(&config)
	return &config, err
}

func (c *prometheusClient) PutConfiguration(ctx context.Context, name string, cfg *instance.Config) error {
	url := fmt.Sprintf("%s/agent/api/v1/config/%s", c.addr, name)

	bb, err := instance.MarshalConfig(cfg, false)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(ctx, "POST", url, bytes.NewReader(bb))
	if err != nil {
		return err
	}

	return unmarshalPrometheusAPIResponse(resp.Body, nil)
}

func (c *prometheusClient) DeleteConfiguration(ctx context.Context, name string) error {
	url := fmt.Sprintf("%s/agent/api/v1/config/%s", c.addr, name)

	resp, err := c.doRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	return unmarshalPrometheusAPIResponse(resp.Body, nil)
}

func (c *prometheusClient) doRequest(ctx context.Context, method string, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

// unmarshalPrometheusAPIResponse will unmarshal a response from the Prometheus
// subsystem API.
//
// r will be closed after this method is called.
func unmarshalPrometheusAPIResponse(r io.ReadCloser, v interface{}) error {
	defer func() {
		_ = r.Close()
	}()

	resp := struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}{}

	err := json.NewDecoder(r).Decode(&resp)
	if err != nil {
		return fmt.Errorf("could not read response: %w", err)
	}

	if v != nil && resp.Status == "success" {
		err := json.Unmarshal(resp.Data, v)
		if err != nil {
			return fmt.Errorf("unmarshaling response: %w", err)
		}
	} else if resp.Status == "error" {
		var errResp configapi.ErrorResponse
		err := json.Unmarshal(resp.Data, &errResp)
		if err != nil {
			return fmt.Errorf("unmarshaling error: %w", err)
		}

		return fmt.Errorf("%s", errResp.Error)
	}

	if resp.Status != "success" && resp.Status != "error" {
		return fmt.Errorf("unknown API response status: %s", resp.Status)
	}

	return nil
}

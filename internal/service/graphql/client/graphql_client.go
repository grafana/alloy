package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

//
// This package allows alloy to execute GraphQL queries against the Alloy GraphQL API.
//

// GraphQlClient represents a simple GraphQL client
type GraphQlClient struct {
	endpoint   string
	httpClient *http.Client
	headers    map[string]string
}

// GraphQlRequest represents a GraphQL request
type GraphQlRequest struct {
	Query string `json:"query"`
}

// GraphQlResponse represents a GraphQL response
type GraphQlResponse struct {
	Data   any   `json:"data,omitempty"`
	Errors []any `json:"errors,omitempty"`
	Raw    []byte
}

// NewGraphQlClient creates a new GraphQL client
func NewGraphQlClient(endpoint string) *GraphQlClient {
	return &GraphQlClient{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		headers: make(map[string]string),
	}
}

// SetHeader sets a custom header (useful for future auth support)
func (c *GraphQlClient) SetHeader(key, value string) {
	c.headers[key] = value
}

// Execute sends a GraphQL query and returns the raw response bytes
func (c *GraphQlClient) Execute(query string) (*GraphQlResponse, error) {
	reqBody := GraphQlRequest{Query: query}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return c.parseResponse(buf.Bytes())
}

// parseResponse converts raw GraphQL response bytes into a basic GraphQlResponse struct
func (c *GraphQlClient) parseResponse(data []byte) (*GraphQlResponse, error) {
	var resp GraphQlResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL response: %w", err)
	}
	resp.Raw = data
	return &resp, nil
}

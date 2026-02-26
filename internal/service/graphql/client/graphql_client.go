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

// GraphQLClient represents a simple GraphQL client
type GraphQLClient struct {
	endpoint   string
	httpClient *http.Client
	headers    map[string]string
}

// GraphQLRequest represents a GraphQL request
type GraphQLRequest struct {
	Query string `json:"query"`
}

// GraphQLResponse represents a GraphQL response
type GraphQLResponse struct {
	Data   any   `json:"data,omitempty"`
	Errors []any `json:"errors,omitempty"`
	Raw    []byte
}

// NewGraphQLClient creates a new GraphQL client
func NewGraphQLClient(endpoint string) *GraphQLClient {
	return &GraphQLClient{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		headers: make(map[string]string),
	}
}

// SetHeader sets a custom header (useful for future auth support)
func (c *GraphQLClient) SetHeader(key, value string) {
	c.headers[key] = value
}

// Execute sends a GraphQL query and returns the raw response bytes
func (c *GraphQLClient) Execute(query string) (*GraphQLResponse, error) {
	reqBody := GraphQLRequest{Query: query}
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
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to execute request: %s", resp.Status)
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return c.parseResponse(buf.Bytes())
}

// parseResponse converts raw GraphQL response bytes into a basic GraphQLResponse struct
func (c *GraphQLClient) parseResponse(data []byte) (*GraphQLResponse, error) {
	var resp GraphQLResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL response: %w", err)
	}
	resp.Raw = data
	return &resp, nil
}

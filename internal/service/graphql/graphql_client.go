package graphql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// GraphQlClient represents a simple GraphQL client
type GraphQlClient struct {
	endpoint   string
	httpClient *http.Client
	headers    map[string]string

	// Cached schema with mutex for thread safety
	schema   *ast.Schema
	schemaMu sync.RWMutex
}

// GraphQLRequest represents a GraphQL request
type GraphQLRequest struct {
	Query string `json:"query"`
}

// GraphQLResponse represents a GraphQL response
type GraphQLResponse struct {
	Data   json.RawMessage `json:"data,omitempty"`
	Errors []GraphQLError  `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error
type GraphQLError struct {
	Message   string                 `json:"message"`
	Locations []GraphQLErrorLocation `json:"locations,omitempty"`
	Path      []any                  `json:"path,omitempty"`
}

// GraphQLErrorLocation represents error location
type GraphQLErrorLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// NewGraphQLClient creates a new GraphQL client
func NewGraphQLClient(endpoint string) *GraphQlClient {
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

// Execute sends a GraphQL query and returns the response as a pretty-printed JSON string
func (c *GraphQlClient) Execute(query string) (string, error) {
	reqBody := GraphQLRequest{Query: query}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the response into GraphQLResponse struct and pretty print it
	var gqlResp GraphQLResponse
	if err := json.Unmarshal(buf.Bytes(), &gqlResp); err != nil {
		return "", fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	// Pretty print the entire response
	prettyBytes, err := json.MarshalIndent(gqlResp, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to pretty print response: %w", err)
	}

	return string(prettyBytes), nil
}

// Introspect performs GraphQL introspection and returns the parsed schema. The schema is cached
// for future calls.
func (c *GraphQlClient) Introspect() (*ast.Schema, error) {
	// Check if schema is already cached
	c.schemaMu.RLock()
	if c.schema != nil {
		defer c.schemaMu.RUnlock()
		return c.schema, nil
	}
	c.schemaMu.RUnlock()

	// Acquire write lock to perform introspection
	c.schemaMu.Lock()
	defer c.schemaMu.Unlock()

	// Double-check in case another goroutine cached it while we were waiting
	if c.schema != nil {
		return c.schema, nil
	}

	respStr, err := c.Execute(introspectionQuery)
	if err != nil {
		return nil, fmt.Errorf("introspection query failed: %w", err)
	}

	// Parse the response string into a GraphQLResponse struct
	var resp GraphQLResponse
	if err := json.Unmarshal([]byte(respStr), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse introspection response: %w", err)
	}

	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("introspection errors: %v", resp.Errors)
	}

	// Convert introspection result to schema using gqlparser
	schema, err := gqlparser.LoadSchema(&ast.Source{
		Name:    "introspection",
		Input:   string(resp.Data),
		BuiltIn: false,
	})
	if err != nil {
		// TODO: Is this the error message we'll get if the server is down? If so, improve it.
		return nil, fmt.Errorf("failed to parse introspection data with gqlparser")
	}

	// Cache the schema
	c.schema = schema
	return schema, nil
}

// ValidateQuery validates a GraphQL query against the introspected schema
func (c *GraphQlClient) ValidateQuery(query string) error {
	schema, err := c.Introspect()
	if err != nil {
		return fmt.Errorf("failed to get schema for validation: %w", err)
	}

	// Parse and validate the query
	_, gqlErr := gqlparser.LoadQuery(schema, query)
	if gqlErr != nil {
		return fmt.Errorf("query validation failed: %v", gqlErr)
	}

	return nil
}

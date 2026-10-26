package pipelinetest

import (
	"fmt"

	"github.com/grafana/alloy/internal/pipelinetest/harness"
	"github.com/grafana/alloy/syntax/scanner"
	builder "github.com/grafana/alloy/syntax/token/builder"
)

// MockSchema groups stub upstreams that the pipeline under test can call.
type MockSchema struct {
	HTTP []HTTPMockSchema `yaml:"http"`
}

// HTTPMockSchema describes one stub HTTP server. Reference its base URL from
// the config as pipelinetest.mocks.http.<name>.url, for example
//
//	address = pipelinetest.mocks.http.upstream.url + "/healthy"
//
// The server listens on a free local port. Requests that match no route get a
// 404 response.
type HTTPMockSchema struct {
	// Name identifies the mock in the config. It must be a valid Alloy
	// identifier and unique within the test.
	Name   string                `yaml:"name"`
	Routes []HTTPMockRouteSchema `yaml:"routes"`
}

// HTTPMockRouteSchema describes one canned response. A request matches when
// its path equals Path exactly, query string excluded, and its method equals
// Method. Omit Method to match any method. When both match a request, the
// route with Method set wins.
type HTTPMockRouteSchema struct {
	Path   string `yaml:"path"`
	Method string `yaml:"method,omitempty"`
	// Status is the response status code. Defaults to 200.
	Status  int               `yaml:"status,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty"`
}

func validateMocks(mocks MockSchema) error {
	seen := make(map[string]struct{}, len(mocks.HTTP))
	for i, mock := range mocks.HTTP {
		if mock.Name == "" {
			return fmt.Errorf("http mock %d requires a name", i)
		}
		if !scanner.IsValidIdentifier(mock.Name) {
			return fmt.Errorf("http mock name %q must be a valid Alloy identifier", mock.Name)
		}
		if _, ok := seen[mock.Name]; ok {
			return fmt.Errorf("duplicate http mock name %q", mock.Name)
		}
		seen[mock.Name] = struct{}{}

		if len(mock.Routes) == 0 {
			return fmt.Errorf("http mock %q requires at least one route", mock.Name)
		}
		for j, route := range mock.Routes {
			if route.Path == "" {
				return fmt.Errorf("http mock %q route %d requires a path", mock.Name, j)
			}
		}
	}
	return nil
}

// appendMocks adds one pipelinetest.mocks.http component per HTTP mock. The
// component validates the routes when the config loads.
func appendMocks(body *builder.Body, mocks MockSchema) {
	for _, mock := range mocks.HTTP {
		args := harness.MockHTTPArguments{Routes: make([]harness.MockHTTPRoute, 0, len(mock.Routes))}
		for _, route := range mock.Routes {
			args.Routes = append(args.Routes, harness.MockHTTPRoute{
				Path:    route.Path,
				Method:  route.Method,
				Status:  route.Status,
				Headers: route.Headers,
				Body:    route.Body,
			})
		}

		block := builder.NewBlock([]string{"pipelinetest", "mocks", "http"}, mock.Name)
		block.Body().AppendFrom(args)
		body.AppendBlock(block)
	}
}

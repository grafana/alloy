package pipelinetest

import (
	"testing"

	"github.com/stretchr/testify/require"

	builder "github.com/grafana/alloy/syntax/token/builder"
)

func TestValidateMocks(t *testing.T) {
	route := []HTTPMockRouteSchema{{Path: "/"}}

	tests := []struct {
		name    string
		mocks   []HTTPMockSchema
		wantErr string
	}{
		{name: "valid", mocks: []HTTPMockSchema{{Name: "a", Routes: route}, {Name: "b", Routes: route}}},
		{name: "missing name", mocks: []HTTPMockSchema{{Routes: route}}, wantErr: "requires a name"},
		{name: "invalid name", mocks: []HTTPMockSchema{{Name: "my-mock", Routes: route}}, wantErr: "valid Alloy identifier"},
		{name: "duplicate name", mocks: []HTTPMockSchema{{Name: "a", Routes: route}, {Name: "a", Routes: route}}, wantErr: "duplicate http mock name"},
		{name: "no routes", mocks: []HTTPMockSchema{{Name: "a"}}, wantErr: "at least one route"},
		{name: "missing path", mocks: []HTTPMockSchema{{Name: "a", Routes: []HTTPMockRouteSchema{{Body: "x"}}}}, wantErr: "requires a path"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMocks(MockSchema{HTTP: tc.mocks})
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestAppendMocks(t *testing.T) {
	file := builder.NewFile()
	appendMocks(file.Body(), MockSchema{HTTP: []HTTPMockSchema{{
		Name: "upstream",
		Routes: []HTTPMockRouteSchema{
			{Path: "/healthy", Body: "ok"},
			{Path: "/api", Method: "POST", Status: 201, Headers: map[string]string{"Content-Type": "application/json"}, Body: `{"ok":true}`},
		},
	}}})

	expected := `pipelinetest.mocks.http "upstream" {
	route {
		path = "/healthy"
		body = "ok"
	}

	route {
		path    = "/api"
		method  = "POST"
		status  = 201
		headers = {
			"Content-Type" = "application/json",
		}
		body = "{\"ok\":true}"
	}
}`
	require.Equal(t, expected, string(file.Bytes()))
}

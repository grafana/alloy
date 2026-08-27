package alloycli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAlloyGqlRunExecutesQuery(t *testing.T) {
	var receivedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req struct {
			Query string `json:"query"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		receivedQuery = req.Query

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"data":{"alloy":{"isReady":true}}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	g := &alloyGql{endpoint: server.URL}
	var output bytes.Buffer
	require.NoError(t, g.Run("alloy { isReady }", &output))

	require.Equal(t, "query { alloy { isReady } }", receivedQuery)
	require.JSONEq(t, `{"data":{"alloy":{"isReady":true}}}`, output.String())
}

func TestAlloyGqlRunLeavesCompleteQueryUnchanged(t *testing.T) {
	var receivedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query string `json:"query"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		receivedQuery = req.Query

		_, err := w.Write([]byte(`{"data":{"alloy":{"version":"dev"}}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	g := &alloyGql{endpoint: server.URL}
	var output bytes.Buffer
	require.NoError(t, g.Run("{ alloy { version } }", &output))

	require.Equal(t, "{ alloy { version } }", receivedQuery)
	require.JSONEq(t, `{"data":{"alloy":{"version":"dev"}}}`, output.String())
}

func TestFormatGraphQLQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		want    string
		wantErr string
	}{
		{
			name:  "wraps field selection",
			query: "alloy { isReady }",
			want:  "query { alloy { isReady } }",
		},
		{
			name: "leaves shorthand query unchanged",
			query: `{
	alloy {
		isReady
	}
}`,
			want: `{
	alloy {
		isReady
	}
}`,
		},
		{
			name:  "leaves compact shorthand query unchanged",
			query: "{alloy{version}}",
			want:  "{alloy{version}}",
		},
		{
			name:  "leaves compact operation unchanged",
			query: "query{ alloy { isReady } }",
			want:  "query{ alloy { isReady } }",
		},
		{
			name: "leaves operation with newline before selection unchanged",
			query: `query
{
	alloy {
		isReady
	}
}`,
			want: `query
{
	alloy {
		isReady
	}
}`,
		},
		{
			name: "leaves named operation with multiline selection unchanged",
			query: `query GetAlloy
{
	alloy {
		isReady
	}
}`,
			want: `query GetAlloy
{
	alloy {
		isReady
	}
}`,
		},
		{
			name:  "leaves mutation with tab before selection unchanged",
			query: "mutation\t{ reload { success } }",
			want:  "mutation\t{ reload { success } }",
		},
		{
			name: "leaves subscription with newline before selection unchanged",
			query: `subscription
{
	events {
		id
	}
}`,
			want: `subscription
{
	events {
		id
	}
}`,
		},
		{
			name:    "rejects parameterized operation",
			query:   `query($componentID: String!) { component(id: $componentID) { id } }`,
			wantErr: "query parameters are not supported",
		},
		{
			name:    "rejects named parameterized operation",
			query:   `query GetComponent($componentID: String!) { component(id: $componentID) { id } }`,
			wantErr: "query parameters are not supported",
		},
		{
			name:    "rejects field selection with arguments before selection",
			query:   `component(id: "local.file") { id }`,
			wantErr: "query parameters are not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatGraphQLQuery(tt.query)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestAlloyGqlRunReturnsErrorForGraphQLErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"errors":[{"message":"bad query"}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	g := &alloyGql{endpoint: server.URL}
	var output bytes.Buffer
	runErr := g.Run("{ alloy {", &output)

	require.Error(t, runErr)
	require.ErrorContains(t, runErr, "GraphQL response contains errors")
	require.JSONEq(t, `{"errors":[{"message":"bad query"}]}`, output.String())
}

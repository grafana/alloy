package podlogs

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
    forward_to = []
	client {
		api_server = "localhost:9091"
	}
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

func TestBadAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
    forward_to = []
	client {
		api_server = "localhost:9091"
		bearer_token = "token"
		bearer_token_file = "/path/to/file.token"
	}
`

	// Make sure the squashed HTTPClientConfig Validate function is being utilized correctly
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.ErrorContains(t, err, "at most one of basic_auth, authorization, oauth2, bearer_token & bearer_token_file must be configured")
}

func TestNodeFilterConfig(t *testing.T) {
	tests := []struct {
		name           string
		config         string
		expectedError  string
		expectedResult Arguments
	}{
		{
			name: "node filter disabled by default",
			config: `
				forward_to = []
			`,
			expectedResult: Arguments{
				NodeFilter: NodeFilterConfig{
					Enabled:  false,
					NodeName: "",
				},
			},
		},
		{
			name: "node filter enabled with explicit node name",
			config: `
				forward_to = []
				node_filter {
					enabled = true
					node_name = "worker-node-1"
				}
			`,
			expectedResult: Arguments{
				NodeFilter: NodeFilterConfig{
					Enabled:  true,
					NodeName: "worker-node-1",
				},
			},
		},
		{
			name: "node filter enabled without node name",
			config: `
				forward_to = []
				node_filter {
					enabled = true
				}
			`,
			expectedResult: Arguments{
				NodeFilter: NodeFilterConfig{
					Enabled:  true,
					NodeName: "",
				},
			},
		},
		{
			name: "node filter with only node name specified",
			config: `
				forward_to = []
				node_filter {
					node_name = "worker-node-2"
				}
			`,
			expectedResult: Arguments{
				NodeFilter: NodeFilterConfig{
					Enabled:  false, // default value
					NodeName: "worker-node-2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tt.config), &args)

			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedResult.NodeFilter.Enabled, args.NodeFilter.Enabled)
			require.Equal(t, tt.expectedResult.NodeFilter.NodeName, args.NodeFilter.NodeName)
		})
	}
}

func TestNodeFilterConfigDefaults(t *testing.T) {
	var args Arguments
	args.SetToDefault()

	// Verify node filter is disabled by default
	require.False(t, args.NodeFilter.Enabled)
	require.Empty(t, args.NodeFilter.NodeName)
}

func TestPreserveDiscoveredLabelsConfig(t *testing.T) {
	tests := []struct {
		name           string
		config         string
		expectedError  string
		expectedResult bool
	}{
		{
			name: "preserve_discovered_labels disabled by default",
			config: `
				forward_to = []
			`,
			expectedResult: false,
		},
		{
			name: "preserve_discovered_labels enabled explicitly",
			config: `
				forward_to = []
				preserve_discovered_labels = true
			`,
			expectedResult: true,
		},
		{
			name: "preserve_discovered_labels disabled explicitly",
			config: `
				forward_to = []
				preserve_discovered_labels = false
			`,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tt.config), &args)

			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedResult, args.PreserveDiscoveredLabels)
		})
	}
}

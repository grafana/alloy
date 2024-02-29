package common_test

import (
	"fmt"
	"testing"

	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/stretchr/testify/require"
)

func TestInvalidValidationType(t *testing.T) {
	diags := common.ValidateSupported(-1, nil, nil, "", "")
	require.Len(t, diags, 1)
	var expectedDiags diag.Diagnostics
	expectedDiags.Add(diag.SeverityLevelCritical, "Invalid converter validation type was requested: -1.")
	require.Equal(t, expectedDiags, diags)
}

func TestValidateSupported(t *testing.T) {
	tt := []struct {
		tcName         string
		validationType int
		value1         any
		value2         any
		name           string
		message        string
		expectDiag     bool
	}{
		{
			tcName:         "Unsupported Equals",
			validationType: common.Equals,
			value1:         "match",
			value2:         "match",
			name:           "test",
			message:        "",
			expectDiag:     true,
		},
		{
			tcName:         "Supported Equals",
			validationType: common.Equals,
			value1:         "not",
			value2:         "match",
			name:           "test",
			message:        "",
			expectDiag:     false,
		},
		{
			tcName:         "Unsupported NotEquals",
			validationType: common.NotEquals,
			value1:         "not",
			value2:         "match",
			name:           "test",
			message:        "message",
			expectDiag:     true,
		},
		{
			tcName:         "Supported NotEquals",
			validationType: common.NotEquals,
			value1:         "match",
			value2:         "match",
			name:           "test",
			message:        "message",
			expectDiag:     false,
		},
		{
			tcName:         "Unsupported DeepEquals",
			validationType: common.DeepEquals,
			value1:         []string{"match"},
			value2:         []string{"match"},
			name:           "test",
			message:        "",
			expectDiag:     true,
		},
		{
			tcName:         "Supported DeepEquals",
			validationType: common.DeepEquals,
			value1:         []string{"not"},
			value2:         []string{"match"},
			name:           "test",
			message:        "message",
			expectDiag:     false,
		},
		{
			tcName:         "Supported NotDeepEquals",
			validationType: common.NotDeepEquals,
			value1:         []string{"not"},
			value2:         []string{"match"},
			name:           "test",
			message:        "",
			expectDiag:     true,
		},
		{
			tcName:         "Supported NotDeepEquals",
			validationType: common.NotDeepEquals,
			value1:         []string{"match"},
			value2:         []string{"match"},
			name:           "test",
			message:        "message",
			expectDiag:     false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.tcName, func(t *testing.T) {
			diags := common.ValidateSupported(tc.validationType, tc.value1, tc.value2, tc.name, tc.message)
			if tc.expectDiag {
				require.Len(t, diags, 1)
				var expectedDiags diag.Diagnostics
				if tc.message != "" {
					expectedDiags.Add(diag.SeverityLevelError, fmt.Sprintf("The converter does not support converting the provided %s config: %s", tc.name, tc.message))
				} else {
					expectedDiags.Add(diag.SeverityLevelError, fmt.Sprintf("The converter does not support converting the provided %s config.", tc.name))
				}

				require.Equal(t, expectedDiags, diags)
			} else {
				require.Len(t, diags, 0)
			}
		})
	}
}

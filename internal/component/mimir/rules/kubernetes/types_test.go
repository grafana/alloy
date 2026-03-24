package rules

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatcherValidate(t *testing.T) {
	tests := []struct {
		name      string
		matcher   Matcher
		expectErr string
	}{
		{
			name:      "valid with Value",
			matcher:   Matcher{MatchType: "=", Value: "foo"},
			expectErr: "",
		},
		{
			name:      "valid with ValueFromLabel",
			matcher:   Matcher{MatchType: "=", ValueFromLabel: "bar"},
			expectErr: "",
		},
		{
			name:      "invalid match type",
			matcher:   Matcher{MatchType: "invalid", Value: "foo"},
			expectErr: "invalid match type",
		},
		{
			name:      "multiple value sources",
			matcher:   Matcher{MatchType: "=", Value: "foo", ValueFromLabel: "bar"},
			expectErr: "exactly one of",
		},
		{
			name:      "no value source provided",
			matcher:   Matcher{MatchType: "="},
			expectErr: "exactly one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.matcher.Validate()
			if tt.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectErr)
			}
		})
	}
}

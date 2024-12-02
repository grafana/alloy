package write

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Key
		wantErr  bool
	}{
		{
			name:  "basic app name",
			input: "simple-app",
			expected: &Key{
				labels: map[string]string{
					"__name__": "simple-app",
				},
			},
		},
		{
			name:  "app name with slashes and tags",
			input: "my/service/name{environment=prod,version=1.0}",
			expected: &Key{
				labels: map[string]string{
					"__name__":    "my/service/name",
					"environment": "prod",
					"version":     "1.0",
				},
			},
		},
		{
			name:  "multiple slashes and special characters",
			input: "app/service/v1.0-beta/component{region=us-west}",
			expected: &Key{
				labels: map[string]string{
					"__name__": "app/service/v1.0-beta/component",
					"region":   "us-west",
				},
			},
		},
		{
			name:    "empty app name",
			input:   "{}",
			wantErr: true,
		},
		{
			name:    "invalid characters in tag key",
			input:   "my/service/name{invalid@key=value}",
			wantErr: true,
		},
		{
			name:  "whitespace handling",
			input: "my/service/name{ tag1 = value1 , tag2 = value2 }",
			expected: &Key{
				labels: map[string]string{
					"__name__": "my/service/name",
					"tag1":     "value1",
					"tag2":     "value2",
				},
			},
		},
		{
			name:  "dots in service name",
			input: "my/service.name/v1.0{environment=prod}",
			expected: &Key{
				labels: map[string]string{
					"__name__":    "my/service.name/v1.0",
					"environment": "prod",
				},
			},
		},
		{
			name:  "app name with slashes",
			input: "my/service/name{}",
			expected: &Key{
				labels: map[string]string{
					"__name__": "my/service/name",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseKey(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestKey_Normalized(t *testing.T) {
	tests := []struct {
		name     string
		key      *Key
		expected string
	}{
		{
			name: "simple normalization",
			key: &Key{
				labels: map[string]string{
					"__name__": "my/service/name",
				},
			},
			expected: "my/service/name{}",
		},
		{
			name: "normalization with tags",
			key: &Key{
				labels: map[string]string{
					"__name__":    "my/service/name",
					"environment": "prod",
					"version":     "1.0",
				},
			},
			expected: "my/service/name{environment=prod,version=1.0}",
		},
		{
			name: "tags should be sorted",
			key: &Key{
				labels: map[string]string{
					"__name__": "my/service/name",
					"c":        "3",
					"b":        "2",
					"a":        "1",
				},
			},
			expected: "my/service/name{a=1,b=2,c=3}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.key.Normalized()
			assert.Equal(t, tt.expected, got)
		})
	}
}

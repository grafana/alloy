package remotecfg

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/alloyseed"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultArguments(t *testing.T) {
	defaults := getDefaultArguments()

	// Check that ID is set to alloyseed UID
	assert.Equal(t, alloyseed.Get().UID, defaults.ID)

	// Check default values
	assert.Empty(t, defaults.URL)
	assert.Empty(t, defaults.Name)
	assert.NotNil(t, defaults.Attributes)
	assert.Empty(t, defaults.Attributes)
	assert.Equal(t, 1*time.Minute, defaults.PollFrequency)
	assert.NotNil(t, defaults.HTTPClientConfig)
}

func TestArguments_SetToDefault(t *testing.T) {
	// Create an Arguments with some non-default values
	args := Arguments{
		URL:           "https://example.com",
		ID:            "custom-id",
		Name:          "custom-name",
		Attributes:    map[string]string{"key": "value"},
		PollFrequency: 5 * time.Minute,
	}

	// Call SetToDefault
	args.SetToDefault()

	// Verify it matches getDefaultArguments
	expected := getDefaultArguments()
	assert.Equal(t, expected.ID, args.ID)
	assert.Equal(t, expected.URL, args.URL)
	assert.Equal(t, expected.Name, args.Name)
	assert.Equal(t, expected.PollFrequency, args.PollFrequency)
	assert.NotNil(t, args.Attributes)
	assert.Empty(t, args.Attributes)
	assert.NotNil(t, args.HTTPClientConfig)
}

func TestArguments_InterfaceCompliance(t *testing.T) {
	// Compile-time interface compliance checks
	var _ syntax.Defaulter = (*Arguments)(nil)
	var _ syntax.Validator = (*Arguments)(nil)

	// Runtime interface compliance
	args := &Arguments{}
	var defaulter syntax.Defaulter = args
	var validator syntax.Validator = args

	assert.NotNil(t, defaulter)
	assert.NotNil(t, validator)
}

func TestArguments_Validate_Success(t *testing.T) {
	tests := []struct {
		name string
		args Arguments
	}{
		{
			name: "valid arguments with minimum poll frequency",
			args: Arguments{
				URL:              "https://example.com",
				ID:               "test-id",
				Name:             "test-name",
				Attributes:       map[string]string{"custom": "value"},
				PollFrequency:    10 * time.Second,
				HTTPClientConfig: config.CloneDefaultHTTPClientConfig(),
			},
		},
		{
			name: "valid arguments with longer poll frequency",
			args: Arguments{
				URL:              "https://example.com",
				PollFrequency:    1 * time.Minute,
				HTTPClientConfig: config.CloneDefaultHTTPClientConfig(),
			},
		},
		{
			name: "valid arguments with empty attributes",
			args: Arguments{
				PollFrequency:    30 * time.Second,
				Attributes:       make(map[string]string),
				HTTPClientConfig: config.CloneDefaultHTTPClientConfig(),
			},
		},
		{
			name: "valid arguments with custom attributes",
			args: Arguments{
				PollFrequency: 30 * time.Second,
				Attributes: map[string]string{
					"environment": "production",
					"region":      "us-west-2",
					"version":     "1.0.0",
				},
				HTTPClientConfig: config.CloneDefaultHTTPClientConfig(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestArguments_Validate_PollFrequencyTooShort(t *testing.T) {
	tests := []struct {
		name          string
		pollFrequency time.Duration
	}{
		{
			name:          "9 seconds",
			pollFrequency: 9 * time.Second,
		},
		{
			name:          "1 second",
			pollFrequency: 1 * time.Second,
		},
		{
			name:          "zero duration",
			pollFrequency: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := Arguments{
				PollFrequency:    tt.pollFrequency,
				HTTPClientConfig: config.CloneDefaultHTTPClientConfig(),
			}

			err := args.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "poll_frequency must be at least \"10s\"")
			assert.Contains(t, err.Error(), tt.pollFrequency.String())
		})
	}
}

func TestArguments_Validate_ReservedAttributeNamespace(t *testing.T) {
	tests := []struct {
		name       string
		attributes map[string]string
	}{
		{
			name: "collector.version attribute",
			attributes: map[string]string{
				"collector.version": "1.0.0",
			},
		},
		{
			name: "collector.os attribute",
			attributes: map[string]string{
				"collector.os": "linux",
			},
		},
		{
			name: "collector.custom attribute",
			attributes: map[string]string{
				"collector.custom": "value",
			},
		},
		{
			name: "multiple attributes with one reserved",
			attributes: map[string]string{
				"valid":             "value",
				"collector.invalid": "value",
				"another":           "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := Arguments{
				PollFrequency:    30 * time.Second,
				Attributes:       tt.attributes,
				HTTPClientConfig: config.CloneDefaultHTTPClientConfig(),
			}

			err := args.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "\"collector\" is a reserved namespace")
		})
	}
}

func TestArguments_Validate_HTTPClientConfig(t *testing.T) {
	t.Run("nil HTTP client config", func(t *testing.T) {
		args := Arguments{
			PollFrequency:    30 * time.Second,
			HTTPClientConfig: nil,
		}

		err := args.Validate()
		assert.NoError(t, err) // Should not panic and should pass
	})

	t.Run("valid HTTP client config", func(t *testing.T) {
		args := Arguments{
			PollFrequency:    30 * time.Second,
			HTTPClientConfig: config.CloneDefaultHTTPClientConfig(),
		}

		err := args.Validate()
		assert.NoError(t, err)
	})

	t.Run("invalid HTTP client config", func(t *testing.T) {
		invalidConfig := config.CloneDefaultHTTPClientConfig()
		// Set multiple auth methods to trigger validation failure
		invalidConfig.BearerToken = "token123"
		invalidConfig.BasicAuth = &config.BasicAuth{
			Username: "user",
			Password: "pass",
		}

		args := Arguments{
			PollFrequency:    30 * time.Second,
			HTTPClientConfig: invalidConfig,
		}

		err := args.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at most one of basic_auth, authorization, oauth2, bearer_token & bearer_token_file must be configured")
	})
}

func TestArguments_Hash_Success(t *testing.T) {
	args := Arguments{
		URL:              "https://example.com",
		ID:               "test-id",
		Name:             "test-name",
		Attributes:       map[string]string{"key": "value"},
		PollFrequency:    30 * time.Second,
		HTTPClientConfig: config.CloneDefaultHTTPClientConfig(),
	}

	hash1, err1 := args.Hash()
	require.NoError(t, err1)
	assert.NotEmpty(t, hash1)

	// Same arguments should produce same hash
	hash2, err2 := args.Hash()
	require.NoError(t, err2)
	assert.Equal(t, hash1, hash2)

	// Different arguments should produce different hash
	args.URL = "https://different.com"
	hash3, err3 := args.Hash()
	require.NoError(t, err3)
	assert.NotEqual(t, hash1, hash3)
}

func TestArguments_Hash_Consistency(t *testing.T) {
	// Test that identical arguments produce identical hashes
	args1 := Arguments{
		URL:              "https://example.com",
		ID:               "test-id",
		PollFrequency:    1 * time.Minute,
		Attributes:       map[string]string{"env": "test"},
		HTTPClientConfig: config.CloneDefaultHTTPClientConfig(),
	}

	args2 := Arguments{
		URL:              "https://example.com",
		ID:               "test-id",
		PollFrequency:    1 * time.Minute,
		Attributes:       map[string]string{"env": "test"},
		HTTPClientConfig: config.CloneDefaultHTTPClientConfig(),
	}

	hash1, err1 := args1.Hash()
	require.NoError(t, err1)

	hash2, err2 := args2.Hash()
	require.NoError(t, err2)

	assert.Equal(t, hash1, hash2)
}

func TestArguments_Hash_EmptyArguments(t *testing.T) {
	// Test hashing of empty/default arguments
	args := Arguments{}

	hash, err := args.Hash()
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Should be deterministic
	hash2, err2 := args.Hash()
	require.NoError(t, err2)
	assert.Equal(t, hash, hash2)
}

func TestArguments_Hash_Format(t *testing.T) {
	args := Arguments{
		URL:              "https://example.com",
		PollFrequency:    30 * time.Second,
		HTTPClientConfig: config.CloneDefaultHTTPClientConfig(),
	}

	hash, err := args.Hash()
	require.NoError(t, err)

	// Hash should be a hex string (from fnv.New32())
	assert.Regexp(t, "^[a-f0-9]+$", hash)
	assert.Len(t, hash, 8) // fnv.New32() produces 4 bytes = 8 hex chars
}

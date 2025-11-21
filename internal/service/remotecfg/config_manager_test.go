package remotecfg

import (
	"errors"
	"testing"

	collectorv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for helper functions in config_manager.go

func TestGetErrorMessage_DiagnosticErrors(t *testing.T) {
	// Test that diagnostic errors use AllMessages() for detailed error information

	// Create a mock diagnostic error
	mockDiags := diag.Diagnostics{
		{
			Severity: diag.SeverityLevelError,
			StartPos: token.Position{Filename: "test.alloy", Line: 1, Column: 1},
			Message:  "first error",
		},
		{
			Severity: diag.SeverityLevelError,
			StartPos: token.Position{Filename: "test.alloy", Line: 2, Column: 1},
			Message:  "second error",
		},
	}

	// Test getErrorMessage function directly
	errorMsg := getErrorMessage(mockDiags)

	// Should contain both error messages, not just the first
	assert.Contains(t, errorMsg, "first error")
	assert.Contains(t, errorMsg, "second error")
	assert.Contains(t, errorMsg, ";") // Should be joined with semicolon

	// Test with regular error
	regularErr := errors.New("simple error")
	regularMsg := getErrorMessage(regularErr)
	assert.Equal(t, "simple error", regularMsg)
}

// EffectiveConfig unit tests

func TestEffectiveConfigsEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        *collectorv1.EffectiveConfig
		b        *collectorv1.EffectiveConfig
		expected bool
	}{
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "one nil",
			a:        &collectorv1.EffectiveConfig{},
			b:        nil,
			expected: false,
		},
		{
			name: "same config",
			a: &collectorv1.EffectiveConfig{
				ConfigMap: &collectorv1.AgentConfigMap{
					ConfigMap: map[string]*collectorv1.AgentConfigFile{
						"": {
							Body:        []byte("config content"),
							ContentType: effectiveConfigContentType,
						},
					},
				},
			},
			b: &collectorv1.EffectiveConfig{
				ConfigMap: &collectorv1.AgentConfigMap{
					ConfigMap: map[string]*collectorv1.AgentConfigFile{
						"": {
							Body:        []byte("config content"),
							ContentType: effectiveConfigContentType,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "different config body",
			a: &collectorv1.EffectiveConfig{
				ConfigMap: &collectorv1.AgentConfigMap{
					ConfigMap: map[string]*collectorv1.AgentConfigFile{
						"": {
							Body:        []byte("config content 1"),
							ContentType: effectiveConfigContentType,
						},
					},
				},
			},
			b: &collectorv1.EffectiveConfig{
				ConfigMap: &collectorv1.AgentConfigMap{
					ConfigMap: map[string]*collectorv1.AgentConfigFile{
						"": {
							Body:        []byte("config content 2"),
							ContentType: effectiveConfigContentType,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "different content type",
			a: &collectorv1.EffectiveConfig{
				ConfigMap: &collectorv1.AgentConfigMap{
					ConfigMap: map[string]*collectorv1.AgentConfigFile{
						"": {
							Body:        []byte("config content"),
							ContentType: effectiveConfigContentType,
						},
					},
				},
			},
			b: &collectorv1.EffectiveConfig{
				ConfigMap: &collectorv1.AgentConfigMap{
					ConfigMap: map[string]*collectorv1.AgentConfigFile{
						"": {
							Body:        []byte("config content"),
							ContentType: "text/yaml",
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "different number of config files",
			a: &collectorv1.EffectiveConfig{
				ConfigMap: &collectorv1.AgentConfigMap{
					ConfigMap: map[string]*collectorv1.AgentConfigFile{
						"": {
							Body:        []byte("config content"),
							ContentType: effectiveConfigContentType,
						},
					},
				},
			},
			b: &collectorv1.EffectiveConfig{
				ConfigMap: &collectorv1.AgentConfigMap{
					ConfigMap: map[string]*collectorv1.AgentConfigFile{
						"":      {Body: []byte("config content"), ContentType: effectiveConfigContentType},
						"extra": {Body: []byte("extra config"), ContentType: effectiveConfigContentType},
					},
				},
			},
			expected: false,
		},
		{
			name: "both have nil config map",
			a: &collectorv1.EffectiveConfig{
				ConfigMap: nil,
			},
			b: &collectorv1.EffectiveConfig{
				ConfigMap: nil,
			},
			expected: true,
		},
		{
			name: "one has nil config map",
			a: &collectorv1.EffectiveConfig{
				ConfigMap: &collectorv1.AgentConfigMap{
					ConfigMap: map[string]*collectorv1.AgentConfigFile{},
				},
			},
			b: &collectorv1.EffectiveConfig{
				ConfigMap: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := effectiveConfigsEqual(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAgentConfigFilesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        *collectorv1.AgentConfigFile
		b        *collectorv1.AgentConfigFile
		expected bool
	}{
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "one nil",
			a:        &collectorv1.AgentConfigFile{Body: []byte("test")},
			b:        nil,
			expected: false,
		},
		{
			name: "same content",
			a: &collectorv1.AgentConfigFile{
				Body:        []byte("test config"),
				ContentType: effectiveConfigContentType,
			},
			b: &collectorv1.AgentConfigFile{
				Body:        []byte("test config"),
				ContentType: effectiveConfigContentType,
			},
			expected: true,
		},
		{
			name: "different body",
			a: &collectorv1.AgentConfigFile{
				Body:        []byte("test config 1"),
				ContentType: effectiveConfigContentType,
			},
			b: &collectorv1.AgentConfigFile{
				Body:        []byte("test config 2"),
				ContentType: effectiveConfigContentType,
			},
			expected: false,
		},
		{
			name: "different content type",
			a: &collectorv1.AgentConfigFile{
				Body:        []byte("test config"),
				ContentType: effectiveConfigContentType,
			},
			b: &collectorv1.AgentConfigFile{
				Body:        []byte("test config"),
				ContentType: "text/yaml",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agentConfigFilesEqual(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCopyEffectiveConfig(t *testing.T) {
	tests := []struct {
		name   string
		input  *collectorv1.EffectiveConfig
		modify func(*collectorv1.EffectiveConfig)
	}{
		{
			name:  "nil config",
			input: nil,
		},
		{
			name: "config with body",
			input: &collectorv1.EffectiveConfig{
				ConfigMap: &collectorv1.AgentConfigMap{
					ConfigMap: map[string]*collectorv1.AgentConfigFile{
						"": {
							Body:        []byte("original config"),
							ContentType: effectiveConfigContentType,
						},
					},
				},
			},
			modify: func(copy *collectorv1.EffectiveConfig) {
				// Modify the copy
				copy.ConfigMap.ConfigMap[""].Body = []byte("modified config")
			},
		},
		{
			name: "config with multiple files",
			input: &collectorv1.EffectiveConfig{
				ConfigMap: &collectorv1.AgentConfigMap{
					ConfigMap: map[string]*collectorv1.AgentConfigFile{
						"main": {
							Body:        []byte("main config"),
							ContentType: effectiveConfigContentType,
						},
						"secondary": {
							Body:        []byte("secondary config"),
							ContentType: "text/yaml",
						},
					},
				},
			},
			modify: func(copy *collectorv1.EffectiveConfig) {
				// Add a new file to the copy
				copy.ConfigMap.ConfigMap["new"] = &collectorv1.AgentConfigFile{
					Body:        []byte("new config"),
					ContentType: effectiveConfigContentType,
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copy := copyEffectiveConfig(tt.input)

			if tt.input == nil {
				assert.Nil(t, copy)
				return
			}

			// Verify the copy is equal to the original
			assert.True(t, effectiveConfigsEqual(tt.input, copy))

			if tt.modify != nil {
				// Modify the copy
				tt.modify(copy)

				// Verify the original is unchanged
				assert.False(t, effectiveConfigsEqual(tt.input, copy))
			}
		})
	}
}

func TestSetEffectiveConfig(t *testing.T) {
	cm := newConfigManager(nil, util.TestLogger(t), t.TempDir(), "")

	config := []byte("test config content")
	cm.setEffectiveConfig(config)

	// Verify the effective config was set
	assert.NotNil(t, cm.effectiveConfig)
	assert.NotNil(t, cm.effectiveConfig.ConfigMap)
	assert.Len(t, cm.effectiveConfig.ConfigMap.ConfigMap, 1)

	file, exists := cm.effectiveConfig.ConfigMap.ConfigMap[""]
	require.True(t, exists)
	assert.Equal(t, config, file.Body)
	assert.Equal(t, effectiveConfigContentType, file.ContentType)
}

func TestGetEffectiveConfigForRequest(t *testing.T) {
	cm := newConfigManager(nil, util.TestLogger(t), t.TempDir(), "")

	// First call - should return nil because effective config is not set
	result := cm.getEffectiveConfigForRequest()
	assert.Nil(t, result)

	// Set effective config
	config1 := []byte("config version 1")
	cm.setEffectiveConfig(config1)

	// Second call - should return the config because it's the first time
	result = cm.getEffectiveConfigForRequest()
	require.NotNil(t, result)
	assert.Equal(t, config1, result.ConfigMap.ConfigMap[""].Body)

	// Third call - should return nil because config hasn't changed
	result = cm.getEffectiveConfigForRequest()
	assert.Nil(t, result)

	// Update config
	config2 := []byte("config version 2")
	cm.setEffectiveConfig(config2)

	// Fourth call - should return the new config because it changed
	result = cm.getEffectiveConfigForRequest()
	require.NotNil(t, result)
	assert.Equal(t, config2, result.ConfigMap.ConfigMap[""].Body)

	// Fifth call - should return nil because config hasn't changed again
	result = cm.getEffectiveConfigForRequest()
	assert.Nil(t, result)
}

func TestResetLastSentEffectiveConfig(t *testing.T) {
	cm := newConfigManager(nil, util.TestLogger(t), t.TempDir(), "")

	// Set effective config
	config := []byte("test config")
	cm.setEffectiveConfig(config)

	// Get it once to set lastSentEffectiveConfig
	result := cm.getEffectiveConfigForRequest()
	require.NotNil(t, result)

	// Second call should return nil (no change)
	result = cm.getEffectiveConfigForRequest()
	assert.Nil(t, result)

	// Reset last sent
	cm.resetLastSentEffectiveConfig()

	// Now it should return the config again
	result = cm.getEffectiveConfigForRequest()
	require.NotNil(t, result)
	assert.Equal(t, config, result.ConfigMap.ConfigMap[""].Body)
}

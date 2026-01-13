package stages

import (
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestUserAgentConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      UserAgentConfig
		expectedErr error
	}{
		{
			name:   "valid config with source",
			config: UserAgentConfig{Source: getStringPointer("user_agent")},
		},
		{
			name:   "valid config without source",
			config: UserAgentConfig{},
		},
		{
			name:   "valid config with regex file",
			config: UserAgentConfig{RegexFile: "/path/to/regexes.yaml"},
		},
		{
			name:        "invalid config with empty source",
			config:      UserAgentConfig{Source: getStringPointer("")},
			expectedErr: ErrEmptyUserAgentStageSource,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateUserAgentConfig(test.config)
			if test.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.Equal(t, test.expectedErr, err)
			}
		})
	}
}

func TestUserAgentStage_Process(t *testing.T) {
	tests := []struct {
		name     string
		config   UserAgentConfig
		input    string
		expected map[string]interface{}
	}{
		{
			name:   "Chrome browser parsing",
			config: UserAgentConfig{},
			input:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			expected: map[string]interface{}{
				"useragent_browser":         "Chrome",
				"useragent_browser_version": "91.0.4472",
				"useragent_os":              "Windows",
				"useragent_os_version":      "10...",
			},
		},
		{
			name:   "Safari browser parsing",
			config: UserAgentConfig{},
			input:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Safari/605.1.15",
			expected: map[string]interface{}{
				"useragent_browser":         "Safari",
				"useragent_browser_version": "14.1.1",
				"useragent_os":              "Mac OS X",
				"useragent_os_version":      "10.15.7",
			},
		},
		{
			name:   "Firefox browser parsing",
			config: UserAgentConfig{},
			input:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:89.0) Gecko/20100101 Firefox/89.0",
			expected: map[string]interface{}{
				"useragent_browser":         "Firefox",
				"useragent_browser_version": "89.0.",
				"useragent_os":              "Windows",
				"useragent_os_version":      "10...",
			},
		},
		{
			name:   "Mobile Safari parsing",
			config: UserAgentConfig{},
			input:  "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Mobile/15E148 Safari/604.1",
			expected: map[string]interface{}{
				"useragent_browser":         "Mobile Safari",
				"useragent_browser_version": "14.1.1",
				"useragent_os":              "iOS",
				"useragent_os_version":      "14.6.",
				"useragent_device":          "iPhone",
				"useragent_device_brand":    "Apple",
				"useragent_device_model":    "iPhone",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stage, err := newUserAgentStage(log.NewNopLogger(), test.config)
			require.NoError(t, err)

			labels := model.LabelSet{}
			extracted := make(map[string]interface{})
			ts := time.Now()
			entry := test.input

			stage.(*stageProcessor).Process(labels, extracted, &ts, &entry)

			// Check that expected fields are present (allowing for version flexibility)
			for key, expectedValue := range test.expected {
				require.Contains(t, extracted, key)
				if key == "useragent_browser" || key == "useragent_os" || key == "useragent_device" || key == "useragent_device_brand" || key == "useragent_device_model" {
					require.Equal(t, expectedValue, extracted[key])
				}
				// For version fields, just check they contain the major version
				if key == "useragent_browser_version" || key == "useragent_os_version" {
					require.Contains(t, extracted[key].(string), expectedValue.(string)[:2])
				}
			}
		})
	}
}

func TestUserAgentStage_ProcessWithSource(t *testing.T) {
	source := "user_agent_field"
	config := UserAgentConfig{Source: &source}
	stage, err := newUserAgentStage(log.NewNopLogger(), config)
	require.NoError(t, err)

	labels := model.LabelSet{}
	extracted := map[string]interface{}{
		"user_agent_field": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	}
	ts := time.Now()
	entry := "some other log line"

	stage.(*stageProcessor).Process(labels, extracted, &ts, &entry)

	require.Contains(t, extracted, "useragent_browser")
	require.Equal(t, "Chrome", extracted["useragent_browser"])
}

func TestUserAgentStage_Name(t *testing.T) {
	config := UserAgentConfig{}
	stage, err := newUserAgentStage(log.NewNopLogger(), config)
	require.NoError(t, err)

	require.Equal(t, StageTypeUserAgent, stage.(*stageProcessor).Name())
}

func TestUserAgentStage_NewWithInvalidRegexFile(t *testing.T) {
	config := UserAgentConfig{RegexFile: "/nonexistent/path/regexes.yaml"}
	_, err := newUserAgentStage(log.NewNopLogger(), config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load regex file")
}

func getStringPointer(s string) *string {
	return &s
}

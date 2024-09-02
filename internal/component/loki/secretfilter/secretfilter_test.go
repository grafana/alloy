package secretfilter

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

// fakeSecret represents a fake secret to be used in the tests
type fakeSecret struct {
	name   string
	value  string
	prefix string // Prefix to add to the redacted secret (if applicable)
}

// testLog represents a log entry to be used in the tests
type testLog struct {
	log     string
	secrets []fakeSecret // List of fake secrets it contains for easy redaction check
}

// Custom gitleaks configs for testing
var customGitleaksConfig = map[string]string{
	"simple": `
		title = "gitleaks custom config"

		[[rules]]
		id = "my-fake-secret"
		description = "Identified a fake secret"
		regex = '''(?i)\b(fakeSecret\d{5})(?:['|\"|\n|\r|\s|\x60|;]|$)'''
	`,
}

var defaultRedactionString = "REDACTED-SECRET"
var customRedactionString = "ALLOY-REDACTED-SECRET"

// Alloy configurations for testing
var testConfigs = map[string]string{
	"default": `
		forward_to = []
	`,
	"custom_redact_string": `
		forward_to = []
		redact_with = "<` + customRedactionString + `:$SECRET_NAME>"
	`,
	"custom_redact_string_with_hash": `
		forward_to = []
		redact_with = "<` + customRedactionString + `:$SECRET_NAME:$SECRET_HASH>"
	`,
	"partial_mask": `
		forward_to = []
		partial_mask = 4
	`,
	"custom_types": `
		forward_to = []
		redact_with = "<` + customRedactionString + `:$SECRET_NAME>"
		types = ["aws", "gcp"]
	`,
	"allow_list": `
		forward_to = []
		allowlist = [".*foobar.*"]
	`,
	"exclude_generic": `
		forward_to = []
		exclude_generic = true
	`,
	"custom_gitleaks_file_simple": `
		forward_to = []
		gitleaks_config = "not-empty" // This will be replaced with the actual path to the temporary gitleaks config file
	`,
}

// List of fake secrets to use for testing
// They are constructed so that they will match the regexes in the gitleaks configs
// Note that some string literals are concatenated to avoid being flagged as secrets
var fakeSecrets = map[string]fakeSecret{
	"grafana-api-key": {
		name:   "grafana-api-key",
		prefix: "eyJr",
		value:  "eyJr" + "Ijoi" + strings.Repeat("A", 70),
	},
	"grafana-api-key-allow": {
		name:  "grafana-api-key",
		value: "eyJr" + "Ijoi" + strings.Repeat("A", 30) + "foobar" + strings.Repeat("A", 34),
	},
	"gcp-api-key": {
		name:  "gcp-api-key",
		value: "AI" + "za" + strings.Repeat("A", 35),
	},
	"generic": {
		name:   "generic-api-key",
		prefix: "tok" + "en" + ":",
		value:  "tok" + "en" + ":" + strings.Repeat("A", 15),
	},
	"custom-fake-secret": {
		name:  "my-fake-secret",
		value: "fakeSecret12345",
	},
}

// List of fake log entries to use for testing
var testLogs = map[string]testLog{
	"no_secret": {
		log: `{
			"message": "This is a simple log message"
		}`,
		secrets: []fakeSecret{},
	},
	"simple_secret": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["grafana-api-key"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["grafana-api-key"]},
	},
	"simple_secret_allow": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["grafana-api-key-allow"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["grafana-api-key-allow"]},
	},
	"simple_secret_gcp": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["gcp-api-key"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["gcp-api-key"]},
	},
	"simple_secret_generic": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["generic"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["generic"]},
	},
	"simple_secret_custom": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["custom-fake-secret"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["custom-fake-secret"]},
	},
	"multiple_secrets": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["grafana-api-key"].value + ` and another secret value ` + fakeSecrets["gcp-api-key"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["grafana-api-key"], fakeSecrets["gcp-api-key"]},
	},
}

// Test cases for the secret filter
var tt = []struct {
	name                  string
	config                string
	gitLeaksConfigContent string
	inputLog              string
	expectedLog           string
}{
	{
		"no_secret",
		testConfigs["default"],
		"",
		testLogs["no_secret"].log,
		testLogs["no_secret"].log,
	},
	{
		"simple",
		testConfigs["default"],
		"",
		testLogs["simple_secret"].log,
		replaceSecrets(testLogs["simple_secret"].log, testLogs["simple_secret"].secrets, false, false, defaultRedactionString),
	},
	{
		"simple_allow",
		testConfigs["allow_list"],
		"",
		testLogs["simple_secret_allow"].log,
		testLogs["simple_secret_allow"].log,
	},
	{
		"custom_redact_string",
		testConfigs["custom_redact_string"],
		"",
		testLogs["simple_secret"].log,
		replaceSecrets(testLogs["simple_secret"].log, testLogs["simple_secret"].secrets, false, false, customRedactionString),
	},
	{
		"custom_redact_string_with_hash",
		testConfigs["custom_redact_string_with_hash"],
		"",
		testLogs["simple_secret"].log,
		replaceSecrets(testLogs["simple_secret"].log, testLogs["simple_secret"].secrets, false, true, customRedactionString),
	},
	{
		"partial_mask",
		testConfigs["partial_mask"],
		"",
		testLogs["simple_secret"].log,
		replaceSecrets(testLogs["simple_secret"].log, testLogs["simple_secret"].secrets, true, false, defaultRedactionString),
	},
	{
		"gcp_secret",
		testConfigs["default"],
		"",
		testLogs["simple_secret_gcp"].log,
		replaceSecrets(testLogs["simple_secret_gcp"].log, testLogs["simple_secret_gcp"].secrets, false, false, defaultRedactionString),
	},
	{
		"custom_types_with_grafana_api_key",
		testConfigs["custom_types"],
		"",
		testLogs["simple_secret"].log,
		testLogs["simple_secret"].log, // Grafana API key is not in the list of types, no redaction expected
	},
	{
		"custom_types_with_gcp_api_key",
		testConfigs["custom_types"],
		"",
		testLogs["simple_secret_gcp"].log,
		replaceSecrets(testLogs["simple_secret_gcp"].log, testLogs["simple_secret_gcp"].secrets, true, false, customRedactionString),
	},
	{
		"generic_secret",
		testConfigs["default"],
		"",
		testLogs["simple_secret_generic"].log,
		replaceSecrets(testLogs["simple_secret_generic"].log, testLogs["simple_secret_generic"].secrets, true, false, defaultRedactionString),
	},
	{
		"exclude_generic",
		testConfigs["exclude_generic"],
		"",
		testLogs["simple_secret_generic"].log,
		testLogs["simple_secret_generic"].log, // Generic secret is excluded so no redaction expected
	},
	{
		"custom_gitleaks_file_simple",
		testConfigs["custom_gitleaks_file_simple"],
		customGitleaksConfig["simple"],
		testLogs["simple_secret_custom"].log,
		replaceSecrets(testLogs["simple_secret_custom"].log, testLogs["simple_secret_custom"].secrets, false, false, defaultRedactionString),
	},
	{
		"multiple_secrets",
		testConfigs["default"],
		"",
		testLogs["multiple_secrets"].log,
		replaceSecrets(testLogs["multiple_secrets"].log, testLogs["multiple_secrets"].secrets, false, false, defaultRedactionString),
	},
}

func TestSecretFiltering(t *testing.T) {
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			runTest(t, tc.config, tc.gitLeaksConfigContent, tc.inputLog, tc.expectedLog)
		})
	}
}

func runTest(t *testing.T, config string, gitLeaksConfigContent string, inputLog string, expectedLog string) {
	ch1 := loki.NewLogsReceiver()
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(config), &args))
	args.ForwardTo = []loki.LogsReceiver{ch1}

	// If needed, create a temporary gitleaks config file
	if args.GitleaksConfig != "" {
		args.GitleaksConfig = createTempGitleaksConfig(t, gitLeaksConfigContent)
	}

	// Create component
	tc, err := componenttest.NewControllerFromID(util.TestLogger(t), "loki.secretfilter")
	require.NoError(t, err)

	// Run it
	go func() {
		err1 := tc.Run(componenttest.TestContext(t), args)
		require.NoError(t, err1)
	}()
	require.NoError(t, tc.WaitExports(time.Second))

	// Get the input channel
	input := tc.Exports().(Exports).Receiver

	// Send the log to the secret filter
	entry := loki.Entry{Labels: model.LabelSet{}, Entry: logproto.Entry{Timestamp: time.Now(), Line: inputLog}}
	input.Chan() <- entry
	tc.WaitRunning(time.Second * 10)

	// Check the output
	select {
	case logEntry := <-ch1.Chan():
		require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
		require.Equal(t, expectedLog, logEntry.Entry.Line)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "failed waiting for log line")
	}

	// If created before, remove the temporary gitleaks config file
	if args.GitleaksConfig != "" {
		deleteTempGitLeaksConfig(t, args.GitleaksConfig)
	}
}

func replaceSecrets(log string, secrets []fakeSecret, withPrefix bool, withHash bool, redactionString string) string {
	var prefix = ""
	if withPrefix {
		prefix = secrets[0].prefix
	}
	var hash = ""
	if withHash {
		hash = ":" + hashSecret(secrets[0].value)
	}
	for _, secret := range secrets {
		log = strings.Replace(log, secret.value, fmt.Sprintf("%s<%s:%s%s>", prefix, redactionString, secret.name, hash), -1)
	}
	return log
}

func createTempGitleaksConfig(t *testing.T, content string) string {
	f, err := os.CreateTemp("", "sample")
	require.NoError(t, err)

	_, err = f.WriteString(content)
	require.NoError(t, err)

	return f.Name()
}

func deleteTempGitLeaksConfig(t *testing.T, path string) {
	require.NoError(t, os.Remove(path))
}

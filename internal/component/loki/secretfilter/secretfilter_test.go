package secretfilter

import (
	"context"
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/loki/pkg/push"
	"github.com/jaswdr/faker/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
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
	"short_secret": `
		title = "gitleaks custom config"

		[[rules]]
		id = "short-secret"
		description = "Identified a fake short secret"
		regex = '''(?i)\b(abc)(?:['|\"|\n|\r|\s|\x60|;]|$)'''
	`,
	"empty_secret": `
		title = "gitleaks custom config"

		[[rules]]
		id = "empty-secret"
		description = "Identified a possibly empty secret"
		regex = '''(?i)(\w*)'''
	`,
	"sha1_secret": `
		title = "gitleaks custom config"

		[[rules]]
		id = "sha1-secret"
		description = "Identified a SHA1 secret"
		regex = '''(?i)\b(?:[0-9a-f]{40})\b'''
	`,
	"allow_list_old": `
		title = "gitleaks custom config"

		[[rules]]
		id = "my-fake-secret"
		description = "Identified a fake secret"
		regex = '''(?i)\b(fakeSecret\d{5})(?:['|\"|\n|\r|\s|\x60|;]|$)'''
		[rules.allowlist]
		regexes = ["abc\\d{3}", "fakeSecret[9]{5}"]
	`,
	"allow_list_new": `
		title = "gitleaks custom config"

		[[rules]]
		id = "my-fake-secret"
		description = "Identified a fake secret"
		regex = '''(?i)\b(fakeSecret\d{5})(?:['|\"|\n|\r|\s|\x60|;]|$)'''
			[[rules.allowlists]]
			regexes = ["def\\d{3}", "test\\d{5}"]
			[[rules.allowlists]]
			regexes = ["abc\\d{3}", "fakeSecret[9]{5}"]
	`,
	`allow_list_global`: `
		title = "gitleaks custom config"

		[[rules]]
		id = "my-fake-secret"
		description = "Identified a fake secret"
		regex = '''(?i)\b(fakeSecret\d{5})(?:['|\"|\n|\r|\s|\x60|;]|$)'''
		[allowlist]
		regexes = ["abc\\d{3}", "fakeSecret[9]{5}"]
	`,
	`with_low_entropy`: `
		title = "gitleaks custom config"

		[[rules]]
		id = "sha1-secret"
		description = "Identified a fake secret"
		regex = '''(?i)\b(?:[0-9a-f]{40})\b'''
		entropy = 2.0
	`,
	`with_high_entropy`: `
		title = "gitleaks custom config"

		[[rules]]
		id = "sha1-secret"
		description = "Identified a fake secret"
		regex = '''(?i)\b(?:[0-9a-f]{40})\b'''
		entropy = 4.5
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
	"partial_mask_custom": `
		forward_to = []
		partial_mask = 4
		gitleaks_config = "not-empty" // This will be replaced with the actual path to the temporary gitleaks config file
	`,
	"custom_types": `
		forward_to = []
		redact_with = "<` + customRedactionString + `:$SECRET_NAME>"
		types = ["aws", "gcp"]
	`,
	"custom_type": `
		forward_to = []
		redact_with = "<` + customRedactionString + `:$SECRET_NAME>"
		types = ["gcp"]
	`,
	"allow_list": `
		forward_to = []
		allowlist = [".*foobar.*"]
	`,
	"include_generic": `
		forward_to = []
		include_generic = true
	`,
	"custom_gitleaks_file_simple": `
		forward_to = []
		gitleaks_config = "not-empty" // This will be replaced with the actual path to the temporary gitleaks config file
	`,
	"custom_redact_string_with_hash_sha1": `
		forward_to = []
		redact_with = "<` + defaultRedactionString + `:$SECRET_NAME:$SECRET_HASH>"
		types = ["sha1-secret"]
		gitleaks_config = "not-empty" // This will be replaced with the actual path to the temporary gitleaks config file
	`,
	"with_entropy": `
		forward_to = []
		enable_entropy = true
		gitleaks_config = "not-empty" // This will be replaced with the actual path to the temporary gitleaks config file
	`,
	"with_entropy_and_generic_default_config": `
		forward_to = []
		enable_entropy = true
		include_generic = true
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
		value:  "tok" + "en" + ":" + "1" + strings.Repeat("A", 15), // All letters are not considered as token
	},
	"custom-fake-secret": {
		name:  "my-fake-secret",
		value: "fakeSec" + "ret12345",
	},
	"custom-fake-secret-all9": {
		name:  "my-fake-secret",
		value: "fakeSec" + "ret99999",
	},
	"short-secret": {
		name:  "short-secret",
		value: "abc",
	},
	"sha1-secret": {
		name:  "sha1-secret",
		value: "0123456789abcdef0123456789abcdef01234567",
	},
	"sha1-secret-low-entropy": {
		name:  "sha1-secret-low-entropy",
		value: "0000000000000000000111111111111111111111",
	},
	"generic-api-key-high-entropy": {
		name:   "generic-api-key",
		prefix: "tok" + "en" + "=",
		value:  "tok" + "en" + "=" + "abcdefghijklmnopqrstuvwxyz12345",
	},
	"generic-api-key-low-entropy": {
		name:   "generic-api-key",
		prefix: "tok" + "en" + "=",
		value:  "tok" + "en" + "=" + "aaa123aaa123aaa123aaa123aaa123",
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
	"simple_secret_custom_all9": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["custom-fake-secret-all9"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["custom-fake-secret-all9"]},
	},
	"multiple_secrets": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["grafana-api-key"].value + ` and another secret value ` + fakeSecrets["gcp-api-key"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["grafana-api-key"], fakeSecrets["gcp-api-key"]},
	},
	"short_secret": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["short-secret"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["short-secret"]},
	},
	"sha1_secret": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["sha1-secret"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["sha1-secret"]},
	},
	"sha1_low_entropy_secret": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["sha1-secret-low-entropy"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["sha1-secret-low-entropy"]},
	},
	"generic_api_key_high_entropy": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["generic-api-key-high-entropy"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["generic-api-key-high-entropy"]},
	},
	"generic_api_key_low_entropy": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["generic-api-key-low-entropy"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["generic-api-key"]},
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
		"partial_mask_too_short",
		testConfigs["partial_mask_custom"],
		customGitleaksConfig["short_secret"],
		testLogs["short_secret"].log,
		replaceSecrets(testLogs["short_secret"].log, testLogs["short_secret"].secrets, false, false, defaultRedactionString),
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
		testLogs["simple_secret_generic"].log, // Generic secret is excluded so no redaction expected
	},
	{
		"include_generic",
		testConfigs["include_generic"],
		"",
		testLogs["simple_secret_generic"].log,
		replaceSecrets(testLogs["simple_secret_generic"].log, testLogs["simple_secret_generic"].secrets, true, false, defaultRedactionString),
	},
	{
		"custom_gitleaks_file_simple",
		testConfigs["custom_gitleaks_file_simple"],
		customGitleaksConfig["simple"],
		testLogs["simple_secret_custom"].log,
		replaceSecrets(testLogs["simple_secret_custom"].log, testLogs["simple_secret_custom"].secrets, false, false, defaultRedactionString),
	},
	{
		"custom_gitleaks_file_allow_list_old",
		testConfigs["custom_gitleaks_file_simple"],
		customGitleaksConfig["allow_list_old"],
		testLogs["simple_secret_custom_all9"].log,
		testLogs["simple_secret_custom_all9"].log, // In the allowlist
	},
	{
		"custom_gitleaks_file_allow_list_new",
		testConfigs["custom_gitleaks_file_simple"],
		customGitleaksConfig["allow_list_new"],
		testLogs["simple_secret_custom_all9"].log,
		testLogs["simple_secret_custom_all9"].log, // In the allowlist
	},
	{
		"custom_gitleaks_file_allow_list_old_redact",
		testConfigs["custom_gitleaks_file_simple"],
		customGitleaksConfig["allow_list_old"],
		testLogs["simple_secret_custom"].log,
		replaceSecrets(testLogs["simple_secret_custom"].log, testLogs["simple_secret_custom"].secrets, false, false, defaultRedactionString),
	},
	{
		"custom_gitleaks_file_allow_list_new_redact",
		testConfigs["custom_gitleaks_file_simple"],
		customGitleaksConfig["allow_list_new"],
		testLogs["simple_secret_custom"].log,
		replaceSecrets(testLogs["simple_secret_custom"].log, testLogs["simple_secret_custom"].secrets, false, false, defaultRedactionString),
	},
	{
		"custom_gitleaks_file_allow_list_global",
		testConfigs["custom_gitleaks_file_simple"],
		customGitleaksConfig["allow_list_global"],
		testLogs["simple_secret_custom_all9"].log,
		testLogs["simple_secret_custom_all9"].log, // In the allowlist
	},
	{
		"custom_gitleaks_file_allow_list_global_redact",
		testConfigs["custom_gitleaks_file_simple"],
		customGitleaksConfig["allow_list_global"],
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
	{
		"empty_secret",
		testConfigs["custom_gitleaks_file_simple"],
		customGitleaksConfig["empty_secret"],
		testLogs["short_secret"].log,
		testLogs["short_secret"].log,
	},
	{
		"sha1_secret",
		testConfigs["custom_redact_string_with_hash_sha1"],
		customGitleaksConfig["sha1_secret"],
		testLogs["sha1_secret"].log,
		replaceSecrets(testLogs["sha1_secret"].log, testLogs["sha1_secret"].secrets, false, true, defaultRedactionString),
	},
	{
		"sha1_secret_entropy",
		testConfigs["with_entropy"],
		customGitleaksConfig["with_low_entropy"],
		testLogs["sha1_secret"].log,
		replaceSecrets(testLogs["sha1_secret"].log, testLogs["sha1_secret"].secrets, false, false, defaultRedactionString),
	},
	{
		"sha1_secret_secret_low_entropy",
		testConfigs["with_entropy"],
		customGitleaksConfig["with_low_entropy"],
		testLogs["sha1_low_entropy_secret"].log,
		testLogs["sha1_low_entropy_secret"].log, // Entropy of the secret too low, no redaction expected
	},
	{
		"sha1_secret_secret_low_entropy_no_entropy_enabled",
		testConfigs["custom_gitleaks_file_simple"],
		customGitleaksConfig["with_low_entropy"],
		testLogs["sha1_low_entropy_secret"].log,
		replaceSecrets(testLogs["sha1_secret"].log, testLogs["sha1_secret"].secrets, false, false, defaultRedactionString),
	},
	{
		"sha1_secret_config_high_entropy",
		testConfigs["with_entropy"],
		customGitleaksConfig["with_high_entropy"],
		testLogs["sha1_secret"].log,
		testLogs["sha1_secret"].log, // Entropy threshold in the rule too high for the secret, no redaction expected
	},
	{
		"sha1_secret_entropy_not_enabled",
		testConfigs["custom_gitleaks_file_simple"],
		customGitleaksConfig["with_high_entropy"],
		testLogs["sha1_secret"].log,
		replaceSecrets(testLogs["sha1_secret"].log, testLogs["sha1_secret"].secrets, false, false, defaultRedactionString),
	},
	{
		"generic_api_key_high_entropy",
		testConfigs["with_entropy_and_generic_default_config"],
		"",
		testLogs["generic_api_key_high_entropy"].log,
		replaceSecrets(testLogs["generic_api_key_high_entropy"].log, testLogs["generic_api_key_high_entropy"].secrets, true, false, defaultRedactionString),
	},
	{
		"generic_api_key_low_entropy",
		testConfigs["with_entropy_and_generic_default_config"],
		"",
		testLogs["generic_api_key_low_entropy"].log,
		testLogs["generic_api_key_low_entropy"].log, // Entropy of the secret too low, no redaction expected
	},
	{
		"generic_api_key_high_entropy_secret_no_entropy_enabled",
		testConfigs["include_generic"],
		"",
		testLogs["generic_api_key_high_entropy"].log,
		replaceSecrets(testLogs["generic_api_key_high_entropy"].log, testLogs["generic_api_key_high_entropy"].secrets, true, false, defaultRedactionString),
	},
}

func TestSecretFiltering(t *testing.T) {
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			runTest(t, tc.config, tc.gitLeaksConfigContent, tc.inputLog, tc.expectedLog)
		})
	}
}

func TestPartialMasking(t *testing.T) {
	// Start testing with common cases
	component := &Component{}
	component.args = Arguments{PartialMask: 4}

	// Too short to be partially masked
	redacted := component.redactLine("This is a very short secret ab in a log line", "ab", "test-rule")
	require.Equal(t, "This is a very short secret <REDACTED-SECRET:test-rule> in a log line", redacted)

	// Too short to be partially masked
	redacted = component.redactLine("This is a short secret abc12 in a log line", "abc12", "test-rule")
	require.Equal(t, "This is a short secret <REDACTED-SECRET:test-rule> in a log line", redacted)

	// Will be partially masked (limited by secret length)
	redacted = component.redactLine("This is a longer secret abc123 in a log line", "abc123", "test-rule")
	require.Equal(t, "This is a longer secret abc<REDACTED-SECRET:test-rule> in a log line", redacted)

	// Will be partially masked
	redacted = component.redactLine("This is a long enough secret abcd1234 in a log line", "abcd1234", "test-rule")
	require.Equal(t, "This is a long enough secret abcd<REDACTED-SECRET:test-rule> in a log line", redacted)

	// Will be partially masked
	redacted = component.redactLine("This is the longest secret abcdef12345678 in a log line", "abcdef12345678", "test-rule")
	require.Equal(t, "This is the longest secret abcd<REDACTED-SECRET:test-rule> in a log line", redacted)

	// Test with a non-ASCII character
	redacted = component.redactLine("This is a line with a complex secret aBc\U0001f512De\U0001f5124 in a log line", "aBc\U0001f512De\U0001f5124", "test-rule")
	require.Equal(t, "This is a line with a complex secret aBc\U0001f512<REDACTED-SECRET:test-rule> in a log line", redacted)

	// Test with different secret lengths and partial masking values
	for partialMasking := range 20 {
		for secretLength := range 50 {
			if secretLength < 2 {
				continue
			}
			expectedPrefixLength := 0
			if secretLength >= 6 {
				expectedPrefixLength = min(secretLength/2, partialMasking)
			}
			checkPartialMasking(t, partialMasking, secretLength, expectedPrefixLength)
		}
	}
}

func checkPartialMasking(t *testing.T, partialMasking int, secretLength int, expectedPrefixLength int) {
	component := &Component{}
	component.args = Arguments{PartialMask: uint(partialMasking)}

	// Test with a simple ASCII character
	secret := strings.Repeat("A", secretLength)
	inputLog := fmt.Sprintf("This is a test with a secret %s in a log line", secret)
	redacted := component.redactLine(inputLog, secret, "test-rule")
	prefix := strings.Repeat("A", expectedPrefixLength)
	expectedLog := fmt.Sprintf("This is a test with a secret %s<REDACTED-SECRET:test-rule> in a log line", prefix)
	require.Equal(t, expectedLog, redacted)

	// Test with a non-ASCII character
	secret = strings.Repeat("\U0001f512", secretLength)
	inputLog = fmt.Sprintf("This is a test with a secret %s in a log line", secret)
	redacted = component.redactLine(inputLog, secret, "test-rule")
	prefix = strings.Repeat("\U0001f512", expectedPrefixLength)
	expectedLog = fmt.Sprintf("This is a test with a secret %s<REDACTED-SECRET:test-rule> in a log line", prefix)
	require.Equal(t, expectedLog, redacted)
}

func runTest(t *testing.T, config string, gitLeaksConfigContent string, inputLog string, expectedLog string) {
	ch1 := loki.NewLogsReceiver()
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(config), &args))
	args.ForwardTo = []loki.LogsReceiver{ch1}

	// Making sure we're not testing with an empty log line by mistake
	require.NotEmpty(t, inputLog)

	// If needed, create a temporary gitleaks config file
	if args.GitleaksConfig != "" {
		args.GitleaksConfig = createTempGitleaksConfig(t, gitLeaksConfigContent)
	}

	// Create component
	tc, err := componenttest.NewControllerFromID(util.TestLogger(t), "loki.secretfilter")
	require.NoError(t, err)

	// Run it
	ctx, cancel := context.WithCancel(t.Context())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		err1 := tc.Run(ctx, args)
		require.NoError(t, err1)
		wg.Done()
	}()
	require.NoError(t, tc.WaitExports(time.Second))

	// Get the input channel
	input := tc.Exports().(Exports).Receiver

	// Send the log to the secret filter
	entry := loki.Entry{Labels: model.LabelSet{}, Entry: push.Entry{Timestamp: time.Now(), Line: inputLog}}
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

	// Stop the component
	cancel()
	wg.Wait()

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
		log = strings.ReplaceAll(log, secret.value, fmt.Sprintf("%s<%s:%s%s>", prefix, redactionString, secret.name, hash))
	}
	return log
}

func createTempGitleaksConfig(t *testing.T, content string) string {
	f, err := os.CreateTemp(t.TempDir(), "sample")
	require.NoError(t, err)

	_, err = f.WriteString(content)
	require.NoError(t, err)

	require.NoError(t, f.Close())

	return f.Name()
}

func deleteTempGitLeaksConfig(t *testing.T, path string) {
	if err := os.Remove(path); err != nil {
		t.Logf("Error deleting temporary gitleaks config file: %v", err)
	}
}

func BenchmarkAllTypesNoSecret(b *testing.B) {
	// Run benchmarks with no secrets in the logs, with all regexes enabled
	runBenchmarks(b, testConfigs["default"], 0, "")
}

func BenchmarkAllTypesWithSecret(b *testing.B) {
	// Run benchmarks with secrets in the logs (20% of log entries), with all regexes enabled
	runBenchmarks(b, testConfigs["default"], 20, "gcp_secret")
}

func BenchmarkAllTypesWithLotsOfSecrets(b *testing.B) {
	// Run benchmarks with secrets in the logs (80% of log entries), with all regexes enabled
	runBenchmarks(b, testConfigs["default"], 80, "gcp_secret")
}

func BenchmarkOneRuleNoSecret(b *testing.B) {
	// Run benchmarks with no secrets in the logs, with a single regex enabled
	runBenchmarks(b, testConfigs["custom_type"], 0, "")
}

func BenchmarkOneRuleWithSecret(b *testing.B) {
	// Run benchmarks with secrets in the logs (20% of log entries), with a single regex enabled
	runBenchmarks(b, testConfigs["custom_type"], 20, "gcp_secret")
}

func BenchmarkOneRuleWithLotsOfSecrets(b *testing.B) {
	// Run benchmarks with secrets in the logs (80% of log entries), with a single regex enabled
	runBenchmarks(b, testConfigs["custom_type"], 80, "gcp_secret")
}

func runBenchmarks(b *testing.B, config string, percentageSecrets int, secretName string) {
	ch1 := loki.NewLogsReceiver()
	var args Arguments
	require.NoError(b, syntax.Unmarshal([]byte(config), &args))
	args.ForwardTo = []loki.LogsReceiver{ch1}

	opts := component.Options{
		Logger:         &noopLogger{}, // Disable logging so that it keeps a clean benchmark output
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
	}

	// Create component
	c, err := New(opts, args)
	require.NoError(b, err)

	// Generate fake log entries with a fixed seed so that it's reproducible
	fake := faker.NewWithSeed(rand.NewPCG(uint64(2014), uint64(2014)))
	nbLogs := 100
	benchInputs := make([]string, nbLogs)
	for i := range benchInputs {
		beginningStr := fake.Lorem().Paragraph(2)
		middleStr := fake.Lorem().Sentence(10)
		endingStr := fake.Lorem().Paragraph(2)

		// Add fake secrets in some log entries
		if i < nbLogs*percentageSecrets/100 {
			middleStr = testLogs[secretName].log
		}

		benchInputs[i] = beginningStr + middleStr + endingStr
	}

	// Run benchmarks
	for i := 0; i < b.N; i++ {
		for _, input := range benchInputs {
			entry := loki.Entry{Labels: model.LabelSet{}, Entry: push.Entry{Timestamp: time.Now(), Line: input}}
			c.processEntry(entry)
		}
	}
}

var sampleFuzzLogLines = []string{
	`key=value1,value2 log=fmt test=1 secret=password`,
	`{"key":["value1","value2"],"log":"fmt","test":1,"secret":"password"}`,
	`1970-01-01 00:00:00 pattern value1,value2 1 secret`,
}

// "0000LLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLL000000\xdf0"
func FuzzProcessEntry(f *testing.F) {
	for _, line := range sampleFuzzLogLines {
		f.Add(line)
	}
	for _, testLog := range testLogs {
		f.Add(testLog.log)
	}

	comps := make([]*Component, 0, len(testConfigs))
	opts := component.Options{
		Logger:         util.TestLogger(f),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
	}
	ch1 := loki.NewLogsReceiver()

	// Create components
	for _, config := range testConfigs {
		var args Arguments
		require.NoError(f, syntax.Unmarshal([]byte(config), &args))
		if args.GitleaksConfig != "" {
			continue // Skip the configs using a custom gitleaks config file
		}

		args.ForwardTo = []loki.LogsReceiver{ch1}
		c, err := New(opts, args)
		require.NoError(f, err)
		comps = append(comps, c)
	}

	f.Fuzz(func(t *testing.T, log string) {
		for _, c := range comps {
			entry := loki.Entry{Labels: model.LabelSet{}, Entry: push.Entry{Timestamp: time.Now(), Line: log}}
			c.processEntry(entry)
		}
	})
}

func FuzzConfig(f *testing.F) {
	for _, testLog := range testLogs {
		f.Add("", false, uint(0), "", "", testLog.log)                               // zero values
		f.Add("REDACTED", true, uint(4), "aws,gcp", "abc.*&.*foobar.*", testLog.log) // sane values
	}
	opts := component.Options{
		Logger:         util.TestLogger(f),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
	}
	ch1 := loki.NewLogsReceiver()

	f.Fuzz(func(t *testing.T, redact string, generic bool, partial uint, types string, allow string, log string) {
		args := Arguments{
			ForwardTo:      []loki.LogsReceiver{ch1}, // not fuzzed
			Types:          strings.Split(types, ","),
			RedactWith:     redact,
			IncludeGeneric: generic,
			AllowList:      strings.Split(allow, "&"), // a character that has no special meaning in go regexp and doesn't appear in the gitleaks regexes
			PartialMask:    partial,
			GitleaksConfig: "", // not fuzzed in this test
		}
		c, err := New(opts, args)
		if err != nil {
			// ignore regex parsing errors
			if strings.HasPrefix(err.Error(), "error parsing regexp") {
				return
			}
			t.Errorf("error configuring component: %v", err)
		}

		entry := loki.Entry{Labels: model.LabelSet{}, Entry: push.Entry{Timestamp: time.Now(), Line: log}}
		c.processEntry(entry)
	})
}

func FuzzGitleaksConfig(f *testing.F) {
	for _, testLog := range testLogs {
		f.Add("", "", "", "", "", 0, "", 0.0, "", "", "", testLog.log)                                                                                                                                // empty values
		f.Add("Secret detection", "pattern1&pattern2", "pattern_1", "Look for a specific pattern", "(i?)pa(tt)+ern*", 0, "pa?er", 2.0, "keyword1,keyword2", "path/to/file", "tag1,tag2", testLog.log) // sane values
	}
	opts := component.Options{
		Logger:         util.TestLogger(f),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
	}
	ch1 := loki.NewLogsReceiver()
	args := Arguments{
		ForwardTo:   []loki.LogsReceiver{ch1}, // not fuzzed
		RedactWith:  "TEST_REDACTION:$SECRET_NAME",
		PartialMask: 4,
	}
	f.Fuzz(func(t *testing.T, title string, global_allow_list string, id string, description string, match string, group int, rule_allow_list string, entropy float64, keywords string, path string, tags string, log string) {
		// Includes all gitleaks config fields, even if they are not supported by the component
		gitleaksConfig := fmt.Sprintf(`title = '''%s'''

		[[rules]]
		id = '''%s'''
		description = '''%s'''
		regex = '''%s'''
		secretGroup = %d
		entropy = %f
		keywords = [%s]
		path = '''%s'''
		tags = [%s]

		[[rules.allowlists]]
		regexes = [%s]

		[allowlist]
		regexes = [%s]`, title, id, description, match, group, entropy, makeList(keywords, ","), path, makeList(tags, ","), makeList(rule_allow_list, "&"), makeList(global_allow_list, "&"))
		args.GitleaksConfig = createTempGitleaksConfig(t, gitleaksConfig)
		defer deleteTempGitLeaksConfig(t, args.GitleaksConfig)

		args.ForwardTo = []loki.LogsReceiver{ch1}
		c, err := New(opts, args)
		if err != nil {
			// ignore regex parsing errors - out of scope
			if strings.HasPrefix(err.Error(), "error parsing regexp") {
				return
			}
			// ignore toml parsing errors - out of scope
			if strings.HasPrefix(err.Error(), "toml:") {
				return
			}
			t.Errorf("error configuring component: %v", err)
		}

		entry := loki.Entry{Labels: model.LabelSet{}, Entry: push.Entry{Timestamp: time.Now(), Line: log}}
		c.processEntry(entry)
	})
}

func makeList(input string, separator string) string {
	parts := strings.Split(input, separator)
	for i, part := range parts {
		parts[i] = fmt.Sprintf(`"%s"`, strings.ReplaceAll(part, `"`, `\"`))
	}
	return strings.Join(parts, ",")
}

func getServiceData(name string) (any, error) {
	switch name {
	case livedebugging.ServiceName:
		return livedebugging.NewLiveDebugging(), nil
	default:
		return nil, fmt.Errorf("service not found %s", name)
	}
}

type noopLogger struct{}

func (d *noopLogger) Log(_ ...any) error {
	return nil
}

// TestMetrics verifies that the metrics for the secretfilter component are
// correctly registered and incremented.
func TestMetrics(t *testing.T) {
	tests := []struct {
		name                        string
		inputLog                    string
		entropyEnabled              bool
		expectedRedactedTotal       int
		expectedRedactedByRule      map[string]int
		expectedAllowlistedBySource map[string]int
		expectedEntropyByRule       map[string]int
		allowlist                   []string
		customConfig                string
	}{
		{
			name:                  "No secrets",
			inputLog:              testLogs["no_secret"].log,
			expectedRedactedTotal: 0,
		},
		{
			name:                  "Single Grafana API key secret",
			inputLog:              testLogs["simple_secret"].log,
			expectedRedactedTotal: 1,
			expectedRedactedByRule: map[string]int{
				"grafana-api-key": 1,
			},
		},
		{
			name:                  "Multiple secrets",
			inputLog:              testLogs["multiple_secrets"].log,
			expectedRedactedTotal: 2,
			expectedRedactedByRule: map[string]int{
				"grafana-api-key": 1,
				"gcp-api-key":     1,
			},
		},
		{
			name:                  "Secret in allowlist",
			inputLog:              testLogs["simple_secret"].log,
			expectedRedactedTotal: 0,
			expectedAllowlistedBySource: map[string]int{
				"alloy config": 1,
			},
			allowlist: []string{fakeSecrets["grafana-api-key"].value},
		},
		{
			name:                   "Not redacted because of entropy",
			inputLog:               testLogs["sha1_low_entropy_secret"].log,
			expectedRedactedTotal:  0,
			expectedRedactedByRule: map[string]int{},
			expectedEntropyByRule: map[string]int{
				"sha1-secret": 1,
			},
			entropyEnabled: true,
			customConfig:   customGitleaksConfig["with_high_entropy"],
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new registry to collect metrics
			registry := prometheus.NewRegistry()

			// Initialize Arguments
			args := Arguments{
				ForwardTo:   []loki.LogsReceiver{loki.NewLogsReceiver()},
				OriginLabel: "job",
			}

			if tc.customConfig != "" {
				args.GitleaksConfig = createTempGitleaksConfig(t, tc.customConfig)
			}

			if tc.entropyEnabled {
				args.EnableEntropy = true
			}

			// Set allowlist if provided
			if len(tc.allowlist) > 0 {
				for i, val := range tc.allowlist {
					// Convert the raw secret value to a valid regex pattern
					// by escaping special characters
					tc.allowlist[i] = regexp.QuoteMeta(val)
				}
				args.AllowList = tc.allowlist
			}

			// Create options with the test registry
			opts := component.Options{
				Logger:         util.TestLogger(t),
				OnStateChange:  func(e component.Exports) {},
				GetServiceData: getServiceData,
				Registerer:     registry,
			}

			// Create component
			c, err := New(opts, args)
			require.NoError(t, err)

			// Create a test entry with labels
			labels := model.LabelSet{
				"job":      "test-job",
				"instance": "test-instance",
			}
			entry := loki.Entry{
				Labels: labels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      tc.inputLog,
				},
			}

			// Process the entry
			c.processEntry(entry)

			// Verify the metrics

			// Check secretsRedactedTotal
			if tc.expectedRedactedTotal > 0 {
				require.Equal(t, float64(tc.expectedRedactedTotal),
					testutil.ToFloat64(c.metrics.secretsRedactedTotal),
					"secretsRedactedTotal metric value is incorrect")
			}

			// Check secretsRedactedByRule - combine all metrics in a single string
			if len(tc.expectedRedactedByRule) > 0 {
				var metricStrings strings.Builder
				metricStrings.WriteString("# HELP loki_secretfilter_secrets_redacted_by_rule_total Number of secrets redacted, partitioned by rule name.\n")
				metricStrings.WriteString("# TYPE loki_secretfilter_secrets_redacted_by_rule_total counter\n")

				// Add each rule metric
				for ruleName, expectedCount := range tc.expectedRedactedByRule {
					metric := fmt.Sprintf(`loki_secretfilter_secrets_redacted_by_rule_total{rule="%s"} %d`,
						ruleName, expectedCount)
					metricStrings.WriteString(metric + "\n")
				}

				// Compare all the metrics at once
				require.NoError(t,
					testutil.GatherAndCompare(registry, strings.NewReader(metricStrings.String()),
						"loki_secretfilter_secrets_redacted_by_rule_total"))
			}

			// Check secretsAllowlistedTotal
			for source, expectedCount := range tc.expectedAllowlistedBySource {
				metric := fmt.Sprintf(`loki_secretfilter_secrets_allowlisted_total{source="%s"}`, source)
				require.NoError(t,
					testutil.GatherAndCompare(registry, strings.NewReader(fmt.Sprintf(`
						# HELP loki_secretfilter_secrets_allowlisted_total Number of secrets that matched a rule but were in an allowlist, partitioned by source.
						# TYPE loki_secretfilter_secrets_allowlisted_total counter
						%s %d
					`, metric, expectedCount)),
						"loki_secretfilter_secrets_allowlisted_total"))
			}

			// Check secretsRedactedByOrigin when redactions occurred
			if tc.expectedRedactedTotal > 0 {
				// Build expected origin label metric
				var metricStrings strings.Builder
				metricStrings.WriteString("# HELP loki_secretfilter_secrets_redacted_by_origin Number of secrets redacted, partitioned by origin label value.\n")
				metricStrings.WriteString("# TYPE loki_secretfilter_secrets_redacted_by_origin counter\n")

				// Add origin label metric
				if jobValue, exists := labels[model.LabelName("job")]; exists {
					metric := fmt.Sprintf(`loki_secretfilter_secrets_redacted_by_origin{origin="%s"} %d`,
						jobValue, tc.expectedRedactedTotal)
					metricStrings.WriteString(metric + "\n")
				}

				// Compare the metrics
				require.NoError(t,
					testutil.GatherAndCompare(registry, strings.NewReader(metricStrings.String()),
						"loki_secretfilter_secrets_redacted_by_origin"))
			}

			// Check entropy metrics
			if len(tc.expectedEntropyByRule) > 0 {
				var metricStrings strings.Builder
				metricStrings.WriteString("# HELP loki_secretfilter_secrets_skipped_entropy_by_rule_total Number of secrets that matched a rule but whose entropy was too low to be redacted, partitioned by rule name.\n")
				metricStrings.WriteString("# TYPE loki_secretfilter_secrets_skipped_entropy_by_rule_total counter\n")
				// Add each rule metric
				for ruleName, expectedCount := range tc.expectedEntropyByRule {
					metric := fmt.Sprintf(`loki_secretfilter_secrets_skipped_entropy_by_rule_total{rule="%s"} %d`,
						ruleName, expectedCount)
					metricStrings.WriteString(metric + "\n")
				}
				// Compare the metrics
				require.NoError(t,
					testutil.GatherAndCompare(registry, strings.NewReader(metricStrings.String()),
						"loki_secretfilter_secrets_skipped_entropy_by_rule_total"))
			}

			// Check processingDuration metric
			// We don't validate the exact value since it will vary, but we verify it exists and has the right structure
			count, err := testutil.GatherAndCount(registry, "loki_secretfilter_processing_duration_seconds")
			require.NoError(t, err)
			require.Equal(t, count, 1, "processingDuration metric should be registered")

			// We only check that the metric exists with the right type, not the actual values
			require.NoError(t, err, "processingDuration metric should be properly registered")

			// Additionally check that the metric has count > 0 (indicating it was observed at least once)
			metricFamilies, err := registry.Gather()
			require.NoError(t, err)

			var foundMetric bool
			for _, mf := range metricFamilies {
				if mf.GetName() == "loki_secretfilter_processing_duration_seconds" {
					foundMetric = true
					for _, m := range mf.GetMetric() {
						summary := m.GetSummary()
						require.NotNil(t, summary, "should have a summary metric")
						require.Greater(t, summary.GetSampleCount(), uint64(0), "summary should have samples")
					}
				}
			}
			require.True(t, foundMetric, "processingDuration metric should be gathered")
		})
	}
}

// Test to verify that the component registers its metrics with the registry
func TestMetricsRegistration(t *testing.T) {
	registry := prometheus.NewRegistry()

	opts := component.Options{
		Logger:         util.TestLogger(t),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
		Registerer:     registry,
		ID:             "test_secretfilter",
	}

	// Create component with empty arguments
	args := Arguments{
		ForwardTo:   []loki.LogsReceiver{loki.NewLogsReceiver()},
		OriginLabel: "job",
	}

	c, err := New(opts, args)
	require.NoError(t, err)

	// Increment all metrics to ensure they will be gathered
	c.metrics.secretsRedactedTotal.Inc()
	c.metrics.secretsRedactedByRule.WithLabelValues("test_rule").Inc()
	c.metrics.secretsRedactedByOrigin.WithLabelValues("test_value").Inc()
	c.metrics.secretsAllowlistedTotal.WithLabelValues("test_source").Inc()
	c.metrics.secretsSkippedByEntropy.WithLabelValues("test_rule").Inc()
	c.metrics.processingDuration.Observe(0.123)

	// Check that the metrics are registered
	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	// Create a map of expected metrics
	expectedMetrics := map[string]bool{
		"loki_secretfilter_secrets_redacted_total":                false,
		"loki_secretfilter_secrets_redacted_by_rule_total":        false,
		"loki_secretfilter_secrets_redacted_by_origin":            false,
		"loki_secretfilter_secrets_allowlisted_total":             false,
		"loki_secretfilter_processing_duration_seconds":           false,
		"loki_secretfilter_secrets_skipped_entropy_by_rule_total": false,
	}

	// Check each metric family
	for _, metricFamily := range metricFamilies {
		name := metricFamily.GetName()
		if _, exists := expectedMetrics[name]; exists {
			expectedMetrics[name] = true
		}
	}

	// Verify all expected metrics were found
	for metric, found := range expectedMetrics {
		require.True(t, found, "Expected metric %s to be registered", metric)
	}
}

// Test metrics for secrets across multiple log lines
func TestMetricsMultipleEntries(t *testing.T) {
	registry := prometheus.NewRegistry()

	args := Arguments{
		ForwardTo:   []loki.LogsReceiver{loki.NewLogsReceiver()},
		OriginLabel: "job",
	}

	opts := component.Options{
		Logger:         util.TestLogger(t),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
		Registerer:     registry,
	}

	c, err := New(opts, args)
	require.NoError(t, err)

	// Process multiple entries with secrets
	entries := []loki.Entry{
		{
			Labels: model.LabelSet{"job": "test1"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      testLogs["simple_secret"].log,
			},
		},
		{
			Labels: model.LabelSet{"job": "test2"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      testLogs["simple_secret_gcp"].log,
			},
		},
		{
			Labels: model.LabelSet{"job": "test3"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      testLogs["no_secret"].log,
			},
		},
		{
			Labels: model.LabelSet{"job": "test4"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      testLogs["simple_secret"].log,
			},
		},
	}

	for _, entry := range entries {
		c.processEntry(entry)
	}

	// Verify the metrics
	// We should have 3 redacted secrets (2 grafana-api-key and 1 gcp-api-key)
	require.Equal(t, float64(3), testutil.ToFloat64(c.metrics.secretsRedactedTotal),
		"secretsRedactedTotal should count all secrets across multiple entries")

	// Check secretsRedactedByRule for each rule type
	require.NoError(t,
		testutil.GatherAndCompare(registry, strings.NewReader(`
			# HELP loki_secretfilter_secrets_redacted_by_rule_total Number of secrets redacted, partitioned by rule name.
			# TYPE loki_secretfilter_secrets_redacted_by_rule_total counter
			loki_secretfilter_secrets_redacted_by_rule_total{rule="grafana-api-key"} 2
			loki_secretfilter_secrets_redacted_by_rule_total{rule="gcp-api-key"} 1
		`),
			"loki_secretfilter_secrets_redacted_by_rule_total"))

	// Check secretsRedactedByOrigin values
	require.NoError(t,
		testutil.GatherAndCompare(registry, strings.NewReader(`
			# HELP loki_secretfilter_secrets_redacted_by_origin Number of secrets redacted, partitioned by origin label value.
			# TYPE loki_secretfilter_secrets_redacted_by_origin counter
			loki_secretfilter_secrets_redacted_by_origin{origin="test1"} 1
			loki_secretfilter_secrets_redacted_by_origin{origin="test2"} 1
			loki_secretfilter_secrets_redacted_by_origin{origin="test4"} 1
		`),
			"loki_secretfilter_secrets_redacted_by_origin"))
}

// TestArgumentsUpdate validates that the secretfilter component works correctly
// when its arguments are updated multiple times during runtime
func TestArgumentsUpdate(t *testing.T) {
	// Create a new registry to collect metrics
	registry := prometheus.NewRegistry()

	// Create a receiver to collect filtered logs
	ch1 := loki.NewLogsReceiver()

	// Initial arguments with basic configuration
	initialArgs := Arguments{
		ForwardTo:   []loki.LogsReceiver{ch1},
		OriginLabel: "",
		Types:       []string{"grafana"}, // Only redact Grafana API keys initially
	}

	// Create options with the test registry
	opts := component.Options{
		Logger:         util.TestLogger(t),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
		Registerer:     registry,
	}

	// Create component with initial arguments
	c, err := New(opts, initialArgs)
	require.NoError(t, err)

	// Test data for different configurations
	testData := []struct {
		description          string
		args                 Arguments
		inputLog             string
		expectedRedact       bool
		expectedOriginCounts map[string]int64
		labelToCheck         string
	}{
		{
			description:          "Initial config - should redact Grafana API key but not GCP key",
			args:                 initialArgs,
			inputLog:             testLogs["simple_secret"].log, // Grafana API key
			expectedRedact:       true,
			labelToCheck:         "",
			expectedOriginCounts: map[string]int64{},
		},
		{
			description: "Update 1 - Change to only track GCP keys",
			args: Arguments{
				ForwardTo:   []loki.LogsReceiver{ch1},
				OriginLabel: "job",
				Types:       []string{"gcp"}, // Only track GCP API keys
			},
			inputLog:             testLogs["simple_secret_gcp"].log, // GCP API key
			expectedRedact:       true,
			labelToCheck:         "job",
			expectedOriginCounts: map[string]int64{"test-job": 1},
		},
		{
			description: "Update 2 - Add custom redaction string and use instance label as origin",
			args: Arguments{
				ForwardTo:   []loki.LogsReceiver{ch1},
				OriginLabel: "instance",
				Types:       []string{"gcp"},
				RedactWith:  "<CUSTOM-REDACTED:$SECRET_NAME>",
			},
			inputLog:             testLogs["simple_secret_gcp"].log, // GCP API key
			expectedRedact:       true,
			labelToCheck:         "instance",
			expectedOriginCounts: map[string]int64{"test-job": 1, "test-instance": 1},
		},
		{
			description: "Update 3 - Add allowlist for GCP keys",
			args: Arguments{
				ForwardTo:   []loki.LogsReceiver{ch1},
				OriginLabel: "instance",
				Types:       []string{"gcp"},
				RedactWith:  "<CUSTOM-REDACTED:$SECRET_NAME>",
				AllowList:   []string{regexp.QuoteMeta(fakeSecrets["gcp-api-key"].value)},
			},
			inputLog:             testLogs["simple_secret_gcp"].log, // GCP API key (now allowlisted)
			expectedRedact:       false,
			labelToCheck:         "instance",
			expectedOriginCounts: map[string]int64{"test-job": 1, "test-instance": 1}, // no increase due to allowlist
		},
		{
			description: "Update 4 - Change origin label back to job",
			args: Arguments{
				ForwardTo:   []loki.LogsReceiver{ch1},
				OriginLabel: "job",
				Types:       []string{"gcp"},
				RedactWith:  "<CUSTOM-REDACTED:$SECRET_NAME>",
			},
			inputLog:             testLogs["simple_secret_gcp"].log, // GCP API key
			expectedRedact:       true,
			labelToCheck:         "job",
			expectedOriginCounts: map[string]int64{"test-job": 2, "test-instance": 1},
		},
	}

	// For each configuration update
	for i, td := range testData {
		t.Run(td.description, func(t *testing.T) {
			// Update component arguments if not the first test
			if i > 0 {
				err = c.Update(td.args)
				require.NoError(t, err)
			}

			// Create a test entry with labels
			labels := model.LabelSet{
				"job":      "test-job",
				"instance": "test-instance",
			}
			entry := loki.Entry{
				Labels: labels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      td.inputLog,
				},
			}

			// Process the entry
			processedEntry := c.processEntry(entry)

			// Check if redaction happened
			if td.expectedRedact {
				// Log should be different after redaction
				require.NotEqual(t, td.inputLog, processedEntry.Line,
					"Log should have been redacted but wasn't")

				// Verify the redaction total metric was incremented
				require.GreaterOrEqual(t, testutil.ToFloat64(c.metrics.secretsRedactedTotal), 1.0,
					"secretsRedactedTotal metric should have been incremented")

				// Check that the label metric was incremented for the expected label
				expectedLabelValue := string(labels[model.LabelName(td.labelToCheck)])

				// Find the metric with the right label
				metricFamilies, err := registry.Gather()
				require.NoError(t, err)

				var foundLabelMetric bool
				foundOriginMetric := make(map[string]int64, len(td.expectedOriginCounts))
				for _, mf := range metricFamilies {
					if mf.GetName() == "loki_secretfilter_secrets_redacted_by_origin" {
						for _, m := range mf.GetMetric() {
							for _, l := range m.GetLabel() {
								if l.GetName() == "origin" && l.GetValue() == expectedLabelValue {
									foundLabelMetric = true
									require.GreaterOrEqual(t, m.GetCounter().GetValue(), 1.0,
										"Origin metric should have been incremented")
								}
								if l.GetName() == "origin" {
									foundOriginMetric[l.GetValue()] = int64(m.GetCounter().GetValue())
								}
							}
						}
					}
				}

				// Check that the origin metric counts match the expected values
				require.True(t, maps.Equal(td.expectedOriginCounts, foundOriginMetric),
					"Origin metric counts should match the expected values")

				// Skip label check if running first test with new registry
				if i != 0 {
					require.True(t, foundLabelMetric,
						"Origin metric should have been recorded for %s=%s",
						td.labelToCheck, expectedLabelValue)
				}
			} else {
				// Log should remain unchanged if not redacted
				require.Equal(t, td.inputLog, processedEntry.Line,
					"Log should not have been redacted")

				// Verify the allowlisted metric was incremented
				if len(td.args.AllowList) > 0 {
					count, err := testutil.GatherAndCount(registry, "loki_secretfilter_secrets_allowlisted_total")
					require.NoError(t, err)
					require.Equal(t, 1, count, "allowlisted metric should be registered")
				}
			}
		})
	}
}

func TestEntropy(t *testing.T) {
	tc := []struct {
		input    string
		expected float64
	}{
		{"", 0.0},
		{"Hello world!", 3.022055},
		{"c0535e4be2b79ffd93291305436bf889314e4a3faec05ecffcbb7df31ad9e51a", 3.857009},
	}
	for _, test := range tc {
		t.Run(test.input, func(t *testing.T) {
			result := calculateEntropy(test.input)
			// Use a tolerance for float comparison
			tolerance := 1e-6
			require.True(t, compareFloats(result, test.expected, tolerance),
				"Expected entropy for '%s' to be %f, got %f", test.input, test.expected, result)
		})
	}
}

func compareFloats(a, b float64, tolerance float64) bool {
	if a == b {
		return true
	}

	// Compare two floats with a tolerance
	return math.Abs(a-b) < tolerance
}

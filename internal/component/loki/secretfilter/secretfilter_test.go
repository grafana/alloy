package secretfilter

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/jaswdr/faker/v2"
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
	fake := faker.NewWithSeed(rand.NewSource(2014))
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
			entry := loki.Entry{Labels: model.LabelSet{}, Entry: logproto.Entry{Timestamp: time.Now(), Line: input}}
			c.processEntry(entry)
		}
	}
}

var sampleFuzzLogLines = []string{
	`key=value1,value2 log=fmt test=1 secret=password`,
	`{"key":["value1","value2"],"log":"fmt","test":1,"secret":"password"}`,
	`1970-01-01 00:00:00 pattern value1,value2 1 secret`,
}

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
			entry := loki.Entry{Labels: model.LabelSet{}, Entry: logproto.Entry{Timestamp: time.Now(), Line: log}}
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

		entry := loki.Entry{Labels: model.LabelSet{}, Entry: logproto.Entry{Timestamp: time.Now(), Line: log}}
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

		entry := loki.Entry{Labels: model.LabelSet{}, Entry: logproto.Entry{Timestamp: time.Now(), Line: log}}
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

func getServiceData(name string) (interface{}, error) {
	switch name {
	case livedebugging.ServiceName:
		return livedebugging.NewLiveDebugging(), nil
	default:
		return nil, fmt.Errorf("service not found %s", name)
	}
}

type noopLogger struct{}

func (d *noopLogger) Log(_ ...interface{}) error {
	return nil
}

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
	eros.Remove(path)
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

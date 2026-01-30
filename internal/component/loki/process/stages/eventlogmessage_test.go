package stages

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
)

var testEvtLogMsgYamlDefaults = `
stage.eventlogmessage {}
`

var testEvtLogMsgYamlCustomSource = `
stage.eventlogmessage { source = "Message" }
`

var testEvtLogMsgYamlDropInvalidLabels = `
stage.eventlogmessage { drop_invalid_labels = true }
`

var testEvtLogMsgYamlOverwriteExisting = `
stage.eventlogmessage { overwrite_existing = true }
`

var (
	testEvtLogMsgSimple        = "Key1: Value 1\r\nKey2: Value 2\r\nKey3: Value: 3"
	testEvtLogMsgInvalidLabels = "Key 1: Value 1\r\n0Key2: Value 2\r\nKey@3: Value 3\r\n: Value 4"
	testEvtLogMsgOverwriteTest = "test: new value"
)

func TestEventLogMessage_simple(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config          string
		sourcekey       string
		msgdata         string
		extractedValues map[string]any
	}{
		"successfully ran a pipeline with sample event log message stage using default source": {
			testEvtLogMsgYamlDefaults,
			"message",
			testEvtLogMsgSimple,
			map[string]any{
				"Key1": "Value 1",
				"Key2": "Value 2",
				"Key3": "Value: 3",
				"test": "existing value",
			},
		},
		"successfully ran a pipeline with sample event log message stage using custom source": {
			testEvtLogMsgYamlCustomSource,
			"Message",
			testEvtLogMsgSimple,
			map[string]any{
				"Key1": "Value 1",
				"Key2": "Value 2",
				"Key3": "Value: 3",
				"test": "existing value",
			},
		},
		"successfully ran a pipeline with sample event log message stage containing invalid labels": {
			testEvtLogMsgYamlDefaults,
			"message",
			testEvtLogMsgInvalidLabels,
			map[string]any{
				"Key_1": "Value 1",
				"_Key2": "Value 2",
				"Key_3": "Value 3",
				"_":     "Value 4",
				"test":  "existing value",
			},
		},
		"successfully ran a pipeline with sample event log message stage without overwriting existing labels": {
			testEvtLogMsgYamlDefaults,
			"message",
			testEvtLogMsgOverwriteTest,
			map[string]any{
				"test":           "existing value",
				"test_extracted": "new value",
			},
		},
		"successfully ran a pipeline with sample event log message stage overwriting existing labels": {
			testEvtLogMsgYamlOverwriteExisting,
			"message",
			testEvtLogMsgOverwriteTest,
			map[string]any{
				"test": "new value",
			},
		},
	}

	for testName, testData := range tests {
		testData := testData
		testData.extractedValues[testData.sourcekey] = testData.msgdata

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			pl, err := NewPipeline(log.NewNopLogger(), loadConfig(testData.config), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			assert.NoError(t, err, "Expected pipeline creation to not result in error")
			out := processEntries(pl,
				newEntry(map[string]any{
					testData.sourcekey: testData.msgdata,
					"test":             "existing value",
				}, nil, testData.msgdata, time.Now()))[0]
			assert.Equal(t, testData.extractedValues, out.Extracted)
		})
	}
}

func TestEventLogMessageConfig_validate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config string
		err    error
	}{
		"valid config": {
			`stage.eventlogmessage { source = "msg"}`,
			nil,
		},
		"invalid config": {
			`stage.eventlogmessage { source = 1}`,
			errors.New("invalid label name: 1"),
		},
		"invalid source": {
			`stage.eventlogmessage { source = "the message"}`,
			fmt.Errorf(ErrInvalidLabelName, "the message"),
		},
		"empty source": {
			`stage.eventlogmessage { source = ""}`,
			fmt.Errorf(ErrInvalidLabelName, ""),
		},
	}
	for tName, tt := range tests {
		tt := tt
		t.Run(tName, func(t *testing.T) {
			var config Configs
			err := syntax.Unmarshal([]byte(tt.config), &config)
			if err == nil {
				require.Len(t, config.Stages, 1)
				err = config.Stages[0].EventLogMessageConfig.Validate()
			}

			if err == nil && tt.err != nil {
				assert.NotNil(t, err, "EventLogMessage.validate() expected error = %v, but got nil", tt.err)
			}
			if err != nil {
				assert.Equal(t, tt.err.Error(), err.Error(), "EventLogMessage.validate() expected error = %v, actual error = %v", tt.err, err)
			}
		})
	}
}

var testEvtLogMsgNetworkConn = "Network connection detected:\r\nRuleName: Usermode\r\n" +
	"UtcTime: 2023-01-31 08:07:23.782\r\nProcessGuid: {44ffd2c7-cc3a-63d8-2002-000000000d00}\r\n" +
	"ProcessId: 7344\r\nImage: C:\\Users\\User\\promtail\\promtail-windows-amd64.exe\r\n" +
	"User: WINTEST2211\\User\r\nProtocol: tcp\r\nInitiated: true\r\nSourceIsIpv6: false\r\n" +
	"SourceIp: 10.0.2.15\r\nSourceHostname: WinTest2211..\r\nSourcePort: 49992\r\n" +
	"SourcePortName: -\r\nDestinationIsIpv6: false\r\nDestinationIp: 34.117.8.58\r\n" +
	"DestinationHostname: 58.8.117.34.bc.googleusercontent.com\r\nDestinationPort: 443\r\n" +
	"DestinationPortName: https"

func TestEventLogMessage_Real(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config          string
		sourcekey       string
		msgdata         string
		extractedValues map[string]any
	}{
		"successfully ran a pipeline with network event log message stage using default source": {
			testEvtLogMsgYamlDefaults,
			"message",
			testEvtLogMsgNetworkConn,
			map[string]any{
				"Network_connection_detected": "",
				"RuleName":                    "Usermode",
				"UtcTime":                     "2023-01-31 08:07:23.782",
				"ProcessGuid":                 "{44ffd2c7-cc3a-63d8-2002-000000000d00}",
				"ProcessId":                   "7344",
				"Image":                       "C:\\Users\\User\\promtail\\promtail-windows-amd64.exe",
				"User":                        "WINTEST2211\\User",
				"Protocol":                    "tcp",
				"Initiated":                   "true",
				"SourceIsIpv6":                "false",
				"SourceIp":                    "10.0.2.15",
				"SourceHostname":              "WinTest2211..",
				"SourcePort":                  "49992",
				"SourcePortName":              "-",
				"DestinationIsIpv6":           "false",
				"DestinationIp":               "34.117.8.58",
				"DestinationHostname":         "58.8.117.34.bc.googleusercontent.com",
				"DestinationPort":             "443",
				"DestinationPortName":         "https",
			},
		},
		"successfully ran a pipeline with network event log message stage using custom source": {
			testEvtLogMsgYamlCustomSource,
			"Message",
			testEvtLogMsgNetworkConn,
			map[string]any{
				"Network_connection_detected": "",
				"RuleName":                    "Usermode",
				"UtcTime":                     "2023-01-31 08:07:23.782",
				"ProcessGuid":                 "{44ffd2c7-cc3a-63d8-2002-000000000d00}",
				"ProcessId":                   "7344",
				"Image":                       "C:\\Users\\User\\promtail\\promtail-windows-amd64.exe",
				"User":                        "WINTEST2211\\User",
				"Protocol":                    "tcp",
				"Initiated":                   "true",
				"SourceIsIpv6":                "false",
				"SourceIp":                    "10.0.2.15",
				"SourceHostname":              "WinTest2211..",
				"SourcePort":                  "49992",
				"SourcePortName":              "-",
				"DestinationIsIpv6":           "false",
				"DestinationIp":               "34.117.8.58",
				"DestinationHostname":         "58.8.117.34.bc.googleusercontent.com",
				"DestinationPort":             "443",
				"DestinationPortName":         "https",
			},
		},
		"successfully ran a pipeline with network event log message stage dropping invalid labels": {
			testEvtLogMsgYamlDropInvalidLabels,
			"message",
			testEvtLogMsgNetworkConn,
			map[string]any{
				"RuleName":            "Usermode",
				"UtcTime":             "2023-01-31 08:07:23.782",
				"ProcessGuid":         "{44ffd2c7-cc3a-63d8-2002-000000000d00}",
				"ProcessId":           "7344",
				"Image":               "C:\\Users\\User\\promtail\\promtail-windows-amd64.exe",
				"User":                "WINTEST2211\\User",
				"Protocol":            "tcp",
				"Initiated":           "true",
				"SourceIsIpv6":        "false",
				"SourceIp":            "10.0.2.15",
				"SourceHostname":      "WinTest2211..",
				"SourcePort":          "49992",
				"SourcePortName":      "-",
				"DestinationIsIpv6":   "false",
				"DestinationIp":       "34.117.8.58",
				"DestinationHostname": "58.8.117.34.bc.googleusercontent.com",
				"DestinationPort":     "443",
				"DestinationPortName": "https",
			},
		},
	}

	for testName, testData := range tests {
		testData := testData
		testData.extractedValues[testData.sourcekey] = testData.msgdata

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			pl, err := NewPipeline(log.NewNopLogger(), loadConfig(testData.config), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			assert.NoError(t, err, "Expected pipeline creation to not result in error")
			out := processEntries(pl,
				newEntry(map[string]any{testData.sourcekey: testData.msgdata}, nil, testData.msgdata, time.Now()))[0]
			assert.Equal(t, testData.extractedValues, out.Extracted)
		})
	}
}

var (
	testEvtLogMsgInvalidStructure = "\n\rwhat; is this?\n\r"
	testEvtLogMsgInvalidValue     = "Key1: " + string([]byte{0xff, 0xfe, 0xfd})
)

func TestEventLogMessage_invalid(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config          string
		sourcekey       string
		msgdata         string
		extractedValues map[string]any
	}{
		"successfully ran a pipeline with an invalid event log message": {
			testEvtLogMsgYamlDefaults,
			"message",
			testEvtLogMsgInvalidStructure,
			map[string]any{},
		},
		"successfully ran a pipeline with sample event log message stage on the wrong default source": {
			testEvtLogMsgYamlDefaults,
			"notmessage",
			testEvtLogMsgSimple,
			map[string]any{},
		},
		"successfully ran a pipeline with sample event log message stage dropping invalid labels": {
			testEvtLogMsgYamlDropInvalidLabels,
			"message",
			testEvtLogMsgInvalidLabels,
			map[string]any{},
		},
		"successfully ran a pipeline with an invalid event log message value (not UTF-8)": {
			testEvtLogMsgYamlDefaults,
			"message",
			testEvtLogMsgInvalidValue,
			map[string]any{},
		},
	}

	for testName, testData := range tests {
		testData := testData
		testData.extractedValues[testData.sourcekey] = testData.msgdata

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			pl, err := NewPipeline(log.NewNopLogger(), loadConfig(testData.config), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			assert.NoError(t, err, "Expected pipeline creation to not result in error")
			out := processEntries(pl,
				newEntry(map[string]any{testData.sourcekey: testData.msgdata}, nil, testData.msgdata, time.Now()))[0]
			assert.Equal(t, testData.extractedValues, out.Extracted)
		})
	}
}

func TestEventLogMessage_invalidString(t *testing.T) {
	t.Parallel()

	pl, err := NewPipeline(log.NewNopLogger(), loadConfig(testEvtLogMsgYamlDefaults), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	assert.NoError(t, err, "Expected pipeline creation to not result in error")
	out := processEntries(pl,
		newEntry(map[string]any{"message": nil}, nil, "", time.Now()))
	assert.Len(t, out, 0, "No output should be produced with a nil input")
}

var (
	inputJustKey       = "Key 1:"
	inputBoth          = "Key 1: Value 1"
	RegexSplitKeyValue = regexp.MustCompile(": ?")
)

func BenchmarkSplittingKeyValuesRegex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var val string
		resultKey := RegexSplitKeyValue.Split(inputJustKey, 2)
		if len(resultKey) > 1 {
			val = resultKey[1]
		}
		resultKeyValue := RegexSplitKeyValue.Split(inputBoth, 2)
		if len(resultKeyValue) > 1 {
			val = resultKeyValue[1]
		}
		_ = val
	}
}

func BenchmarkSplittingKeyValuesSplitTrim(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var val string
		resultKey := strings.SplitN(inputJustKey, ":", 2)
		if len(resultKey) > 1 {
			val = strings.TrimSpace(resultKey[1])
		}
		resultKeyValue := strings.SplitN(inputBoth, ":", 2)
		if len(resultKey) > 1 {
			val = strings.TrimSpace(resultKeyValue[1])
		}
		_ = val
	}
}

func BenchmarkSplittingKeyValuesSplitSubstr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var val string
		resultKey := strings.SplitN(inputJustKey, ":", 2)
		if len(resultKey) > 1 && len(resultKey[1]) > 0 {
			val = resultKey[1][1:]
		}
		resultKeyValue := strings.SplitN(inputBoth, ":", 2)
		if len(resultKey) > 1 && len(resultKey[1]) > 0 {
			val = resultKeyValue[1][1:]
		}
		_ = val
	}
}

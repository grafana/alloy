package stages

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

func TestEventLogMessageStage(t *testing.T) {
	var (
		now                      = time.Now()
		testEvtLogMsgNetworkConn = "Network connection detected:\r\nRuleName: Usermode\r\n" +
			"UtcTime: 2023-01-31 08:07:23.782\r\nProcessGuid: {44ffd2c7-cc3a-63d8-2002-000000000d00}\r\n" +
			"ProcessId: 7344\r\nImage: C:\\Users\\User\\promtail\\promtail-windows-amd64.exe\r\n" +
			"User: WINTEST2211\\User\r\nProtocol: tcp\r\nInitiated: true\r\nSourceIsIpv6: false\r\n" +
			"SourceIp: 10.0.2.15\r\nSourceHostname: WinTest2211..\r\nSourcePort: 49992\r\n" +
			"SourcePortName: -\r\nDestinationIsIpv6: false\r\nDestinationIp: 34.117.8.58\r\n" +
			"DestinationHostname: 58.8.117.34.bc.googleusercontent.com\r\nDestinationPort: 443\r\n" +
			"DestinationPortName: https"

		testEvtLogMsgSimple           = "Key1: Value 1\r\nKey2: Value 2\r\nKey3: Value: 3"
		testEvtLogMsgInvalidLabels    = "Key 1: Value 1\r\n0Key2: Value 2\r\nKey@3: Value 3\r\n: Value 4"
		testEvtLogMsgOverwriteTest    = "test: new value"
		testEvtLogMsgInvalidStructure = "\n\rwhat; is this?\n\r"
		testEvtLogMsgInvalidValue     = "Key1: " + string([]byte{0xff, 0xfe, 0xfd})
	)

	type testCase struct {
		name     string
		cfg      EventLogMessageConfig
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "using default source",
			cfg:  EventLogMessageConfig{Source: "message"},
			entries: []Entry{
				newEntry(map[string]any{"message": testEvtLogMsgSimple, "test": "existing value"}, model.LabelSet{}, testEvtLogMsgSimple, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"message": testEvtLogMsgSimple,
					"Key1":    "Value 1",
					"Key2":    "Value 2",
					"Key3":    "Value: 3",
					"test":    "existing value",
				}, model.LabelSet{}, testEvtLogMsgSimple, now),
			},
		},
		{
			name: "using custom source",
			cfg:  EventLogMessageConfig{Source: "Message"},
			entries: []Entry{
				newEntry(map[string]any{"Message": testEvtLogMsgSimple, "test": "existing value"}, model.LabelSet{}, testEvtLogMsgSimple, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"Message": testEvtLogMsgSimple,
					"Key1":    "Value 1",
					"Key2":    "Value 2",
					"Key3":    "Value: 3",
					"test":    "existing value",
				}, model.LabelSet{}, testEvtLogMsgSimple, now),
			},
		},
		{
			name: "containing invalid labels",
			cfg:  EventLogMessageConfig{Source: "message"},
			entries: []Entry{
				newEntry(map[string]any{"message": testEvtLogMsgInvalidLabels, "test": "existing value"}, model.LabelSet{}, testEvtLogMsgInvalidLabels, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"message": testEvtLogMsgInvalidLabels,
					"Key_1":   "Value 1",
					"_Key2":   "Value 2",
					"Key_3":   "Value 3",
					"_":       "Value 4",
					"test":    "existing value",
				}, model.LabelSet{}, testEvtLogMsgInvalidLabels, now),
			},
		},
		{
			name: "without overwriting existing labels",
			cfg:  EventLogMessageConfig{Source: "message"},
			entries: []Entry{
				newEntry(map[string]any{"message": testEvtLogMsgOverwriteTest, "test": "existing value"}, model.LabelSet{}, testEvtLogMsgOverwriteTest, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"message":        testEvtLogMsgOverwriteTest,
					"test":           "existing value",
					"test_extracted": "new value",
				}, model.LabelSet{}, testEvtLogMsgOverwriteTest, now),
			},
		},
		{
			name: "overwriting existing labels",
			cfg:  EventLogMessageConfig{Source: "message", OverwriteExisting: true},
			entries: []Entry{
				newEntry(map[string]any{"message": testEvtLogMsgOverwriteTest, "test": "existing value"}, model.LabelSet{}, testEvtLogMsgOverwriteTest, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"message": testEvtLogMsgOverwriteTest,
					"test":    "new value",
				}, model.LabelSet{}, testEvtLogMsgOverwriteTest, now),
			},
		},
		{
			name: "invalid message structure",
			cfg:  EventLogMessageConfig{Source: "message"},
			entries: []Entry{
				newEntry(map[string]any{"message": testEvtLogMsgInvalidStructure}, model.LabelSet{}, testEvtLogMsgInvalidStructure, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"message": testEvtLogMsgInvalidStructure}, model.LabelSet{}, testEvtLogMsgInvalidStructure, now),
			},
		},
		{
			name: "wrong source",
			cfg:  EventLogMessageConfig{Source: "message"},
			entries: []Entry{
				newEntry(map[string]any{"notmessage": testEvtLogMsgSimple}, model.LabelSet{}, testEvtLogMsgSimple, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"notmessage": testEvtLogMsgSimple}, model.LabelSet{}, testEvtLogMsgSimple, now),
			},
		},
		{
			name: "dropping invalid labels",
			cfg:  EventLogMessageConfig{Source: "message", DropInvalidLabels: true},
			entries: []Entry{
				newEntry(map[string]any{"message": testEvtLogMsgInvalidLabels}, model.LabelSet{}, testEvtLogMsgInvalidLabels, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"message": testEvtLogMsgInvalidLabels}, model.LabelSet{}, testEvtLogMsgInvalidLabels, now),
			},
		},
		{
			name: "invalid value not utf-8",
			cfg:  EventLogMessageConfig{Source: "message"},
			entries: []Entry{
				newEntry(map[string]any{"message": testEvtLogMsgInvalidValue}, model.LabelSet{}, testEvtLogMsgInvalidValue, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"message": testEvtLogMsgInvalidValue}, model.LabelSet{}, testEvtLogMsgInvalidValue, now),
			},
		},
		{
			name: "network connection using default source",
			cfg:  EventLogMessageConfig{Source: "message"},
			entries: []Entry{
				newEntry(map[string]any{"message": testEvtLogMsgNetworkConn}, model.LabelSet{}, testEvtLogMsgNetworkConn, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"message":                     testEvtLogMsgNetworkConn,
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
				}, model.LabelSet{}, testEvtLogMsgNetworkConn, now),
			},
		},
		{
			name: "network connection using custom source",
			cfg:  EventLogMessageConfig{Source: "Message"},
			entries: []Entry{
				newEntry(map[string]any{"Message": testEvtLogMsgNetworkConn}, model.LabelSet{}, testEvtLogMsgNetworkConn, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"Message":                     testEvtLogMsgNetworkConn,
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
				}, model.LabelSet{}, testEvtLogMsgNetworkConn, now),
			},
		},
		{
			name: "network connection dropping invalid labels",
			cfg:  EventLogMessageConfig{Source: "message", DropInvalidLabels: true},
			entries: []Entry{
				newEntry(map[string]any{"message": testEvtLogMsgNetworkConn}, model.LabelSet{}, testEvtLogMsgNetworkConn, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"message":             testEvtLogMsgNetworkConn,
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
				}, model.LabelSet{}, testEvtLogMsgNetworkConn, now),
			},
		},
		{
			name: "invalid value not a string",
			cfg:  EventLogMessageConfig{Source: "message"},
			entries: []Entry{
				newEntry(map[string]any{"message": nil}, model.LabelSet{}, "", now),
			},
			expected: []Entry{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, []StageConfig{{EventLogMessageConfig: &tt.cfg}}, tt.entries, tt.expected, "")
		})
	}
}

func TestValidateEventLogMessageConfig(t *testing.T) {
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
			fmt.Errorf(errInvalidLabelName, "the message"),
		},
		"empty source": {
			`stage.eventlogmessage { source = ""}`,
			fmt.Errorf(errInvalidLabelName, ""),
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

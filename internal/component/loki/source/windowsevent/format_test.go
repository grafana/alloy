//go:build windows

package windowsevent

import (
	"testing"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/goleak"
	"github.com/grafana/loki/v3/clients/pkg/promtail/targets/windows/win_eventlog"
	"github.com/stretchr/testify/require"
)

func TestFormatIncludeEventData(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	args := Arguments{
		Locale:               0,
		EventLogName:         "Application",
		XPathQuery:           "*",
		UseIncomingTimestamp: false,
		BookmarkPath:         "",
		ExcludeEventData:     false,
		ExcludeEventMessage:  false,
		ExcludeUserdata:      false,
		IncludeEventDataMap:  true,
	}

	event := win_eventlog.Event{
		EventID: 1234,
		EventData: win_eventlog.EventData{
			InnerXML: []byte("<Data Name=\"test_key_name\">test_value</Data><Data Name=\"test_key_name2\">test_value2</Data>"),
		},
	}

	formatted, err := formatLine(args, event)
	require.NoError(t, err)
	var jdata map[string]interface{}
	err = jsoniter.Unmarshal([]byte(formatted), &jdata)
	require.NoError(t, err)
	require.Equal(t, "test_value", jdata["event_data_map"].(map[string]interface{})["test_key_name"])
	require.Equal(t, "test_value2", jdata["event_data_map"].(map[string]interface{})["test_key_name2"])
}

func TestFormatExcludeEventData(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	args := Arguments{
		Locale:               0,
		EventLogName:         "Application",
		XPathQuery:           "*",
		UseIncomingTimestamp: false,
		BookmarkPath:         "",
		ExcludeEventData:     true,
		ExcludeEventMessage:  false,
		ExcludeUserdata:      false,
		IncludeEventDataMap:  false,
	}
	
	event := win_eventlog.Event{
		EventID: 1234,
		EventData: win_eventlog.EventData{
			InnerXML: []byte("<Data Name=\"test_key_name\">test_value</Data><Data Name=\"test_key_name2\">test_value2</Data>"),
		},
	}

	formatted, err := formatLine(args, event)
	require.NoError(t, err)
	var jdata map[string]interface{}
	err = jsoniter.Unmarshal([]byte(formatted), &jdata)
	require.NoError(t, err)
	require.Nil(t, jdata["event_data_map"])
	require.Nil(t, jdata["event_data"])
}	
	
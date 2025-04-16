package windowsevent

// NOTE: The arguments here are based on commit bde6566
// of Promtail's arguments in Loki's repository:
// https://github.com/grafana/loki/blob/bde65667f7c88af17b7729e3621d7bd5d1d3b45f/clients/pkg/promtail/scrapeconfig/scrapeconfig.go#L211-L255

import (
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// Arguments holds values which are used to configure the loki.source.windowsevent
// component.
type Arguments struct {
	Locale               int                 `alloy:"locale,attr,optional"`
	EventLogName         string              `alloy:"eventlog_name,attr,optional"`
	XPathQuery           string              `alloy:"xpath_query,attr,optional"`
	BookmarkPath         string              `alloy:"bookmark_path,attr,optional"`
	PollInterval         time.Duration       `alloy:"poll_interval,attr,optional"`
	ExcludeEventData     bool                `alloy:"exclude_event_data,attr,optional"`
	ExcludeUserdata      bool                `alloy:"exclude_user_data,attr,optional"`
	ExcludeEventMessage  bool                `alloy:"exclude_event_message,attr,optional"`
	IncludeEventDataMap  bool                `alloy:"include_event_data_map,attr,optional"`
	UseIncomingTimestamp bool                `alloy:"use_incoming_timestamp,attr,optional"`
	ForwardTo            []loki.LogsReceiver `alloy:"forward_to,attr"`
	Labels               map[string]string   `alloy:"labels,attr,optional"`
	LegacyBookmarkPath   string              `alloy:"legacy_bookmark_path,attr,optional"`
}

func defaultArgs() Arguments {
	return Arguments{
		Locale:               0,
		EventLogName:         "",
		XPathQuery:           "*",
		BookmarkPath:         "",
		PollInterval:         3 * time.Second,
		ExcludeEventData:     false,
		ExcludeUserdata:      false,
		ExcludeEventMessage:  false,
		UseIncomingTimestamp: false,
		IncludeEventDataMap:  false,
	}
}

// SetToDefault implements syntax.Defaulter.
func (r *Arguments) SetToDefault() {
	*r = defaultArgs()
}

package windowsevent

// NOTE: The arguments here are based on commit bde6566
// of Promtail's arguments in Loki's repository:
// https://github.com/grafana/loki/blob/bde65667f7c88af17b7729e3621d7bd5d1d3b45f/clients/pkg/promtail/scrapeconfig/scrapeconfig.go#L211-L255

import (
	"time"

	"github.com/grafana/agent/internal/component/common/loki"
)

// Arguments holds values which are used to configure the loki.source.windowsevent
// component.
type Arguments struct {
	Locale               int                 `river:"locale,attr,optional"`
	EventLogName         string              `river:"eventlog_name,attr,optional"`
	XPathQuery           string              `river:"xpath_query,attr,optional"`
	BookmarkPath         string              `river:"bookmark_path,attr,optional"`
	PollInterval         time.Duration       `river:"poll_interval,attr,optional"`
	ExcludeEventData     bool                `river:"exclude_event_data,attr,optional"`
	ExcludeUserdata      bool                `river:"exclude_user_data,attr,optional"`
	ExcludeEventMessage  bool                `river:"exclude_event_message,attr,optional"`
	UseIncomingTimestamp bool                `river:"use_incoming_timestamp,attr,optional"`
	ForwardTo            []loki.LogsReceiver `river:"forward_to,attr"`
	Labels               map[string]string   `river:"labels,attr,optional"`
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
	}
}

// SetToDefault implements river.Defaulter.
func (r *Arguments) SetToDefault() {
	*r = defaultArgs()
}

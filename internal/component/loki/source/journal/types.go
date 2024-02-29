package journal

import (
	"time"

	"github.com/grafana/agent/internal/component/common/loki"
	flow_relabel "github.com/grafana/agent/internal/component/common/relabel"
)

// Arguments are the arguments for the component.
type Arguments struct {
	FormatAsJson bool                `river:"format_as_json,attr,optional"`
	MaxAge       time.Duration       `river:"max_age,attr,optional"`
	Path         string              `river:"path,attr,optional"`
	RelabelRules flow_relabel.Rules  `river:"relabel_rules,attr,optional"`
	Matches      string              `river:"matches,attr,optional"`
	Receivers    []loki.LogsReceiver `river:"forward_to,attr"`
	Labels       map[string]string   `river:"labels,attr,optional"`
}

func defaultArgs() Arguments {
	return Arguments{
		FormatAsJson: false,
		MaxAge:       7 * time.Hour,
		Path:         "",
	}
}

// SetToDefault implements river.Defaulter.
func (r *Arguments) SetToDefault() {
	*r = defaultArgs()
}

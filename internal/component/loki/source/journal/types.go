package journal

import (
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
)

// Arguments are the arguments for the component.
type Arguments struct {
	FormatAsJson   bool                `alloy:"format_as_json,attr,optional"`
	MaxAge         time.Duration       `alloy:"max_age,attr,optional"`
	Path           string              `alloy:"path,attr,optional"`
	RelabelRules   alloy_relabel.Rules `alloy:"relabel_rules,attr,optional"`
	Matches        string              `alloy:"matches,attr,optional"`
	ForwardTo      []loki.LogsReceiver `alloy:"forward_to,attr"`
	Labels         map[string]string   `alloy:"labels,attr,optional"`
	LegacyPosition *LegacyPosition     `alloy:"legacy_position,block,optional"`
}

type LegacyPosition struct {
	File string `alloy:"file,attr"`
	Name string `alloy:"name,attr"`
}

func defaultArgs() Arguments {
	return Arguments{
		FormatAsJson: false,
		MaxAge:       7 * time.Hour,
		Path:         "",
	}
}

// SetToDefault implements syntax.Defaulter.
func (r *Arguments) SetToDefault() {
	*r = defaultArgs()
}

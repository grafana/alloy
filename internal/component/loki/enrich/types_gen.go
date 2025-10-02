// Code generated DO NOT EDIT
package enrich

import (
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
)

type Exports struct {
	// A receiver that can be used to send logs to this component.
	Receiver loki.LogsReceiver `alloy:"receiver,attr,optional"`
}

type Arguments struct {
	// List of receivers to send enriched logs to.
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`
	// List of labels to copy from discovered targets to logs. If empty, all labels will be copied.
	LabelsToCopy []string `alloy:"labels_to_copy,attr,optional"`
	// The label from incoming logs to match against discovered targets.
	LogsMatchLabel string `alloy:"logs_match_label,attr,optional"`
	// The label from discovered targets to match against, for example, "__inventory_consul_service".
	TargetMatchLabel string `alloy:"target_match_label,attr"`
	// List of targets from a discovery component.
	Targets []discovery.Target `alloy:"targets,attr"`
}

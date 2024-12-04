package process

import (
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/prometheus/prometheus/storage"
)

type Arguments struct {
	Wasm                []byte               `alloy:"wasm,attr"`
	Config              map[string]string    `alloy:"config,attr,optional"`
	LokiForwardTo       []loki.LogsReceiver  `alloy:"loki_forward_to,attr,optional"`
	PrometheusForwardTo []storage.Appendable `alloy:"prometheus_forward_to,attr,optional"`
}

type Exports struct {
	PrometheusReceiver storage.Appendable `alloy:"prometheus_receiver,attr"`
	LokiReceiver       loki.LogsReceiver  `alloy:"loki_receiver,attr"`
}

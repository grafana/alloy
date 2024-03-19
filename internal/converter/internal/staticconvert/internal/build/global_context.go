package build

import (
	"github.com/grafana/agent/internal/component/prometheus/remotewrite"
	"github.com/grafana/agent/internal/converter/internal/common"
)

type GlobalContext struct {
	IntegrationsLabelPrefix        string
	IntegrationsRemoteWriteExports *remotewrite.Exports
}

func (g *GlobalContext) InitializeIntegrationsRemoteWriteExports() {
	if g.IntegrationsRemoteWriteExports == nil {
		g.IntegrationsRemoteWriteExports = &remotewrite.Exports{
			Receiver: common.ConvertAppendable{Expr: "prometheus.remote_write." + g.IntegrationsLabelPrefix + ".receiver"},
		}
	}
}

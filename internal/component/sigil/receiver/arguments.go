package receiver

import (
	"github.com/alecthomas/units"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/sigil"
)

type Arguments struct {
	Server             *fnet.ServerConfig          `alloy:",squash"`
	ForwardTo          []sigil.GenerationsReceiver `alloy:"forward_to,attr"`
	MaxRequestBodySize units.Base2Bytes            `alloy:"max_request_body_size,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{
		Server:             fnet.DefaultServerConfig(),
		MaxRequestBodySize: 20 * units.MiB,
	}
}

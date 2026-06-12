package receiver

import (
	"errors"
	"fmt"

	"github.com/alecthomas/units"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/sigil"
)

type Arguments struct {
	Server             *fnet.ServerConfig           `alloy:",squash"`
	ForwardTo          []sigil.GenerationsForwarder `alloy:"forward_to,attr"`
	MaxRequestBodySize units.Base2Bytes             `alloy:"max_request_body_size,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{
		Server:             fnet.DefaultServerConfig(),
		MaxRequestBodySize: 20 * units.MiB,
	}
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if len(a.ForwardTo) == 0 {
		return errors.New("forward_to must contain at least one receiver")
	}
	for i, recv := range a.ForwardTo {
		if recv == nil {
			return fmt.Errorf("forward_to[%d] must not be null", i)
		}
	}
	if a.MaxRequestBodySize <= 0 {
		return errors.New("max_request_body_size must be greater than zero")
	}
	return nil
}

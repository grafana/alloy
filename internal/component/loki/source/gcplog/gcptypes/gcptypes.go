package gcptypes

import (
	"fmt"
	"time"

	fnet "github.com/grafana/alloy/internal/component/common/net"
)

// PullConfig configures a GCPLog target with the 'pull' strategy.
type PullConfig struct {
	ProjectID            string            `alloy:"project_id,attr"`
	Subscription         string            `alloy:"subscription,attr"`
	Labels               map[string]string `alloy:"labels,attr,optional"`
	UseIncomingTimestamp bool              `alloy:"use_incoming_timestamp,attr,optional"`
	UseFullLine          bool              `alloy:"use_full_line,attr,optional"`
}

// PushConfig configures a GCPLog target with the 'push' strategy.
type PushConfig struct {
	Server               *fnet.ServerConfig `alloy:",squash"`
	PushTimeout          time.Duration      `alloy:"push_timeout,attr,optional"`
	Labels               map[string]string  `alloy:"labels,attr,optional"`
	UseIncomingTimestamp bool               `alloy:"use_incoming_timestamp,attr,optional"`
	UseFullLine          bool               `alloy:"use_full_line,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (p *PushConfig) SetToDefault() {
	*p = PushConfig{
		Server: fnet.DefaultServerConfig(),
	}
}

// Validate implements syntax.Validator.
func (p *PushConfig) Validate() error {
	if p.PushTimeout < 0 {
		return fmt.Errorf("push_timeout must be greater than zero")
	}
	return nil
}

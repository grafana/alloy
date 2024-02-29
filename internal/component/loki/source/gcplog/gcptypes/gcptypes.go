package gcptypes

import (
	"fmt"
	"time"

	fnet "github.com/grafana/agent/internal/component/common/net"
)

// PullConfig configures a GCPLog target with the 'pull' strategy.
type PullConfig struct {
	ProjectID            string            `river:"project_id,attr"`
	Subscription         string            `river:"subscription,attr"`
	Labels               map[string]string `river:"labels,attr,optional"`
	UseIncomingTimestamp bool              `river:"use_incoming_timestamp,attr,optional"`
	UseFullLine          bool              `river:"use_full_line,attr,optional"`
}

// PushConfig configures a GCPLog target with the 'push' strategy.
type PushConfig struct {
	Server               *fnet.ServerConfig `river:",squash"`
	PushTimeout          time.Duration      `river:"push_timeout,attr,optional"`
	Labels               map[string]string  `river:"labels,attr,optional"`
	UseIncomingTimestamp bool               `river:"use_incoming_timestamp,attr,optional"`
	UseFullLine          bool               `river:"use_full_line,attr,optional"`
}

// SetToDefault implements river.Defaulter.
func (p *PushConfig) SetToDefault() {
	*p = PushConfig{
		Server: fnet.DefaultServerConfig(),
	}
}

// Validate implements river.Validator.
func (p *PushConfig) Validate() error {
	if p.PushTimeout < 0 {
		return fmt.Errorf("push_timeout must be greater than zero")
	}
	return nil
}

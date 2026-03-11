package gcptypes

import (
	"fmt"
	"time"

	"github.com/alecthomas/units"

	fnet "github.com/grafana/alloy/internal/component/common/net"
)

// PullConfig configures a GCPLog target with the 'pull' strategy.
type PullConfig struct {
	ProjectID            string            `alloy:"project_id,attr"`
	Subscription         string            `alloy:"subscription,attr"`
	Labels               map[string]string `alloy:"labels,attr,optional"`
	UseFullLine          bool              `alloy:"use_full_line,attr,optional"`
	UseIncomingTimestamp bool              `alloy:"use_incoming_timestamp,attr,optional"`
	Limit                LimitConfig       `alloy:"limit,block,optional"`
}

type LimitConfig struct {
	// MaxOutstandingBytes sets the byte budget for unprocessed messages.
	// Hitting this budget throttles delivery and limits concurrent in-flight handling.
	MaxOutstandingBytes units.Base2Bytes `alloy:"max_outstanding_bytes,attr,optional"`
	// MaxOutstandingMessages sets the count budget for unprocessed messages.
	// Hitting this budget stops further delivery and limits concurrent in-flight handling.
	MaxOutstandingMessages int `alloy:"max_outstanding_messages,attr,optional"`
}

var DefaultLimitConfig = LimitConfig{
	// Default from https://github.com/googleapis/google-cloud-go/blob/df64147605e961803c7ea839bc080ffd1b814ac9/pubsub/v2/subscriber.go#L172
	MaxOutstandingBytes: 1 * units.GiB,
	// Default from https://github.com/googleapis/google-cloud-go/blob/df64147605e961803c7ea839bc080ffd1b814ac9/pubsub/v2/subscriber.go#L171
	MaxOutstandingMessages: 1000,
}

func (l *LimitConfig) SetToDefault() {
	*l = DefaultLimitConfig
}

// PushConfig configures a GCPLog target with the 'push' strategy.
type PushConfig struct {
	Server               *fnet.ServerConfig `alloy:",squash"`
	PushTimeout          time.Duration      `alloy:"push_timeout,attr,optional"`
	Labels               map[string]string  `alloy:"labels,attr,optional"`
	UseFullLine          bool               `alloy:"use_full_line,attr,optional"`
	UseIncomingTimestamp bool               `alloy:"use_incoming_timestamp,attr,optional"`
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

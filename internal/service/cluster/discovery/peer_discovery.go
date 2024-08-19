package discovery

import (
	"fmt"
	"net"
	"strings"

	"github.com/go-kit/log"
	godiscover "github.com/hashicorp/go-discover"
	"go.opentelemetry.io/otel/trace"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

type DiscoverFn func() ([]string, error)

type Options struct {
	JoinPeers     []string
	DiscoverPeers string
	DefaultPort   int
	// Logger to surface extra information to the user. Required.
	Logger log.Logger
	// Tracer to emit spans. Required.
	Tracer trace.TracerProvider
	// lookupSRVFn is a function that can be used to lookup SRV records. If nil, net.LookupSRV is used. Used for testing.
	lookupSRVFn lookupSRVFn
	// lookupIPFn is a function that can be used to lookup addresses using A/AAAA DNS records. If nil, net.LookupIP is used. Used for testing.
	lookupIPFn lookupIPFn

	// goDiscoverFactory is a function that can be used to create a new discover.Discover instance.
	// If nil, godiscover.New is used. Used for testing.
	goDiscoverFactory goDiscoverFactory
}

// lookupSRVFn is a function that can be used to lookup SRV records. Matches net.LookupSRV signature.
type lookupSRVFn func(service, proto, name string) (string, []*net.SRV, error)

// lookupIPFn is a function that can be used to lookup IP addresses using A/AAAA DNS records. Matches net.LookupIP signature.
type lookupIPFn func(host string) ([]net.IP, error)

// goDiscoverFactory is a function that can be used to create a new discover.Discover instance.
// Matches discover.New signature.
type goDiscoverFactory func(opts ...godiscover.Option) (*godiscover.Discover, error)

func NewPeerDiscoveryFn(opts Options) (DiscoverFn, error) {
	if opts.Logger == nil {
		return nil, fmt.Errorf("logger is required, got nil")
	}
	if opts.Tracer == nil {
		return nil, fmt.Errorf("tracer is required, got nil")
	}
	if len(opts.JoinPeers) > 0 && opts.DiscoverPeers != "" {
		return nil, fmt.Errorf("at most one of join peers and discover peers may be set, "+
			"got join peers %q and discover peers %q", opts.JoinPeers, opts.DiscoverPeers)
	}

	switch {
	case len(opts.JoinPeers) > 0:
		level.Info(opts.Logger).Log("msg", "using provided peers for discovery", "join_peers", strings.Join(opts.JoinPeers, ", "))
		return newWithJoinPeers(opts), nil
	case opts.DiscoverPeers != "":
		// opts.DiscoverPeers is not logged to avoid leaking sensitive information.
		level.Info(opts.Logger).Log("msg", "using go-discovery to discover peers")
		return newWithGoDiscovery(opts)
	default:
		// Here, both JoinPeers and DiscoverPeers are empty. This is desirable when
		// starting a seed node that other nodes connect to, so we don't require
		// one of the fields to be set.
		level.Info(opts.Logger).Log("msg", "no peer discovery configured: both join and discover peers are empty")
		return nil, nil
	}
}

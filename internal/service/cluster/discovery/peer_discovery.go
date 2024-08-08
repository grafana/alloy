package discovery

import (
	"fmt"
	"net"

	"github.com/go-kit/log"
	godiscover "github.com/hashicorp/go-discover"
	"go.opentelemetry.io/otel/trace"
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
	// goDiscoverFactory is a function that can be used to create a new discover.Discover instance.
	// If nil, godiscover.New is used. Used for testing.
	goDiscoverFactory goDiscoverFactory
}

// lookupSRVFn is a function that can be used to lookup SRV records. Matches net.LookupSRV signature.
type lookupSRVFn func(service, proto, name string) (string, []*net.SRV, error)

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
	srvLookupFn := net.LookupSRV
	if opts.lookupSRVFn != nil {
		srvLookupFn = opts.lookupSRVFn
	}
	discoverFactory := godiscover.New
	if opts.goDiscoverFactory != nil {
		discoverFactory = opts.goDiscoverFactory
	}

	switch {
	case len(opts.JoinPeers) > 0:
		return newStaticDiscovery(opts.JoinPeers, opts.DefaultPort, opts.Logger, srvLookupFn), nil
	case opts.DiscoverPeers != "":
		return newDynamicDiscovery(opts.Logger, opts.DiscoverPeers, opts.DefaultPort, discoverFactory)
	default:
		// Here, both JoinPeers and DiscoverPeers are empty. This is desirable when
		// starting a seed node that other nodes connect to, so we don't require
		// one of the fields to be set.
		return nil, nil
	}
}

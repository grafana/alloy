package discovery

import (
	"fmt"
	"log/slog"
	"net"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

type DiscoverFn func() ([]string, error)

type Options struct {
	JoinPeers     []string
	DiscoverPeers string
	DefaultPort   int
	// Logger to surface extra information to the user. Required.
	Logger *slog.Logger
	// Tracer to emit spans. Required.
	Tracer trace.TracerProvider
	// lookupSRVFn is a function that can be used to lookup SRV records. If nil, net.LookupSRV is used. Used for testing.
	lookupSRVFn lookupSRVFn
	// lookupIPFn is a function that can be used to lookup addresses using A/AAAA DNS records. If nil, net.LookupIP is used. Used for testing.
	lookupIPFn lookupIPFn

	// goDiscoverFactory is an optional override used for testing. In the full
	// build it holds a func(...godiscover.Option) (*godiscover.Discover, error);
	// it is unused in slim builds. Typed as any to keep this file tag-agnostic.
	goDiscoverFactory any
}

// lookupSRVFn is a function that can be used to lookup SRV records. Matches net.LookupSRV signature.
type lookupSRVFn func(service, proto, name string) (string, []*net.SRV, error)

// lookupIPFn is a function that can be used to lookup IP addresses using A/AAAA DNS records. Matches net.LookupIP signature.
type lookupIPFn func(host string) ([]net.IP, error)

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
		opts.Logger.Info("using provided peers for discovery", "join_peers", strings.Join(opts.JoinPeers, ", "))
		return newWithJoinPeers(opts), nil
	case opts.DiscoverPeers != "":
		// opts.DiscoverPeers is not logged to avoid leaking sensitive information.
		opts.Logger.Info("using go-discovery to discover peers")
		return newWithGoDiscovery(opts)
	default:
		// Here, both JoinPeers and DiscoverPeers are empty. This is desirable when
		// starting a seed node that other nodes connect to, so we don't require
		// one of the fields to be set.
		opts.Logger.Info("no peer discovery configured: both join and discover peers are empty")
		return nil, nil
	}
}

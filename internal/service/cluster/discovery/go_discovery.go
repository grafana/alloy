//go:build !slim

package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"

	"github.com/hashicorp/go-discover"
	"github.com/hashicorp/go-discover/provider/k8s"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// goDiscoverFactory is a function that can be used to create a new discover.Discover instance.
// Matches discover.New signature.
type goDiscoverFactory func(opts ...discover.Option) (*discover.Discover, error)

// newWithGoDiscovery creates a new peer discovery function that uses the github.com/hashicorp/go-discover library to
// discover peer addresses that can be used for clustering.
func newWithGoDiscovery(opt Options) (DiscoverFn, error) {
	// Default to discover.New if no factory is provided.
	var factory goDiscoverFactory = discover.New
	if opt.goDiscoverFactory != nil {
		switch f := opt.goDiscoverFactory.(type) {
		case goDiscoverFactory:
			factory = f
		case func(opts ...discover.Option) (*discover.Discover, error):
			factory = goDiscoverFactory(f)
		default:
			return nil, fmt.Errorf("goDiscoverFactory has unexpected type %T", opt.goDiscoverFactory)
		}
	}

	providers := make(map[string]discover.Provider, len(discover.Providers)+1)
	for k, v := range discover.Providers {
		providers[k] = v
	}

	// Custom providers that aren't enabled by default
	providers["k8s"] = &k8s.Provider{}

	discoverer, err := factory(discover.WithProviders(providers))
	if err != nil {
		return nil, fmt.Errorf("bootstrapping peer discovery: %w", err)
	}

	return func() ([]string, error) {
		_, span := opt.Tracer.Tracer("").Start(
			context.Background(),
			"DiscoverClusterPeers",
			trace.WithSpanKind(trace.SpanKindInternal),
		)
		defer span.End()

		addrs, err := discoverer.Addrs(opt.DiscoverPeers, slog.NewLogLogger(opt.Logger.Handler(), slog.LevelDebug))
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("discovering peers: %w", err)
		}

		for i := range addrs {
			// Default to using the same advertise port as the local node.
			addrs[i] = appendPortIfAbsent(addrs[i], strconv.Itoa(opt.DefaultPort))
		}

		span.SetAttributes(attribute.Int("discovered_addresses_count", len(addrs)))
		span.SetStatus(codes.Ok, "discovered peers")
		return addrs, nil
	}, nil
}

func appendPortIfAbsent(addr string, port string) string {
	_, _, err := net.SplitHostPort(addr)
	if err == nil {
		// No error means there was a port in the string
		return addr
	}
	return net.JoinHostPort(addr, port)
}

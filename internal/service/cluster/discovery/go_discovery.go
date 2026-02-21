package discovery

import (
	"context"
	"fmt"
	stdlog "log"
	"maps"
	"net"
	"strconv"

	"github.com/go-kit/log"
	"github.com/hashicorp/go-discover"
	"github.com/hashicorp/go-discover/provider/k8s"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// newWithGoDiscovery creates a new peer discovery function that uses the github.com/hashicorp/go-discover library to
// discover peer addresses that can be used for clustering.
func newWithGoDiscovery(opt Options) (DiscoverFn, error) {
	// Default to discover.New if no factory is provided.
	factory := opt.goDiscoverFactory
	if factory == nil {
		factory = discover.New
	}

	providers := make(map[string]discover.Provider, len(discover.Providers)+1)
	maps.Copy(providers, discover.Providers)

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

		addrs, err := discoverer.Addrs(opt.DiscoverPeers, stdlog.New(log.NewStdlibAdapter(opt.Logger), "", 0))
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

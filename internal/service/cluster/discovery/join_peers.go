package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/go-kit/log"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// newWithJoinPeers creates a DiscoverFn that resolves the provided list of peers to a list of addresses that can be
// used for clustering. See docs/sources/reference/cli/run.md and the tests for more information.
func newWithJoinPeers(opts Options) DiscoverFn {
	return func() ([]string, error) {
		ctx, span := opts.Tracer.Tracer("").Start(
			context.Background(),
			"ResolveClusterJoinAddresses",
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(attribute.Int("join_peers_count", len(opts.JoinPeers))),
		)
		defer span.End()

		// Use these resolvers in order to resolve the provided addresses into a form that can be used by clustering.
		// NOTE: dnsSDURLResolver should be above other DNS resolvers.
		resolvers := []addressResolver{
			dnsSDURLResolver(opts, ctx),
			ipResolver(opts.Logger),
			dnsAResolver(opts, ctx),
			dnsSRVResolver(opts, ctx),
		}

		// Get the addresses.
		addresses, err := buildJoinAddresses(opts, resolvers)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("static peer discovery: %w", err)
		}

		// Return unique addresses.
		result := lo.Uniq(addresses)
		span.SetAttributes(attribute.Int("resolved_addresses_count", len(result)))
		span.SetStatus(codes.Ok, "resolved addresses")
		return result, nil
	}
}

func buildJoinAddresses(opts Options, resolvers []addressResolver) ([]string, error) {
	var (
		result      []string
		deferredErr error
	)

	for _, addr := range opts.JoinPeers {
		// See if we have a port override, if not use the default port.
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
			port = strconv.Itoa(opts.DefaultPort)
		}

		atLeastOneSuccess := false
		for _, resolver := range resolvers {
			resolved, err := resolver(host)
			deferredErr = errors.Join(deferredErr, err)
			for _, foundAddr := range resolved {
				result = append(result, net.JoinHostPort(foundAddr, port))
			}
			// we stop once we find a resolver that succeeded for given address
			if len(resolved) > 0 {
				atLeastOneSuccess = true
				break
			}
		}

		if !atLeastOneSuccess {
			// It is still useful to know if user provided an address that we could not resolve, even
			// if another addresses resolve successfully, and we don't return an error. To keep things simple, we're
			// not including more detail as it's available through debug level.
			level.Warn(opts.Logger).Log("msg", "failed to resolve provided join address", "addr", addr)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("failed to find any valid join addresses: %w", deferredErr)
	}
	return result, nil
}

type addressResolver func(addr string) ([]string, error)

func ipResolver(log log.Logger) addressResolver {
	return func(addr string) ([]string, error) {
		// Check if it's IP and use it if so.
		ip := net.ParseIP(addr)
		if ip == nil {
			return nil, fmt.Errorf("could not parse as an IP or IP:port address: %q", addr)
		}
		level.Debug(log).Log("msg", "found an IP cluster join address", "addr", addr)
		return []string{ip.String()}, nil
	}
}

func dnsAResolver(opts Options, ctx context.Context) addressResolver {
	// Default to net.LookupIP if not provided. By default, this will look up A/AAAA records.
	ipLookup := opts.lookupIPFn
	if ipLookup == nil {
		ipLookup = net.LookupIP
	}
	return dnsResolver(opts, ctx, "A/AAAA", func(addr string) ([]string, error) {
		ips, err := ipLookup(addr)
		result := make([]string, 0, len(ips))
		for _, ip := range ips {
			result = append(result, ip.String())
		}
		return result, err
	})
}

func dnsSRVResolver(opts Options, ctx context.Context) addressResolver {
	// Default to net.LookupSRV if not provided.
	srvLookup := opts.lookupSRVFn
	if srvLookup == nil {
		srvLookup = net.LookupSRV
	}

	return dnsResolver(opts, ctx, "SRV", func(addr string) ([]string, error) {
		_, addresses, err := srvLookup("", "", addr)
		result := make([]string, 0, len(addresses))
		for _, a := range addresses {
			result = append(result, a.Target)
		}
		return result, err
	})
}

const (
	dnsSchemeDNS       = "dns+"
	dnsSchemeDNSSRV    = "dnssrv+"
	dnsSchemeDNSSRVNOA = "dnssrvnoa+"
)

// dnsSDURLResolver handles DNS-SD URLs which explicitly states what DNS query should be used for host resolve.
//
// Example: `dnssrv+_memcached._tcp.memcached.namespace.svc.cluster.local`
//
// Resolver rejects any non-URL values.
func dnsSDURLResolver(opts Options, ctx context.Context) addressResolver {
	srvLookup := opts.lookupSRVFn
	if srvLookup == nil {
		srvLookup = net.LookupSRV
	}

	return func(addr string) ([]string, error) {
		var (
			nextAddr     string
			nextResolver addressResolver
		)

		switch {
		case strings.HasPrefix(addr, dnsSchemeDNS):
			nextAddr = addr[len(dnsSchemeDNS):]
			nextResolver = dnsAResolver(opts, ctx)
		case strings.HasPrefix(addr, dnsSchemeDNSSRV):
			nextAddr = addr[len(dnsSchemeDNSSRV):]
			nextResolver = dnsSRVResolver(opts, ctx)
		case strings.HasPrefix(addr, dnsSchemeDNSSRVNOA):
			nextAddr = addr[len(dnsSchemeDNSSRVNOA):]
			nextResolver = dnsResolver(opts, ctx, "SRVNOA", func(addr string) ([]string, error) {
				// NOTE: the only difference between SRVNOA and SRV, as SRV request should do N+1 query for A/AAAA.
				_, addresses, err := srvLookup("", "", addr)
				result := make([]string, 0, len(addresses))
				for _, a := range addresses {
					result = append(result, a.Target)
				}
				return result, err
			})

		default:
			// skip and pass control to a next resolver.
			return nil, nil
		}

		return nextResolver(nextAddr)
	}
}

func dnsResolver(opts Options, ctx context.Context, recordType string, dnsLookupFn func(string) ([]string, error)) addressResolver {
	return func(addr string) ([]string, error) {
		_, span := opts.Tracer.Tracer("").Start(
			ctx,
			"ClusterPeersDNSLookup",
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(attribute.String("addr", addr)),
			trace.WithAttributes(attribute.String("record_type", recordType)),
		)
		defer span.End()

		result, err := dnsLookupFn(addr)
		if err != nil {
			level.Debug(opts.Logger).Log("msg", "failed to resolve DNS records", "addr", addr, "record_type", recordType, "err", err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("failed to resolve %q records: %w", recordType, err)
		}

		level.Debug(opts.Logger).Log("msg", "received DNS query response", "addr", addr, "record_type", recordType, "records_count", len(result))
		span.SetAttributes(attribute.Int("resolved_addresses_count", len(result)))
		span.SetStatus(codes.Ok, "resolved addresses")
		return result, nil
	}
}

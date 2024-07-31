package discovery

import (
	"errors"
	"fmt"
	"net"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// newWithJoinPeers creates a DiscoverFn that resolves the provided list of peers to a list of addresses that can be
// used for clustering. The peers can be given, for example, via a host:port pair, an IP address or a DNS name.
// TODO(thampiotr): describe the behaviour in more detail
func newWithJoinPeers(providedAddr []string, defaultPort int, log log.Logger, srvLookup lookupSRVFn) DiscoverFn {
	return func() ([]string, error) {
		// Use these resolvers in order to resolve the provided addresses into a form that can be used by clustering.
		resolvers := []addressResolver{
			hostAndPortResolver(log),
			ipResolver(log),
			dnsResolver(srvLookup, log),
		}

		// Get the addresses.
		addresses, err := buildJoinAddresses(providedAddr, resolvers)
		if err != nil {
			return nil, fmt.Errorf("static peer discovery: %w", err)
		}

		// Normalize the addresses by appending the default port. It's important that all nodes in the cluster use
		// the same default port.
		for i := range addresses {
			addresses[i] = appendDefaultPort(addresses[i], defaultPort)
		}
		return addresses, nil
	}
}

func buildJoinAddresses(providedAddr []string, resolvers []addressResolver) ([]string, error) {
	var (
		result      []string
		deferredErr error
	)

	for ind, resolver := range resolvers {
		for _, addr := range providedAddr {
			resolved, err := resolver(addr)
			result = append(result, resolved...)
			deferredErr = errors.Join(deferredErr, err)
			//TODO(thampiotr): this is a temporary ugly code just to match the existing tests when refactoring, so that
			// we can avoid changing both tests and implementation at the same time. This behaviour will be changed in
			// the subsequent commits.
			if len(resolved) > 0 && ind != len(resolvers)-1 {
				break
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("failed to find any valid join addresses: %w", deferredErr)
	}
	return result, nil
}

type addressResolver func(addr string) ([]string, error)

func hostAndPortResolver(log log.Logger) addressResolver {
	return func(addr string) ([]string, error) {
		_, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("failed to extract host and port: %w", err)
		}
		level.Debug(log).Log("msg", "found a host:port cluster join address", "addr", addr)
		return []string{addr}, nil
	}
}

func ipResolver(log log.Logger) addressResolver {
	return func(addr string) ([]string, error) {
		ip := net.ParseIP(addr)
		if ip == nil {
			return nil, nil
		}
		level.Debug(log).Log("msg", "found an IP cluster join address", "addr", addr)
		return []string{addr}, nil
	}
}

func dnsResolver(srvLookup lookupSRVFn, log log.Logger) addressResolver {
	return func(addr string) ([]string, error) {
		_, srvAddresses, err := srvLookup("", "", addr)
		if err != nil {
			level.Warn(log).Log("msg", "failed to resolve SRV records", "addr", addr, "err", err)
			return nil, fmt.Errorf("failed to resolve SRV records: %w", err)
		}
		level.Debug(log).Log("msg", "found cluster join addresses via SRV records", "addr", addr, "count", len(srvAddresses))
		var result []string
		for _, srv := range srvAddresses {
			result = append(result, srv.Target)
		}
		return result, nil
	}
}

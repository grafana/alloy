package discovery

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/go-kit/log"
	"github.com/samber/lo"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// newWithJoinPeers creates a DiscoverFn that resolves the provided list of peers to a list of addresses that can be
// used for clustering. The peers can be given in the following formats:
//   - IP address, e.g. 10.10.10.10, in which case the default port is used
//   - IP address with port, e.g. 10.10.10.10:12345, in which case the provided port is used
//   - Hostname, e.g. example.com - in this case the SRV records are looked up for the hostname and all the resolved
//     IP addresses are used with the default port.
//   - Hostname with port, e.g. example.com:12345 - in this case the SRV records are looked up for the hostname and all
//     the resolved IP addresses will be used with the provided port.
//
// TODO(thampiotr): Update this after adding A record support and probably move this to documentation.
func newWithJoinPeers(opts Options) DiscoverFn {
	return func() ([]string, error) {
		// Use these resolvers in order to resolve the provided addresses into a form that can be used by clustering.
		resolvers := []addressResolver{
			ipResolver(opts.Logger),
			dnsSRVResolver(opts.lookupSRVFn, opts.Logger),
		}

		// Get the addresses.
		addresses, err := buildJoinAddresses(opts.JoinPeers, resolvers, strconv.Itoa(opts.DefaultPort))
		if err != nil {
			return nil, fmt.Errorf("static peer discovery: %w", err)
		}

		// Return unique addresses.
		return lo.Uniq(addresses), nil
	}
}

func buildJoinAddresses(providedAddr []string, resolvers []addressResolver, defaultPort string) ([]string, error) {
	var (
		result      []string
		deferredErr error
	)

	for _, addr := range providedAddr {
		// See if we have a port override, if not use the default port.
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
			port = defaultPort
		}

		for _, resolver := range resolvers {
			resolved, err := resolver(host)
			deferredErr = errors.Join(deferredErr, err)
			for _, foundAddr := range resolved {
				result = append(result, appendPortIfAbsent(foundAddr, port))
			}
			// we stop once we find a resolver that succeeded for given address
			if len(resolved) > 0 {
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

func dnsSRVResolver(srvLookup lookupSRVFn, log log.Logger) addressResolver {
	// Default to net.LookupSRV if not provided.
	if srvLookup == nil {
		srvLookup = net.LookupSRV
	}
	return func(addr string) ([]string, error) {
		// Do SRV lookup.
		_, srvAddresses, err := srvLookup("", "", addr)
		if err != nil {
			level.Warn(log).Log("msg", "failed to resolve SRV records", "addr", addr, "err", err)
			return nil, fmt.Errorf("failed to resolve SRV records: %w", err)
		}

		// Use all the addresses found
		level.Debug(log).Log("msg", "found cluster join addresses via SRV records", "addr", addr, "count", len(srvAddresses))
		var result []string
		for _, srv := range srvAddresses {
			result = append(result, srv.Target)
		}

		return result, nil
	}
}

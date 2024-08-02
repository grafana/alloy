package discovery

import (
	"errors"
	"fmt"
	"net"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func newStaticDiscovery(providedAddr []string, defaultPort int, log log.Logger, srvLookup lookupSRVFn) DiscoverFn {
	return func() ([]string, error) {
		addresses, err := buildJoinAddresses(providedAddr, log, srvLookup)
		if err != nil {
			return nil, fmt.Errorf("static peer discovery: %w", err)
		}
		for i := range addresses {
			// Default to using the same advertise port as the local node. This may
			// break in some cases, so the user should make sure the port numbers
			// align on as many nodes as possible.
			addresses[i] = appendDefaultPort(addresses[i], defaultPort)
		}
		return addresses, nil
	}
}

func buildJoinAddresses(providedAddr []string, log log.Logger, srvLookup lookupSRVFn) ([]string, error) {
	// Currently we don't consider it an error to not have any join addresses.
	if len(providedAddr) == 0 {
		return nil, nil
	}
	var (
		result      []string
		deferredErr error
	)
	for _, addr := range providedAddr {
		// If it's a host:port, use it as is.
		_, _, err := net.SplitHostPort(addr)
		if err != nil {
			deferredErr = errors.Join(deferredErr, fmt.Errorf("failed to extract host and port: %w", err))
		} else {
			level.Debug(log).Log("msg", "found a host:port cluster join address", "addr", addr)
			result = append(result, addr)
			break
		}

		// If it's an IP address, use it.
		ip := net.ParseIP(addr)
		if ip != nil {
			level.Debug(log).Log("msg", "found an IP cluster join address", "addr", addr)
			result = append(result, ip.String())
			break
		}

		// Otherwise, do a DNS lookup and return all the records found.
		_, srvs, err := srvLookup("", "", addr)
		if err != nil {
			level.Warn(log).Log("msg", "failed to resolve SRV records", "addr", addr, "err", err)
			deferredErr = errors.Join(deferredErr, fmt.Errorf("failed to resolve SRV records: %w", err))
		} else {
			level.Debug(log).Log("msg", "found cluster join addresses via SRV records", "addr", addr, "count", len(srvs))
			for _, srv := range srvs {
				result = append(result, srv.Target)
			}
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("failed to find any valid join addresses: %w", deferredErr)
	}
	return result, nil
}

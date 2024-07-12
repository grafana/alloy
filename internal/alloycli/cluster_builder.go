package alloycli

import (
	"errors"
	"fmt"
	stdlog "log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/advertise"
	"github.com/hashicorp/go-discover"
	"github.com/hashicorp/go-discover/provider/k8s"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"

	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/cluster"
)

type clusterOptions struct {
	Log     log.Logger
	Metrics prometheus.Registerer
	Tracer  trace.TracerProvider

	EnableClustering          bool
	NodeName                  string
	AdvertiseAddress          string
	ListenAddress             string
	JoinPeers                 []string
	DiscoverPeers             string
	RejoinInterval            time.Duration
	AdvertiseInterfaces       []string
	ClusterMaxJoinPeers       int
	ClusterName               string
	EnableStateUpdatesLimiter bool
}

func buildClusterService(opts clusterOptions) (*cluster.Service, error) {
	listenPort := findPort(opts.ListenAddress, 80)

	config := cluster.Options{
		Log:     opts.Log,
		Metrics: opts.Metrics,
		Tracer:  opts.Tracer,

		EnableClustering:          opts.EnableClustering,
		NodeName:                  opts.NodeName,
		RejoinInterval:            opts.RejoinInterval,
		ClusterMaxJoinPeers:       opts.ClusterMaxJoinPeers,
		ClusterName:               opts.ClusterName,
		EnableStateUpdatesLimiter: opts.EnableStateUpdatesLimiter,
	}

	if config.NodeName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("generating node name: %w", err)
		}
		config.NodeName = hostname
	}

	var err error
	config.AdvertiseAddress, err = getAdvertiseAddress(opts, listenPort)
	if err != nil {
		return nil, err
	}

	switch {
	case len(opts.JoinPeers) > 0 && opts.DiscoverPeers != "":
		return nil, fmt.Errorf("at most one of join peers and discover peers may be set")

	case len(opts.JoinPeers) > 0:
		config.DiscoverPeers = newStaticDiscovery(opts.JoinPeers, listenPort, opts.Log)

	case opts.DiscoverPeers != "":
		discoverFunc, err := newDynamicDiscovery(config.Log, opts.DiscoverPeers, listenPort)
		if err != nil {
			return nil, err
		}
		config.DiscoverPeers = discoverFunc

	default:
		// Here, both JoinPeers and DiscoverPeers are empty. This is desirable when
		// starting a seed node that other nodes connect to, so we don't require
		// one of the fields to be set.
	}

	return cluster.New(config)
}

func useAllInterfaces(interfaces []string) bool {
	return len(interfaces) == 1 && interfaces[0] == "all"
}

func getAdvertiseAddress(opts clusterOptions, listenPort int) (string, error) {
	if opts.AdvertiseAddress != "" {
		return appendDefaultPort(opts.AdvertiseAddress, listenPort), nil
	}
	advertiseAddress := net.JoinHostPort("127.0.0.1", strconv.Itoa(listenPort))
	if opts.EnableClustering {
		advertiseInterfaces := opts.AdvertiseInterfaces
		if useAllInterfaces(advertiseInterfaces) {
			advertiseInterfaces = nil
		}
		addr, err := advertise.FirstAddress(advertiseInterfaces)
		if err != nil {
			level.Warn(opts.Log).Log("msg", "could not find advertise address using network interfaces", opts.AdvertiseInterfaces,
				"falling back to localhost", "err", err)
		} else if !addr.Is4() && !addr.Is6() {
			return "", fmt.Errorf("type unknown for address: %s", addr.String())
		} else {
			advertiseAddress = net.JoinHostPort(addr.String(), strconv.Itoa(listenPort))
		}
	}
	return advertiseAddress, nil
}

func findPort(addr string, defaultPort int) int {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return defaultPort
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return defaultPort
	}
	return port
}

func appendDefaultPort(addr string, port int) string {
	_, _, err := net.SplitHostPort(addr)
	if err == nil {
		// No error means there was a port in the string
		return addr
	}
	return net.JoinHostPort(addr, strconv.Itoa(port))
}

type discoverFunc func() ([]string, error)

func newStaticDiscovery(providedAddr []string, defaultPort int, log log.Logger) discoverFunc {
	return func() ([]string, error) {
		addresses, err := buildJoinAddresses(providedAddr, log)
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

func buildJoinAddresses(providedAddr []string, log log.Logger) ([]string, error) {
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
			deferredErr = errors.Join(deferredErr, err)
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
		_, srvs, err := net.LookupSRV("", "", addr)
		if err != nil {
			level.Warn(log).Log("msg", "failed to resolve SRV records", "addr", addr, "err", err)
			deferredErr = errors.Join(deferredErr, err)
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

func newDynamicDiscovery(l log.Logger, config string, defaultPort int) (discoverFunc, error) {
	providers := make(map[string]discover.Provider, len(discover.Providers)+1)
	for k, v := range discover.Providers {
		providers[k] = v
	}

	// Custom providers that aren't enabled by default
	providers["k8s"] = &k8s.Provider{}

	discoverer, err := discover.New(discover.WithProviders(providers))
	if err != nil {
		return nil, fmt.Errorf("bootstrapping peer discovery: %w", err)
	}

	return func() ([]string, error) {
		addrs, err := discoverer.Addrs(config, stdlog.New(log.NewStdlibAdapter(l), "", 0))
		if err != nil {
			return nil, fmt.Errorf("discovering peers: %w", err)
		}

		for i := range addrs {
			// Default to using the same advertise port as the local node. This may
			// break in some cases, so the user should make sure the port numbers
			// align on as many nodes as possible.
			addrs[i] = appendDefaultPort(addrs[i], defaultPort)
		}

		return addrs, nil
	}, nil
}

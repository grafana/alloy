package alloycli

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/advertise"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"

	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/cluster/discovery"
)

type ClusterOptions struct {
	Log     log.Logger
	Metrics prometheus.Registerer
	Tracer  trace.TracerProvider

	EnableClustering       bool
	MinimumClusterSize     int
	MinimumSizeWaitTimeout time.Duration
	NodeName               string
	AdvertiseAddress       string
	ListenAddress          string
	JoinPeers              []string
	DiscoverPeers          string
	RejoinInterval         time.Duration
	AdvertiseInterfaces    []string
	ClusterMaxJoinPeers    int
	ClusterName            string
	EnableTLS              bool
	TLSCAPath              string
	TLSCertPath            string
	TLSKeyPath             string
	TLSServerName          string
}

func buildClusterService(opts ClusterOptions) (*cluster.Service, error) {
	return NewClusterService(opts, discovery.NewPeerDiscoveryFn)
}

// NewClusterService is visible to make it easier to test clustering e2e.
func NewClusterService(
	opts ClusterOptions,
	getDiscoveryFn func(options discovery.Options) (discovery.DiscoverFn, error),
) (*cluster.Service, error) {

	listenPort := findPort(opts.ListenAddress, 80)

	config := cluster.Options{
		Log:     opts.Log,
		Metrics: opts.Metrics,
		Tracer:  opts.Tracer,

		EnableClustering:       opts.EnableClustering,
		MinimumClusterSize:     opts.MinimumClusterSize,
		MinimumSizeWaitTimeout: opts.MinimumSizeWaitTimeout,
		NodeName:               opts.NodeName,
		RejoinInterval:         opts.RejoinInterval,
		ClusterMaxJoinPeers:    opts.ClusterMaxJoinPeers,
		ClusterName:            opts.ClusterName,
		EnableTLS:              opts.EnableTLS,
		TLSCAPath:              opts.TLSCAPath,
		TLSCertPath:            opts.TLSCertPath,
		TLSKeyPath:             opts.TLSKeyPath,
		TLSServerName:          opts.TLSServerName,
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

	config.DiscoverPeers, err = getDiscoveryFn(discovery.Options{
		JoinPeers:     opts.JoinPeers,
		DiscoverPeers: opts.DiscoverPeers,
		DefaultPort:   listenPort,
		Logger:        opts.Log,
		Tracer:        opts.Tracer,
	})
	if err != nil {
		return nil, err
	}
	return cluster.New(config)
}

func useAllInterfaces(interfaces []string) bool {
	return len(interfaces) == 1 && interfaces[0] == "all"
}

func getAdvertiseAddress(opts ClusterOptions, listenPort int) (string, error) {
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

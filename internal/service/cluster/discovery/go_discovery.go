package discovery

import (
	"fmt"
	stdlog "log"
	"strconv"

	"github.com/go-kit/log"
	"github.com/hashicorp/go-discover"
	"github.com/hashicorp/go-discover/provider/k8s"
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
		addrs, err := discoverer.Addrs(opt.DiscoverPeers, stdlog.New(log.NewStdlibAdapter(opt.Logger), "", 0))
		if err != nil {
			return nil, fmt.Errorf("discovering peers: %w", err)
		}

		for i := range addrs {
			// Default to using the same advertise port as the local node.
			addrs[i] = appendPortIfAbsent(addrs[i], strconv.Itoa(opt.DefaultPort))
		}

		return addrs, nil
	}, nil
}

//go:build !windows

package dns

import (
	"github.com/prometheus/common/model"
	promdns "github.com/prometheus/prometheus/discovery/dns"

	"github.com/grafana/alloy/internal/component/discovery"
)

func newDiscovererConfig(args Arguments) discovery.DiscovererConfig {
	return &promdns.SDConfig{
		Names:           args.Names,
		RefreshInterval: model.Duration(args.RefreshInterval),
		Type:            args.Type,
		Port:            args.Port,
	}
}

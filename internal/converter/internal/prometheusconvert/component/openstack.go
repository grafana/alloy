package component

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/discovery/openstack"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert/build"
	"github.com/grafana/river/rivertypes"
	prom_openstack "github.com/prometheus/prometheus/discovery/openstack"
)

func appendDiscoveryOpenstack(pb *build.PrometheusBlocks, label string, sdConfig *prom_openstack.SDConfig) discovery.Exports {
	discoveryOpenstackArgs := toDiscoveryOpenstack(sdConfig)
	name := []string{"discovery", "openstack"}
	block := common.NewBlockWithOverride(name, label, discoveryOpenstackArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.openstack." + label + ".targets")
}

func ValidateDiscoveryOpenstack(sdConfig *prom_openstack.SDConfig) diag.Diagnostics {
	return nil
}

func toDiscoveryOpenstack(sdConfig *prom_openstack.SDConfig) *openstack.Arguments {
	if sdConfig == nil {
		return nil
	}

	return &openstack.Arguments{
		IdentityEndpoint:            sdConfig.IdentityEndpoint,
		Username:                    sdConfig.Username,
		UserID:                      sdConfig.UserID,
		Password:                    rivertypes.Secret(sdConfig.Password),
		ProjectName:                 sdConfig.ProjectName,
		ProjectID:                   sdConfig.ProjectID,
		DomainName:                  sdConfig.DomainName,
		DomainID:                    sdConfig.DomainID,
		ApplicationCredentialName:   sdConfig.ApplicationCredentialName,
		ApplicationCredentialID:     sdConfig.ApplicationCredentialID,
		ApplicationCredentialSecret: rivertypes.Secret(sdConfig.ApplicationCredentialSecret),
		Role:                        string(sdConfig.Role),
		Region:                      sdConfig.Region,
		RefreshInterval:             time.Duration(sdConfig.RefreshInterval),
		Port:                        sdConfig.Port,
		AllTenants:                  sdConfig.AllTenants,
		TLSConfig:                   *common.ToTLSConfig(&sdConfig.TLSConfig),
		Availability:                sdConfig.Availability,
	}
}

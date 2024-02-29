package component

import (
	"fmt"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert/build"

	prom_discover "github.com/prometheus/prometheus/discovery"
	prom_http "github.com/prometheus/prometheus/discovery/http"
	_ "github.com/prometheus/prometheus/discovery/install" // Register Prometheus SDs

	prom_aws "github.com/prometheus/prometheus/discovery/aws"
	prom_azure "github.com/prometheus/prometheus/discovery/azure"
	prom_consul "github.com/prometheus/prometheus/discovery/consul"
	prom_digitalocean "github.com/prometheus/prometheus/discovery/digitalocean"
	prom_dns "github.com/prometheus/prometheus/discovery/dns"
	prom_file "github.com/prometheus/prometheus/discovery/file"
	prom_gce "github.com/prometheus/prometheus/discovery/gce"
	prom_ionos "github.com/prometheus/prometheus/discovery/ionos"
	prom_kubernetes "github.com/prometheus/prometheus/discovery/kubernetes"
	prom_linode "github.com/prometheus/prometheus/discovery/linode"
	prom_marathon "github.com/prometheus/prometheus/discovery/marathon"
	prom_docker "github.com/prometheus/prometheus/discovery/moby"
	prom_openstack "github.com/prometheus/prometheus/discovery/openstack"
	prom_ovhcloud "github.com/prometheus/prometheus/discovery/ovhcloud"
	prom_scaleway "github.com/prometheus/prometheus/discovery/scaleway"
	prom_triton "github.com/prometheus/prometheus/discovery/triton"
	prom_xds "github.com/prometheus/prometheus/discovery/xds"
	prom_zk "github.com/prometheus/prometheus/discovery/zookeeper"
)

func AppendServiceDiscoveryConfig(pb *build.PrometheusBlocks, serviceDiscoveryConfig prom_discover.Config, label string, labelCounts map[string]int) discovery.Exports {
	switch sdc := serviceDiscoveryConfig.(type) {
	case prom_discover.StaticConfig:
		return discovery.Exports{
			Targets: getScrapeTargets(sdc),
		}
	case *prom_azure.SDConfig:
		labelCounts["azure"]++
		return appendDiscoveryAzure(pb, common.LabelWithIndex(labelCounts["azure"]-1, label), sdc)
	case *prom_consul.SDConfig:
		labelCounts["consul"]++
		return appendDiscoveryConsul(pb, common.LabelWithIndex(labelCounts["consul"]-1, label), sdc)
	case *prom_digitalocean.SDConfig:
		labelCounts["digitalocean"]++
		return appendDiscoveryDigitalOcean(pb, common.LabelWithIndex(labelCounts["digitalocean"]-1, label), sdc)
	case *prom_dns.SDConfig:
		labelCounts["dns"]++
		return appendDiscoveryDns(pb, common.LabelWithIndex(labelCounts["dns"]-1, label), sdc)
	case *prom_docker.DockerSDConfig:
		labelCounts["docker"]++
		return appendDiscoveryDocker(pb, common.LabelWithIndex(labelCounts["docker"]-1, label), sdc)
	case *prom_aws.EC2SDConfig:
		labelCounts["ec2"]++
		return appendDiscoveryEC2(pb, common.LabelWithIndex(labelCounts["ec2"]-1, label), sdc)
	case *prom_file.SDConfig:
		labelCounts["file"]++
		return appendDiscoveryFile(pb, common.LabelWithIndex(labelCounts["file"]-1, label), sdc)
	case *prom_gce.SDConfig:
		labelCounts["gce"]++
		return appendDiscoveryGCE(pb, common.LabelWithIndex(labelCounts["gce"]-1, label), sdc)
	case *prom_http.SDConfig:
		labelCounts["http"]++
		return appendDiscoveryHttp(pb, common.LabelWithIndex(labelCounts["http"]-1, label), sdc)
	case *prom_kubernetes.SDConfig:
		labelCounts["kubernetes"]++
		return appendDiscoveryKubernetes(pb, common.LabelWithIndex(labelCounts["kubernetes"]-1, label), sdc)
	case *prom_aws.LightsailSDConfig:
		labelCounts["lightsail"]++
		return appendDiscoveryLightsail(pb, common.LabelWithIndex(labelCounts["lightsail"]-1, label), sdc)
	case *prom_marathon.SDConfig:
		labelCounts["marathon"]++
		return appendDiscoveryMarathon(pb, common.LabelWithIndex(labelCounts["marathon"]-1, label), sdc)
	case *prom_ionos.SDConfig:
		labelCounts["ionos"]++
		return appendDiscoveryIonos(pb, common.LabelWithIndex(labelCounts["ionos"]-1, label), sdc)
	case *prom_triton.SDConfig:
		labelCounts["triton"]++
		return appendDiscoveryTriton(pb, common.LabelWithIndex(labelCounts["triton"]-1, label), sdc)
	case *prom_xds.SDConfig:
		labelCounts["kuma"]++
		return appendDiscoveryKuma(pb, common.LabelWithIndex(labelCounts["kuma"]-1, label), sdc)
	case *prom_scaleway.SDConfig:
		labelCounts["scaleway"]++
		return appendDiscoveryScaleway(pb, common.LabelWithIndex(labelCounts["scaleway"]-1, label), sdc)
	case *prom_zk.ServersetSDConfig:
		labelCounts["serverset"]++
		return appendDiscoveryServerset(pb, common.LabelWithIndex(labelCounts["serverset"]-1, label), sdc)
	case *prom_linode.SDConfig:
		labelCounts["linode"]++
		return appendDiscoveryLinode(pb, common.LabelWithIndex(labelCounts["linode"]-1, label), sdc)
	case *prom_zk.NerveSDConfig:
		labelCounts["nerve"]++
		return appendDiscoveryNerve(pb, common.LabelWithIndex(labelCounts["nerve"]-1, label), sdc)
	case *prom_openstack.SDConfig:
		labelCounts["openstack"]++
		return appendDiscoveryOpenstack(pb, common.LabelWithIndex(labelCounts["openstack"]-1, label), sdc)
	case *prom_docker.DockerSwarmSDConfig:
		labelCounts["dockerswarm"]++
		return appendDiscoveryDockerswarm(pb, common.LabelWithIndex(labelCounts["dockerswarm"]-1, label), sdc)
	case *prom_ovhcloud.SDConfig:
		labelCounts["ovhcloud"]++
		return appendDiscoveryOvhcloud(pb, common.LabelWithIndex(labelCounts["ovhcloud"]-1, label), sdc)
	default:
		return discovery.Exports{}
	}
}

func ValidateServiceDiscoveryConfig(serviceDiscoveryConfig prom_discover.Config) diag.Diagnostics {
	switch sdc := serviceDiscoveryConfig.(type) {
	case prom_discover.StaticConfig:
		return ValidateScrapeTargets(sdc)
	case *prom_azure.SDConfig:
		return ValidateDiscoveryAzure(sdc)
	case *prom_consul.SDConfig:
		return ValidateDiscoveryConsul(sdc)
	case *prom_digitalocean.SDConfig:
		return ValidateDiscoveryDigitalOcean(sdc)
	case *prom_dns.SDConfig:
		return ValidateDiscoveryDns(sdc)
	case *prom_docker.DockerSDConfig:
		return ValidateDiscoveryDocker(sdc)
	case *prom_aws.EC2SDConfig:
		return ValidateDiscoveryEC2(sdc)
	case *prom_file.SDConfig:
		return ValidateDiscoveryFile(sdc)
	case *prom_gce.SDConfig:
		return ValidateDiscoveryGCE(sdc)
	case *prom_http.SDConfig:
		return ValidateDiscoveryHttp(sdc)
	case *prom_kubernetes.SDConfig:
		return ValidateDiscoveryKubernetes(sdc)
	case *prom_aws.LightsailSDConfig:
		return ValidateDiscoveryLightsail(sdc)
	case *prom_xds.SDConfig:
		return ValidateDiscoveryKuma(sdc)
	case *prom_linode.SDConfig:
		return ValidateDiscoveryLinode(sdc)
	case *prom_triton.SDConfig:
		return ValidateDiscoveryTriton(sdc)
	case *prom_scaleway.SDConfig:
		return ValidateDiscoveryScaleway(sdc)
	case *prom_marathon.SDConfig:
		return ValidateDiscoveryMarathon(sdc)
	case *prom_ionos.SDConfig:
		return ValidateDiscoveryIonos(sdc)
	case *prom_zk.ServersetSDConfig:
		return ValidateDiscoveryServerset(sdc)
	case *prom_zk.NerveSDConfig:
		return ValidateDiscoveryNerve(sdc)
	case *prom_openstack.SDConfig:
		return ValidateDiscoveryOpenstack(sdc)
	case *prom_docker.DockerSwarmSDConfig:
		return ValidateDiscoveryDockerswarm(sdc)
	case *prom_ovhcloud.SDConfig:
		return ValidateDiscoveryOvhcloud(sdc)
	default:
		var diags diag.Diagnostics
		diags.Add(diag.SeverityLevelError, fmt.Sprintf("The converter does not support converting the provided %s service discovery.", serviceDiscoveryConfig.Name()))
		return diags
	}
}

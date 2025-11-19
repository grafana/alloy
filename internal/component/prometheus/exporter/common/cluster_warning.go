package common

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/cluster"
)

func WarningIfUsedInCluster(o component.Options) {
	data, err := o.GetServiceData(cluster.ServiceName)
	if err != nil { // this should never happen as all Alloy instances have clustering service.
		level.Warn(o.Logger).Log("msg", "error getting clustering service data", "err", err)
		return
	}

	clusterData := data.(cluster.Cluster)
	if clusterData == nil { // this should also never happen, but adding a check just in case
		level.Warn(o.Logger).Log("msg", "cluster data is nil", "component", o.ID)
		return
	}

	if !clusterData.Ready() || len(clusterData.Peers()) > 0 {
		level.Warn(o.Logger).Log(
			"msg",
			"detected clustering is configured while using a host-specific exporter - please make sure your configuration is correct",
			"exporter",
			o.ID,
			"details",
			"The default instance label set by this exporter is the hostname of the machine running Alloy. Alloy clustering uses consistent hashing to distribute targets across the instances. This approach requires the discovered targets to be the same and have the same labels across all cluster instances. Please make sure you correctly set the instance label for this exporter or use a prometheus.scrape with disabled clustering. Alternatively, you can move this pipeline to a different Alloy deployment that does not use clustering.",
		)
	}
}

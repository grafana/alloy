// Package vcenter provides an otelcol.receiver.vcenter component.
package vcenter

import (
	"fmt"
	"net/url"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/receiver"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river/rivertypes"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.vcenter",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := vcenterreceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

type MetricConfig struct {
	Enabled bool `river:"enabled,attr"`
}

func (r *MetricConfig) Convert() map[string]interface{} {
	if r == nil {
		return nil
	}

	return map[string]interface{}{
		"enabled": r.Enabled,
	}
}

type MetricsConfig struct {
	VcenterClusterCPUEffective      MetricConfig `river:"vcenter.cluster.cpu.effective,block,optional"`
	VcenterClusterCPULimit          MetricConfig `river:"vcenter.cluster.cpu.limit,block,optional"`
	VcenterClusterHostCount         MetricConfig `river:"vcenter.cluster.host.count,block,optional"`
	VcenterClusterMemoryEffective   MetricConfig `river:"vcenter.cluster.memory.effective,block,optional"`
	VcenterClusterMemoryLimit       MetricConfig `river:"vcenter.cluster.memory.limit,block,optional"`
	VcenterClusterMemoryUsed        MetricConfig `river:"vcenter.cluster.memory.used,block,optional"`
	VcenterClusterVMCount           MetricConfig `river:"vcenter.cluster.vm.count,block,optional"`
	VcenterDatastoreDiskUsage       MetricConfig `river:"vcenter.datastore.disk.usage,block,optional"`
	VcenterDatastoreDiskUtilization MetricConfig `river:"vcenter.datastore.disk.utilization,block,optional"`
	VcenterHostCPUUsage             MetricConfig `river:"vcenter.host.cpu.usage,block,optional"`
	VcenterHostCPUUtilization       MetricConfig `river:"vcenter.host.cpu.utilization,block,optional"`
	VcenterHostDiskLatencyAvg       MetricConfig `river:"vcenter.host.disk.latency.avg,block,optional"`
	VcenterHostDiskLatencyMax       MetricConfig `river:"vcenter.host.disk.latency.max,block,optional"`
	VcenterHostDiskThroughput       MetricConfig `river:"vcenter.host.disk.throughput,block,optional"`
	VcenterHostMemoryUsage          MetricConfig `river:"vcenter.host.memory.usage,block,optional"`
	VcenterHostMemoryUtilization    MetricConfig `river:"vcenter.host.memory.utilization,block,optional"`
	VcenterHostNetworkPacketCount   MetricConfig `river:"vcenter.host.network.packet.count,block,optional"`
	VcenterHostNetworkPacketErrors  MetricConfig `river:"vcenter.host.network.packet.errors,block,optional"`
	VcenterHostNetworkThroughput    MetricConfig `river:"vcenter.host.network.throughput,block,optional"`
	VcenterHostNetworkUsage         MetricConfig `river:"vcenter.host.network.usage,block,optional"`
	VcenterResourcePoolCPUShares    MetricConfig `river:"vcenter.resource_pool.cpu.shares,block,optional"`
	VcenterResourcePoolCPUUsage     MetricConfig `river:"vcenter.resource_pool.cpu.usage,block,optional"`
	VcenterResourcePoolMemoryShares MetricConfig `river:"vcenter.resource_pool.memory.shares,block,optional"`
	VcenterResourcePoolMemoryUsage  MetricConfig `river:"vcenter.resource_pool.memory.usage,block,optional"`
	VcenterVMCPUUsage               MetricConfig `river:"vcenter.vm.cpu.usage,block,optional"`
	VcenterVMCPUUtilization         MetricConfig `river:"vcenter.vm.cpu.utilization,block,optional"`
	VcenterVMDiskLatencyAvg         MetricConfig `river:"vcenter.vm.disk.latency.avg,block,optional"`
	VcenterVMDiskLatencyMax         MetricConfig `river:"vcenter.vm.disk.latency.max,block,optional"`
	VcenterVMDiskThroughput         MetricConfig `river:"vcenter.vm.disk.throughput,block,optional"`
	VcenterVMDiskUsage              MetricConfig `river:"vcenter.vm.disk.usage,block,optional"`
	VcenterVMDiskUtilization        MetricConfig `river:"vcenter.vm.disk.utilization,block,optional"`
	VcenterVMMemoryBallooned        MetricConfig `river:"vcenter.vm.memory.ballooned,block,optional"`
	VcenterVMMemorySwapped          MetricConfig `river:"vcenter.vm.memory.swapped,block,optional"`
	VcenterVMMemorySwappedSsd       MetricConfig `river:"vcenter.vm.memory.swapped_ssd,block,optional"`
	VcenterVMMemoryUsage            MetricConfig `river:"vcenter.vm.memory.usage,block,optional"`
	VcenterVMMemoryUtilization      MetricConfig `river:"vcenter.vm.memory.utilization,block,optional"`
	VcenterVMNetworkPacketCount     MetricConfig `river:"vcenter.vm.network.packet.count,block,optional"`
	VcenterVMNetworkThroughput      MetricConfig `river:"vcenter.vm.network.throughput,block,optional"`
	VcenterVMNetworkUsage           MetricConfig `river:"vcenter.vm.network.usage,block,optional"`
}

func (args *MetricsConfig) SetToDefault() {
	*args = MetricsConfig{
		VcenterClusterCPUEffective:      MetricConfig{Enabled: true},
		VcenterClusterCPULimit:          MetricConfig{Enabled: true},
		VcenterClusterHostCount:         MetricConfig{Enabled: true},
		VcenterClusterMemoryEffective:   MetricConfig{Enabled: true},
		VcenterClusterMemoryLimit:       MetricConfig{Enabled: true},
		VcenterClusterMemoryUsed:        MetricConfig{Enabled: true},
		VcenterClusterVMCount:           MetricConfig{Enabled: true},
		VcenterDatastoreDiskUsage:       MetricConfig{Enabled: true},
		VcenterDatastoreDiskUtilization: MetricConfig{Enabled: true},
		VcenterHostCPUUsage:             MetricConfig{Enabled: true},
		VcenterHostCPUUtilization:       MetricConfig{Enabled: true},
		VcenterHostDiskLatencyAvg:       MetricConfig{Enabled: true},
		VcenterHostDiskLatencyMax:       MetricConfig{Enabled: true},
		VcenterHostDiskThroughput:       MetricConfig{Enabled: true},
		VcenterHostMemoryUsage:          MetricConfig{Enabled: true},
		VcenterHostMemoryUtilization:    MetricConfig{Enabled: true},
		VcenterHostNetworkPacketCount:   MetricConfig{Enabled: true},
		VcenterHostNetworkPacketErrors:  MetricConfig{Enabled: true},
		VcenterHostNetworkThroughput:    MetricConfig{Enabled: true},
		VcenterHostNetworkUsage:         MetricConfig{Enabled: true},
		VcenterResourcePoolCPUShares:    MetricConfig{Enabled: true},
		VcenterResourcePoolCPUUsage:     MetricConfig{Enabled: true},
		VcenterResourcePoolMemoryShares: MetricConfig{Enabled: true},
		VcenterResourcePoolMemoryUsage:  MetricConfig{Enabled: true},
		VcenterVMCPUUsage:               MetricConfig{Enabled: true},
		VcenterVMCPUUtilization:         MetricConfig{Enabled: true},
		VcenterVMDiskLatencyAvg:         MetricConfig{Enabled: true},
		VcenterVMDiskLatencyMax:         MetricConfig{Enabled: true},
		VcenterVMDiskThroughput:         MetricConfig{Enabled: true},
		VcenterVMDiskUsage:              MetricConfig{Enabled: true},
		VcenterVMDiskUtilization:        MetricConfig{Enabled: true},
		VcenterVMMemoryBallooned:        MetricConfig{Enabled: true},
		VcenterVMMemorySwapped:          MetricConfig{Enabled: true},
		VcenterVMMemorySwappedSsd:       MetricConfig{Enabled: true},
		VcenterVMMemoryUsage:            MetricConfig{Enabled: true},
		VcenterVMMemoryUtilization:      MetricConfig{Enabled: false},
		VcenterVMNetworkPacketCount:     MetricConfig{Enabled: true},
		VcenterVMNetworkThroughput:      MetricConfig{Enabled: true},
		VcenterVMNetworkUsage:           MetricConfig{Enabled: true},
	}
}

func (args *MetricsConfig) Convert() map[string]interface{} {
	if args == nil {
		return nil
	}

	return map[string]interface{}{
		"vcenter.cluster.cpu.effective":       args.VcenterClusterCPUEffective.Convert(),
		"vcenter.cluster.cpu.limit":           args.VcenterClusterCPULimit.Convert(),
		"vcenter.cluster.host.count":          args.VcenterClusterHostCount.Convert(),
		"vcenter.cluster.memory.effective":    args.VcenterClusterMemoryEffective.Convert(),
		"vcenter.cluster.memory.limit":        args.VcenterClusterMemoryLimit.Convert(),
		"vcenter.cluster.memory.used":         args.VcenterClusterMemoryUsed.Convert(),
		"vcenter.cluster.vm.count":            args.VcenterClusterVMCount.Convert(),
		"vcenter.datastore.disk.usage":        args.VcenterDatastoreDiskUsage.Convert(),
		"vcenter.datastore.disk.utilization":  args.VcenterDatastoreDiskUtilization.Convert(),
		"vcenter.host.cpu.usage":              args.VcenterHostCPUUsage.Convert(),
		"vcenter.host.cpu.utilization":        args.VcenterHostCPUUtilization.Convert(),
		"vcenter.host.disk.latency.avg":       args.VcenterHostDiskLatencyAvg.Convert(),
		"vcenter.host.disk.latency.max":       args.VcenterHostDiskLatencyMax.Convert(),
		"vcenter.host.disk.throughput":        args.VcenterHostDiskThroughput.Convert(),
		"vcenter.host.memory.usage":           args.VcenterHostMemoryUsage.Convert(),
		"vcenter.host.memory.utilization":     args.VcenterHostMemoryUtilization.Convert(),
		"vcenter.host.network.packet.count":   args.VcenterHostNetworkPacketCount.Convert(),
		"vcenter.host.network.packet.errors":  args.VcenterHostNetworkPacketErrors.Convert(),
		"vcenter.host.network.throughput":     args.VcenterHostNetworkThroughput.Convert(),
		"vcenter.host.network.usage":          args.VcenterHostNetworkUsage.Convert(),
		"vcenter.resource_pool.cpu.shares":    args.VcenterResourcePoolCPUShares.Convert(),
		"vcenter.resource_pool.cpu.usage":     args.VcenterResourcePoolCPUUsage.Convert(),
		"vcenter.resource_pool.memory.shares": args.VcenterResourcePoolMemoryShares.Convert(),
		"vcenter.resource_pool.memory.usage":  args.VcenterResourcePoolMemoryUsage.Convert(),
		"vcenter.vm.cpu.usage":                args.VcenterVMCPUUsage.Convert(),
		"vcenter.vm.cpu.utilization":          args.VcenterVMCPUUtilization.Convert(),
		"vcenter.vm.disk.latency.avg":         args.VcenterVMDiskLatencyAvg.Convert(),
		"vcenter.vm.disk.latency.max":         args.VcenterVMDiskLatencyMax.Convert(),
		"vcenter.vm.disk.throughput":          args.VcenterVMDiskThroughput.Convert(),
		"vcenter.vm.disk.usage":               args.VcenterVMDiskUsage.Convert(),
		"vcenter.vm.disk.utilization":         args.VcenterVMDiskUtilization.Convert(),
		"vcenter.vm.memory.ballooned":         args.VcenterVMMemoryBallooned.Convert(),
		"vcenter.vm.memory.swapped":           args.VcenterVMMemorySwapped.Convert(),
		"vcenter.vm.memory.swapped_ssd":       args.VcenterVMMemorySwappedSsd.Convert(),
		"vcenter.vm.memory.usage":             args.VcenterVMMemoryUsage.Convert(),
		"vcenter.vm.memory.utilization":       args.VcenterVMMemoryUtilization.Convert(),
		"vcenter.vm.network.packet.count":     args.VcenterVMNetworkPacketCount.Convert(),
		"vcenter.vm.network.throughput":       args.VcenterVMNetworkThroughput.Convert(),
		"vcenter.vm.network.usage":            args.VcenterVMNetworkUsage.Convert()}
}

type ResourceAttributeConfig struct {
	Enabled bool `river:"enabled,attr"`
}

func (r *ResourceAttributeConfig) Convert() map[string]interface{} {
	if r == nil {
		return nil
	}

	return map[string]interface{}{
		"enabled": r.Enabled,
	}
}

type ResourceAttributesConfig struct {
	VcenterClusterName               ResourceAttributeConfig `river:"vcenter.cluster.name,block,optional"`
	VcenterDatastoreName             ResourceAttributeConfig `river:"vcenter.datastore.name,block,optional"`
	VcenterHostName                  ResourceAttributeConfig `river:"vcenter.host.name,block,optional"`
	VcenterResourcePoolInventoryPath ResourceAttributeConfig `river:"vcenter.resource_pool.inventory_path,block,optional"`
	VcenterResourcePoolName          ResourceAttributeConfig `river:"vcenter.resource_pool.name,block,optional"`
	VcenterVMID                      ResourceAttributeConfig `river:"vcenter.vm.id,block,optional"`
	VcenterVMName                    ResourceAttributeConfig `river:"vcenter.vm.name,block,optional"`
}

func (args *ResourceAttributesConfig) SetToDefault() {
	*args = ResourceAttributesConfig{
		VcenterClusterName:               ResourceAttributeConfig{Enabled: true},
		VcenterDatastoreName:             ResourceAttributeConfig{Enabled: true},
		VcenterHostName:                  ResourceAttributeConfig{Enabled: true},
		VcenterResourcePoolInventoryPath: ResourceAttributeConfig{Enabled: true},
		VcenterResourcePoolName:          ResourceAttributeConfig{Enabled: true},
		VcenterVMID:                      ResourceAttributeConfig{Enabled: true},
		VcenterVMName:                    ResourceAttributeConfig{Enabled: true},
	}
}

func (args *ResourceAttributesConfig) Convert() map[string]interface{} {
	if args == nil {
		return nil
	}

	res := map[string]interface{}{
		"vcenter.cluster.name":                 args.VcenterClusterName.Convert(),
		"vcenter.datastore.name":               args.VcenterDatastoreName.Convert(),
		"vcenter.host.name":                    args.VcenterHostName.Convert(),
		"vcenter.resource_pool.inventory_path": args.VcenterResourcePoolInventoryPath.Convert(),
		"vcenter.resource_pool.name":           args.VcenterResourcePoolName.Convert(),
		"vcenter.vm.id":                        args.VcenterVMID.Convert(),
		"vcenter.vm.name":                      args.VcenterVMName.Convert(),
	}

	return res
}

type MetricsBuilderConfig struct {
	Metrics            MetricsConfig            `river:"metrics,block,optional"`
	ResourceAttributes ResourceAttributesConfig `river:"resource_attributes,block,optional"`
}

func (mbc *MetricsBuilderConfig) SetToDefault() {
	*mbc = MetricsBuilderConfig{}
	mbc.Metrics.SetToDefault()
	mbc.ResourceAttributes.SetToDefault()
}

func (args *MetricsBuilderConfig) Convert() map[string]interface{} {
	if args == nil {
		return nil
	}

	res := map[string]interface{}{
		"metrics":             args.Metrics.Convert(),
		"resource_attributes": args.ResourceAttributes.Convert(),
	}

	return res
}

// Arguments configures the otelcol.receiver.vcenter component.
type Arguments struct {
	Endpoint string            `river:"endpoint,attr"`
	Username string            `river:"username,attr"`
	Password rivertypes.Secret `river:"password,attr"`

	MetricsBuilderConfig MetricsBuilderConfig `river:",squash"`

	ScraperControllerArguments otelcol.ScraperControllerArguments `river:",squash"`
	TLS                        otelcol.TLSClientArguments         `river:"tls,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcol.DebugMetricsArguments `river:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `river:"output,block"`
}

var _ receiver.Arguments = Arguments{}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		ScraperControllerArguments: otelcol.DefaultScraperControllerArguments,
	}
	args.MetricsBuilderConfig.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	cfg := args.MetricsBuilderConfig.Convert()

	var result vcenterreceiver.Config
	err := mapstructure.Decode(cfg, &result)

	if err != nil {
		return nil, err
	}

	result.Endpoint = args.Endpoint
	result.Username = args.Username
	result.Password = configopaque.String(args.Password)
	result.TLSClientSetting = *args.TLS.Convert()
	result.ScraperControllerSettings = *args.ScraperControllerArguments.Convert()

	return &result, nil
}

// Validate checks to see if the supplied config will work for the receiver
func (args Arguments) Validate() error {
	res, err := url.Parse(args.Endpoint)
	if err != nil {
		return fmt.Errorf("unable to parse url %s: %w", args.Endpoint, err)
	}

	if res.Scheme != "http" && res.Scheme != "https" {
		return fmt.Errorf("url scheme must be http or https")
	}
	return nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

// Exporters implements receiver.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements receiver.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcol.DebugMetricsArguments {
	return args.DebugMetrics
}

// Package vcenter provides an otelcol.receiver.vcenter component.
package vcenter

import (
	"fmt"
	"net/url"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
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
	Enabled bool `alloy:"enabled,attr"`
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
	VcenterClusterCPUEffective      MetricConfig `alloy:"vcenter.cluster.cpu.effective,block,optional"`
	VcenterClusterCPULimit          MetricConfig `alloy:"vcenter.cluster.cpu.limit,block,optional"`
	VcenterClusterHostCount         MetricConfig `alloy:"vcenter.cluster.host.count,block,optional"`
	VcenterClusterMemoryEffective   MetricConfig `alloy:"vcenter.cluster.memory.effective,block,optional"`
	VcenterClusterMemoryLimit       MetricConfig `alloy:"vcenter.cluster.memory.limit,block,optional"`
	VcenterClusterMemoryUsed        MetricConfig `alloy:"vcenter.cluster.memory.used,block,optional"`
	VcenterClusterVMCount           MetricConfig `alloy:"vcenter.cluster.vm.count,block,optional"`
	VcenterDatastoreDiskUsage       MetricConfig `alloy:"vcenter.datastore.disk.usage,block,optional"`
	VcenterDatastoreDiskUtilization MetricConfig `alloy:"vcenter.datastore.disk.utilization,block,optional"`
	VcenterHostCPUUsage             MetricConfig `alloy:"vcenter.host.cpu.usage,block,optional"`
	VcenterHostCPUUtilization       MetricConfig `alloy:"vcenter.host.cpu.utilization,block,optional"`
	VcenterHostDiskLatencyAvg       MetricConfig `alloy:"vcenter.host.disk.latency.avg,block,optional"`
	VcenterHostDiskLatencyMax       MetricConfig `alloy:"vcenter.host.disk.latency.max,block,optional"`
	VcenterHostDiskThroughput       MetricConfig `alloy:"vcenter.host.disk.throughput,block,optional"`
	VcenterHostMemoryUsage          MetricConfig `alloy:"vcenter.host.memory.usage,block,optional"`
	VcenterHostMemoryUtilization    MetricConfig `alloy:"vcenter.host.memory.utilization,block,optional"`
	VcenterHostNetworkPacketCount   MetricConfig `alloy:"vcenter.host.network.packet.count,block,optional"`
	VcenterHostNetworkPacketErrors  MetricConfig `alloy:"vcenter.host.network.packet.errors,block,optional"`
	VcenterHostNetworkThroughput    MetricConfig `alloy:"vcenter.host.network.throughput,block,optional"`
	VcenterHostNetworkUsage         MetricConfig `alloy:"vcenter.host.network.usage,block,optional"`
	VcenterResourcePoolCPUShares    MetricConfig `alloy:"vcenter.resource_pool.cpu.shares,block,optional"`
	VcenterResourcePoolCPUUsage     MetricConfig `alloy:"vcenter.resource_pool.cpu.usage,block,optional"`
	VcenterResourcePoolMemoryShares MetricConfig `alloy:"vcenter.resource_pool.memory.shares,block,optional"`
	VcenterResourcePoolMemoryUsage  MetricConfig `alloy:"vcenter.resource_pool.memory.usage,block,optional"`
	VcenterVMCPUUsage               MetricConfig `alloy:"vcenter.vm.cpu.usage,block,optional"`
	VcenterVMCPUUtilization         MetricConfig `alloy:"vcenter.vm.cpu.utilization,block,optional"`
	VcenterVMDiskLatencyAvg         MetricConfig `alloy:"vcenter.vm.disk.latency.avg,block,optional"`
	VcenterVMDiskLatencyMax         MetricConfig `alloy:"vcenter.vm.disk.latency.max,block,optional"`
	VcenterVMDiskThroughput         MetricConfig `alloy:"vcenter.vm.disk.throughput,block,optional"`
	VcenterVMDiskUsage              MetricConfig `alloy:"vcenter.vm.disk.usage,block,optional"`
	VcenterVMDiskUtilization        MetricConfig `alloy:"vcenter.vm.disk.utilization,block,optional"`
	VcenterVMMemoryBallooned        MetricConfig `alloy:"vcenter.vm.memory.ballooned,block,optional"`
	VcenterVMMemorySwapped          MetricConfig `alloy:"vcenter.vm.memory.swapped,block,optional"`
	VcenterVMMemorySwappedSsd       MetricConfig `alloy:"vcenter.vm.memory.swapped_ssd,block,optional"`
	VcenterVMMemoryUsage            MetricConfig `alloy:"vcenter.vm.memory.usage,block,optional"`
	VcenterVMMemoryUtilization      MetricConfig `alloy:"vcenter.vm.memory.utilization,block,optional"`
	VcenterVMNetworkPacketCount     MetricConfig `alloy:"vcenter.vm.network.packet.count,block,optional"`
	VcenterVMNetworkThroughput      MetricConfig `alloy:"vcenter.vm.network.throughput,block,optional"`
	VcenterVMNetworkUsage           MetricConfig `alloy:"vcenter.vm.network.usage,block,optional"`
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
	Enabled bool `alloy:"enabled,attr"`
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
	VcenterClusterName               ResourceAttributeConfig `alloy:"vcenter.cluster.name,block,optional"`
	VcenterDatastoreName             ResourceAttributeConfig `alloy:"vcenter.datastore.name,block,optional"`
	VcenterHostName                  ResourceAttributeConfig `alloy:"vcenter.host.name,block,optional"`
	VcenterResourcePoolInventoryPath ResourceAttributeConfig `alloy:"vcenter.resource_pool.inventory_path,block,optional"`
	VcenterResourcePoolName          ResourceAttributeConfig `alloy:"vcenter.resource_pool.name,block,optional"`
	VcenterVMID                      ResourceAttributeConfig `alloy:"vcenter.vm.id,block,optional"`
	VcenterVMName                    ResourceAttributeConfig `alloy:"vcenter.vm.name,block,optional"`
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
	Metrics            MetricsConfig            `alloy:"metrics,block,optional"`
	ResourceAttributes ResourceAttributesConfig `alloy:"resource_attributes,block,optional"`
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
	Endpoint string            `alloy:"endpoint,attr"`
	Username string            `alloy:"username,attr"`
	Password alloytypes.Secret `alloy:"password,attr"`

	MetricsBuilderConfig MetricsBuilderConfig `alloy:",squash"`

	ScraperControllerArguments otelcol.ScraperControllerArguments `alloy:",squash"`
	TLS                        otelcol.TLSClientArguments         `alloy:"tls,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcol.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

var _ receiver.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
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
	result.ClientConfig = *args.TLS.Convert()
	result.ControllerConfig = *args.ScraperControllerArguments.Convert()

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

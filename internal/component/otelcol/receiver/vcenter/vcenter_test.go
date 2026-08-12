package vcenter_test

import (
	"testing"
	"time"

	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/vcenter"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	in := `
		endpoint = "http://localhost:1234"
		username = "user"
		password = "pass"
		collection_interval = "2m"
		max_query_metrics = 128

		resource_attributes {
			vcenter.datacenter.name {
				enabled = true
			}
			vcenter.cluster.name {
				enabled = true
			}
			vcenter.datastore.name {
				enabled = true
			}
			vcenter.host.name {
				enabled = true
			}
			vcenter.resource_pool.inventory_path {
				enabled = false
			}
			vcenter.resource_pool.name {
				enabled = true
			}
			vcenter.virtual_app.inventory_path {
				enabled = false
			}
			vcenter.virtual_app.name {
				enabled = true
			}
			vcenter.vm.name {
				enabled = true
			}
			vcenter.vm_template.name {
				enabled = true
			}
		}

		metrics {
			vcenter.cluster.cpu.effective {
				enabled = false
			}
			vcenter.cluster.cpu.limit {
				enabled = true
			}
			vcenter.cluster.host.count {
				enabled = true
			}
			vcenter.cluster.memory.effective {
				enabled = true
			}
			vcenter.cluster.memory.limit {
				enabled = true
			}
			vcenter.cluster.vm.count {
				enabled = true
			}
			vcenter.cluster.vm_template.count {
				enabled = true
			}
			vcenter.cluster.vsan.congestions {
				enabled = true
			}
			vcenter.cluster.vsan.latency.avg {
				enabled = true
			}
			vcenter.cluster.vsan.operations {
				enabled = true
			}
			vcenter.cluster.vsan.throughput {
				enabled = true
			}
			vcenter.datastore.disk.usage {
				enabled = true
			}
			vcenter.datastore.disk.utilization {
				enabled = true
			}
			vcenter.host.cpu.usage {
				enabled = true
			}
			vcenter.host.cpu.utilization {
				enabled = true
			}
			vcenter.host.disk.latency.avg {
				enabled = true
			}
			vcenter.host.disk.latency.max {
				enabled = true
			}
			vcenter.host.disk.throughput {
				enabled = true
			}
			vcenter.host.memory.capacity {
				enabled = false
			}
			vcenter.host.memory.usage {
				enabled = true
			}
			vcenter.host.memory.utilization {
				enabled = true
			}
			vcenter.host.network.packet.rate {
				enabled = true
			}
			vcenter.host.network.packet.error.rate {
				enabled = true
			}
			vcenter.host.network.throughput {
				enabled = true
			}
			vcenter.host.network.usage {
				enabled = true
			}
			vcenter.host.vsan.cache.hit_rate {
				enabled = true
			}
			vcenter.host.vsan.congestions {
				enabled = true
			}
			vcenter.host.vsan.latency.avg {
				enabled = true
			}
			vcenter.host.vsan.operations {
				enabled = true
			}
			vcenter.host.vsan.throughput {
				enabled = true
			}
			vcenter.resource_pool.cpu.shares {
				enabled = true
			}
			vcenter.resource_pool.cpu.usage {
				enabled = true
			}
			vcenter.resource_pool.memory.shares {
				enabled = true
			}
			vcenter.resource_pool.memory.usage {
				enabled = true
			}
			vcenter.vm.cpu.time {
				enabled = true
			}
			vcenter.vm.cpu.usage {
				enabled = true
			}
			vcenter.vm.cpu.utilization {
				enabled = true
			}
			vcenter.vm.disk.latency.avg {
				enabled = true
			}
			vcenter.vm.disk.latency.max {
				enabled = true
			}
			vcenter.vm.disk.throughput {
				enabled = true
			}
			vcenter.vm.disk.usage {
				enabled = true
			}
			vcenter.vm.disk.utilization {
				enabled = true
			}
			vcenter.vm.memory.ballooned {
				enabled = true
			}
			vcenter.vm.memory.granted {
				enabled = false
			}
			vcenter.vm.memory.swapped {
				enabled = true
			}
			vcenter.vm.memory.swapped_ssd {
				enabled = true
			}
			vcenter.vm.memory.usage {
				enabled = true
			}
			vcenter.vm.memory.utilization {
				enabled = true
			}
			vcenter.vm.network.broadcast.packet.rate {
				enabled = true
			}
			vcenter.vm.network.multicast.packet.rate {
				enabled = true
			}
			vcenter.vm.network.packet.rate {
				enabled = true
			}
			vcenter.vm.network.packet.drop.rate {
				enabled = true
			}
			vcenter.vm.network.throughput {
				enabled = true
			}
			vcenter.vm.network.usage {
				enabled = true
			}
			vcenter.vm.vsan.latency.avg {
				enabled = true
			}
			vcenter.vm.vsan.operations {
				enabled = true
			}
			vcenter.vm.vsan.throughput {
				enabled = true
			}
		}

		output { /* no-op */ }
	`

	var args vcenter.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(in), &args))
	args.Convert()
	ext, err := args.Convert()
	require.NoError(t, err)
	otelArgs, ok := (ext).(*vcenterreceiver.Config)

	require.True(t, ok)

	require.Equal(t, "user", otelArgs.Username)
	require.Equal(t, "pass", string(otelArgs.Password))
	require.Equal(t, "http://localhost:1234", otelArgs.Endpoint)

	require.Equal(t, 2*time.Minute, otelArgs.ControllerConfig.CollectionInterval)
	require.Equal(t, time.Second, otelArgs.ControllerConfig.InitialDelay)
	require.Equal(t, 0*time.Second, otelArgs.ControllerConfig.Timeout)
	require.Equal(t, 128, otelArgs.MaxQueryMetrics)

	// Verify ResourceAttributesConfig fields
	require.True(t, otelArgs.MetricsBuilderConfig.ResourceAttributes.VcenterClusterName.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.ResourceAttributes.VcenterDatastoreName.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.ResourceAttributes.VcenterHostName.Enabled)
	require.False(t, otelArgs.MetricsBuilderConfig.ResourceAttributes.VcenterResourcePoolInventoryPath.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.ResourceAttributes.VcenterResourcePoolName.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.ResourceAttributes.VcenterVMName.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.ResourceAttributes.VcenterVMID.Enabled)

	// Verify MetricsConfig fields
	require.False(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterClusterCPUEffective.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterClusterCPULimit.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterClusterHostCount.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterClusterMemoryEffective.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterClusterMemoryLimit.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterClusterVMCount.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterClusterVsanCongestions.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterClusterVsanLatencyAvg.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterClusterVsanOperations.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterClusterVsanThroughput.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterDatastoreDiskUsage.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterDatastoreDiskUtilization.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostCPUUsage.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostCPUUtilization.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostDiskLatencyAvg.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostDiskLatencyMax.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostDiskThroughput.Enabled)
	require.False(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostMemoryCapacity.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostMemoryUsage.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostMemoryUtilization.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostNetworkPacketRate.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostNetworkPacketErrorRate.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostNetworkThroughput.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostNetworkUsage.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostVsanCacheHitRate.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostVsanCongestions.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostVsanLatencyAvg.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostVsanOperations.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterHostVsanThroughput.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterResourcePoolCPUShares.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterResourcePoolCPUUsage.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterResourcePoolMemoryShares.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterResourcePoolMemoryUsage.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMCPUTime.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMCPUUsage.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMCPUUtilization.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMDiskLatencyAvg.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMDiskLatencyMax.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMDiskThroughput.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMDiskUsage.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMDiskUtilization.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMMemoryBallooned.Enabled)
	require.False(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMMemoryGranted.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMMemorySwapped.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMMemorySwappedSsd.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMMemoryUsage.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMMemoryUtilization.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMNetworkBroadcastPacketRate.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMNetworkMulticastPacketRate.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMNetworkPacketRate.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMNetworkPacketDropRate.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMNetworkThroughput.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMNetworkUsage.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMVsanLatencyAvg.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMVsanOperations.Enabled)
	require.True(t, otelArgs.MetricsBuilderConfig.Metrics.VcenterVMVsanThroughput.Enabled)
}

func TestDebugMetricsConfig(t *testing.T) {
	tests := []struct {
		testName string
		alloyCfg string
		expected otelcolCfg.DebugMetricsArguments
	}{
		{
			testName: "default",
			alloyCfg: `
			endpoint = "http://localhost:1234"
			username = "user"
			password = "pass"

			output {}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_false",
			alloyCfg: `
			endpoint = "http://localhost:1234"
			username = "user"
			password = "pass"

			debug_metrics {
				disable_high_cardinality_metrics = false
			}

			output {}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: false,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_true",
			alloyCfg: `
			endpoint = "http://localhost:1234"
			username = "user"
			password = "pass"

			debug_metrics {
				disable_high_cardinality_metrics = true
			}

			output {}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args vcenter.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.alloyCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}

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

	// Verify ResourceAttributesConfig fields
	require.True(t, otelArgs.ResourceAttributes.VcenterClusterName.Enabled)
	require.True(t, otelArgs.ResourceAttributes.VcenterDatastoreName.Enabled)
	require.True(t, otelArgs.ResourceAttributes.VcenterHostName.Enabled)
	require.False(t, otelArgs.ResourceAttributes.VcenterResourcePoolInventoryPath.Enabled)
	require.True(t, otelArgs.ResourceAttributes.VcenterResourcePoolName.Enabled)
	require.True(t, otelArgs.ResourceAttributes.VcenterVMName.Enabled)
	require.True(t, otelArgs.ResourceAttributes.VcenterVMID.Enabled)

	// Verify MetricsConfig fields
	require.False(t, otelArgs.Metrics.VcenterClusterCPUEffective.Enabled)
	require.True(t, otelArgs.Metrics.VcenterClusterCPULimit.Enabled)
	require.True(t, otelArgs.Metrics.VcenterClusterHostCount.Enabled)
	require.True(t, otelArgs.Metrics.VcenterClusterMemoryEffective.Enabled)
	require.True(t, otelArgs.Metrics.VcenterClusterMemoryLimit.Enabled)
	require.True(t, otelArgs.Metrics.VcenterClusterVMCount.Enabled)
	require.True(t, otelArgs.Metrics.VcenterDatastoreDiskUsage.Enabled)
	require.True(t, otelArgs.Metrics.VcenterDatastoreDiskUtilization.Enabled)
	require.True(t, otelArgs.Metrics.VcenterHostCPUUsage.Enabled)
	require.True(t, otelArgs.Metrics.VcenterHostCPUUtilization.Enabled)
	require.True(t, otelArgs.Metrics.VcenterHostDiskLatencyAvg.Enabled)
	require.True(t, otelArgs.Metrics.VcenterHostDiskLatencyMax.Enabled)
	require.True(t, otelArgs.Metrics.VcenterHostDiskThroughput.Enabled)
	require.True(t, otelArgs.Metrics.VcenterHostMemoryUsage.Enabled)
	require.True(t, otelArgs.Metrics.VcenterHostMemoryUtilization.Enabled)
	require.True(t, otelArgs.Metrics.VcenterHostNetworkPacketRate.Enabled)
	require.True(t, otelArgs.Metrics.VcenterHostNetworkPacketErrorRate.Enabled)
	require.True(t, otelArgs.Metrics.VcenterHostNetworkThroughput.Enabled)
	require.True(t, otelArgs.Metrics.VcenterHostNetworkUsage.Enabled)
	require.True(t, otelArgs.Metrics.VcenterResourcePoolCPUShares.Enabled)
	require.True(t, otelArgs.Metrics.VcenterResourcePoolCPUUsage.Enabled)
	require.True(t, otelArgs.Metrics.VcenterResourcePoolMemoryShares.Enabled)
	require.True(t, otelArgs.Metrics.VcenterResourcePoolMemoryUsage.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMCPUUsage.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMCPUUtilization.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMDiskLatencyAvg.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMDiskLatencyMax.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMDiskThroughput.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMDiskUsage.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMDiskUtilization.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMMemoryBallooned.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMMemorySwapped.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMMemorySwappedSsd.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMMemoryUsage.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMMemoryUtilization.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMNetworkPacketRate.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMNetworkPacketDropRate.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMNetworkThroughput.Enabled)
	require.True(t, otelArgs.Metrics.VcenterVMNetworkUsage.Enabled)
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

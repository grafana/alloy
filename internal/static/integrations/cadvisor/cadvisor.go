//go:build linux

package cadvisor

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-logr/logr"
	"github.com/google/cadvisor/cache/memory"
	"github.com/google/cadvisor/container"
	v2 "github.com/google/cadvisor/info/v2"
	"github.com/google/cadvisor/manager"
	"github.com/google/cadvisor/metrics"
	"github.com/google/cadvisor/storage"
	"github.com/google/cadvisor/utils/sysfs"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"

	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/static/integrations"

	// Register container providers

	"github.com/google/cadvisor/container/containerd"
	"github.com/google/cadvisor/container/crio"
	"github.com/google/cadvisor/container/docker"
	"github.com/google/cadvisor/container/raw"
	"github.com/google/cadvisor/container/systemd"
)

// Matching the default disabled set from cadvisor - https://github.com/google/cadvisor/blob/3c6e3093c5ca65c57368845ddaea2b4ca6bc0da8/cmd/cadvisor.go#L78-L93
// Note: This *could* be kept in sync with upstream by using the following. However, that would require importing the github.com/google/cadvisor/cmd package, which introduces some dependency conflicts that weren't worth the hassle IMHO.
// var disabledMetrics = *flag.Lookup("disable_metrics").Value.(*container.MetricSet)
var disabledMetrics = container.MetricSet{
	container.MemoryNumaMetrics:              struct{}{},
	container.NetworkTcpUsageMetrics:         struct{}{},
	container.NetworkUdpUsageMetrics:         struct{}{},
	container.NetworkAdvancedTcpUsageMetrics: struct{}{},
	container.ProcessSchedulerMetrics:        struct{}{},
	container.ProcessMetrics:                 struct{}{},
	container.HugetlbUsageMetrics:            struct{}{},
	container.ReferencedMemoryMetrics:        struct{}{},
	container.CPUTopologyMetrics:             struct{}{},
	container.ResctrlMetrics:                 struct{}{},
	container.CPUSetMetrics:                  struct{}{},
}

// GetIncludedMetrics applies some logic to determine the final set of metrics to be scraped and returned by the cAdvisor integration
func (c *Config) GetIncludedMetrics() (container.MetricSet, error) {
	var enabledMetrics, includedMetrics container.MetricSet

	if c.DisabledMetrics != nil {
		if err := disabledMetrics.Set(strings.Join(c.DisabledMetrics, ",")); err != nil {
			return includedMetrics, fmt.Errorf("failed to set disabled metrics: %w", err)
		}
	}

	if c.EnabledMetrics != nil {
		if err := enabledMetrics.Set(strings.Join(c.EnabledMetrics, ",")); err != nil {
			return includedMetrics, fmt.Errorf("failed to set enabled metrics: %w", err)
		}
	}

	if len(enabledMetrics) > 0 {
		includedMetrics = enabledMetrics
	} else {
		includedMetrics = container.AllMetrics.Difference(disabledMetrics)
	}

	return includedMetrics, nil
}

// NewIntegration creates a new cadvisor integration
func (c *Config) NewIntegration(logger log.Logger) (integrations.Integration, error) {
	return New(logger, c)
}

// New creates a new cadvisor integration
func New(logger log.Logger, c *Config) (integrations.Integration, error) {
	c.logger = logger
	// Do gross global configs. This works, so long as there is only one instance of the cAdvisor integration
	// per host.

	klog.SetLogger(logr.FromSlogHandler(logging.NewSlogGoKitHandler(logger)))
	plugins := map[string]container.Plugin{
		"containerd": containerd.NewPluginWithOptions(&containerd.Options{
			ContainerdEndpoint:  c.Containerd,
			ContainerdNamespace: c.ContainerdNamespace,
		}),
		"crio": crio.NewPlugin(),
		"docker": docker.NewPluginWithOptions(&docker.Options{
			DockerEndpoint:     c.Docker,
			DockerTLS:          c.DockerTLS,
			DockerCert:         c.DockerTLSCert,
			DockerKey:          c.DockerTLSKey,
			DockerCA:           c.DockerTLSCA,
			ContainerDEndpoint: c.Containerd,
		}),
		"systemd": systemd.NewPlugin(),
	}

	// Only using in-memory storage, with no backup storage for cadvisor stats
	memoryStorage := memory.New(c.StorageDuration, []storage.StorageDriver{})

	sysFs := sysfs.NewRealSysFs()

	var collectorHTTPClient http.Client

	includedMetrics, err := c.GetIncludedMetrics()
	if err != nil {
		return nil, fmt.Errorf("unable to determine included metrics: %w", err)
	}

	rawOpts := raw.Options{
		DockerOnly:             c.DockerOnly,
		DisableRootCgroupStats: c.DisableRootCgroupStats,
	}
	rm, err := manager.New(plugins, memoryStorage, sysFs, manager.HousekeepingConfigFlags, includedMetrics, &collectorHTTPClient, c.RawCgroupPrefixAllowlist, c.EnvMetadataAllowlist, c.PerfEventsConfig, time.Duration(c.ResctrlInterval), rawOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create a manager: %w", err)
	}

	if err := rm.Start(); err != nil {
		return nil, fmt.Errorf("failed to start manager: %w", err)
	}

	containerLabelFunc := metrics.DefaultContainerLabels
	if !c.StoreContainerLabels {
		containerLabelFunc = metrics.BaseContainerLabels(c.AllowlistedContainerLabels)
	}

	machCol := metrics.NewPrometheusMachineCollector(rm, includedMetrics)
	// This is really just a concatenation of the defaults found at;
	// https://github.com/google/cadvisor/tree/f89291a53b80b2c3659fff8954c11f1fc3de8a3b/cmd/internal/api/versions.go#L536-L540
	// https://github.com/google/cadvisor/tree/f89291a53b80b2c3659fff8954c11f1fc3de8a3b/cmd/internal/http/handlers.go#L109-L110
	// AFAIK all we are ever doing is the "default" metrics request, and we don't need to support the "docker" request type.
	reqOpts := v2.RequestOptions{
		IdType:    v2.TypeName,
		Count:     1,
		Recursive: true,
	}
	contCol := metrics.NewPrometheusCollector(rm, containerLabelFunc, includedMetrics, clock.RealClock{}, reqOpts)

	start := func(ctx context.Context) error {
		<-ctx.Done()

		if err := rm.Stop(); err != nil {
			return fmt.Errorf("failed to stop manager: %w", err)
		}
		return nil
	}

	ci := integrations.NewCollectorIntegration(
		c.Name(),
		integrations.WithRunner(start),
		integrations.WithCollectors(machCol, contCol),
	)

	return ci, nil
}

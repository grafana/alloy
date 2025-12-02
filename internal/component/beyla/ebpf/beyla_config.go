//go:build (linux && arm64) || (linux && amd64)

//go:generate go run ./gen/main.go

package beyla

import (
	"fmt"

	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func (c *Component) writeConfigFile() (string, func(), error) {
	config := c.buildConfig()

	configData, err := yaml.Marshal(config)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	level.Debug(c.opts.Logger).Log("msg", "generated Beyla YAML config", "yaml", string(configData))

	// No MFD_CLOEXEC: the fd must survive fork→exec so Beyla can open
	// /proc/self/fd/{N} after exec. Works on read-only root filesystems.
	fd, err := unix.MemfdCreate("beyla-config", 0)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create in-memory config file: %w", err)
	}

	data := configData
	for len(data) > 0 {
		n, err := unix.Write(fd, data)
		if err != nil {
			unix.Close(fd)
			return "", nil, fmt.Errorf("failed to write config: %w", err)
		}
		data = data[n:]
	}

	// Seek to the beginning so Beyla reads from the start.
	if _, err := unix.Seek(fd, 0, 0); err != nil {
		unix.Close(fd)
		return "", nil, fmt.Errorf("failed to seek config fd: %w", err)
	}

	configPath := fmt.Sprintf("/proc/self/fd/%d", fd)
	cleanup := func() { unix.Close(fd) }

	return configPath, cleanup, nil
}

func (c *Component) buildConfig() map[string]interface{} {
	config := make(map[string]interface{})

	c.addPrometheusConfig(config)
	c.addRoutesConfig(config)
	c.addAttributesConfig(config)
	c.addDiscoveryConfig(config)
	c.addEbpfConfig(config)
	c.addNetworkFlowsConfig(config)
	c.addStatsConfig(config)
	c.addInjectorConfig(config)
	c.addFiltersConfig(config)
	c.addTracesConfig(config)
	c.addOTLPTracesExportConfig(config)
	c.addOTLPMetricsExportConfig(config)
	c.addInternalMetricsConfig(config)
	c.addLogLevelConfig(config)
	c.addTracePrinterConfig(config)
	c.addEnforceSysCapsConfig(config)

	return config
}

func (c *Component) addPrometheusConfig(config map[string]interface{}) {
	c.mut.Lock()
	port := c.subprocessPort
	c.mut.Unlock()

	prometheus := map[string]interface{}{
		"port": port,
	}

	c.fillPrometheusExportConfig(prometheus)

	config["prometheus_export"] = prometheus
}

func (c *Component) addAttributesConfig(config map[string]interface{}) {
	if c.args.Attributes.Kubernetes.Enable == "" && c.args.Attributes.InstanceID.OverrideHostname == "" && len(c.args.Attributes.Select) == 0 {
		return
	}

	attributes := make(map[string]interface{})

	// Kubernetes attributes
	if c.args.Attributes.Kubernetes.Enable != "" {
		kubernetes := c.buildKubernetesConfig()
		attributes["kubernetes"] = kubernetes
	}

	// InstanceID attributes
	if c.args.Attributes.InstanceID.HostnameDNSResolution || c.args.Attributes.InstanceID.OverrideHostname != "" {
		instanceID := c.buildInstanceIDConfig()
		attributes["instance_id"] = instanceID
	}

	// Select attributes
	if len(c.args.Attributes.Select) > 0 {
		selectMap := c.buildSelectConfig()
		if len(selectMap) > 0 {
			attributes["select"] = selectMap
		}
	}

	config["attributes"] = attributes
}

func (c *Component) buildKubernetesConfig() map[string]interface{} {
	kubernetes := map[string]interface{}{
		"enable": c.args.Attributes.Kubernetes.Enable,
	}

	if c.args.Attributes.Kubernetes.ClusterName != "" {
		kubernetes["cluster_name"] = c.args.Attributes.Kubernetes.ClusterName
	}
	if c.args.Attributes.Kubernetes.InformersSyncTimeout != 0 {
		kubernetes["informers_sync_timeout"] = c.args.Attributes.Kubernetes.InformersSyncTimeout.String()
	}
	if c.args.Attributes.Kubernetes.InformersResyncPeriod != 0 {
		kubernetes["informers_resync_period"] = c.args.Attributes.Kubernetes.InformersResyncPeriod.String()
	}
	if len(c.args.Attributes.Kubernetes.DisableInformers) > 0 {
		kubernetes["disable_informers"] = c.args.Attributes.Kubernetes.DisableInformers
	}
	if c.args.Attributes.Kubernetes.MetaRestrictLocalNode {
		kubernetes["meta_restrict_local_node"] = true
	}
	if c.args.Attributes.Kubernetes.MetaCacheAddress != "" {
		kubernetes["meta_cache_address"] = c.args.Attributes.Kubernetes.MetaCacheAddress
	}

	return kubernetes
}

func (c *Component) buildInstanceIDConfig() map[string]interface{} {
	instanceID := make(map[string]interface{})

	if c.args.Attributes.InstanceID.HostnameDNSResolution {
		instanceID["dns"] = true
	}
	if c.args.Attributes.InstanceID.OverrideHostname != "" {
		instanceID["override_hostname"] = c.args.Attributes.InstanceID.OverrideHostname
	}

	return instanceID
}

func (c *Component) buildSelectConfig() map[string]interface{} {
	selectMap := make(map[string]interface{})

	for _, sel := range c.args.Attributes.Select {
		selConfig := make(map[string]interface{})
		if len(sel.Include) > 0 {
			selConfig["include"] = sel.Include
		}
		if len(sel.Exclude) > 0 {
			selConfig["exclude"] = sel.Exclude
		}
		if len(selConfig) > 0 {
			selectMap[sel.Section] = selConfig
		}
	}

	return selectMap
}

func (c *Component) addNetworkFlowsConfig(config map[string]interface{}) {
	// Gated on metrics.features containing "network" OR the deprecated network.enable flag.
	// enable:true must always be present — Beyla requires it to activate network flows.
	if !c.args.Metrics.hasNetworkFeature() && !c.args.Metrics.Network.Enable {
		return
	}

	networkFlows := map[string]interface{}{
		"enable": true,
	}

	c.fillNetworkConfig(networkFlows)

	config["network"] = networkFlows
}

func (c *Component) addFiltersConfig(config map[string]interface{}) {
	if len(c.args.Filters.Application) == 0 && len(c.args.Filters.Network) == 0 {
		return
	}

	filters := make(map[string]interface{})

	if len(c.args.Filters.Application) > 0 {
		app := make(map[string]interface{})
		fillAttributeFamiliesConfig(app, c.args.Filters.Application)
		if len(app) > 0 {
			filters["application"] = app
		}
	}

	if len(c.args.Filters.Network) > 0 {
		net := make(map[string]interface{})
		fillAttributeFamiliesConfig(net, c.args.Filters.Network)
		if len(net) > 0 {
			filters["network"] = net
		}
	}

	if len(filters) > 0 {
		config["filters"] = filters
	}
}

func (c *Component) addTracesConfig(config map[string]interface{}) {
	if len(c.args.Traces.Instrumentations) == 0 && c.args.Traces.Sampler.Name == "" {
		return
	}

	traces := make(map[string]interface{})

	if len(c.args.Traces.Instrumentations) > 0 {
		traces["instrumentations"] = c.args.Traces.Instrumentations
	}

	if c.args.Traces.Sampler.Name != "" {
		sampler := map[string]interface{}{
			"name": c.args.Traces.Sampler.Name,
		}
		if c.args.Traces.Sampler.Arg != "" {
			sampler["arg"] = c.args.Traces.Sampler.Arg
		}
		traces["sampler"] = sampler
	}

	if len(traces) > 0 {
		config["traces"] = traces
	}
}

func (c *Component) addOTLPTracesExportConfig(config map[string]interface{}) {
	if c.args.Output == nil || len(c.args.Output.Traces) == 0 {
		return
	}

	c.mut.Lock()
	otlpPort := c.otlpReceiverPort
	c.mut.Unlock()

	if otlpPort > 0 {
		otelTracesExport := map[string]interface{}{
			"endpoint": fmt.Sprintf("http://127.0.0.1:%d", otlpPort),
			"protocol": "http/protobuf",
		}
		config["otel_traces_export"] = otelTracesExport
		level.Debug(c.opts.Logger).Log("msg", "configured OTLP traces export", "endpoint", fmt.Sprintf("http://localhost:%d", otlpPort))
	}
}

func (c *Component) addOTLPMetricsExportConfig(config map[string]interface{}) {
	if c.args.Output == nil || len(c.args.Output.Metrics) == 0 {
		return
	}

	c.mut.Lock()
	otlpPort := c.otlpReceiverPort
	c.mut.Unlock()

	if otlpPort > 0 {
		config["otel_metrics_export"] = map[string]interface{}{
			"endpoint": fmt.Sprintf("http://127.0.0.1:%d", otlpPort),
			"protocol": "http/protobuf",
		}
		level.Debug(c.opts.Logger).Log("msg", "configured OTLP metrics export", "endpoint", fmt.Sprintf("http://127.0.0.1:%d", otlpPort))
	}
}

func (c *Component) addLogLevelConfig(config map[string]interface{}) {
	if c.args.Debug && c.args.LogLevel == "" {
		config["log_level"] = "debug"
		return
	}

	if c.args.LogLevel != "" {
		// TODO: auto-derive from component.Options once Alloy exposes the level there
		config["log_level"] = c.args.LogLevel
	}
}

func (c *Component) addTracePrinterConfig(config map[string]interface{}) {
	if c.args.TracePrinter != "" && c.args.TracePrinter != "disabled" {
		config["trace_printer"] = c.args.TracePrinter
	}
}

func (c *Component) addEnforceSysCapsConfig(config map[string]interface{}) {
	if c.args.EnforceSysCaps {
		config["enforce_sys_caps"] = true
	}
}

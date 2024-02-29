package otelcolconvert

import (
	"fmt"

	"github.com/alecthomas/units"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/exporter/loadbalancing"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"
	"go.opentelemetry.io/collector/component"
)

func init() {
	converters = append(converters, loadbalancingExporterConverter{})
}

type loadbalancingExporterConverter struct{}

func (loadbalancingExporterConverter) Factory() component.Factory {
	return loadbalancingexporter.NewFactory()
}

func (loadbalancingExporterConverter) InputComponentName() string {
	return "otelcol.exporter.loadbalancing"
}

func (loadbalancingExporterConverter) ConvertAndAppend(state *state, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.FlowComponentLabel()

	args := toLoadbalancingExporter(cfg.(*loadbalancingexporter.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "exporter", "loadbalancing"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", stringifyInstanceID(id), stringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toLoadbalancingExporter(cfg *loadbalancingexporter.Config) *loadbalancing.Arguments {
	return &loadbalancing.Arguments{
		Protocol:   toProtocol(cfg.Protocol),
		Resolver:   toResolver(cfg.Resolver),
		RoutingKey: cfg.RoutingKey,

		DebugMetrics: common.DefaultValue[loadbalancing.Arguments]().DebugMetrics,
	}
}

func toProtocol(cfg loadbalancingexporter.Protocol) loadbalancing.Protocol {
	return loadbalancing.Protocol{
		// NOTE(rfratto): this has a lot of overlap with converting the
		// otlpexporter, but otelcol.exporter.loadbalancing uses custom types to
		// remove unwanted fields.
		OTLP: loadbalancing.OtlpConfig{
			Timeout: cfg.OTLP.Timeout,
			Queue:   toQueueArguments(cfg.OTLP.QueueSettings),
			Retry:   toRetryArguments(cfg.OTLP.RetrySettings),
			Client: loadbalancing.GRPCClientArguments{
				Compression: otelcol.CompressionType(cfg.OTLP.Compression),

				TLS:       toTLSClientArguments(cfg.OTLP.TLSSetting),
				Keepalive: toKeepaliveClientArguments(cfg.OTLP.Keepalive),

				ReadBufferSize:  units.Base2Bytes(cfg.OTLP.ReadBufferSize),
				WriteBufferSize: units.Base2Bytes(cfg.OTLP.WriteBufferSize),
				WaitForReady:    cfg.OTLP.WaitForReady,
				Headers:         toHeadersMap(cfg.OTLP.Headers),
				BalancerName:    cfg.OTLP.BalancerName,
				Authority:       cfg.OTLP.Authority,

				// TODO(rfratto): handle auth
			},
		},
	}
}

func toResolver(cfg loadbalancingexporter.ResolverSettings) loadbalancing.ResolverSettings {
	return loadbalancing.ResolverSettings{
		Static:     toStaticResolver(cfg.Static),
		DNS:        toDNSResolver(cfg.DNS),
		Kubernetes: toKubernetesResolver(cfg.K8sSvc),
	}
}

func toStaticResolver(cfg *loadbalancingexporter.StaticResolver) *loadbalancing.StaticResolver {
	if cfg == nil {
		return nil
	}

	return &loadbalancing.StaticResolver{
		Hostnames: cfg.Hostnames,
	}
}

func toDNSResolver(cfg *loadbalancingexporter.DNSResolver) *loadbalancing.DNSResolver {
	if cfg == nil {
		return nil
	}

	return &loadbalancing.DNSResolver{
		Hostname: cfg.Hostname,
		Port:     cfg.Port,
		Interval: cfg.Interval,
		Timeout:  cfg.Timeout,
	}
}

func toKubernetesResolver(cfg *loadbalancingexporter.K8sSvcResolver) *loadbalancing.KubernetesResolver {
	if cfg == nil {
		return nil
	}

	return &loadbalancing.KubernetesResolver{
		Service: cfg.Service,
		Ports:   cfg.Ports,
	}
}

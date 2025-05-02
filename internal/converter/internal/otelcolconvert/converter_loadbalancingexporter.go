package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/loadbalancing"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
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

func (loadbalancingExporterConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()
	overrideHook := func(val interface{}) interface{} {
		switch val.(type) {
		case auth.Handler:
			ext := state.LookupExtension(cfg.(*loadbalancingexporter.Config).Protocol.OTLP.ClientConfig.Auth.AuthenticatorID)
			return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
		case extension.ExtensionHandler:
			ext := state.LookupExtension(*cfg.(*loadbalancingexporter.Config).QueueSettings.StorageID)
			return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
		}
		return common.GetAlloyTypesOverrideHook()(val)
	}

	args := toLoadbalancingExporter(cfg.(*loadbalancingexporter.Config))
	block := common.NewBlockWithOverrideFn([]string{"otelcol", "exporter", "loadbalancing"}, label, args, overrideHook)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toLoadbalancingExporter(cfg *loadbalancingexporter.Config) *loadbalancing.Arguments {
	routingKey := "traceID"
	if cfg.RoutingKey != "" {
		routingKey = cfg.RoutingKey
	}
	return &loadbalancing.Arguments{
		Protocol:   toProtocol(cfg.Protocol),
		Resolver:   toResolver(cfg.Resolver),
		RoutingKey: routingKey,
		Timeout:    cfg.TimeoutSettings.Timeout,
		Queue:      toQueueArguments(cfg.QueueSettings),
		Retry:      toRetryArguments(cfg.BackOffConfig),

		DebugMetrics: common.DefaultValue[loadbalancing.Arguments]().DebugMetrics,
	}
}

func toProtocol(cfg loadbalancingexporter.Protocol) loadbalancing.Protocol {
	var a *auth.Handler
	if cfg.OTLP.ClientConfig.Auth != nil {
		a = &auth.Handler{}
	}

	// Set default value for `balancer_name` to sync up with upstream's
	balancerName := cfg.OTLP.ClientConfig.BalancerName
	if balancerName == "" {
		balancerName = otelcol.DefaultBalancerName
	}

	return loadbalancing.Protocol{
		// NOTE(rfratto): this has a lot of overlap with converting the
		// otlpexporter, but otelcol.exporter.loadbalancing uses custom types to
		// remove unwanted fields.
		OTLP: loadbalancing.OtlpConfig{
			Timeout: cfg.OTLP.TimeoutConfig.Timeout,
			Queue:   toQueueArguments(cfg.OTLP.QueueConfig),
			Retry:   toRetryArguments(cfg.OTLP.RetryConfig),
			Client: loadbalancing.GRPCClientArguments{
				Compression: otelcol.CompressionType(cfg.OTLP.ClientConfig.Compression),

				TLS:       toTLSClientArguments(cfg.OTLP.ClientConfig.TLSSetting),
				Keepalive: toKeepaliveClientArguments(cfg.OTLP.ClientConfig.Keepalive),

				ReadBufferSize:  units.Base2Bytes(cfg.OTLP.ClientConfig.ReadBufferSize),
				WriteBufferSize: units.Base2Bytes(cfg.OTLP.ClientConfig.WriteBufferSize),
				WaitForReady:    cfg.OTLP.ClientConfig.WaitForReady,
				Headers:         toHeadersMap(cfg.OTLP.ClientConfig.Headers),
				BalancerName:    balancerName,
				Authority:       cfg.OTLP.ClientConfig.Authority,

				Authentication: a,
			},
		},
	}
}

func toResolver(cfg loadbalancingexporter.ResolverSettings) loadbalancing.ResolverSettings {
	return loadbalancing.ResolverSettings{
		Static:      toStaticResolver(cfg.Static),
		DNS:         toDNSResolver(cfg.DNS),
		Kubernetes:  toKubernetesResolver(cfg.K8sSvc),
		AWSCloudMap: toAWSCloudMap(cfg.AWSCloudMap),
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
		Service:         cfg.Service,
		Ports:           cfg.Ports,
		Timeout:         cfg.Timeout,
		ReturnHostnames: cfg.ReturnHostnames,
	}
}

func toAWSCloudMap(cfg *loadbalancingexporter.AWSCloudMapResolver) *loadbalancing.AWSCloudMapResolver {
	if cfg == nil {
		return nil
	}

	return &loadbalancing.AWSCloudMapResolver{
		NamespaceName: cfg.NamespaceName,
		ServiceName:   cfg.ServiceName,
		HealthStatus:  string(cfg.HealthStatus),
		Interval:      cfg.Interval,
		Timeout:       cfg.Timeout,
		Port:          cfg.Port,
	}
}

// Package loadbalancing provides an otelcol.exporter.loadbalancing component.
package loadbalancing

import (
	"fmt"
	"time"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelconfigauth "go.opentelemetry.io/collector/config/configauth"
	otelconfiggrpc "go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.loadbalancing",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := loadbalancingexporter.NewFactory()
			//TODO(ptodev): LB exporter cannot yet work with metrics due to a limitation in Alloy:
			// https://github.com/grafana/agent/pull/5684
			// Once the limitation is removed, we may be able to remove the need for exporter.TypeSignal altogether.
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeLogs|exporter.TypeTraces)
		},
	})
}

// Arguments configures the otelcol.exporter.loadbalancing component.
type Arguments struct {
	Protocol   Protocol         `alloy:"protocol,block"`
	Resolver   ResolverSettings `alloy:"resolver,block"`
	RoutingKey string           `alloy:"routing_key,attr,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcol.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var (
	_ exporter.Arguments = Arguments{}
	_ syntax.Defaulter   = &Arguments{}
	_ syntax.Validator   = &Arguments{}
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		RoutingKey: "traceID",
	}
	args.DebugMetrics.SetToDefault()
	args.Protocol.OTLP.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	//TODO(ptodev): Add support for "resource" and "metric" routing keys later.
	// The reason we can't add them yet is that otelcol.exporter.loadbalancing
	// is labeled as "beta", but those routing keys are experimental.
	// We need a way to label otelcol.exporter.loadbalancing as "public-preview"
	// for logs and traces, but "experimental" for metrics.
	switch args.RoutingKey {
	case "service", "traceID":
		// The routing key is valid.
	default:
		return fmt.Errorf("invalid routing key %q", args.RoutingKey)
	}
	return nil
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &loadbalancingexporter.Config{
		Protocol:   args.Protocol.Convert(),
		Resolver:   args.Resolver.Convert(),
		RoutingKey: args.RoutingKey,
	}, nil
}

// Protocol holds the individual protocol-specific settings. Only OTLP is supported at the moment.
type Protocol struct {
	OTLP OtlpConfig `alloy:"otlp,block"`
}

func (protocol Protocol) Convert() loadbalancingexporter.Protocol {
	return loadbalancingexporter.Protocol{
		OTLP: protocol.OTLP.Convert(),
	}
}

// OtlpConfig defines the config for an OTLP exporter
type OtlpConfig struct {
	Timeout time.Duration          `alloy:"timeout,attr,optional"`
	Queue   otelcol.QueueArguments `alloy:"queue,block,optional"`
	Retry   otelcol.RetryArguments `alloy:"retry,block,optional"`
	// Most of the time, the user will not have to set anything in the client block.
	// However, the block should not be "optional" so that the defaults are populated.
	Client GRPCClientArguments `alloy:"client,block"`
}

func (oc *OtlpConfig) SetToDefault() {
	*oc = OtlpConfig{
		Timeout: otelcol.DefaultTimeout,
	}
	oc.Client.SetToDefault()
	oc.Retry.SetToDefault()
	oc.Queue.SetToDefault()
}

func (oc OtlpConfig) Convert() otlpexporter.Config {
	return otlpexporter.Config{
		TimeoutSettings: exporterhelper.TimeoutSettings{
			Timeout: oc.Timeout,
		},
		QueueConfig:  *oc.Queue.Convert(),
		RetryConfig:  *oc.Retry.Convert(),
		ClientConfig: *oc.Client.Convert(),
	}
}

// ResolverSettings defines the configurations for the backend resolver
type ResolverSettings struct {
	Static     *StaticResolver     `alloy:"static,block,optional"`
	DNS        *DNSResolver        `alloy:"dns,block,optional"`
	Kubernetes *KubernetesResolver `alloy:"kubernetes,block,optional"`
	// TODO: add AWSCloudMap
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/27588
	//AWSCloudMap *AWSCloudMapResolver `alloy:"aws_cloud_map,block,optional"`
}

func (resolverSettings ResolverSettings) Convert() loadbalancingexporter.ResolverSettings {
	res := loadbalancingexporter.ResolverSettings{}

	if resolverSettings.Static != nil {
		staticResolver := resolverSettings.Static.Convert()
		res.Static = &staticResolver
	}

	if resolverSettings.DNS != nil {
		dnsResolver := resolverSettings.DNS.Convert()
		res.DNS = &dnsResolver
	}

	if resolverSettings.Kubernetes != nil {
		kubernetesResolver := resolverSettings.Kubernetes.Convert()
		res.K8sSvc = &kubernetesResolver
	}

	return res
}

// StaticResolver defines the configuration for the resolver providing a fixed list of backends
type StaticResolver struct {
	Hostnames []string `alloy:"hostnames,attr"`
}

func (staticResolver StaticResolver) Convert() loadbalancingexporter.StaticResolver {
	return loadbalancingexporter.StaticResolver{
		Hostnames: staticResolver.Hostnames,
	}
}

// DNSResolver defines the configuration for the DNS resolver
type DNSResolver struct {
	Hostname string        `alloy:"hostname,attr"`
	Port     string        `alloy:"port,attr,optional"`
	Interval time.Duration `alloy:"interval,attr,optional"`
	Timeout  time.Duration `alloy:"timeout,attr,optional"`
}

var _ syntax.Defaulter = &DNSResolver{}

// DefaultDNSResolver holds default values for DNSResolver.
var DefaultDNSResolver = DNSResolver{
	Port:     "4317",
	Interval: 5 * time.Second,
	Timeout:  1 * time.Second,
}

// SetToDefault implements syntax.Defaulter.
func (args *DNSResolver) SetToDefault() {
	*args = DefaultDNSResolver
}

func (dnsResolver *DNSResolver) Convert() loadbalancingexporter.DNSResolver {
	return loadbalancingexporter.DNSResolver{
		Hostname: dnsResolver.Hostname,
		Port:     dnsResolver.Port,
		Interval: dnsResolver.Interval,
		Timeout:  dnsResolver.Timeout,
	}
}

// KubernetesResolver defines the configuration for the k8s resolver
type KubernetesResolver struct {
	Service string        `alloy:"service,attr"`
	Ports   []int32       `alloy:"ports,attr,optional"`
	Timeout time.Duration `alloy:"timeout,attr,optional"`
}

var _ syntax.Defaulter = &KubernetesResolver{}

// SetToDefault implements syntax.Defaulter.
func (args *KubernetesResolver) SetToDefault() {
	if args == nil {
		args = &KubernetesResolver{}
	}
	args.Ports = []int32{4317}
	args.Timeout = 1 * time.Second
}

func (k8sSvcResolver *KubernetesResolver) Convert() loadbalancingexporter.K8sSvcResolver {
	return loadbalancingexporter.K8sSvcResolver{
		Service: k8sSvcResolver.Service,
		Ports:   append([]int32{}, k8sSvcResolver.Ports...),
		Timeout: k8sSvcResolver.Timeout,
	}
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return args.Protocol.OTLP.Client.Extensions()
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcol.DebugMetricsArguments {
	return args.DebugMetrics
}

// GRPCClientArguments is the same as otelcol.GRPCClientArguments, but without an "endpoint" attribute
type GRPCClientArguments struct {
	Compression otelcol.CompressionType `alloy:"compression,attr,optional"`

	TLS       otelcol.TLSClientArguments        `alloy:"tls,block,optional"`
	Keepalive *otelcol.KeepaliveClientArguments `alloy:"keepalive,block,optional"`

	ReadBufferSize  units.Base2Bytes  `alloy:"read_buffer_size,attr,optional"`
	WriteBufferSize units.Base2Bytes  `alloy:"write_buffer_size,attr,optional"`
	WaitForReady    bool              `alloy:"wait_for_ready,attr,optional"`
	Headers         map[string]string `alloy:"headers,attr,optional"`
	BalancerName    string            `alloy:"balancer_name,attr,optional"`
	Authority       string            `alloy:"authority,attr,optional"`

	// Auth is a binding to an otelcol.auth.* component extension which handles
	// authentication.
	Auth *auth.Handler `alloy:"auth,attr,optional"`
}

var _ syntax.Defaulter = &GRPCClientArguments{}

// Convert converts args into the upstream type.
func (args *GRPCClientArguments) Convert() *otelconfiggrpc.ClientConfig {
	if args == nil {
		return nil
	}

	opaqueHeaders := make(map[string]configopaque.String)
	for headerName, headerVal := range args.Headers {
		opaqueHeaders[headerName] = configopaque.String(headerVal)
	}

	// Configure the authentication if args.Auth is set.
	var auth *otelconfigauth.Authentication
	if args.Auth != nil {
		auth = &otelconfigauth.Authentication{AuthenticatorID: args.Auth.ID}
	}

	balancerName := args.BalancerName
	if balancerName == "" {
		balancerName = otelcol.DefaultBalancerName
	}

	return &otelconfiggrpc.ClientConfig{
		Compression: args.Compression.Convert(),

		TLSSetting: *args.TLS.Convert(),
		Keepalive:  args.Keepalive.Convert(),

		ReadBufferSize:  int(args.ReadBufferSize),
		WriteBufferSize: int(args.WriteBufferSize),
		WaitForReady:    args.WaitForReady,
		Headers:         opaqueHeaders,
		BalancerName:    balancerName,
		Authority:       args.Authority,

		Auth: auth,
	}
}

// Extensions exposes extensions used by args.
func (args *GRPCClientArguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	m := make(map[otelcomponent.ID]otelextension.Extension)
	if args.Auth != nil {
		m[args.Auth.ID] = args.Auth.Extension
	}
	return m
}

// SetToDefault implements syntax.Defaulter.
func (args *GRPCClientArguments) SetToDefault() {
	*args = GRPCClientArguments{
		Headers:         map[string]string{},
		Compression:     otelcol.CompressionTypeGzip,
		WriteBufferSize: 512 * 1024,
		BalancerName:    otelcol.DefaultBalancerName,
	}
}

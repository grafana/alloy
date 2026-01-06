// Package loadbalancing provides an otelcol.exporter.loadbalancing component.
package loadbalancing

import (
	"fmt"
	"maps"
	"time"

	"github.com/alecthomas/units"
	"github.com/aws/aws-sdk-go-v2/service/servicediscovery/types"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelconfigauth "go.opentelemetry.io/collector/config/configauth"
	otelconfiggrpc "go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.loadbalancing",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := loadbalancingexporter.NewFactory()

			// As per https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/loadbalancingexporter/README.md
			// metrics is considered "development" stability level

			typeSignalFunc := func(opts component.Options, args component.Arguments) exporter.TypeSignal {
				myArgs := args.(Arguments)
				var typeSignal exporter.TypeSignal
				switch myArgs.RoutingKey {
				case "traceID":
					typeSignal = exporter.TypeLogs | exporter.TypeTraces
				case "service":
					if opts.MinStability.Permits(featuregate.StabilityExperimental) {
						typeSignal = exporter.TypeLogs | exporter.TypeTraces | exporter.TypeMetrics
					} else {
						level.Warn(opts.Logger).Log("msg", "disabling metrics exporter as stability level does not allow it")
						typeSignal = exporter.TypeLogs | exporter.TypeTraces
					}
				case "resource", "metric", "streamID":
					if opts.MinStability.Permits(featuregate.StabilityExperimental) {
						typeSignal = exporter.TypeMetrics
					} else {
						level.Warn(opts.Logger).Log("msg", "disabling metrics exporter as stability level does not allow it")
					}
				}
				return typeSignal
			}
			return exporter.New(opts, fact, args.(Arguments), typeSignalFunc)
		},
	})
}

// Arguments configures the otelcol.exporter.loadbalancing component.
type Arguments struct {
	Protocol   Protocol         `alloy:"protocol,block"`
	Resolver   ResolverSettings `alloy:"resolver,block"`
	RoutingKey string           `alloy:"routing_key,attr,optional"`

	Timeout time.Duration          `alloy:"timeout,attr,optional"`
	Retry   otelcol.RetryArguments `alloy:"retry_on_failure,block,optional"`
	Queue   otelcol.QueueArguments `alloy:"sending_queue,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
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
	// Do not set these two to their default values.
	// Upstream doesn't do that for backwards compatibility.
	// args.Retry.SetToDefault()
	// args.Queue.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	// Allow routing keys for all signal types. Metrics exporter will be disabled
	// if stability level is above experimental
	switch args.RoutingKey {
	case "service", "traceID", "resource", "metric", "streamID":
		// The routing key is valid.
	default:
		return fmt.Errorf("invalid routing key %q", args.RoutingKey)
	}

	if err := args.Resolver.AWSCloudMap.Validate(); err != nil {
		return err
	}

	return nil
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	protocol, err := args.Protocol.Convert()
	if err != nil {
		return nil, err
	}

	q, err := args.Queue.Convert()
	if err != nil {
		return nil, err
	}

	return &loadbalancingexporter.Config{
		Protocol:   *protocol,
		Resolver:   args.Resolver.Convert(),
		RoutingKey: args.RoutingKey,
		TimeoutSettings: exporterhelper.TimeoutConfig{
			Timeout: args.Timeout,
		},
		BackOffConfig: *args.Retry.Convert(),
		QueueSettings: q,
	}, nil
}

// Protocol holds the individual protocol-specific settings. Only OTLP is supported at the moment.
type Protocol struct {
	OTLP OtlpConfig `alloy:"otlp,block"`
}

func (protocol Protocol) Convert() (*loadbalancingexporter.Protocol, error) {
	otlp, err := protocol.OTLP.Convert()
	if err != nil {
		return nil, err
	}
	return &loadbalancingexporter.Protocol{
		OTLP: *otlp,
	}, nil
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

func (oc OtlpConfig) Convert() (*otlpexporter.Config, error) {
	clientConfig, err := oc.Client.Convert()
	if err != nil {
		return nil, err
	}
	q, err := oc.Queue.Convert()
	if err != nil {
		return nil, err
	}

	return &otlpexporter.Config{
		TimeoutConfig: exporterhelper.TimeoutConfig{
			Timeout: oc.Timeout,
		},
		QueueConfig:  q,
		RetryConfig:  *oc.Retry.Convert(),
		ClientConfig: *clientConfig,
	}, nil
}

// ResolverSettings defines the configurations for the backend resolver
type ResolverSettings struct {
	Static      *StaticResolver      `alloy:"static,block,optional"`
	DNS         *DNSResolver         `alloy:"dns,block,optional"`
	Kubernetes  *KubernetesResolver  `alloy:"kubernetes,block,optional"`
	AWSCloudMap *AWSCloudMapResolver `alloy:"aws_cloud_map,block,optional"`
}

func (resolverSettings ResolverSettings) Convert() loadbalancingexporter.ResolverSettings {
	res := loadbalancingexporter.ResolverSettings{}

	res.Static = resolverSettings.Static.Convert()
	res.DNS = resolverSettings.DNS.Convert()
	res.K8sSvc = resolverSettings.Kubernetes.Convert()
	res.AWSCloudMap = resolverSettings.AWSCloudMap.Convert()

	return res
}

// StaticResolver defines the configuration for the resolver providing a fixed list of backends
type StaticResolver struct {
	Hostnames []string `alloy:"hostnames,attr"`
}

func (r *StaticResolver) Convert() configoptional.Optional[loadbalancingexporter.StaticResolver] {
	if r == nil {
		return configoptional.None[loadbalancingexporter.StaticResolver]()
	}

	return configoptional.Some(loadbalancingexporter.StaticResolver{
		Hostnames: r.Hostnames,
	})
}

// DNSResolver defines the configuration for the DNS resolver
type DNSResolver struct {
	Hostname string        `alloy:"hostname,attr"`
	Port     string        `alloy:"port,attr,optional"`
	Interval time.Duration `alloy:"interval,attr,optional"`
	Timeout  time.Duration `alloy:"timeout,attr,optional"`
}

var _ syntax.Defaulter = &DNSResolver{}

// SetToDefault implements syntax.Defaulter.
func (r *DNSResolver) SetToDefault() {
	*r = DNSResolver{
		Port:     "4317",
		Interval: 5 * time.Second,
		Timeout:  1 * time.Second,
	}
}

func (r *DNSResolver) Convert() configoptional.Optional[loadbalancingexporter.DNSResolver] {
	if r == nil {
		return configoptional.None[loadbalancingexporter.DNSResolver]()
	}

	return configoptional.Some(loadbalancingexporter.DNSResolver{
		Hostname: r.Hostname,
		Port:     r.Port,
		Interval: r.Interval,
		Timeout:  r.Timeout,
	})
}

// KubernetesResolver defines the configuration for the k8s resolver
type KubernetesResolver struct {
	Service         string        `alloy:"service,attr"`
	Ports           []int32       `alloy:"ports,attr,optional"`
	Timeout         time.Duration `alloy:"timeout,attr,optional"`
	ReturnHostnames bool          `alloy:"return_hostnames,attr,optional"`
}

var _ syntax.Defaulter = &KubernetesResolver{}

// SetToDefault implements syntax.Defaulter.
func (r *KubernetesResolver) SetToDefault() {
	*r = KubernetesResolver{
		Ports:   []int32{4317},
		Timeout: 1 * time.Second,
	}
}

func (r *KubernetesResolver) Convert() configoptional.Optional[loadbalancingexporter.K8sSvcResolver] {
	if r == nil {
		return configoptional.None[loadbalancingexporter.K8sSvcResolver]()
	}

	return configoptional.Some(loadbalancingexporter.K8sSvcResolver{
		Service:         r.Service,
		Ports:           append([]int32{}, r.Ports...),
		Timeout:         r.Timeout,
		ReturnHostnames: r.ReturnHostnames,
	})
}

// Possible values for "health_status"
const (
	HealthStatusFilterHealthy          string = "HEALTHY"
	HealthStatusFilterUnhealthy        string = "UNHEALTHY"
	HealthStatusFilterAll              string = "ALL"
	HealthStatusFilterHealthyOrElseAll string = "HEALTHY_OR_ELSE_ALL"
)

// AWSCloudMapResolver allows users to use this exporter when
// using ECS over EKS in an AWS infrastructure.
type AWSCloudMapResolver struct {
	NamespaceName string        `alloy:"namespace,attr"`
	ServiceName   string        `alloy:"service_name,attr"`
	HealthStatus  string        `alloy:"health_status,attr,optional"`
	Interval      time.Duration `alloy:"interval,attr,optional"`
	Timeout       time.Duration `alloy:"timeout,attr,optional"`
	Port          *uint16       `alloy:"port,attr,optional"`
}

var _ syntax.Defaulter = &AWSCloudMapResolver{}

// SetToDefault implements syntax.Defaulter.
func (r *AWSCloudMapResolver) SetToDefault() {
	*r = AWSCloudMapResolver{
		Interval:     30 * time.Second,
		Timeout:      5 * time.Second,
		HealthStatus: HealthStatusFilterHealthy,
	}
}

func (r *AWSCloudMapResolver) Validate() error {
	if r == nil {
		return nil
	}

	switch types.HealthStatusFilter(r.HealthStatus) {
	case types.HealthStatusFilterAll,
		types.HealthStatusFilterHealthy,
		types.HealthStatusFilterUnhealthy,
		types.HealthStatusFilterHealthyOrElseAll:
		return nil
	default:
		return fmt.Errorf("invalid health status %q", r.HealthStatus)
	}
}

func (r *AWSCloudMapResolver) Convert() configoptional.Optional[loadbalancingexporter.AWSCloudMapResolver] {
	if r == nil {
		return configoptional.None[loadbalancingexporter.AWSCloudMapResolver]()
	}

	// Deep copy the port
	var port *uint16
	if r.Port != nil {
		portNum := *r.Port
		port = &portNum
	}

	return configoptional.Some(loadbalancingexporter.AWSCloudMapResolver{
		NamespaceName: r.NamespaceName,
		ServiceName:   r.ServiceName,
		HealthStatus:  types.HealthStatusFilter(r.HealthStatus),
		Interval:      r.Interval,
		Timeout:       r.Timeout,
		Port:          port,
	})
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	ext := args.Protocol.OTLP.Client.Extensions()
	maps.Copy(ext, args.Queue.Extensions())
	maps.Copy(ext, args.Protocol.OTLP.Queue.Extensions())
	return ext
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements exporter.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
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
	// alloy name is auth instead of authentication to not break user interface compatibility.
	Authentication *auth.Handler `alloy:"auth,attr,optional"`
}

var _ syntax.Defaulter = &GRPCClientArguments{}

// Convert converts args into the upstream type.
func (args *GRPCClientArguments) Convert() (*otelconfiggrpc.ClientConfig, error) {
	if args == nil {
		return nil, nil
	}

	opaqueHeaders := configopaque.MapList{}
	for headerName, headerVal := range args.Headers {
		opaqueHeaders.Set(headerName, configopaque.String(headerVal))
	}

	// Configure the authentication if args.Auth is set.
	var authentication configoptional.Optional[otelconfigauth.Config]
	if args.Authentication != nil {
		ext, err := args.Authentication.GetExtension(auth.Client)
		if err != nil {
			return nil, err
		}

		authentication = configoptional.Some(otelconfigauth.Config{AuthenticatorID: ext.ID})
	}

	balancerName := args.BalancerName
	if balancerName == "" {
		balancerName = otelcol.DefaultBalancerName
	}

	return &otelconfiggrpc.ClientConfig{
		Compression: args.Compression.Convert(),

		TLS:       *args.TLS.Convert(),
		Keepalive: args.Keepalive.Convert(),

		ReadBufferSize:  int(args.ReadBufferSize),
		WriteBufferSize: int(args.WriteBufferSize),
		WaitForReady:    args.WaitForReady,
		Headers:         opaqueHeaders,
		BalancerName:    balancerName,
		Authority:       args.Authority,

		Auth: authentication,
	}, nil
}

// Extensions exposes extensions used by args.
func (args *GRPCClientArguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	m := make(map[otelcomponent.ID]otelcomponent.Component)
	if args.Authentication != nil {
		ext, err := args.Authentication.GetExtension(auth.Client)
		if err != nil {
			return m
		}
		m[ext.ID] = ext.Extension
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

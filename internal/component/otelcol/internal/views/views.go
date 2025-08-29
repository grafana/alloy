package views

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.13.0"
)

var (
	grpcScope = "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	// grpcUnacceptableKeyValues is a list of high cardinality grpc attributes that should be filtered out.
	grpcUnacceptableKeyValues = []attribute.KeyValue{
		attribute.String(string(semconv.NetSockPeerAddrKey), ""),
		attribute.String(string(semconv.NetSockPeerPortKey), ""),
		attribute.String(string(semconv.NetSockPeerNameKey), ""),
	}

	httpScope = "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	// httpUnacceptableKeyValues is a list of high cardinality http attributes that should be filtered out.
	httpUnacceptableKeyValues = []attribute.KeyValue{
		attribute.String(string(semconv.NetHostNameKey), ""),
		attribute.String(string(semconv.NetHostPortKey), ""),
		attribute.String(string(semconv.NetSockPeerPortKey), ""),
		attribute.String(string(semconv.NetSockPeerAddrKey), ""),
		attribute.String(string(semconv.HTTPClientIPKey), ""),
	}
)

func cardinalityFilter(kvs ...attribute.KeyValue) attribute.Filter {
	filter := attribute.NewSet(kvs...)
	return func(kv attribute.KeyValue) bool {
		return !filter.HasValue(kv.Key)
	}
}

// DropHighCardinalityServerAttributes drops certain high cardinality attributes from grpc/http server metrics
//
// This is a fix to an upstream issue:
// https://github.com/open-telemetry/opentelemetry-go-contrib/issues/3765
// The long-term solution for the Collector is to set view settings in the Collector config:
// https://github.com/open-telemetry/opentelemetry-collector/issues/7517#issuecomment-1511168350
// In the future, when Collector supports such config, we may want to support similar view settings in Alloy.
func DropHighCardinalityServerAttributes() []metric.View {
	var views []metric.View

	views = append(views, metric.NewView(
		metric.Instrument{Scope: instrumentation.Scope{Name: grpcScope}},
		metric.Stream{AttributeFilter: cardinalityFilter(grpcUnacceptableKeyValues...)}))

	views = append(views, metric.NewView(
		metric.Instrument{Scope: instrumentation.Scope{Name: httpScope}},
		metric.Stream{AttributeFilter: cardinalityFilter(httpUnacceptableKeyValues...)},
	))

	return views
}

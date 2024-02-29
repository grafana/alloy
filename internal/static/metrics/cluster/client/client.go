package client

import (
	"flag"
	"io"
	"reflect"

	"github.com/grafana/agent/internal/static/agentproto"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/dskit/grpcclient"
	"github.com/grafana/dskit/middleware"
	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ScrapingServiceClient wraps agentproto.ScrapingServiceClient with a Close method.
type ScrapingServiceClient interface {
	agentproto.ScrapingServiceClient
	io.Closer
}

var (
	// DefaultConfig provides default Config values.
	DefaultConfig = *util.DefaultConfigFromFlags(&Config{}).(*Config)
)

// Config controls how scraping service clients are created.
type Config struct {
	GRPCClientConfig grpcclient.Config `yaml:"grpc_client_config,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

func (c Config) IsZero() bool {
	return reflect.DeepEqual(c, Config{}) || reflect.DeepEqual(c, DefaultConfig)
}

// RegisterFlags registers flags to the provided flag set.
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	c.RegisterFlagsWithPrefix("prometheus.", f)
	c.RegisterFlagsWithPrefix("metrics.", f)
}

// RegisterFlagsWithPrefix registers flags to the provided flag set with the
// specified prefix.
func (c *Config) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	c.GRPCClientConfig.RegisterFlagsWithPrefix(prefix+"service-client", f)
}

// New returns a new scraping service client.
func New(cfg Config, addr string) (ScrapingServiceClient, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(cfg.GRPCClientConfig.CallOptions()...),
	}
	grpcDialOpts, err := cfg.GRPCClientConfig.DialOption(instrumentation())
	if err != nil {
		return nil, err
	}
	opts = append(opts, grpcDialOpts...)
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}

	return struct {
		agentproto.ScrapingServiceClient
		io.Closer
	}{
		ScrapingServiceClient: agentproto.NewScrapingServiceClient(conn),
		Closer:                conn,
	}, nil
}

func instrumentation() ([]grpc.UnaryClientInterceptor, []grpc.StreamClientInterceptor) {
	unary := []grpc.UnaryClientInterceptor{
		otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer()),
		middleware.ClientUserHeaderInterceptor,
	}
	stream := []grpc.StreamClientInterceptor{
		otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer()),
		middleware.StreamClientUserHeaderInterceptor,
	}
	return unary, stream
}

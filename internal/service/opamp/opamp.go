package opamp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/alloy/logging/level"
	"github.com/grafana/alloy/internal/alloyseed"
	"github.com/grafana/alloy/internal/build"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service"

	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
)

// ServiceName defines the name used for the opamp service.
const ServiceName = "opamp"

// Arguments holds runtime settings for the remotecfg service.
type Arguments struct {
	URL string `alloy:"url,attr,optional"`
}

// GetDefaultArguments populates the default values for the Arguments struct.
func GetDefaultArguments() Arguments {
	return Arguments{}
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = GetDefaultArguments()
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	return nil
}

var _ service.Service = (*Service)(nil)

type Service struct {
	currentArgs *Arguments
	updateCh    chan struct{}
	mut         sync.Mutex
	logger      log.Logger
}

func New(logger log.Logger) *Service {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	return &Service{
		updateCh: make(chan struct{}, 1),
		logger:   logger,
	}
}

// Data implements service.Service. It returns nil, as the otel service does
// not have any runtime data.
func (*Service) Data() any {
	return nil
}

// Definition implements service.Service.
func (*Service) Definition() service.Definition {
	return service.Definition{
		Name:       ServiceName,
		ConfigType: Arguments{},
		DependsOn:  []string{},
		Stability:  featuregate.StabilityExperimental,
	}
}

type logAdapter struct {
	inner log.Logger
}

func (l logAdapter) Debugf(ctx context.Context, format string, v ...interface{}) {
	level.Debug(l.inner).Log("msg", fmt.Sprintf(format, v...))
}
func (l logAdapter) Errorf(ctx context.Context, format string, v ...interface{}) {
	level.Error(l.inner).Log("msg", fmt.Sprintf(format, v...))
}

// Run implements service.Service.
func (s *Service) Run(ctx context.Context, host service.Host) error {
	var args *Arguments
	var opCli client.OpAMPClient
	started := false
	ulid := alloyseed.Get().Ulid()
	hostName := getHost()
	for {
		select {
		case <-ctx.Done():
			if started && opCli != nil {
				err := opCli.Stop(ctx)
				if err != nil {
					return err
				}
			}
			return nil
		case <-s.updateCh:
			if started && opCli != nil {
				err := opCli.Stop(ctx)
				if err != nil {
					return err
				}
			}
			started = false
			s.mut.Lock()
			args = s.currentArgs
			s.mut.Unlock()
			if args == nil || args.URL == "" {
				continue
			}
			// todo: add log context
			opCli = client.NewWebSocket(logAdapter{inner: s.logger})
			opCli = client.NewHTTP(logAdapter{inner: s.logger})
			err := opCli.SetAgentDescription(&protobufs.AgentDescription{
				IdentifyingAttributes: []*protobufs.KeyValue{

					{Key: "service.instance.id", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: ulid}}},
					{Key: "service.name", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "alloy"}}},
					{Key: "service.version", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: build.Version}}},
				},
				NonIdentifyingAttributes: []*protobufs.KeyValue{
					{Key: "os.type", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: runtime.GOOS}}},
					{Key: "host.arch", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: runtime.GOARCH}}},
					{Key: "host.name", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: hostName}}},
				},
			})
			if err != nil {
				return err
			}
			err = opCli.Start(ctx, types.StartSettings{
				InstanceUid:    ulid,
				OpAMPServerURL: args.URL,
				Header:         http.Header{"X-Scope-OrgID": []string{"123"}},
				Capabilities:   protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig,
			})
			if err != nil {
				return err
			}
			level.Error(s.logger).Log("msg", "Opamp success?")
			started = true
		}
	}
}

// Update implements service.Service.
func (s *Service) Update(newConfig any) error {
	level.Info(s.logger).Log("msg", "UPDATE")
	newArgs := newConfig.(Arguments)
	s.mut.Lock()
	s.currentArgs = &newArgs
	s.mut.Unlock()
	select {
	case s.updateCh <- struct{}{}:
	default:
	}
	return nil
}

func getHost() string {
	hostname := os.Getenv("HOSTNAME")
	if hostname != "" {
		return hostname
	}

	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

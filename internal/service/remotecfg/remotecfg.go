package remotecfg

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/go-kit/log"
	collectorv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1"
	"github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1/collectorv1connect"
	"github.com/grafana/alloy/internal/build"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/featuregate"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/prometheus/client_golang/prometheus"
)

// Service implements a service for remote configuration.
// The default value of ch is nil; this means it will block forever if the
// remotecfg service is not configured. In addition, we're keeping track of
// the ticker so we can avoid leaking goroutines.
type Service struct {
	opts Options
	args Arguments

	mut         sync.RWMutex
	metrics     *metrics
	apiClient   collectorv1connect.CollectorServiceClient
	systemAttrs map[string]string
	attrs       map[string]string
	cm          *configManager

	// runCtx is the context from Run method, used for service lifecycle operations
	runCtx context.Context
}

// ServiceName defines the name used for the remotecfg service.
const ServiceName = "remotecfg"
const reservedAttributeNamespace = "collector"
const namespaceDelimiter = "."

// Options are used to configure the remotecfg service. Options are
// constant for the lifetime of the remotecfg service.
type Options struct {
	Logger      log.Logger            // Where to send logs.
	StoragePath string                // Where to cache configuration on-disk.
	ConfigPath  string                // Where the root config file is.
	Metrics     prometheus.Registerer // Where to send metrics to.
}

// New returns a new instance of the remotecfg service.
func New(opts Options) (*Service, error) {
	remotecfgPath := filepath.Join(opts.StoragePath, ServiceName)
	err := os.MkdirAll(remotecfgPath, 0750)
	if err != nil {
		opts.Logger.Log("level", "error", "msg", "failed to create remotecfg storage directory", "path", remotecfgPath, "err", err)
		return nil, err
	}

	metrics := registerMetrics(opts.Metrics)

	svc := &Service{
		opts:        opts,
		systemAttrs: getSystemAttributes(),
		metrics:     metrics,
		cm:          newConfigManager(metrics, opts.Logger, remotecfgPath, opts.ConfigPath),
	}

	return svc, nil
}

// Data returns an instance of [Data]. Calls to Data are cachable by the
// caller.
// Data must only be called after Run.
func (s *Service) Data() any {
	// While the contract specifies that Data must be called after Run,
	// the other services start in parallel and Cluster attempts to access
	// the controller via Data, this locking prevents a race.
	s.mut.RLock()
	defer s.mut.RUnlock()

	if s.cm == nil {
		return Data{Host: nil}
	}

	if s.cm.getController() == nil {
		return Data{Host: nil}
	}

	host := s.cm.getController().(alloy_runtime.ServiceController).GetHost()
	return Data{Host: host}
}

// Data includes information associated with the HTTP service.
type Data struct {
	// Host exposes the Host of the isolated controller that is created by the
	// remotecfg service.
	Host service.Host
}

// Definition returns the definition of the remotecfg service.
func (s *Service) Definition() service.Definition {
	return service.Definition{
		Name:       ServiceName,
		ConfigType: Arguments{},
		DependsOn:  nil, // remotecfg has no dependencies.
		Stability:  featuregate.StabilityGenerallyAvailable,
	}
}

var _ service.Service = (*Service)(nil)

// Run implements [service.Service] and starts the remotecfg service. It will
// run until the provided context is canceled or there is a fatal error.
func (s *Service) Run(ctx context.Context, host service.Host) error {
	// Store the Run context for use in other operations
	s.mut.Lock()
	s.runCtx = ctx
	s.mut.Unlock()

	s.cm.setController(host.NewController(ServiceName))

	defer func() {
		s.cm.cleanup()
		s.mut.Lock()
		s.runCtx = nil
		s.mut.Unlock()
	}()

	s.fetchLoadConfig(true) // Allow cache fallback on startup
	err := s.registerCollector()
	if err != nil && err != errNoopClient {
		s.opts.Logger.Log("level", "error", "msg", "failed to register collector during service startup", "err", err)
		return err
	}

	// Run the service's own controller.
	go func() {
		s.cm.getController().Run(ctx)
	}()

	for {
		select {
		case <-s.cm.getTickerC():
			s.fetchLoadConfig(false) // Don't reload cache during periodic polling
		case <-s.cm.getUpdateTickerChan():
			s.cm.getTicker().Reset(s.cm.getPollFrequency())
		case <-ctx.Done():
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			if err := s.unregisterCollector(cleanupCtx); err != nil {
				s.opts.Logger.Log("level", "error", "msg", "failed to unregister collector during service shutdown", "err", err)
			}
			return nil
		}
	}
}

// Update implements [service.Service] and applies settings.
func (s *Service) Update(newConfig any) error {
	newArgs := newConfig.(Arguments)

	// We either never set the block on the first place, or recently removed
	// it. Make sure we stop everything gracefully before returning.
	if newArgs.URL == "" {
		s.updateHandleEmptyUrl(newArgs)
		return nil
	}

	err := s.updateHandleArgs(newArgs)
	if err != nil {
		s.opts.Logger.Log("level", "error", "msg", "failed to update service arguments", "err", err)
		return err
	}

	err = s.registerCollector()
	if err != nil {
		s.opts.Logger.Log("level", "error", "msg", "failed to register collector during configuration update", "err", err)
		return err
	}

	// If we've already called Run, then immediately trigger an API call with
	// the updated Arguments, and/or fall back to the updated cache location.
	if s.cm.getController() != nil && s.cm.getController().Ready() {
		s.fetchLoadConfig(true) // Allow cache fallback when config is updated
	}

	return nil
}

// updateHandleEmptyUrl handles impacts of changes to the arguments when the URL is empty.
func (s *Service) updateHandleEmptyUrl(args Arguments) {
	s.mut.Lock()
	defer s.mut.Unlock()

	s.apiClient = newNoopClient()
	s.args.HTTPClientConfig = config.CloneDefaultHTTPClientConfig()
	s.args = args

	// Clean up the old configManager before creating a new one
	var remotecfgPath, configPath string
	if s.cm != nil {
		remotecfgPath = s.cm.getRemotecfgPath()
		configPath = s.cm.getConfigPath()
		s.cm.cleanup()
	}
	s.cm = newConfigManager(s.metrics, s.opts.Logger, remotecfgPath, configPath)
	s.cm.setLastLoadedCfgHash("")
	s.cm.setLastReceivedCfgHash("")
	s.cm.setPollFrequency(disablePollingFrequency)
}

// updateHandleArgs handles impacts of changes to the arguments.
func (s *Service) updateHandleArgs(newArgs Arguments) error {
	// Detect changes in the arguments which point us at a new cache location.
	hash, err := newArgs.Hash()
	if err != nil {
		s.opts.Logger.Log("level", "error", "msg", "failed to compute arguments hash", "err", err)
		return err
	}

	// Check if we need to create a new API client
	s.mut.RLock()
	needsNewClient := !reflect.DeepEqual(s.args.HTTPClientConfig, newArgs.HTTPClientConfig) || s.args.URL != newArgs.URL
	s.mut.RUnlock()

	// Create new API client if needed
	if needsNewClient {
		newAPIClient, err := createAPIClient(newArgs, s.metrics)
		if err != nil {
			s.opts.Logger.Log("level", "error", "msg", "failed to create API client", "url", newArgs.URL, "err", err)
			return err
		}
		s.mut.Lock()
		s.apiClient = newAPIClient
		s.mut.Unlock()
	}

	// Now acquire write lock only for state updates (fast operations)
	s.mut.Lock()
	defer s.mut.Unlock()

	// Update cache location hash
	s.cm.setArgsHash(hash)

	// Update the poll frequency
	s.cm.setPollFrequency(newArgs.PollFrequency)

	// Combine the new attributes on top of the system attributes
	s.attrs = maps.Clone(s.systemAttrs)
	maps.Copy(s.attrs, newArgs.Attributes)

	// Update the args as the last step to avoid polluting any comparisons
	s.args = newArgs

	return nil
}

// fetchLoadConfig attempts to read configuration from the API and the local cache
// and then parse/load their contents in order of preference.
// If allowCacheFallback is false, it will not attempt to load from cache on remote failure.
func (s *Service) fetchLoadConfig(allowCacheFallback bool) {
	if !s.isEnabled() {
		return
	}

	s.cm.fetchLoadConfig(s.getConfig, allowCacheFallback)
}

func (s *Service) getConfig() (*collectorv1.GetConfigResponse, error) {
	s.mut.RLock()
	defer s.mut.RUnlock()

	response, err := s.apiClient.GetConfig(s.getContext(), &connect.Request[collectorv1.GetConfigRequest]{
		Msg: &collectorv1.GetConfigRequest{
			Id:                 s.args.ID,
			LocalAttributes:    s.attrs,
			Hash:               s.cm.getRemoteHash(),
			RemoteConfigStatus: s.cm.getRemoteConfigStatusForRequest(),
			EffectiveConfig:    s.cm.getEffectiveConfigForRequest(),
		},
	})

	if err != nil {
		// Don't log error or reset status for "not modified" responses
		if !errors.Is(err, errNotModified) {
			// Reset lastSentConfigStatus and lastSentEffectiveConfig since the API request failed
			// and they weren't actually sent
			s.cm.resetLastSentConfigStatus()
			s.cm.resetLastSentEffectiveConfig()
			s.opts.Logger.Log("level", "error", "msg", "failed to get configuration from remote server", "id", s.args.ID, "err", err)
		}
		return nil, err
	}
	return response.Msg, nil
}

func (s *Service) registerCollector() error {
	s.mut.RLock()
	defer s.mut.RUnlock()

	_, err := s.apiClient.RegisterCollector(s.getContext(), &connect.Request[collectorv1.RegisterCollectorRequest]{
		Msg: &collectorv1.RegisterCollectorRequest{
			Id:              s.args.ID,
			LocalAttributes: s.attrs,
			Name:            s.args.Name,
		},
	})

	if err != nil {
		s.opts.Logger.Log("level", "error", "msg", "failed to register collector with remote server", "id", s.args.ID, "name", s.args.Name, "err", err)
		return err
	}
	return nil
}

func (s *Service) unregisterCollector(ctx context.Context) error {
	s.mut.RLock()
	defer s.mut.RUnlock()

	_, err := s.apiClient.UnregisterCollector(ctx, &connect.Request[collectorv1.UnregisterCollectorRequest]{
		Msg: &collectorv1.UnregisterCollectorRequest{
			Id: s.args.ID,
		},
	})
	if err != nil {
		s.opts.Logger.Log("level", "error", "msg", "failed to unregister collector with remote server", "id", s.args.ID, "err", err)
		return err
	}
	return nil
}

func (s *Service) isEnabled() bool {
	s.mut.RLock()
	defer s.mut.RUnlock()

	return s.args.URL != "" && s.apiClient != nil
}

func getSystemAttributes() map[string]string {
	return map[string]string{
		reservedAttributeNamespace + namespaceDelimiter + "version": build.Version,
		reservedAttributeNamespace + namespaceDelimiter + "os":      runtime.GOOS,
	}
}

// getContext returns the appropriate context for operations.
// If Run has been called, it returns the Run context.
// Otherwise, it returns a background context for operations before Run.
func (s *Service) getContext() context.Context {
	s.mut.RLock()
	defer s.mut.RUnlock()

	if s.runCtx != nil {
		return s.runCtx
	}
	return context.Background()
}

// GetHost returns the host for the remotecfg service.
func GetHost(host service.Host) (service.Host, error) {
	svc, found := host.GetService(ServiceName)
	if !found {
		return nil, fmt.Errorf("remote config service not available")
	}

	data := svc.Data().(Data)
	if data.Host == nil {
		return nil, fmt.Errorf("remote config service startup in progress")
	}
	return data.Host, nil
}

// GetCachedAstFile returns the AST file that was parsed from the configuration.
func (s *Service) GetCachedAstFile() *ast.File {
	s.mut.RLock()
	defer s.mut.RUnlock()
	return s.cm.getAstFile()
}

package remotecfg

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-kit/log"
	collectorv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/util/jitter"
	"github.com/grafana/alloy/syntax/ast"
)

const baseJitter = 100 * time.Millisecond

// This value is used when we want to disable polling. We use a value that is
// slightly less than MaxInt to avoid overflowing
const disablePollingFrequency = math.MaxInt64 - baseJitter

var errNotModified = errors.New("config not modified since last fetch")

// configManager is responsible for managing the configuration of the remotecfg service.
type configManager struct {
	// Mutex to protect internal state
	mut sync.RWMutex

	// The metrics for the remotecfg service.
	metrics *metrics

	// Logger for the config manager
	logger log.Logger

	// The root path for the remotecfg storage on disk.
	remotecfgPath string

	// Controller for loading configuration
	ctrl service.Controller

	// Configuration file path for parsing
	configPath string

	// This is the hash of the arguments passed to the service. It is used to determine where
	// to store and retrieve the cached configuration.
	argsHash string

	// This is the ticker that is used to poll the API for configuration changes.
	ticker           *jitter.Ticker
	updateTickerChan chan struct{}

	// This is the base frequency at which we poll the API for configuration changes. A jitter is applied to this value.
	pollFrequency time.Duration

	// This is the hash of the last successfully loaded and running configuration.
	// Used to track what config is actually active in the controller.
	lastLoadedConfigHash string

	// This is the hash of the last configuration received from the remote API.
	// Used to avoid re-fetching the same config, regardless of whether it loaded successfully.
	lastReceivedConfigHash string

	// This is the hash received from the API in the current request. It is used to determine if
	// the configuration has changed since the last fetch
	remoteHash string

	// This is the AST file parsed from the configuration. This is used
	// for the support bundle
	astFile *ast.File
}

func newConfigManager(metrics *metrics, logger log.Logger, remotecfgPath string, ctrl service.Controller, configPath string) *configManager {
	return &configManager{
		metrics:          metrics,
		logger:           logger,
		remotecfgPath:    remotecfgPath,
		ctrl:             ctrl,
		configPath:       configPath,
		updateTickerChan: make(chan struct{}, 1),
		pollFrequency:    disablePollingFrequency,
		ticker:           jitter.NewTicker(disablePollingFrequency, baseJitter),
	}
}

func (cm *configManager) setPollFrequency(t time.Duration) {
	cm.mut.Lock()
	defer cm.mut.Unlock()

	if cm.pollFrequency == t {
		return
	}

	cm.pollFrequency = t
	select {
	// If the channel is full it means there's already an update triggered
	// or Run is not running. In both cases, we don't need to trigger another
	// update or block.
	case cm.updateTickerChan <- struct{}{}:
	default:
	}
}

func (cm *configManager) getCachedConfigPath() string {
	cm.mut.RLock()
	defer cm.mut.RUnlock()
	return filepath.Join(cm.remotecfgPath, cm.argsHash)
}

func (cm *configManager) getCachedConfig() ([]byte, error) {
	p := cm.getCachedConfigPath()
	return os.ReadFile(p)
}

func (cm *configManager) setCachedConfig(b []byte) {
	p := cm.getCachedConfigPath()
	err := os.WriteFile(p, b, 0750)
	if err != nil {
		level.Error(cm.logger).Log("msg", "failed to flush remote configuration contents the on-disk cache", "err", err)
	}
}

func (cm *configManager) getLastLoadedCfgHash() string {
	cm.mut.RLock()
	defer cm.mut.RUnlock()
	return cm.lastLoadedConfigHash
}

func (cm *configManager) getLastReceivedCfgHash() string {
	cm.mut.RLock()
	defer cm.mut.RUnlock()
	return cm.lastReceivedConfigHash
}

func (cm *configManager) setLastReceivedCfgHash(h string) {
	cm.mut.Lock()
	defer cm.mut.Unlock()
	cm.lastReceivedConfigHash = h
	cm.metrics.lastReceivedConfigHash.WithLabelValues(h).Set(1)
}

func (cm *configManager) parseAndLoad(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	cm.mut.RLock()
	ctrl := cm.ctrl
	configPath := cm.configPath
	cm.mut.RUnlock()

	if ctrl == nil {
		return fmt.Errorf("controller not available - parseAndLoad called before Run()")
	}

	file, err := ctrl.LoadSource(b, nil, configPath)
	if err != nil {
		level.Error(cm.logger).Log("msg", "failed to parse and load configuration", "config_size", len(b), "err", err)
		return err
	}

	cm.mut.Lock()
	cm.astFile = file
	cm.mut.Unlock()
	return nil
}

func (cm *configManager) setLastLoadedCfgHash(h string) {
	cm.mut.Lock()
	defer cm.mut.Unlock()
	cm.lastLoadedConfigHash = h
	cm.metrics.configHash.WithLabelValues(h).Set(1)
}

func (cm *configManager) getAstFile() *ast.File {
	cm.mut.RLock()
	defer cm.mut.RUnlock()
	return cm.astFile
}

// fetchLoadConfig attempts to read configuration from the API and the local cache
// and then parse/load their contents in order of preference. useCacheAsFallback
// determines whether to fall back to the cache on remote failure.
func (cm *configManager) fetchLoadConfig(getAPIConfig func() (*collectorv1.GetConfigResponse, error), useCacheAsFallback bool) {
	if err := cm.fetchLoadRemoteConfig(getAPIConfig); err != nil && err != errNotModified {
		if useCacheAsFallback {
			level.Error(cm.logger).Log("msg", "failed to fetch remote config, falling back to cache", "err", err)
			cm.fetchLoadLocalConfig()
		} else {
			level.Error(cm.logger).Log("msg", "failed to fetch remote config, continuing with current config", "err", err)
		}
	}
}

func (cm *configManager) fetchLoadRemoteConfig(getAPIConfig func() (*collectorv1.GetConfigResponse, error)) error {
	level.Debug(cm.logger).Log("msg", "fetching remote configuration")

	gcr, err := getAPIConfig()
	cm.metrics.totalAttempts.Add(1)

	// Handle "not modified" response specifically
	if err == errNotModified {
		level.Debug(cm.logger).Log("msg", "skipping over API response since it has not been modified since last fetch")
		cm.metrics.lastFetchNotModified.Set(1)
		return nil
	}

	// Handle other errors
	if err != nil {
		level.Error(cm.logger).Log("msg", "failed to fetch remote config", "err", err)
		cm.metrics.totalFailures.Add(1)
		cm.metrics.lastLoadSuccess.Set(0)
		return err
	}

	cm.metrics.lastFetchSuccessTime.SetToCurrentTime()
	cm.metrics.lastFetchNotModified.Set(0)

	// Store the remote hash from the API response
	if gcr.Hash != "" {
		cm.mut.Lock()
		cm.remoteHash = gcr.Hash
		cm.mut.Unlock()
	}

	b := []byte(gcr.GetContent())
	newConfigHash := getHash(b)

	// Check if we already received this exact config from remote
	alreadyReceived := cm.getLastReceivedCfgHash() == newConfigHash
	alreadyLoaded := cm.getLastLoadedCfgHash() == newConfigHash

	if alreadyReceived {
		level.Debug(cm.logger).Log("msg", "skipping over API response since it matched the last received one", "config_hash", newConfigHash)
		return nil
	}

	// Record that we received this config from remote (regardless of parse success)
	cm.setLastReceivedCfgHash(newConfigHash)

	// It's possible someone will set a broken config back to the original config such
	// that the newConfigHash is the same as the lastLoadedConfigHash. We do not need
	// to reload the config in this case since it is already loaded.
	if alreadyLoaded {
		level.Debug(cm.logger).Log("msg", "skipping over API response since it matched the last loaded one", "config_hash", newConfigHash)
		return nil
	}

	level.Info(cm.logger).Log("msg", "attempting to parse and load new remote configuration", "config_hash", newConfigHash)
	err = cm.parseAndLoad(b)
	if err != nil {
		// Failed to parse/load the configuration - received hash is recorded, but loaded hash unchanged
		level.Error(cm.logger).Log("msg", "failed to parse and load new remote configuration",
			"received_hash", newConfigHash, "loaded_hash", cm.getLastLoadedCfgHash(), "err", err)
		cm.metrics.lastLoadSuccess.Set(0)

		// If we have a cached config, attempt to reload it to restore component health.
		// Otherwise a partial working config will be left in the controller.
		if cm.getLastLoadedCfgHash() != "" {
			level.Info(cm.logger).Log("msg", "attempting to reload cached configuration to restore component health")
			cachedConfig, err := cm.getCachedConfig()
			if err != nil {
				level.Error(cm.logger).Log("msg", "failed to read cached configuration for fallback", "err", err)
				return err
			}

			err = cm.parseAndLoad(cachedConfig)
			if err != nil {
				level.Error(cm.logger).Log("msg", "failed to reload cached configuration", "err", err)
				return err
			}

			level.Info(cm.logger).Log("msg", "successfully restored cached configuration")
			cm.metrics.lastLoadSuccess.Set(1)
			return nil
		}

		return err
	}

	// Successfully loaded the configuration - now update the loaded hash
	cm.setLastLoadedCfgHash(newConfigHash)
	cm.metrics.lastLoadSuccess.Set(1)

	level.Info(cm.logger).Log("msg", "successfully loaded remote configuration",
		"config_hash", newConfigHash, "config_size", len(b))

	// If successful, flush to disk and keep a copy.
	cm.setCachedConfig(b)
	return nil
}

func (cm *configManager) fetchLoadLocalConfig() {
	b, err := cm.getCachedConfig()
	if err != nil {
		level.Error(cm.logger).Log("msg", "failed to read from cache", "cache_path", cm.getCachedConfigPath(), "err", err)
		return
	}

	err = cm.parseAndLoad(b)
	if err != nil {
		level.Error(cm.logger).Log("msg", "failed to load from cache", "cache_path", cm.getCachedConfigPath(), "err", err)
		return
	}

	// Successfully loaded from cache - update the loaded hash (but not received hash, since this came from cache)
	cacheHash := getHash(b)
	cm.setLastLoadedCfgHash(cacheHash)

	level.Info(cm.logger).Log("msg", "successfully loaded configuration from cache",
		"config_hash", cacheHash, "config_size", len(b), "cache_path", cm.getCachedConfigPath())
}

// cleanup properly stops and cleans up the configManager's resources.
func (cm *configManager) cleanup() {
	cm.mut.Lock()
	defer cm.mut.Unlock()

	if cm.ticker != nil {
		cm.ticker.Stop()
		cm.ticker = nil
	}
}

// Getters for safe access to configManager fields

func (cm *configManager) getController() service.Controller {
	cm.mut.RLock()
	defer cm.mut.RUnlock()
	return cm.ctrl
}

func (cm *configManager) setController(ctrl service.Controller) {
	cm.mut.Lock()
	defer cm.mut.Unlock()
	cm.ctrl = ctrl
}

func (cm *configManager) getTicker() *jitter.Ticker {
	cm.mut.RLock()
	defer cm.mut.RUnlock()
	return cm.ticker
}

func (cm *configManager) getTickerC() <-chan time.Time {
	cm.mut.RLock()
	defer cm.mut.RUnlock()
	if cm.ticker != nil {
		return cm.ticker.C
	}
	return nil
}

func (cm *configManager) getUpdateTickerChan() chan struct{} {
	cm.mut.RLock()
	defer cm.mut.RUnlock()
	return cm.updateTickerChan
}

func (cm *configManager) getPollFrequency() time.Duration {
	cm.mut.RLock()
	defer cm.mut.RUnlock()
	return cm.pollFrequency
}

func (cm *configManager) getRemotecfgPath() string {
	cm.mut.RLock()
	defer cm.mut.RUnlock()
	return cm.remotecfgPath
}

func (cm *configManager) getConfigPath() string {
	cm.mut.RLock()
	defer cm.mut.RUnlock()
	return cm.configPath
}

func (cm *configManager) setArgsHash(hash string) {
	cm.mut.Lock()
	defer cm.mut.Unlock()
	cm.argsHash = hash
}

func (cm *configManager) getRemoteHash() string {
	cm.mut.RLock()
	defer cm.mut.RUnlock()
	return cm.remoteHash
}

func getHash(in []byte) string {
	fnvHash := fnv.New32()
	fnvHash.Write(in)
	return fmt.Sprintf("%x", fnvHash.Sum(nil))
}

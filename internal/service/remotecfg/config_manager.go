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
	"github.com/grafana/alloy/syntax/diag"
)

const baseJitter = 100 * time.Millisecond

// This value is used when we want to disable polling. We use a value that is
// slightly less than MaxInt to avoid overflowing
const disablePollingFrequency = math.MaxInt64 - baseJitter

var errNotModified = errors.New("config not modified since last fetch")

// effectiveConfigContentType is the MIME type used when sending the effective
// Alloy configuration to the remote config service.
const effectiveConfigContentType = "text/plain"

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

	// remoteConfigStatus tracks the current status of the remote configuration
	remoteConfigStatus *collectorv1.RemoteConfigStatus

	// lastSentConfigStatus tracks the last status sent to the server to avoid redundant updates
	lastSentConfigStatus *collectorv1.RemoteConfigStatus

	// effectiveConfig tracks the current effective configuration running in Alloy
	effectiveConfig *collectorv1.EffectiveConfig

	// lastSentEffectiveConfig tracks the last effective config sent to the server to avoid redundant updates
	lastSentEffectiveConfig *collectorv1.EffectiveConfig
}

func newConfigManager(metrics *metrics, logger log.Logger, remotecfgPath string, configPath string) *configManager {
	return &configManager{
		metrics:          metrics,
		logger:           logger,
		remotecfgPath:    remotecfgPath,
		configPath:       configPath,
		updateTickerChan: make(chan struct{}, 1),
		pollFrequency:    disablePollingFrequency,
		ticker:           jitter.NewTicker(disablePollingFrequency, baseJitter),
		remoteConfigStatus: &collectorv1.RemoteConfigStatus{
			Status:       collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_UNSET,
			ErrorMessage: "",
		},
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

	// Update effective config after successful load
	cm.setEffectiveConfig(b)

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

	cm.notifyStatusUpdate(getAPIConfig)
}

// notifyStatusUpdate makes an immediate GetConfig call to notify the server of status changes.
// This is used when we want to immediately report status changes without waiting for the next poll cycle.
func (cm *configManager) notifyStatusUpdate(getAPIConfig func() (*collectorv1.GetConfigResponse, error)) {
	// Avoid unnecessary immediate calls if there's nothing new to report.
	if !cm.hasPendingUpdates() {
		level.Debug(cm.logger).Log("msg", "no pending status/effective-config updates; skipping notify")
		return
	}

	// Make the API call but ignore the response and any errors
	// This is not a critical operation since the GetConfig call will
	// be made again on the polling frequency
	level.Debug(cm.logger).Log("msg", "making immediate GetConfig call to report status update")
	_, err := getAPIConfig()
	if err != nil && err != errNotModified {
		level.Error(cm.logger).Log("msg", "status notification call failed, will retry on next poll", "err", err)
	} else {
		level.Debug(cm.logger).Log("msg", "successfully notified server of status update")
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

		// Only mark APPLIED if the last received remote config matches the currently
		// loaded config. This prevents flipping to APPLIED when the server continues
		// to serve a bad config that failed to load previously.
		loaded := cm.getLastLoadedCfgHash()
		received := cm.getLastReceivedCfgHash()
		if loaded != "" && received != "" && loaded == received {
			cm.setRemoteConfigStatus(collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED, "")
		} else {
			level.Debug(cm.logger).Log("msg", "not modified but loaded config does not match last received; retaining status", "loaded_hash", loaded, "received_hash", received)
		}

		return nil
	}

	// Handle other errors
	if err != nil {
		level.Error(cm.logger).Log("msg", "failed to fetch remote config", "err", err)
		cm.metrics.totalFailures.Add(1)
		cm.metrics.lastLoadSuccess.Set(0)

		cm.setRemoteConfigStatus(collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_FAILED, getErrorMessage(err))
		return err
	}

	cm.metrics.lastFetchSuccessTime.SetToCurrentTime()
	cm.metrics.lastFetchNotModified.Set(0)

	// Store the remote hash from the API response
	if gcr.Hash != "" {
		level.Debug(cm.logger).Log("msg", "setting remote hash", "hash", gcr.Hash)
		cm.setRemoteHash(gcr.Hash)
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
		// Set status to APPLIED since the new remote config was previously loaded.
		cm.setRemoteConfigStatus(collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED, "")
		return nil
	}

	level.Info(cm.logger).Log("msg", "attempting to parse and load new remote configuration", "config_hash", newConfigHash)

	// Set status to APPLYING when we start processing remote config
	cm.setRemoteConfigStatus(collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_APPLYING, "")
	err = cm.parseAndLoad(b)
	if err != nil {
		// Failed to parse/load the configuration - received hash is recorded, but loaded hash unchanged
		level.Error(cm.logger).Log("msg", "failed to parse and load new remote configuration",
			"received_hash", newConfigHash, "loaded_hash", cm.getLastLoadedCfgHash(), "err", err)
		cm.metrics.lastLoadSuccess.Set(0)

		// Make immediate GetConfig call to notify server of parse/load failure
		cm.setRemoteConfigStatus(collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_FAILED, getErrorMessage(err))

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

	// Set status to APPLIED for successful remote config load and notify immediately
	// so the server knows about both the status change and the effective config update
	cm.setRemoteConfigStatus(collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED, "")

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

func (cm *configManager) setRemoteHash(hash string) {
	cm.mut.Lock()
	defer cm.mut.Unlock()
	cm.remoteHash = hash
}

// setRemoteConfigStatus updates the remote config status.
func (cm *configManager) setRemoteConfigStatus(status collectorv1.RemoteConfigStatuses, errorMessage string) {
	cm.mut.Lock()
	defer cm.mut.Unlock()

	cm.remoteConfigStatus = &collectorv1.RemoteConfigStatus{
		Status:       status,
		ErrorMessage: errorMessage,
	}
}

// getRemoteConfigStatus returns a copy of the current remote config status.
func (cm *configManager) getRemoteConfigStatus() *collectorv1.RemoteConfigStatus {
	cm.mut.RLock()
	defer cm.mut.RUnlock()

	// Return a copy to avoid race conditions
	return cm.copyRemoteConfigStatus()
}

// getRemoteConfigStatusForRequest returns the remote config status if
// lastSentConfigStatus is nil or if the status has changed.
func (cm *configManager) getRemoteConfigStatusForRequest() *collectorv1.RemoteConfigStatus {
	cm.mut.Lock()
	defer cm.mut.Unlock()

	// Send status if we've never sent one before (first call) or if it has changed
	if cm.lastSentConfigStatus == nil ||
		cm.remoteConfigStatus.Status != cm.lastSentConfigStatus.Status ||
		cm.remoteConfigStatus.ErrorMessage != cm.lastSentConfigStatus.ErrorMessage {

		// Update the last sent status to current status
		cm.lastSentConfigStatus = cm.copyRemoteConfigStatus()

		// Return a copy of the current status
		return cm.copyRemoteConfigStatus()
	}

	// Status hasn't changed, don't send it
	return nil
}

// resetLastSentConfigStatus resets the lastSentConfigStatus to nil so the status will be sent on next request.
// This should be called when an API request fails to ensure the status is retried.
func (cm *configManager) resetLastSentConfigStatus() {
	cm.mut.Lock()
	defer cm.mut.Unlock()
	cm.lastSentConfigStatus = nil
}

// copyRemoteConfigStatus creates a copy of the current remoteConfigStatus to avoid race conditions.
// This method assumes the caller already holds the appropriate lock.
func (cm *configManager) copyRemoteConfigStatus() *collectorv1.RemoteConfigStatus {
	return &collectorv1.RemoteConfigStatus{
		Status:       cm.remoteConfigStatus.Status,
		ErrorMessage: cm.remoteConfigStatus.ErrorMessage,
	}
}

// setEffectiveConfig updates the effective configuration that is currently running.
func (cm *configManager) setEffectiveConfig(config []byte) {
	cm.mut.Lock()
	defer cm.mut.Unlock()

	// Create the effective config structure
	cm.effectiveConfig = &collectorv1.EffectiveConfig{
		ConfigMap: &collectorv1.AgentConfigMap{
			ConfigMap: map[string]*collectorv1.AgentConfigFile{
				"": { // Single config file with empty string key
					Body:        config,
					ContentType: effectiveConfigContentType, // Alloy config format
				},
			},
		},
	}
}

// getEffectiveConfigForRequest returns the effective config if it has changed
// since the last time it was sent.
func (cm *configManager) getEffectiveConfigForRequest() *collectorv1.EffectiveConfig {
	cm.mut.Lock()
	defer cm.mut.Unlock()

	// Don't send if we haven't set effective config yet
	if cm.effectiveConfig == nil {
		return nil
	}

	// Send if config has changed (effectiveConfigsEqual handles nil lastSentEffectiveConfig)
	if !effectiveConfigsEqual(cm.effectiveConfig, cm.lastSentEffectiveConfig) {
		// Update the last sent config to current config
		cm.lastSentEffectiveConfig = copyEffectiveConfig(cm.effectiveConfig)

		// Return a copy of the current config
		return copyEffectiveConfig(cm.effectiveConfig)
	}

	// Config hasn't changed, don't send it
	return nil
}

// hasPendingUpdates returns true if there is either a remote config status change
// or an effective config change that has not yet been sent to the server.
func (cm *configManager) hasPendingUpdates() bool {
	cm.mut.RLock()
	defer cm.mut.RUnlock()

	// Pending status if never sent, or fields differ
	statusPending := cm.lastSentConfigStatus == nil ||
		cm.remoteConfigStatus.Status != cm.lastSentConfigStatus.Status ||
		cm.remoteConfigStatus.ErrorMessage != cm.lastSentConfigStatus.ErrorMessage

	// Pending effective config if we have one and it differs from last sent
	effPending := false
	if cm.effectiveConfig != nil {
		effPending = !effectiveConfigsEqual(cm.effectiveConfig, cm.lastSentEffectiveConfig)
	}

	return statusPending || effPending
}

// resetLastSentEffectiveConfig resets the lastSentEffectiveConfig to nil so the config will be sent on next request.
// This should be called when an API request fails to ensure the config is retried.
func (cm *configManager) resetLastSentEffectiveConfig() {
	cm.mut.Lock()
	defer cm.mut.Unlock()
	cm.lastSentEffectiveConfig = nil
}

// effectiveConfigsEqual checks if two EffectiveConfig objects are equal.
func effectiveConfigsEqual(a, b *collectorv1.EffectiveConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare config maps
	if a.ConfigMap == nil && b.ConfigMap == nil {
		return true
	}
	if a.ConfigMap == nil || b.ConfigMap == nil {
		return false
	}

	aMap := a.ConfigMap.ConfigMap
	bMap := b.ConfigMap.ConfigMap

	if len(aMap) != len(bMap) {
		return false
	}

	for key, aFile := range aMap {
		bFile, exists := bMap[key]
		if !exists {
			return false
		}

		// Compare the file contents
		if !agentConfigFilesEqual(aFile, bFile) {
			return false
		}
	}

	return true
}

// agentConfigFilesEqual checks if two AgentConfigFile objects are equal.
func agentConfigFilesEqual(a, b *collectorv1.AgentConfigFile) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare body (config content)
	if string(a.Body) != string(b.Body) {
		return false
	}

	// Compare content type
	if a.ContentType != b.ContentType {
		return false
	}

	return true
}

// copyEffectiveConfig creates a deep copy of an EffectiveConfig.
func copyEffectiveConfig(config *collectorv1.EffectiveConfig) *collectorv1.EffectiveConfig {
	if config == nil {
		return nil
	}

	result := &collectorv1.EffectiveConfig{}

	if config.ConfigMap != nil {
		result.ConfigMap = &collectorv1.AgentConfigMap{
			ConfigMap: make(map[string]*collectorv1.AgentConfigFile),
		}

		for key, file := range config.ConfigMap.ConfigMap {
			if file != nil {
				result.ConfigMap.ConfigMap[key] = &collectorv1.AgentConfigFile{
					Body:        append([]byte(nil), file.Body...),
					ContentType: file.ContentType,
				}
			}
		}
	}

	return result
}

// getErrorMessage extracts the best error message from an error,
// using AllMessages() for diagnostic errors and Error() for others.
func getErrorMessage(err error) string {
	var diags diag.Diagnostics
	if errors.As(err, &diags) {
		return diags.AllMessages()
	}
	return err.Error()
}

func getHash(in []byte) string {
	fnvHash := fnv.New32()
	fnvHash.Write(in)
	return fmt.Sprintf("%x", fnvHash.Sum(nil))
}

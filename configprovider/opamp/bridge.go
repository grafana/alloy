// Copyright Grafana Labs and OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opamp

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	serverTypes "github.com/open-telemetry/opamp-go/server/types"
	"go.opentelemetry.io/collector/confmap"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

const defaultInitialRemoteConfigWait = 2 * time.Minute

func initialRemoteWaitContext(parent context.Context) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(defaultInitialRemoteConfigWait)
	if d, ok := parent.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	return context.WithDeadline(parent, deadline)
}

// Bridge holds local OpAMP server, remote client, and merged collector YAML.
type Bridge struct {
	bootstrapPath string
	logger        *zap.Logger

	startOnce sync.Once
	startErr  error

	mu sync.RWMutex

	mgmt            *BootstrapOpamp
	userCollector   map[string]any
	instanceID      uuid.UUID
	localPort       int
	persistentState *persistentState

	opampLocal  server.OpAMPServer
	opampRemote client.OpAMPClient

	runCtx    context.Context
	runCancel context.CancelFunc

	remoteCfg atomic.Value // *protobufs.AgentRemoteConfig

	mergedYAML atomic.Value // string

	agentConn          atomic.Value // serverTypes.Connection
	effectiveFromAgent atomic.Value // string

	watcher atomic.Value // confmap.WatcherFunc

	remoteReachableLogged atomic.Bool

	initialRemoteWait     chan error
	initialRemoteWaitOnce sync.Once
}

func newBridge(bootstrapPath string, logger *zap.Logger) *Bridge {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Bridge{
		bootstrapPath: bootstrapPath,
		logger:        logger,
	}
}

func (b *Bridge) setWatcher(w confmap.WatcherFunc) {
	if w != nil {
		b.watcher.Store(w)
	}
}

func (b *Bridge) fireWatcher() {
	w, ok := b.watcher.Load().(confmap.WatcherFunc)
	if !ok || w == nil {
		return
	}
	w(&confmap.ChangeEvent{})
}

func (b *Bridge) ensureStarted(ctx context.Context) error {
	b.startOnce.Do(func() {
		b.startErr = b.start(ctx)
	})
	return b.startErr
}

func (b *Bridge) getMergedYAML() string {
	s, _ := b.mergedYAML.Load().(string)
	return s
}

func (b *Bridge) start(ctx context.Context) error {
	mgmt, userRoot, err := readAndParseBootstrap(b.bootstrapPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(mgmt.Storage.Directory, 0o700); err != nil {
		return fmt.Errorf("opamp storage directory: %w", err)
	}

	ps, err := loadOrCreatePersistentState(mgmt.Storage.Directory, b.logger)
	if err != nil {
		return fmt.Errorf("persistent state: %w", err)
	}

	b.mu.Lock()
	b.mgmt = mgmt
	b.userCollector = userRoot
	b.persistentState = ps
	b.instanceID = ps.InstanceID
	b.mu.Unlock()

	port := mgmt.Agent.OpAMPServerPort
	if port == 0 {
		var perr error
		port, perr = findRandomPort()
		if perr != nil {
			return fmt.Errorf("local opamp port: %w", perr)
		}
	}
	b.localPort = port

	yamlBytes, err := b.recomposeLocked()
	if err != nil {
		return err
	}
	b.mergedYAML.Store(string(yamlBytes))

	b.runCtx, b.runCancel = context.WithCancel(context.Background())

	localSrv := server.New(newOpAMPLogger(b.logger.With(zap.String("component", "opamp-local-srv"))))
	connected := &atomic.Bool{}
	if err := localSrv.Start((flattenedSettings{
		endpoint: fmt.Sprintf("localhost:%d", b.localPort),
		onConnecting: func(*http.Request) (bool, int) {
			already := connected.Swap(true)
			if already {
				return false, http.StatusConflict
			}
			return true, http.StatusOK
		},
		onMessage:         b.handleAgentOpAMPMessage,
		onConnectionClose: func(serverTypes.Connection) { connected.Store(false) },
	}).toServerSettings()); err != nil {
		return fmt.Errorf("start local opamp server: %w", err)
	}
	b.opampLocal = localSrv

	b.initialRemoteWait = make(chan error, 1)
	if err := b.startRemoteClient(); err != nil {
		b.initialRemoteWait = nil
		_ = localSrv.Stop(context.Background())
		return err
	}

	if err := b.waitForInitialRemoteConfigApply(ctx, localSrv, mgmt.Server.Endpoint); err != nil {
		return err
	}
	b.initialRemoteWait = nil

	b.logger.Info("opamp bridge started",
		zap.String("bootstrap", b.bootstrapPath),
		zap.Int("local_opamp_port", b.localPort),
		zap.String("instance_uid", b.instanceID.String()),
	)

	return nil
}

func (b *Bridge) waitForInitialRemoteConfigApply(ctx context.Context, localSrv server.OpAMPServer, remoteEndpoint string) error {
	waitCtx, waitCancel := initialRemoteWaitContext(ctx)
	defer waitCancel()

	abortPartialStartup := func() {
		if b.runCancel != nil {
			b.runCancel()
		}
		if b.opampRemote != nil {
			stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = b.opampRemote.Stop(stopCtx)
			cancel()
		}
		_ = localSrv.Stop(context.Background())
	}

	select {
	case err := <-b.initialRemoteWait:
		if err != nil {
			abortPartialStartup()
			return fmt.Errorf("opamp remote config: %w", err)
		}
		return nil
	case <-waitCtx.Done():
		abortPartialStartup()
		return fmt.Errorf("opamp: timed out waiting for first remote config from %s: %w", remoteEndpoint, waitCtx.Err())
	}
}

func (b *Bridge) recomposeLocked() ([]byte, error) {
	b.mu.RLock()
	user := b.userCollector
	inst := b.instanceID
	port := b.localPort
	b.mu.RUnlock()

	var remote *protobufs.AgentRemoteConfig
	if v := b.remoteCfg.Load(); v != nil {
		remote = v.(*protobufs.AgentRemoteConfig)
	}
	return Compose(user, remote, inst, port)
}

func (b *Bridge) startRemoteClient() error {
	b.mu.RLock()
	mgmt := b.mgmt
	inst := b.instanceID
	b.mu.RUnlock()

	parsedURL, err := url.Parse(mgmt.Server.Endpoint)
	if err != nil {
		return fmt.Errorf("remote endpoint: %w", err)
	}

	var tlsCfg *tls.Config
	if parsedURL.Scheme == "wss" || parsedURL.Scheme == "https" {
		tlsCfg, err = mgmt.Server.TLS.LoadTLSConfig(context.Background())
		if err != nil {
			return fmt.Errorf("remote tls: %w", err)
		}
	}

	log := newOpAMPLogger(b.logger.With(zap.String("component", "opamp-remote-client")))
	var c client.OpAMPClient
	switch parsedURL.Scheme {
	case "ws", "wss":
		c = client.NewWebSocket(log)
	case "http", "https":
		hc := client.NewHTTP(log)
		hc.SetPollingInterval(500 * time.Millisecond)
		c = hc
	default:
		return fmt.Errorf("unsupported remote opamp scheme %q", parsedURL.Scheme)
	}
	b.opampRemote = c

	headers := headersFromMap(mgmt.Server.Headers)
	if ca := mgmt.Server.ClientAuth; ca != nil {
		b.setBasicAuthHeader(headers, ca.Username, ca.Password)
	}

	settings := types.StartSettings{
		OpAMPServerURL:     mgmt.Server.Endpoint,
		Header:             headers,
		TLSConfig:          tlsCfg,
		InstanceUid:        types.InstanceUid(inst),
		RemoteConfigStatus: b.persistentState.GetLastRemoteConfigStatus(),
		Callbacks: types.Callbacks{
			OnConnect: func(context.Context) {
				if !b.remoteReachableLogged.CompareAndSwap(false, true) {
					return
				}
			},
			OnConnectFailed: func(_ context.Context, err error) {
				b.logger.Error("remote OpAMP transport connect failed",
					zap.Error(err))
			},
			OnError: func(_ context.Context, err *protobufs.ServerErrorResponse) {
				b.logger.Error("remote opamp server error", zap.String("message", err.ErrorMessage))
			},
			OnMessage:          b.onRemoteMessage,
			GetEffectiveConfig: func(context.Context) (*protobufs.EffectiveConfig, error) { return b.createEffectiveConfigMsg(), nil },
			SaveRemoteConfigStatus: func(_ context.Context, rcs *protobufs.RemoteConfigStatus) {
				if rcs == nil || b.persistentState == nil {
					return
				}
				if err := b.persistentState.SetLastRemoteConfigStatus(rcs); err != nil {
					b.logger.Error("persist remote config status", zap.Error(err))
				}
			},
		},
	}

	ad := &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{Key: string(conventions.ServiceInstanceIDKey), Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: inst.String()}}},
			{Key: string(conventions.ServiceNameKey), Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "alloy-otel"}}},
			{Key: string(conventions.ServiceVersionKey), Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "unknown"}}},
		},
	}
	if err := c.SetAgentDescription(ad); err != nil {
		return err
	}
	if err := c.SetHealth(&protobufs.ComponentHealth{Healthy: false}); err != nil {
		return err
	}

	// Before Start(), SetAvailableComponents only updates ClientSyncedState.
	// SetCapabilities runs validateCapabilities and requires AvailableComponents when
	// ReportsAvailableComponents is set — so seed state before SetCapabilities.
	if err := c.SetAvailableComponents(bridgePendingAvailableComponents()); err != nil {
		return err
	}

	cap := remoteAgentCapabilities()
	if err := c.SetCapabilities(&cap); err != nil {
		return err
	}

	if err := c.Start(b.runCtx, settings); err != nil {
		return fmt.Errorf("start remote opamp client: %w", err)
	}

	return nil
}

func (b *Bridge) signalInitialRemoteWaitDone(err error) {
	if b.initialRemoteWait == nil {
		return
	}
	b.initialRemoteWaitOnce.Do(func() {
		select {
		case b.initialRemoteWait <- err:
		default:
		}
	})
}

// bridgePendingAvailableComponents is a hash-only placeholder until the collector's opamp
// extension connects and forwards real AvailableComponents to the remote Fleet endpoint.
func bridgePendingAvailableComponents() *protobufs.AvailableComponents {
	h := sha256.Sum256([]byte("github.com/grafana/alloy/configprovider/opamp:pending-local-agent"))
	return &protobufs.AvailableComponents{Hash: h[:]}
}

func remoteAgentCapabilities() protobufs.AgentCapabilities {
	var c protobufs.AgentCapabilities
	c |= protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus
	c |= protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig
	c |= protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth
	c |= protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig
	c |= protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig
	c |= protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents
	return c
}

func (b *Bridge) createEffectiveConfigMsg() *protobufs.EffectiveConfig {
	cfgStr, _ := b.effectiveFromAgent.Load().(string)
	if cfgStr == "" {
		cfgStr = b.getMergedYAML()
	}
	return &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"": {Body: []byte(cfgStr)},
			},
		},
	}
}

func (b *Bridge) handleAgentOpAMPMessage(conn serverTypes.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
	b.agentConn.Store(conn)
	if b.opampRemote == nil {
		return &protobufs.ServerToAgent{}
	}
	msgCtx := b.runCtx
	if msgCtx == nil {
		msgCtx = context.Background()
	}
	if message.AgentDescription != nil {
		if err := b.opampRemote.SetAgentDescription(message.AgentDescription); err != nil {
			b.logger.Debug("forward agent description", zap.Error(err))
		}
	}
	if message.EffectiveConfig != nil {
		if cfg, ok := message.EffectiveConfig.GetConfigMap().GetConfigMap()[""]; ok {
			b.effectiveFromAgent.Store(string(cfg.Body))
			if err := b.opampRemote.UpdateEffectiveConfig(msgCtx); err != nil {
				b.logger.Debug("update effective config on remote", zap.Error(err))
			}
		}
	}
	if message.Health != nil {
		if err := b.opampRemote.SetHealth(message.Health); err != nil {
			b.logger.Debug("forward health", zap.Error(err))
		}
	}
	if message.AvailableComponents != nil {
		if err := b.opampRemote.SetAvailableComponents(message.AvailableComponents); err != nil {
			b.logger.Debug("forward available components", zap.Error(err))
		}
	}
	if message.CustomCapabilities != nil {
		if err := b.opampRemote.SetCustomCapabilities(message.CustomCapabilities); err != nil {
			b.logger.Debug("forward custom capabilities", zap.Error(err))
		}
	}
	return &protobufs.ServerToAgent{}
}

func (b *Bridge) onRemoteMessage(ctx context.Context, msg *types.MessageData) {
	if msg == nil {
		return
	}
	if msg.RemoteConfig != nil {
		b.processRemoteConfig(ctx, msg.RemoteConfig)
	}
	if msg.CustomCapabilities != nil || msg.CustomMessage != nil {
		st, ok := b.agentConn.Load().(serverTypes.Connection)
		if !ok || st == nil {
			return
		}
		out := &protobufs.ServerToAgent{InstanceUid: b.instanceID[:]}
		if msg.CustomCapabilities != nil {
			out.CustomCapabilities = msg.CustomCapabilities
		}
		if msg.CustomMessage != nil {
			out.CustomMessage = msg.CustomMessage
		}
		if err := st.Send(ctx, out); err != nil {
			b.logger.Debug("forward custom message to extension", zap.Error(err))
		}
	}
}

func (b *Bridge) processRemoteConfig(ctx context.Context, rc *protobufs.AgentRemoteConfig) {
	cloned := proto.Clone(rc).(*protobufs.AgentRemoteConfig)
	if err := b.opampRemote.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
		LastRemoteConfigHash: rc.ConfigHash,
		Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLYING,
	}); err != nil {
		b.logger.Debug("report applying remote config", zap.Error(err))
	}

	if err := SaveLastReceivedRemoteConfig(b.mgmt.Storage.Directory, cloned); err != nil {
		b.logger.Error("save remote config", zap.Error(err))
	}
	b.remoteCfg.Store(cloned)

	yamlBytes, err := b.recomposeLocked()
	if err != nil {
		b.logger.Error("merge remote config failed", zap.Error(err))
		_ = b.opampRemote.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
			LastRemoteConfigHash: rc.ConfigHash,
			Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED,
			ErrorMessage:         err.Error(),
		})
		b.signalInitialRemoteWaitDone(err)
		return
	}

	old := b.getMergedYAML()
	newStr := string(yamlBytes)
	b.mergedYAML.Store(newStr)

	if err := b.opampRemote.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
		LastRemoteConfigHash: rc.ConfigHash,
		Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
	}); err != nil {
		b.logger.Debug("report applied remote config", zap.Error(err))
	}

	if err := b.opampRemote.UpdateEffectiveConfig(ctx); err != nil {
		b.logger.Debug("remote UpdateEffectiveConfig after apply", zap.Error(err))
	}

	if old != newStr {
		b.fireWatcher()
	}

	b.signalInitialRemoteWaitDone(nil)
}

func (b *Bridge) setBasicAuthHeader(h http.Header, username, password string) {
	token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	h.Set("Authorization", "Basic "+token)
}

func headersFromMap(m map[string]any) http.Header {
	h := make(http.Header)
	for k, v := range m {
		switch t := v.(type) {
		case string:
			h.Set(k, t)
		case []any:
			for _, item := range t {
				if s, ok := item.(string); ok {
					h.Add(k, s)
				}
			}
		case []string:
			for _, s := range t {
				h.Add(k, s)
			}
		}
	}
	return h
}

func (b *Bridge) shutdown(ctx context.Context) error {
	if b.runCancel != nil {
		b.runCancel()
	}
	var errs error
	if b.opampRemote != nil {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		errs = errors.Join(errs, b.opampRemote.Stop(stopCtx))
		cancel()
	}
	if b.opampLocal != nil {
		errs = errors.Join(errs, b.opampLocal.Stop(ctx))
	}
	return errs
}

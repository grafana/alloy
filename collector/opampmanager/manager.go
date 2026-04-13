package opampmanager

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.opentelemetry.io/collector/otelcol"
)

type opampManagerLogger struct {
	lg *log.Logger
}

func (o *opampManagerLogger) Debugf(ctx context.Context, format string, v ...interface{}) {
	_ = ctx
	o.lg.Printf("opampmanager (opamp): "+format, v...)
}

func (o *opampManagerLogger) Errorf(ctx context.Context, format string, v ...interface{}) {
	_ = ctx
	o.lg.Printf("opampmanager (opamp error): "+format, v...)
}

func Start(ctx context.Context, cfg Config, baseSettings otelcol.CollectorSettings, lg *log.Logger) {
	if lg == nil {
		lg = log.Default()
	}
	go runManager(ctx, cfg, baseSettings, lg)
}

func runManager(ctx context.Context, cfg Config, baseSettings otelcol.CollectorSettings, lg *log.Logger) {
	var applyMu sync.Mutex

	var uid types.InstanceUid
	if _, err := rand.Read(uid[:]); err != nil {
		lg.Printf("opampmanager: instance uid: %v", err)
		return
	}

	c := client.NewHTTP(&opampManagerLogger{lg: lg})

	descr := &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{Key: "service.name", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "com.grafana.alloy.otel.opampmanager"}}},
			{Key: "service.version", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: baseSettings.BuildInfo.Version}}},
			{Key: "host.name", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "Mac.fritz.box"}}},
		},
	}
	if err := c.SetAgentDescription(descr); err != nil {
		lg.Printf("opampmanager: SetAgentDescription: %v", err)
		return
	}

	caps := protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig |
		protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig |
		protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig
	if err := c.SetCapabilities(&caps); err != nil {
		lg.Printf("opampmanager: SetCapabilities: %v", err)
		return
	}

	cb := types.Callbacks{}
	cb.OnConnect = func(ctx context.Context) {
		_ = ctx
		lg.Printf("opampmanager: connected")
	}
	cb.OnConnectFailed = func(ctx context.Context, err error) {
		_ = ctx
		lg.Printf("opampmanager: connect failed: %v", err)
	}
	cb.GetEffectiveConfig = func(ctx context.Context) (*protobufs.EffectiveConfig, error) {
		b, err := os.ReadFile(cfg.EffectivePath)
		if err != nil {
			return nil, err
		}
		return &protobufs.EffectiveConfig{
			ConfigMap: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"effective.yaml": {Body: b, ContentType: "text/yaml"},
				},
			},
		}, nil
	}
	cb.OnMessage = func(ctx context.Context, msg *types.MessageData) {
		if msg == nil || msg.RemoteConfig == nil {
			return
		}
		applyMu.Lock()
		defer applyMu.Unlock()

		body, err := OtelYAMLFromRemoteConfig(msg.RemoteConfig)
		if err != nil {
			lg.Printf("opampmanager: remote config: %v", err)
			_ = c.SetRemoteConfigStatus(remoteStatus(msg.RemoteConfig, body, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED, err.Error()))
			return
		}

		_ = c.SetRemoteConfigStatus(remoteStatus(msg.RemoteConfig, body, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLYING, ""))

		if err := applyReload(ctx, cfg, body, msg.RemoteConfig, lg); err != nil {
			lg.Printf("opampmanager: apply: %v", err)
			_ = c.SetRemoteConfigStatus(remoteStatus(msg.RemoteConfig, body, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED, err.Error()))
			_ = writeStateAtomic(cfg.StatePath, State{Phase: PhaseApplyFailed, CandidateHash: hashBytes(body)})
			return
		}
		_ = c.SetRemoteConfigStatus(remoteStatus(msg.RemoteConfig, body, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED, ""))
		if err := c.UpdateEffectiveConfig(ctx); err != nil {
			lg.Printf("opampmanager: UpdateEffectiveConfig: %v", err)
		}
	}
	cb.SetDefaults()

	settings := types.StartSettings{
		OpAMPServerURL: cfg.ServerURL,
		InstanceUid:    uid,
		Callbacks:      cb,
	}
	if cfg.TLSInsecure {
		if u, err := url.Parse(cfg.ServerURL); err == nil && u.Scheme == "https" {
			settings.TLSConfig = &tls.Config{InsecureSkipVerify: true}
		}
	}
	if hdr := opampRequestHeaders(cfg); hdr != nil {
		settings.Header = hdr.Clone()
	}

	if err := c.Start(ctx, settings); err != nil {
		lg.Printf("opampmanager: Start: %v", err)
		return
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := c.Stop(stopCtx); err != nil {
			lg.Printf("opampmanager: Stop: %v", err)
		}
	}()

	<-ctx.Done()
}

func remoteStatus(rc *protobufs.AgentRemoteConfig, body []byte, st protobufs.RemoteConfigStatuses, errMsg string) *protobufs.RemoteConfigStatus {
	h := append([]byte(nil), rc.GetConfigHash()...)
	if len(h) == 0 {
		sum := sha256.Sum256(body)
		h = append([]byte(nil), sum[:]...)
	}
	rs := &protobufs.RemoteConfigStatus{
		LastRemoteConfigHash: h,
		Status:               st,
	}
	if errMsg != "" {
		rs.ErrorMessage = errMsg
	}
	return rs
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func applyReload(ctx context.Context, cfg Config, candidate []byte, rc *protobufs.AgentRemoteConfig, lg *log.Logger) error {
	effective := cfg.EffectivePath
	prev := effective + ".prev"

	if _, err := os.Stat(prev); os.IsNotExist(err) {
		cur, err := os.ReadFile(effective)
		if err == nil && len(cur) > 0 {
			if err := atomicWriteFile(prev, cur, 0o600); err != nil {
				return fmt.Errorf("seed .prev: %w", err)
			}
		}
	}

	st := State{
		Phase:         PhasePendingApply,
		CandidateHash: hashBytes(candidate),
	}
	if err := writeStateAtomic(cfg.StatePath, st); err != nil {
		return fmt.Errorf("state pending_apply: %w", err)
	}

	if err := atomicWriteFile(effective, candidate, 0o600); err != nil {
		return fmt.Errorf("write effective: %w", err)
	}

	if err := SignalReload(); err != nil {
		lg.Printf("opampmanager: reload signal: %v", err)
	}

	if cfg.HealthCheckURL != "" {
		var lastErr error
		for attempt := 1; attempt <= 3; attempt++ {
			if err := probeHealth(ctx, cfg.HealthCheckURL, cfg.TLSInsecure); err != nil {
				lastErr = err
				time.Sleep(300 * time.Millisecond)
				continue
			}
			lastErr = nil
			break
		}
		if lastErr != nil {
			if b, err := os.ReadFile(prev); err == nil && len(b) > 0 {
				if err := atomicWriteFile(effective, b, 0o600); err != nil {
					return fmt.Errorf("health fail restore: %w", err)
				}
				_ = SignalReload()
			}
			return fmt.Errorf("health probe: %w", lastErr)
		}
	}

	if err := copyFileAtomic(prev, effective, 0o600); err != nil {
		lg.Printf("opampmanager: refresh .prev: %v", err)
	}

	if err := writeStateAtomic(cfg.StatePath, State{Phase: PhaseAppliedOK, CandidateHash: hashBytes(candidate)}); err != nil {
		return fmt.Errorf("state applied_ok: %w", err)
	}
	return nil
}

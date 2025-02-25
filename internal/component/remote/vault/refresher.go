package vault

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service/logging/level"
	vault "github.com/hashicorp/vault/api"
	"github.com/prometheus/client_golang/prometheus"
)

const tokenManagerInitializeTimeout = time.Minute

type getTokenFunc func(ctx context.Context, client *vault.Client) (*vault.Secret, error)

// A tokenManager retrieves and manages the lifecycle of tokens. tokenManager,
// when running, will renew tokens before expiry, and will retrieve new tokens
// once expired tokens can no longer be renewed.
type tokenManager struct {
	log           log.Logger
	refreshTicker *ticker
	getter        getTokenFunc
	onStateChange chan struct{} // Written to when cli or token changes.

	readCounter    prometheus.Counter
	refreshCounter prometheus.Counter

	mut   sync.RWMutex
	cli   *vault.Client
	token *vault.Secret

	healthMut sync.RWMutex
	health    component.Health

	debugMut  sync.RWMutex
	debugInfo secretInfo
}

type tokenManagerOptions struct {
	Log    log.Logger
	Getter getTokenFunc

	ReadCounter, RefreshCounter prometheus.Counter

	Client          *vault.Client
	RefreshInterval time.Duration
}

// newTokenManager creates a new, unstarted tokenManager. tokenManager will
// retrieve the initial token from getter.
func newTokenManager(opts tokenManagerOptions) (*tokenManager, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tokenManagerInitializeTimeout)
	defer cancel()

	tm := &tokenManager{
		log:           opts.Log,
		refreshTicker: newTicker(opts.RefreshInterval),
		getter:        opts.Getter,
		onStateChange: make(chan struct{}, 1),

		readCounter:    opts.ReadCounter,
		refreshCounter: opts.RefreshCounter,

		cli: opts.Client,
	}
	if err := tm.updateToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	return tm, nil
}

// updateToken attempts to update the token, logging an error if getting the
// token failed.
func (tm *tokenManager) updateToken(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			tm.updateHealth(component.Health{
				Health:     component.HealthTypeUnhealthy,
				Message:    fmt.Sprintf("failed to retrieve token: %s", err),
				UpdateTime: time.Now(),
			})
		} else {
			tm.updateHealth(component.Health{
				Health:     component.HealthTypeHealthy,
				Message:    "retrieved token",
				UpdateTime: time.Now(),
			})
		}

		tm.updateDebugInfo(time.Now())
	}()

	tm.mut.Lock()
	defer tm.mut.Unlock()

	token, err := tm.getter(ctx, tm.cli)
	if err != nil {
		level.Error(tm.log).Log("msg", "failed to get token", "err", err)
		return err
	}

	tm.readCounter.Inc()

	tm.token = token

	select {
	case tm.onStateChange <- struct{}{}:
	default:
	}

	return nil
}

// Run runs the tokenManager, blocking until the provided context is canceled.
func (tm *tokenManager) Run(ctx context.Context) {
	var cancelLifecycleWatcher context.CancelFunc
	defer func() {
		if cancelLifecycleWatcher != nil {
			cancelLifecycleWatcher()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case <-tm.refreshTicker.Chan():
			level.Info(tm.log).Log("msg", "refreshing token")
			// Error is handled via setting health and debug info.
			_ = tm.updateToken(ctx)

		case <-tm.onStateChange:
			if cancelLifecycleWatcher != nil {
				cancelLifecycleWatcher()
			}

			ctx, cancel := context.WithCancel(ctx)
			cancelLifecycleWatcher = cancel

			tm.updateLifecycleWatcher(ctx)
		}
	}
}

func (tm *tokenManager) updateHealth(h component.Health) {
	tm.healthMut.Lock()
	defer tm.healthMut.Unlock()

	tm.health = h
}

func (tm *tokenManager) updateDebugInfo(updateTime time.Time) {
	tm.mut.RLock()
	token := tm.token
	tm.mut.RUnlock()

	tm.debugMut.Lock()
	defer tm.debugMut.Unlock()

	tm.debugInfo = getSecretInfo(token, updateTime)
}

func (tm *tokenManager) updateLifecycleWatcher(ctx context.Context) {
	tm.mut.RLock()
	defer tm.mut.RUnlock()

	if !needsLifecycleWatcher(tm.token) {
		return
	}

	lw, err := tm.cli.NewLifetimeWatcher(&vault.LifetimeWatcherInput{
		Secret:        tm.token,
		RenewBehavior: vault.RenewBehaviorIgnoreErrors,
	})
	if err != nil {
		level.Error(tm.log).Log("msg", "failed to create lifetime watcher, lease will not renew automatically", "err", err)
		return
	}

	go lw.Start()

	go func() {
		for {
			select {
			case <-ctx.Done():
				lw.Stop()
				return

			case <-lw.DoneCh():
				if ctx.Err() != nil {
					return
				}
				// Error is logged as health and debug info.
				_ = tm.updateToken(ctx)

			case output := <-lw.RenewCh():
				tm.refreshCounter.Inc()
				level.Debug(tm.log).Log("msg", "token has renewed")
				tm.updateDebugInfo(output.RenewedAt)
			}
		}
	}()
}

// needsLifecycleWatcher determines if a secret needs a lifecycle watcher.
// Secrets only need a lifecycle watcher if they are renewable or have a lease
// duration.
func needsLifecycleWatcher(secret *vault.Secret) bool {
	if secret == nil {
		return false
	}

	if secret.Auth != nil {
		return secret.Auth.Renewable || secret.Auth.LeaseDuration > 0
	}
	return secret.Renewable || secret.LeaseDuration > 0
}

// SetClient updates the client associated with the tokenManager. This will
// force a new retrieval of the token.
func (tm *tokenManager) SetClient(cli *vault.Client) {
	tm.mut.Lock()
	tm.cli = cli
	tm.mut.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), tokenManagerInitializeTimeout)
	defer cancel()

	// Error is handled via setting health and debug info.
	_ = tm.updateToken(ctx)
}

// SetRefreshInterval sets a forced refresh interval, separate from automatic
// renewal based on the token lease.
func (tm *tokenManager) SetRefreshInterval(interval time.Duration) {
	tm.refreshTicker.Reset(interval)
}

// CurrentHealth returns the health of the tokenManager.
func (tm *tokenManager) CurrentHealth() component.Health {
	tm.healthMut.RLock()
	defer tm.healthMut.RUnlock()

	return tm.health
}

// DebugInfo returns the current DebugInfo for the tokenManager.
func (tm *tokenManager) DebugInfo() secretInfo {
	tm.debugMut.RLock()
	defer tm.debugMut.RUnlock()

	return tm.debugInfo
}

type secretInfo struct {
	LatestRequestID  string    `alloy:"latest_request_id,attr"`
	LastUpdateTime   time.Time `alloy:"last_update_time,attr"`
	SecretExpireTime time.Time `alloy:"secret_expire_time,attr"`
	Renewable        bool      `alloy:"renewable,attr"`
	Warnings         []string  `alloy:"warnings,attr"`
}

func getSecretInfo(secret *vault.Secret, updateTime time.Time) secretInfo {
	if secret == nil {
		return secretInfo{
			LastUpdateTime: updateTime,
			Warnings:       []string{"no secret necessary for configured auth mechanism"},
		}
	}

	return secretInfo{
		LatestRequestID:  secret.RequestID,
		LastUpdateTime:   updateTime,
		SecretExpireTime: secretExpireTime(secret),
		Renewable:        secret.Renewable,
		Warnings:         secret.Warnings,
	}
}

func secretExpireTime(secret *vault.Secret) time.Time {
	ttl, err := secret.TokenTTL()
	if err != nil || ttl == 0 {
		return time.Time{}
	}

	return time.Now().UTC().Add(ttl)
}

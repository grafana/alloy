package relay

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	promconfig "github.com/prometheus/common/config"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/useragent"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.alertmanager.relay",
		Community: true,
		Args:      Arguments{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type forwardingState struct {
	client             *http.Client
	endpointURL        string
	timeout            time.Duration
	maxRequestBodySize int64
}

type serverConfig struct {
	listenAddress  string
	listenPort     int
	webhookPath    string
	requestTimeout time.Duration
}

// Component receives Alertmanager webhook notifications and relays their
// alerts to another Alertmanager.
type Component struct {
	opts    component.Options
	metrics *metrics

	stateMut     sync.RWMutex
	state        forwardingState
	serverConfig serverConfig
	initialized  bool

	restartCh chan struct{}

	healthMut sync.RWMutex
	health    component.Health
}

var (
	_ component.Component       = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
)

// New creates a new prometheus.alertmanager.relay component.
func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:      opts,
		metrics:   newMetrics(opts.Registerer),
		restartCh: make(chan struct{}, 1),
		health: component.Health{
			Health:     component.HealthTypeUnknown,
			Message:    "component is starting",
			UpdateTime: time.Now(),
		},
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run starts the webhook listener and handles listener restarts after updates.
func (c *Component) Run(ctx context.Context) error {
	var (
		actorCancel context.CancelFunc
		actorWG     sync.WaitGroup
	)
	defer func() {
		if actorCancel != nil {
			actorCancel()
		}
		actorWG.Wait()
		if client := c.getState().client; client != nil {
			client.CloseIdleConnections()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.restartCh:
			if actorCancel != nil {
				actorCancel()
				actorWG.Wait()
			}

			cfg := c.getServerConfig()
			actorCtx, cancel := context.WithCancel(ctx)
			actorCancel = cancel
			actorWG.Add(1)
			go func() {
				defer actorWG.Done()
				if err := c.runServer(actorCtx, cfg); err != nil {
					c.opts.Logger.Error("webhook server exited with error", "err", err)
					c.setHealth(component.HealthTypeUnhealthy, fmt.Sprintf("webhook server terminated: %s", err))
				}
			}()
		}
	}
}

// Update applies a new listener and destination configuration.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	if err := newArgs.Validate(); err != nil {
		return err
	}

	endpointURL, err := normalizeEndpointURL(newArgs.Endpoint.URL)
	if err != nil {
		return err
	}
	client, err := promconfig.NewClientFromConfig(
		*newArgs.Endpoint.HTTPClientConfig.Convert(),
		c.opts.ID,
		promconfig.WithUserAgent(useragent.Get()),
	)
	if err != nil {
		return fmt.Errorf("creating destination HTTP client: %w", err)
	}

	newServerConfig := serverConfig{
		listenAddress:  newArgs.ListenAddress,
		listenPort:     newArgs.ListenPort,
		webhookPath:    newArgs.WebhookPath,
		requestTimeout: newArgs.Endpoint.Timeout,
	}
	newState := forwardingState{
		client:             client,
		endpointURL:        endpointURL,
		timeout:            newArgs.Endpoint.Timeout,
		maxRequestBodySize: int64(newArgs.MaxRequestBodySize),
	}

	c.stateMut.Lock()
	serverNeedsRestart := !c.initialized || c.serverConfig != newServerConfig || c.CurrentHealth().Health == component.HealthTypeUnhealthy
	previousClient := c.state.client
	c.state = newState
	c.serverConfig = newServerConfig
	c.initialized = true
	c.stateMut.Unlock()
	if previousClient != nil {
		previousClient.CloseIdleConnections()
	}

	if serverNeedsRestart {
		select {
		case c.restartCh <- struct{}{}:
		default:
		}
	}

	return nil
}

func (c *Component) getState() forwardingState {
	c.stateMut.RLock()
	defer c.stateMut.RUnlock()
	return c.state
}

func (c *Component) getServerConfig() serverConfig {
	c.stateMut.RLock()
	defer c.stateMut.RUnlock()
	return c.serverConfig
}

func (c *Component) setHealth(healthType component.HealthType, message string) {
	c.healthMut.Lock()
	defer c.healthMut.Unlock()
	c.health = component.Health{
		Health:     healthType,
		Message:    message,
		UpdateTime: time.Now(),
	}
}

// CurrentHealth reports whether the webhook listener is running.
func (c *Component) CurrentHealth() component.Health {
	c.healthMut.RLock()
	defer c.healthMut.RUnlock()
	return c.health
}

func loggerWithFailure(logger *slog.Logger, failure *outboundFailure) *slog.Logger {
	if failure.statusCode != 0 {
		return logger.With("reason", failure.reason, "status_code", failure.statusCode)
	}
	return logger.With("reason", failure.reason)
}

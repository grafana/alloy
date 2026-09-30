package secrets_manager

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/grafana/alloy/internal/component"
	awscommon "github.com/grafana/alloy/internal/component/common/config/aws"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "remote.aws.secrets_manager",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// requestTimeout limits one config load and fetch, so that a hung AWS call cannot block Update.
const requestTimeout = 30 * time.Second

type secretsGetter interface {
	GetSecretValue(ctx context.Context, in *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

type clientFactory func(ctx context.Context, c awscommon.Client) (secretsGetter, error)

func newSDKClient(ctx context.Context, c awscommon.Client) (secretsGetter, error) {
	cfg, err := c.LoadConfig(ctx)
	if err != nil {
		return nil, err
	}
	return secretsmanager.NewFromConfig(cfg, func(o *secretsmanager.Options) {
		o.BaseEndpoint = c.BaseEndpoint()
	}), nil
}

// Component implements the remote.aws.secrets_manager component.
type Component struct {
	opts      component.Options
	newClient clientFactory
	metrics   *metrics
	reset     chan struct{}

	// fetchMut serializes Update and polls. Without it, a poll that started
	// with old arguments can export an old secret after Update exports the new one.
	fetchMut sync.Mutex

	mut         sync.Mutex
	args        Arguments
	client      secretsGetter
	health      component.Health
	lastExports *Exports
}

var (
	_ component.Component       = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
)

// New creates a remote.aws.secrets_manager component. It fetches the secret
// immediately and returns an error if the fetch fails.
func New(opts component.Options, args Arguments) (*Component, error) {
	return newComponent(opts, args, newSDKClient)
}

func newComponent(opts component.Options, args Arguments, newClient clientFactory) (*Component, error) {
	m, err := newMetrics(opts.Registerer)
	if err != nil {
		return nil, err
	}
	c := &Component{
		opts:      opts,
		newClient: newClient,
		metrics:   m,
		reset:     make(chan struct{}, 1),
	}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run polls the secret until ctx is cancelled.
func (c *Component) Run(ctx context.Context) error {
	var ticker *time.Ticker
	var tick <-chan time.Time
	stopTicker := func() {
		if ticker != nil {
			ticker.Stop()
			ticker, tick = nil, nil
		}
	}
	defer stopTicker()

	resetTicker := func() {
		stopTicker()
		if freq := c.pollFrequency(); freq > 0 {
			ticker = time.NewTicker(freq)
			tick = ticker.C
		}
	}
	resetTicker()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.reset:
			resetTicker()
		case <-tick:
			c.poll(ctx)
		}
	}
}

// Update fetches the secret with the new arguments. If the fetch fails, the
// component keeps its previous arguments and exports.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.fetchMut.Lock()
	defer c.fetchMut.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	client, err := c.newClient(ctx, newArgs.Client)
	if err != nil {
		err = fmt.Errorf("creating AWS client: %w", err)
		c.metrics.fetchesTotal.WithLabelValues("error").Inc()
		c.setHealth(err)
		return err
	}
	if err := c.fetch(ctx, client, newArgs); err != nil {
		return err
	}

	c.mut.Lock()
	c.args, c.client = newArgs, client
	c.mut.Unlock()

	select {
	case c.reset <- struct{}{}:
	default:
	}
	return nil
}

// CurrentHealth implements component.HealthComponent.
func (c *Component) CurrentHealth() component.Health {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.health
}

func (c *Component) pollFrequency() time.Duration {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.args.PollFrequency
}

func (c *Component) poll(ctx context.Context) {
	c.fetchMut.Lock()
	defer c.fetchMut.Unlock()

	c.mut.Lock()
	client, args := c.client, c.args
	c.mut.Unlock()

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if err := c.fetch(ctx, client, args); err != nil {
		c.opts.Logger.Warn("failed to poll secret", "err", err)
	}
}

// fetch reads the secret and exports it if it changed. The caller must hold fetchMut.
func (c *Component) fetch(ctx context.Context, client secretsGetter, args Arguments) error {
	exports, err := getExports(ctx, client, args)
	if err != nil {
		err = fmt.Errorf("fetching secret %q: %w", args.SecretID, err)
		c.metrics.fetchesTotal.WithLabelValues("error").Inc()
		c.setHealth(err)
		return err
	}
	c.metrics.fetchesTotal.WithLabelValues("success").Inc()
	c.metrics.lastSuccess.SetToCurrentTime()

	c.mut.Lock()
	changed := c.lastExports == nil || !reflect.DeepEqual(*c.lastExports, exports)
	if changed {
		c.lastExports = &exports
	}
	c.mut.Unlock()

	// Most polls return the same secret. Skip the export to stop useless
	// re-evaluation of dependent components.
	if changed {
		c.opts.OnStateChange(exports)
	}
	c.setHealth(nil)
	return nil
}

func getExports(ctx context.Context, client secretsGetter, args Arguments) (Exports, error) {
	out, err := client.GetSecretValue(ctx, args.input())
	if err != nil {
		return Exports{}, err
	}
	return toExports(out)
}

func (c *Component) setHealth(err error) {
	h := component.Health{
		Health:     component.HealthTypeHealthy,
		Message:    "secret fetched",
		UpdateTime: time.Now(),
	}
	if err != nil {
		h.Health = component.HealthTypeUnhealthy
		h.Message = err.Error()
	}
	c.mut.Lock()
	c.health = h
	c.mut.Unlock()
}

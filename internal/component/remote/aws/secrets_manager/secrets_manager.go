package secrets_manager

import (
	"context"
	"fmt"
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

// maxRetryInterval limits the wait before a retry after a failed poll.
// Without it, a failure with the default poll_frequency lasts for an hour.
const maxRetryInterval = time.Minute

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
		// The SDK sets BaseEndpoint from AWS_ENDPOINT_URL* before it calls this function.
		// Do not replace that value with nil.
		if endpoint := c.BaseEndpoint(); endpoint != nil {
			o.BaseEndpoint = endpoint
		}
	}), nil
}

// Component implements the remote.aws.secrets_manager component.
type Component struct {
	opts             component.Options
	newClient        clientFactory
	metrics          *metrics
	reset            chan struct{}
	maxRetryInterval time.Duration

	// fetchMut serializes fetches and their exports, so that exports occur in fetch order.
	// Never lock fetchMut while you hold mut.
	fetchMut sync.Mutex

	mut    sync.Mutex
	args   Arguments
	client secretsGetter // Client for the current arguments.
	health component.Health
	// gen increments at the start of each Update. A fetch drops its result
	// if gen changed while the fetch ran, because the result is stale.
	gen uint64
	// pending counts the Updates that did not store their arguments yet.
	// Polls do not start while it is not zero, because the arguments are stale.
	pending    int
	pollCancel context.CancelFunc
	// failing is true when the last poll with the stored arguments failed.
	// A failed Update does not change it, because the stored arguments stay in use.
	failing bool
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
	c := &Component{
		opts:             opts,
		newClient:        newClient,
		metrics:          newMetrics(opts.Registerer),
		reset:            make(chan struct{}, 1),
		maxRetryInterval: maxRetryInterval,
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
		if interval := c.nextInterval(); interval > 0 {
			ticker = time.NewTicker(interval)
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
			if c.poll(ctx) {
				resetTicker()
			}
		}
	}
}

// Update fetches the secret with the new arguments. If the fetch fails, the
// component keeps the previous arguments and client, and it restarts polling
// with them. This matches the controller, which keeps the previous arguments
// when Update fails.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	// The loader holds its lock while it calls Update and cancels any
	// polls in flight.
	c.mut.Lock()
	c.gen++
	myGen := c.gen
	c.pending++
	if c.pollCancel != nil {
		c.pollCancel()
	}
	c.mut.Unlock()

	c.fetchMut.Lock()
	defer c.fetchMut.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	// Always make a new client, so that changes to the environment or credentials apply.
	client, exports, err := c.fetch(ctx, nil, newArgs)

	c.mut.Lock()
	c.pending--
	current := c.gen == myGen
	if current && err == nil {
		c.args, c.client, c.failing = newArgs, client, false
	}
	c.mut.Unlock()

	if current {
		c.report(exports, err)
	}

	select {
	case c.reset <- struct{}{}:
	default:
	}
	return err
}

// CurrentHealth implements component.HealthComponent.
func (c *Component) CurrentHealth() component.Health {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.health
}

// nextInterval returns the wait before the next poll, or 0 for no poll.
// If the last poll failed, the wait is at most maxRetryInterval.
func (c *Component) nextInterval() time.Duration {
	c.mut.Lock()
	defer c.mut.Unlock()
	freq := c.args.PollFrequency
	if freq > 0 && c.failing {
		return min(freq, c.maxRetryInterval)
	}
	return freq
}

// poll fetches the secret with the current arguments. It returns true if the
// poll schedule changed, because a fetch started or stopped to fail.
func (c *Component) poll(runCtx context.Context) bool {
	c.fetchMut.Lock()
	defer c.fetchMut.Unlock()

	c.mut.Lock()
	if c.pending > 0 {
		// If pending is nonzero there is an update in progress that will fetch and set the schedule.
		c.mut.Unlock()
		return false
	}
	myGen, args, client := c.gen, c.args, c.client
	ctx, cancel := context.WithTimeout(runCtx, requestTimeout)
	c.pollCancel = cancel
	c.mut.Unlock()
	defer cancel()

	newClient, exports, err := c.fetch(ctx, client, args)

	c.mut.Lock()
	// Polls do not overlap, so pollCancel is still this poll's cancel or nil.
	c.pollCancel = nil
	if c.gen != myGen || runCtx.Err() != nil {
		// An Update or a shutdown cancelled this poll. Its result is not a real failure.
		c.mut.Unlock()
		return false
	}
	c.client = newClient
	wasFailing := c.failing
	c.failing = err != nil
	c.mut.Unlock()

	if err != nil {
		c.opts.Logger.Warn("failed to poll secret", "err", err)
	}
	c.report(exports, err)
	return wasFailing != (err != nil)
}

// fetch reads the secret. If client is nil, fetch first makes a client from args.
// It returns the client that it used, or nil if it could not make one.
func (c *Component) fetch(ctx context.Context, client secretsGetter, args Arguments) (secretsGetter, Exports, error) {
	if client == nil {
		var err error
		if client, err = c.newClient(ctx, args.Client); err != nil {
			return nil, Exports{}, fmt.Errorf("creating AWS client: %w", err)
		}
	}
	exports, err := getExports(ctx, client, args)
	if err != nil {
		return client, Exports{}, fmt.Errorf("fetching secret %q: %w", args.SecretID, err)
	}
	return client, exports, nil
}

// report records the result of a fetch that is not stale. The caller must hold fetchMut.
// The controller ignores exports that did not change, so report always exports.
func (c *Component) report(exports Exports, err error) {
	if err != nil {
		c.metrics.fetchesTotal.WithLabelValues("error").Inc()
		c.setHealth(err)
		return
	}
	c.metrics.fetchesTotal.WithLabelValues("success").Inc()
	c.metrics.lastSuccess.SetToCurrentTime()
	c.opts.OnStateChange(exports)
	c.setHealth(nil)
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

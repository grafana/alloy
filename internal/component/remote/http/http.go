// Package http implements the remote.http component.
package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	common_config "github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/logging/level"
	"github.com/grafana/alloy/internal/useragent"
	"github.com/grafana/alloy/syntax/alloytypes"
	prom_config "github.com/prometheus/common/config"
)

var userAgent = useragent.Get()

func init() {
	component.Register(component.Registration{
		Name:      "remote.http",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments control the remote.http component.
type Arguments struct {
	URL           string        `alloy:"url,attr"`
	PollFrequency time.Duration `alloy:"poll_frequency,attr,optional"`
	PollTimeout   time.Duration `alloy:"poll_timeout,attr,optional"`
	IsSecret      bool          `alloy:"is_secret,attr,optional"`

	Method  string            `alloy:"method,attr,optional"`
	Headers map[string]string `alloy:"headers,attr,optional"`
	Body    string            `alloy:"body,attr,optional"`

	Client common_config.HTTPClientConfig `alloy:"client,block,optional"`
}

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
	PollFrequency: 1 * time.Minute,
	PollTimeout:   10 * time.Second,
	Client:        common_config.DefaultHTTPClientConfig,
	Method:        http.MethodGet,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.PollFrequency <= 0 {
		return fmt.Errorf("poll_frequency must be greater than 0")
	}
	if args.PollTimeout <= 0 {
		return fmt.Errorf("poll_timeout must be greater than 0")
	}
	if args.PollTimeout >= args.PollFrequency {
		return fmt.Errorf("poll_timeout must be less than poll_frequency")
	}

	if _, err := http.NewRequest(args.Method, args.URL, nil); err != nil {
		return err
	}

	return nil
}

// Exports holds settings exported by remote.http.
type Exports struct {
	Content alloytypes.OptionalSecret `alloy:"content,attr"`
}

// Component implements the remote.http component.
type Component struct {
	log  log.Logger
	opts component.Options

	mut         sync.Mutex
	args        Arguments
	cli         *http.Client
	lastPoll    time.Time
	lastExports Exports // Used for determining whether exports should be updated

	// Updated is written to whenever args updates.
	updated chan struct{}

	healthMut sync.RWMutex
	health    component.Health
}

var (
	_ component.Component       = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
)

// New returns a new, unstarted, remote.http component.
func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		log:  opts.Logger,
		opts: opts,

		updated: make(chan struct{}, 1),

		health: component.Health{
			Health:     component.HealthTypeUnknown,
			Message:    "component started",
			UpdateTime: time.Now(),
		},
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run starts the remote.http component.
func (c *Component) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(c.nextPoll()):
			c.poll()
		case <-c.updated:
			// no-op; force the next wait to be reread.
		}
	}
}

// nextPoll returns how long to wait to poll given the last time a
// poll occurred. nextPoll returns 0 if a poll should occur immediately.
func (c *Component) nextPoll() time.Duration {
	c.mut.Lock()
	defer c.mut.Unlock()

	nextPoll := c.lastPoll.Add(c.args.PollFrequency)
	now := time.Now()

	if now.After(nextPoll) {
		// Poll immediately; next poll period was in the past.
		return 0
	}
	return nextPoll.Sub(now)
}

// poll performs a HTTP GET for the component's configured URL. c.mut must
// not be held when calling. After polling, the component's health is updated
// with the success or failure status.
func (c *Component) poll() {
	err := c.pollError()
	c.updatePollHealth(err)
}

func (c *Component) updatePollHealth(err error) {
	c.healthMut.Lock()
	defer c.healthMut.Unlock()

	if err == nil {
		c.health = component.Health{
			Health:     component.HealthTypeHealthy,
			Message:    "polled endpoint",
			UpdateTime: time.Now(),
		}
	} else {
		c.health = component.Health{
			Health:     component.HealthTypeUnhealthy,
			Message:    fmt.Sprintf("polling failed: %s", err),
			UpdateTime: time.Now(),
		}
	}
}

// pollError is like poll but returns an error if one occurred.
func (c *Component) pollError() error {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.lastPoll = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), c.args.PollTimeout)
	defer cancel()

	var body io.Reader
	if c.args.Body != "" {
		body = strings.NewReader(c.args.Body)
	}

	req, err := http.NewRequest(c.args.Method, c.args.URL, body)
	if err != nil {
		level.Error(c.log).Log("msg", "failed to build request", "err", err)
		return fmt.Errorf("building request: %w", err)
	}
	for name, value := range c.args.Headers {
		req.Header.Set(name, value)
	}
	req = req.WithContext(ctx)

	resp, err := c.cli.Do(req)
	if err != nil {
		level.Error(c.log).Log("msg", "failed to perform request", "err", err)
		return fmt.Errorf("performing request: %w", err)
	}

	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		level.Error(c.log).Log("msg", "failed to read response", "err", err)
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		level.Error(c.log).Log("msg", "unexpected status code from response", "status", resp.Status)
		return fmt.Errorf("unexpected status code %s", resp.Status)
	}

	stringContent := strings.TrimSpace(string(bb))

	newExports := Exports{
		Content: alloytypes.OptionalSecret{
			IsSecret: c.args.IsSecret,
			Value:    stringContent,
		},
	}

	// Only send a state change event if the exports have changed from the
	// previous poll.
	if c.lastExports != newExports {
		c.opts.OnStateChange(newExports)
	}
	c.lastExports = newExports
	return nil
}

// Update updates the remote.http component. After the update completes, a
// poll is forced.
func (c *Component) Update(args component.Arguments) (err error) {
	// Poll after updating and propagate the error if the poll fails. If an error
	// occurred during Update, we don't bother to do anything.
	//
	// It's important to propagate the error in update so the initial state of
	// the component is calculated correctly, otherwise the exports will be empty
	// and may cause unexpected errors in downstream components.
	defer func() {
		if err != nil {
			return
		}
		err = c.pollError()
		c.updatePollHealth(err)
	}()

	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)
	c.args = newArgs

	// Override default UserAgent if another is provided in "headers" section
	customUserAgent, exist := c.args.Headers["User-Agent"]
	if !exist {
		customUserAgent = userAgent
	}

	cli, err := prom_config.NewClientFromConfig(
		*newArgs.Client.Convert(),
		c.opts.ID,
		prom_config.WithUserAgent(customUserAgent),
	)
	if err != nil {
		return err
	}
	c.cli = cli

	// Send an updated event if one wasn't already read.
	select {
	case c.updated <- struct{}{}:
	default:
	}
	return nil
}

// CurrentHealth returns the current health of the component.
func (c *Component) CurrentHealth() component.Health {
	c.healthMut.RLock()
	defer c.healthMut.RUnlock()
	return c.health
}

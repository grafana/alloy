package secrets_manager

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/grafana/alloy/internal/component"
	aws_common_config "github.com/grafana/alloy/internal/component/common/config/aws"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/prometheus/client_golang/prometheus"
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

type Component struct {
	opts          component.Options
	mut           sync.Mutex
	health        component.Health
	pollFrequency time.Duration
	watcher       *watcher
	updateChan    chan result
	errors        prometheus.Counter
	lastAccessed  prometheus.Gauge
}

var (
	_ component.Component       = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
)

type Arguments struct {
	// Options allows the overriding of default AWS settings.
	Options       aws_common_config.Client `alloy:"client,block,optional"`
	SecretId      string                   `alloy:"id,attr"`
	SecretVersion string                   `alloy:"version,attr,optional"`
	PollFrequency time.Duration            `alloy:"poll_frequency,attr,optional"`
}

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
	SecretVersion: "AWSCURRENT",
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

type Exports struct {
	Data map[string]alloytypes.Secret `alloy:"data,attr"`
}

// New initializes a new component
func New(o component.Options, args Arguments) (*Component, error) {
	// Create AWS and AWS's Secrets Manager client
	awsCfg, err := aws_common_config.GenerateAWSConfig(args.Options)
	if err != nil {
		return nil, err
	}
	client := secretsmanager.NewFromConfig(*awsCfg)

	s := &Component{
		opts:          o,
		pollFrequency: args.PollFrequency,
		health:        component.Health{},
		updateChan:    make(chan result),
		errors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "remote_aws_secrets_manager_errors_total",
			Help: "Total number of errors when accessing AWS Secrets Manager",
		}),
		lastAccessed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "remote_aws_secrets_manager_timestamp_last_accessed_unix_seconds",
			Help: "The last successful access in unix seconds",
		}),
	}

	w := newWatcher(args.SecretId, args.SecretVersion, s.updateChan, args.PollFrequency, client)
	s.watcher = w

	if err := o.Registerer.Register(s.errors); err != nil {
		return nil, err
	}
	if err := o.Registerer.Register(s.lastAccessed); err != nil {
		return nil, err
	}

	res := w.getSecret(context.TODO())
	if res.err != nil {
		return nil, res.err
	}

	s.handlePolledSecret(res)

	return s, nil
}

func (s *Component) Run(ctx context.Context) error {
	if s.pollFrequency > 0 {
		go s.handleSecretUpdate(ctx)
		go s.watcher.run(ctx)
	}
	<-ctx.Done()
	return nil
}

// Update is called whenever the arguments have changed.
func (s *Component) Update(args component.Arguments) (err error) {
	defer s.updateHealth(err)
	newArgs := args.(Arguments)

	// Create AWS and AWS's Secrets Manager client
	awsCfg, err := aws_common_config.GenerateAWSConfig(newArgs.Options)
	if err != nil {
		return err
	}
	client := secretsmanager.NewFromConfig(*awsCfg)

	s.mut.Lock()
	defer s.mut.Unlock()
	s.pollFrequency = newArgs.PollFrequency
	s.watcher.updateValues(newArgs.SecretId, newArgs.SecretVersion, newArgs.PollFrequency, client)

	return nil
}

// handleSecretUpdate reads from update and error channels, setting as approriate
func (s *Component) handleSecretUpdate(ctx context.Context) {
	for {
		select {
		case r := <-s.updateChan:
			s.handlePolledSecret(r)
		case <-ctx.Done():
			return
		}
	}
}

// handledPolledSecret converts the secret into exports and exports it to the
// controller.
func (s *Component) handlePolledSecret(res result) {
	var err error
	if validated := res.Validate(); validated {
		s.opts.OnStateChange(Exports{
			Data: map[string]alloytypes.Secret{
				res.secretId: alloytypes.Secret(res.secret),
			},
		})
		s.lastAccessed.SetToCurrentTime()
	} else {
		s.errors.Inc()
		err = res.err
	}

	s.updateHealth(err)
}

// CurrentHealth returns the health of the component.
func (s *Component) CurrentHealth() component.Health {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.health
}

func (c *Component) updateHealth(err error) {
	c.mut.Lock()
	defer c.mut.Unlock()

	if err != nil {
		c.health = component.Health{
			Health:     component.HealthTypeUnhealthy,
			Message:    err.Error(),
			UpdateTime: time.Now(),
		}
	} else {
		c.health = component.Health{
			Health:     component.HealthTypeHealthy,
			Message:    "remote.aws.secrets_manager updated",
			UpdateTime: time.Now(),
		}
	}
}

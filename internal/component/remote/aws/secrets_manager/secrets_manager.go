package secrets_manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
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
	opts         component.Options
	mut          sync.Mutex
	health       component.Health
	errors       prometheus.Counter
	lastAccessed prometheus.Gauge
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
	s := &Component{
		opts:   o,
		health: component.Health{},
		errors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "remote_aws_secrets_manager_errors_total",
			Help: "Total number of errors when accessing AWS Secrets Manager",
		}),
		lastAccessed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "remote_aws_secrets_manager_timestamp_last_accessed_unix_seconds",
			Help: "The last successful access in unix seconds",
		}),
	}

	if err := o.Registerer.Register(s.errors); err != nil {
		return nil, err
	}
	if err := o.Registerer.Register(s.lastAccessed); err != nil {
		return nil, err
	}

	if err := s.Update(args); err != nil {
		return nil, err
	}

	return s, nil
}

func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update is called whenever the arguments have changed.
func (c *Component) Update(args component.Arguments) (err error) {
	defer c.updateHealth(err)
	newArgs := args.(Arguments)

	awsCfg, err := aws_common_config.GenerateAWSConfig(newArgs.Options)
	if err != nil {
		return err
	}

	// Create Secrets Manager client
	svc := secretsmanager.NewFromConfig(*awsCfg)

	result, err := svc.GetSecretValue(context.TODO(), &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(newArgs.SecretId),
		VersionStage: aws.String(newArgs.SecretVersion),
	})
	if err != nil {
		return err
	}

	if result == nil {
		err = fmt.Errorf("unable to retrieve secret at path %s", newArgs.SecretId)
		return err
	}

	c.exportSecret(result)

	return nil
}

// exportSecret converts the secret into exports and exports it to the
// controller.
func (c *Component) exportSecret(secret *secretsmanager.GetSecretValueOutput) {
	if secret != nil {
		newExports := Exports{
			Data: make(map[string]alloytypes.Secret),
		}
		newExports.Data[*secret.Name] = alloytypes.Secret(*secret.SecretString)
		c.opts.OnStateChange(newExports)
	}
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

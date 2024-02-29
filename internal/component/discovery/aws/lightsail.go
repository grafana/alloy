package aws

import (
	"errors"
	"time"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river/rivertypes"
	promcfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	promaws "github.com/prometheus/prometheus/discovery/aws"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.lightsail",
		Stability: featuregate.StabilityStable,
		Args:      LightsailArguments{},
		Exports:   discovery.Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewLightsail(opts, args.(LightsailArguments))
		},
	})
}

// LightsailArguments is the configuration for AWS Lightsail based service discovery.
type LightsailArguments struct {
	Endpoint         string                  `river:"endpoint,attr,optional"`
	Region           string                  `river:"region,attr,optional"`
	AccessKey        string                  `river:"access_key,attr,optional"`
	SecretKey        rivertypes.Secret       `river:"secret_key,attr,optional"`
	Profile          string                  `river:"profile,attr,optional"`
	RoleARN          string                  `river:"role_arn,attr,optional"`
	RefreshInterval  time.Duration           `river:"refresh_interval,attr,optional"`
	Port             int                     `river:"port,attr,optional"`
	HTTPClientConfig config.HTTPClientConfig `river:",squash"`
}

func (args LightsailArguments) Convert() *promaws.LightsailSDConfig {
	cfg := &promaws.LightsailSDConfig{
		Endpoint:         args.Endpoint,
		Region:           args.Region,
		AccessKey:        args.AccessKey,
		SecretKey:        promcfg.Secret(args.SecretKey),
		Profile:          args.Profile,
		RoleARN:          args.RoleARN,
		RefreshInterval:  model.Duration(args.RefreshInterval),
		Port:             args.Port,
		HTTPClientConfig: *args.HTTPClientConfig.Convert(),
	}
	return cfg
}

// DefaultLightsailSDConfig is the default Lightsail SD configuration.
var DefaultLightsailSDConfig = LightsailArguments{
	Port:             80,
	RefreshInterval:  60 * time.Second,
	HTTPClientConfig: config.DefaultHTTPClientConfig,
}

// SetToDefault implements river.Defaulter.
func (args *LightsailArguments) SetToDefault() {
	*args = DefaultLightsailSDConfig
}

// Validate implements river.Validator.
func (args *LightsailArguments) Validate() error {
	if args.Region == "" {
		sess, err := session.NewSession()
		if err != nil {
			return err
		}
		metadata := ec2metadata.New(sess)
		region, err := metadata.Region()
		if err != nil {
			return errors.New("Lightsail SD configuration requires a region")
		}
		args.Region = region
	}
	return nil
}

// New creates a new discovery.lightsail component.
func NewLightsail(opts component.Options, args LightsailArguments) (component.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		conf := args.(LightsailArguments).Convert()
		return promaws.NewLightsailDiscovery(conf, opts.Logger), nil
	})
}

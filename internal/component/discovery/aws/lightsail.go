package aws

import (
	"context"
	"errors"
	"time"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	promcfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	promaws "github.com/prometheus/prometheus/discovery/aws"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.lightsail",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      LightsailArguments{},
		Exports:   discovery.Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewLightsail(opts, args.(LightsailArguments))
		},
	})
}

// LightsailArguments is the configuration for AWS Lightsail based service discovery.
type LightsailArguments struct {
	Endpoint         string                  `alloy:"endpoint,attr,optional"`
	Region           string                  `alloy:"region,attr,optional"`
	AccessKey        string                  `alloy:"access_key,attr,optional"`
	SecretKey        alloytypes.Secret       `alloy:"secret_key,attr,optional"`
	Profile          string                  `alloy:"profile,attr,optional"`
	RoleARN          string                  `alloy:"role_arn,attr,optional"`
	RefreshInterval  time.Duration           `alloy:"refresh_interval,attr,optional"`
	Port             int                     `alloy:"port,attr,optional"`
	HTTPClientConfig config.HTTPClientConfig `alloy:",squash"`
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

// SetToDefault implements syntax.Defaulter.
func (args *LightsailArguments) SetToDefault() {
	*args = DefaultLightsailSDConfig
}

// Validate implements syntax.Validator.
func (args *LightsailArguments) Validate() error {
	if args.Region == "" {
		cfgCtx := context.TODO()
		cfg, err := awsConfig.LoadDefaultConfig(cfgCtx)
		if err != nil {
			return err
		}

		client := imds.NewFromConfig(cfg)
		region, err := client.GetRegion(cfgCtx, &imds.GetRegionInput{})
		if err != nil {
			return errors.New("lightsail SD configuration requires a region")
		}
		args.Region = region.Region
	}
	return nil
}

// NewLightsail creates a new discovery.lightsail component.
func NewLightsail(opts component.Options, args LightsailArguments) (component.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.DiscovererConfig, error) {
		return args.(LightsailArguments).Convert(), nil
	})
}

package aws

import (
	"context"
	"errors"
	"time"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
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
		Name:      "discovery.ec2",
		Stability: featuregate.StabilityStable,
		Args:      EC2Arguments{},
		Exports:   discovery.Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewEC2(opts, args.(EC2Arguments))
		},
	})
}

// EC2Filter is the configuration for filtering EC2 instances.
type EC2Filter struct {
	Name   string   `river:"name,attr"`
	Values []string `river:"values,attr"`
}

// EC2Arguments is the configuration for EC2 based service discovery.
type EC2Arguments struct {
	Endpoint        string            `river:"endpoint,attr,optional"`
	Region          string            `river:"region,attr,optional"`
	AccessKey       string            `river:"access_key,attr,optional"`
	SecretKey       rivertypes.Secret `river:"secret_key,attr,optional"`
	Profile         string            `river:"profile,attr,optional"`
	RoleARN         string            `river:"role_arn,attr,optional"`
	RefreshInterval time.Duration     `river:"refresh_interval,attr,optional"`
	Port            int               `river:"port,attr,optional"`
	Filters         []*EC2Filter      `river:"filter,block,optional"`

	HTTPClientConfig config.HTTPClientConfig `river:",squash"`
}

func (args EC2Arguments) Convert() *promaws.EC2SDConfig {
	cfg := &promaws.EC2SDConfig{
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
	for _, f := range args.Filters {
		cfg.Filters = append(cfg.Filters, &promaws.EC2Filter{
			Name:   f.Name,
			Values: f.Values,
		})
	}
	return cfg
}

var DefaultEC2SDConfig = EC2Arguments{
	Port:             80,
	RefreshInterval:  60 * time.Second,
	HTTPClientConfig: config.DefaultHTTPClientConfig,
}

// SetToDefault implements river.Defaulter.
func (args *EC2Arguments) SetToDefault() {
	*args = DefaultEC2SDConfig
}

// Validate implements river.Validator.
func (args *EC2Arguments) Validate() error {
	if args.Region == "" {
		cfgCtx := context.TODO()
		cfg, err := awsConfig.LoadDefaultConfig(cfgCtx)
		if err != nil {
			return err
		}

		client := imds.NewFromConfig(cfg)
		region, err := client.GetRegion(cfgCtx, &imds.GetRegionInput{})
		if err != nil {
			return errors.New("EC2 SD configuration requires a region")
		}
		args.Region = region.Region
	}
	for _, f := range args.Filters {
		if len(f.Values) == 0 {
			return errors.New("EC2 SD configuration filter values cannot be empty")
		}
	}
	return nil
}

// New creates a new discovery.ec2 component.
func NewEC2(opts component.Options, args EC2Arguments) (component.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		conf := args.(EC2Arguments).Convert()
		return promaws.NewEC2Discovery(conf, opts.Logger), nil
	})
}

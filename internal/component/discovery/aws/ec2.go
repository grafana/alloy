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
		Name:      "discovery.ec2",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      EC2Arguments{},
		Exports:   discovery.Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(EC2Arguments))
		},
	})
}

// EC2Filter is the configuration for filtering EC2 instances.
type EC2Filter struct {
	Name   string   `alloy:"name,attr"`
	Values []string `alloy:"values,attr"`
}

// EC2Arguments is the configuration for EC2 based service discovery.
type EC2Arguments struct {
	Endpoint        string            `alloy:"endpoint,attr,optional"`
	Region          string            `alloy:"region,attr,optional"`
	AccessKey       string            `alloy:"access_key,attr,optional"`
	SecretKey       alloytypes.Secret `alloy:"secret_key,attr,optional"`
	Profile         string            `alloy:"profile,attr,optional"`
	RoleARN         string            `alloy:"role_arn,attr,optional"`
	RefreshInterval time.Duration     `alloy:"refresh_interval,attr,optional"`
	Port            int               `alloy:"port,attr,optional"`
	Filters         []*EC2Filter      `alloy:"filter,block,optional"`

	HTTPClientConfig config.HTTPClientConfig `alloy:",squash"`
}

func (args EC2Arguments) Convert() discovery.DiscovererConfig {
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

// SetToDefault implements syntax.Defaulter.
func (args *EC2Arguments) SetToDefault() {
	*args = DefaultEC2SDConfig
}

// Validate implements syntax.Validator.
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

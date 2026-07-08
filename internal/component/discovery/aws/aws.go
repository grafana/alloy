package aws

import (
	"context"
	"errors"
	"fmt"
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
		Name:      "discovery.aws",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      AWSArguments{},
		Exports:   discovery.Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(AWSArguments))
		},
	})
}

// AWSArguments is the configuration for AWS unified service discovery. A single
// role selects which AWS service to discover; see the upstream Prometheus
// aws_sd_configs documentation for the supported roles.
type AWSArguments struct {
	Role            string            `alloy:"role,attr"`
	Endpoint        string            `alloy:"endpoint,attr,optional"`
	Region          string            `alloy:"region,attr,optional"`
	AccessKey       string            `alloy:"access_key,attr,optional"`
	SecretKey       alloytypes.Secret `alloy:"secret_key,attr,optional"`
	Profile         string            `alloy:"profile,attr,optional"`
	RoleARN         string            `alloy:"role_arn,attr,optional"`
	ExternalID      string            `alloy:"external_id,attr,optional"`
	RefreshInterval time.Duration     `alloy:"refresh_interval,attr,optional"`
	Port            int               `alloy:"port,attr,optional"`

	// Filters applies only to the ec2 role.
	Filters []*EC2Filter `alloy:"filter,block,optional"`
	// Clusters and RequestConcurrency apply only to the ecs, elasticache, msk, and rds roles.
	Clusters           []string `alloy:"clusters,attr,optional"`
	RequestConcurrency int      `alloy:"request_concurrency,attr,optional"`

	HTTPClientConfig config.HTTPClientConfig `alloy:",squash"`
}

func (args AWSArguments) Convert() discovery.DiscovererConfig {
	role := promaws.Role(args.Role)
	secret := promcfg.Secret(args.SecretKey)
	httpClient := *args.HTTPClientConfig.Convert()

	// Upstream populates the role-specific embedded config only in UnmarshalYAML,
	// which Alloy never calls. NewDiscoverer reads that embedded pointer, so we
	// build and attach it here based on the selected role. For numeric fields we
	// mirror upstream's "override the per-role default only when set" semantics,
	// sourcing the defaults from upstream's Default*SDConfig rather than hardcoding.
	// https://github.com/prometheus/prometheus/blob/v3.12.0/discovery/aws/aws.go#L101
	cfg := &promaws.SDConfig{Role: role}
	switch role {
	case promaws.RoleEC2:
		def := promaws.DefaultEC2SDConfig
		sub := &promaws.EC2SDConfig{
			Endpoint:         args.Endpoint,
			Region:           args.Region,
			AccessKey:        args.AccessKey,
			SecretKey:        secret,
			Profile:          args.Profile,
			RoleARN:          args.RoleARN,
			ExternalID:       args.ExternalID,
			RefreshInterval:  nonZero(model.Duration(args.RefreshInterval), def.RefreshInterval),
			Port:             nonZero(args.Port, def.Port),
			HTTPClientConfig: httpClient,
		}
		for _, f := range args.Filters {
			sub.Filters = append(sub.Filters, &promaws.EC2Filter{Name: f.Name, Values: f.Values})
		}
		cfg.EC2SDConfig = sub
	case promaws.RoleECS:
		def := promaws.DefaultECSSDConfig
		cfg.ECSSDConfig = &promaws.ECSSDConfig{
			Endpoint:           args.Endpoint,
			Region:             args.Region,
			AccessKey:          args.AccessKey,
			SecretKey:          secret,
			Profile:            args.Profile,
			RoleARN:            args.RoleARN,
			ExternalID:         args.ExternalID,
			Clusters:           args.Clusters,
			RefreshInterval:    nonZero(model.Duration(args.RefreshInterval), def.RefreshInterval),
			Port:               nonZero(args.Port, def.Port),
			RequestConcurrency: nonZero(args.RequestConcurrency, def.RequestConcurrency),
			HTTPClientConfig:   httpClient,
		}
	case promaws.RoleElasticache:
		def := promaws.DefaultElasticacheSDConfig
		cfg.ElasticacheSDConfig = &promaws.ElasticacheSDConfig{
			Endpoint:           args.Endpoint,
			Region:             args.Region,
			AccessKey:          args.AccessKey,
			SecretKey:          secret,
			Profile:            args.Profile,
			RoleARN:            args.RoleARN,
			ExternalID:         args.ExternalID,
			Clusters:           args.Clusters,
			RefreshInterval:    nonZero(model.Duration(args.RefreshInterval), def.RefreshInterval),
			Port:               nonZero(args.Port, def.Port),
			RequestConcurrency: nonZero(args.RequestConcurrency, def.RequestConcurrency),
			HTTPClientConfig:   httpClient,
		}
	case promaws.RoleLightsail:
		def := promaws.DefaultLightsailSDConfig
		cfg.LightsailSDConfig = &promaws.LightsailSDConfig{
			Endpoint:         args.Endpoint,
			Region:           args.Region,
			AccessKey:        args.AccessKey,
			SecretKey:        secret,
			Profile:          args.Profile,
			RoleARN:          args.RoleARN,
			ExternalID:       args.ExternalID,
			RefreshInterval:  nonZero(model.Duration(args.RefreshInterval), def.RefreshInterval),
			Port:             nonZero(args.Port, def.Port),
			HTTPClientConfig: httpClient,
		}
	case promaws.RoleMSK:
		def := promaws.DefaultMSKSDConfig
		cfg.MSKSDConfig = &promaws.MSKSDConfig{
			Endpoint:           args.Endpoint,
			Region:             args.Region,
			AccessKey:          args.AccessKey,
			SecretKey:          secret,
			Profile:            args.Profile,
			RoleARN:            args.RoleARN,
			ExternalID:         args.ExternalID,
			Clusters:           args.Clusters,
			RefreshInterval:    nonZero(model.Duration(args.RefreshInterval), def.RefreshInterval),
			Port:               nonZero(args.Port, def.Port),
			RequestConcurrency: nonZero(args.RequestConcurrency, def.RequestConcurrency),
			HTTPClientConfig:   httpClient,
		}
	case promaws.RoleRDS:
		def := promaws.DefaultRDSSDConfig
		cfg.RDSSDConfig = &promaws.RDSSDConfig{
			Endpoint:           args.Endpoint,
			Region:             args.Region,
			AccessKey:          args.AccessKey,
			SecretKey:          secret,
			Profile:            args.Profile,
			RoleARN:            args.RoleARN,
			ExternalID:         args.ExternalID,
			Clusters:           args.Clusters,
			RefreshInterval:    nonZero(model.Duration(args.RefreshInterval), def.RefreshInterval),
			Port:               nonZero(args.Port, def.Port),
			RequestConcurrency: nonZero(args.RequestConcurrency, def.RequestConcurrency),
			HTTPClientConfig:   httpClient,
		}
	}
	return cfg
}

// SetToDefault implements syntax.Defaulter. Numeric defaults are role-specific
// and applied from upstream's Default*SDConfig in Convert; only the HTTP client
// config is defaulted here, since its nested booleans have no unset sentinel.
func (args *AWSArguments) SetToDefault() {
	*args = AWSArguments{HTTPClientConfig: config.DefaultHTTPClientConfig}
}

// nonZero returns v when set, otherwise the upstream per-role default.
func nonZero[T int | model.Duration](v, roleDefault T) T {
	if v != 0 {
		return v
	}
	return roleDefault
}

// resolveRegion mirrors upstream loadRegion: prefer the region from AWS
// env/profile config, falling back to IMDS.
// https://github.com/prometheus/prometheus/blob/v3.12.0/discovery/aws/aws.go#L395
func resolveRegion() (string, error) {
	ctx := context.TODO()
	cfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return "", err
	}
	if cfg.Region != "" {
		return cfg.Region, nil
	}
	region, err := imds.NewFromConfig(cfg).GetRegion(ctx, &imds.GetRegionInput{})
	if err != nil {
		return "", errors.New("AWS SD configuration requires a region")
	}
	return region.Region, nil
}

// clusterRoles are the roles that accept clusters/request_concurrency.
var clusterRoles = map[promaws.Role]struct{}{
	promaws.RoleECS:         {},
	promaws.RoleElasticache: {},
	promaws.RoleMSK:         {},
	promaws.RoleRDS:         {},
}

// Validate implements syntax.Validator.
func (args *AWSArguments) Validate() error {
	role := promaws.Role(args.Role)
	switch role {
	case promaws.RoleEC2, promaws.RoleECS, promaws.RoleElasticache, promaws.RoleLightsail, promaws.RoleMSK, promaws.RoleRDS:
	default:
		return fmt.Errorf("unsupported role %q: must be one of ec2, ecs, elasticache, lightsail, msk, rds", args.Role)
	}

	if args.Region == "" {
		region, err := resolveRegion()
		if err != nil {
			return err
		}
		args.Region = region
	}

	if len(args.Filters) > 0 && role != promaws.RoleEC2 {
		return fmt.Errorf("filter blocks are only supported with the ec2 role, not %q", args.Role)
	}
	for _, f := range args.Filters {
		if len(f.Values) == 0 {
			return errors.New("AWS SD configuration filter values cannot be empty")
		}
	}

	if len(args.Clusters) > 0 {
		if _, ok := clusterRoles[role]; !ok {
			return fmt.Errorf("clusters is only supported with the ecs, elasticache, msk, and rds roles, not %q", args.Role)
		}
	}

	return nil
}

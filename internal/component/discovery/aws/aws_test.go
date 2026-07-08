package aws

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery"
	promaws "github.com/prometheus/prometheus/discovery/aws"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/config"
)

func TestAWSConvertEC2(t *testing.T) {
	args := AWSArguments{
		Role:             "ec2",
		Region:           "us-east-1",
		AccessKey:        "key",
		SecretKey:        "shhh",
		RoleARN:          "arn:aws:iam::123:role/r",
		ExternalID:       "ext-1",
		Port:             8080,
		Filters:          []*EC2Filter{{Name: "tag:env", Values: []string{"prod"}}},
		HTTPClientConfig: config.DefaultHTTPClientConfig,
	}

	cfg := args.Convert().(*promaws.SDConfig)
	require.Equal(t, promaws.RoleEC2, cfg.Role)
	require.NotNil(t, cfg.EC2SDConfig)
	require.Equal(t, "us-east-1", cfg.EC2SDConfig.Region)
	require.Equal(t, "key", cfg.EC2SDConfig.AccessKey)
	require.Equal(t, "shhh", string(cfg.EC2SDConfig.SecretKey))
	require.Equal(t, "arn:aws:iam::123:role/r", cfg.EC2SDConfig.RoleARN)
	require.Equal(t, "ext-1", cfg.EC2SDConfig.ExternalID)
	require.Equal(t, 8080, cfg.EC2SDConfig.Port)
	require.Len(t, cfg.EC2SDConfig.Filters, 1)
	require.Equal(t, "tag:env", cfg.EC2SDConfig.Filters[0].Name)
	require.Equal(t, []string{"prod"}, cfg.EC2SDConfig.Filters[0].Values)
}

// TestAWSRequestConcurrencyDefault guards the per-role default: upstream uses 20
// for ecs and 10 for elasticache/msk/rds. An explicit value overrides both.
func TestAWSRequestConcurrencyDefault(t *testing.T) {
	get := map[string]func(*promaws.SDConfig) int{
		"ecs":         func(c *promaws.SDConfig) int { return c.ECSSDConfig.RequestConcurrency },
		"elasticache": func(c *promaws.SDConfig) int { return c.ElasticacheSDConfig.RequestConcurrency },
		"msk":         func(c *promaws.SDConfig) int { return c.MSKSDConfig.RequestConcurrency },
		"rds":         func(c *promaws.SDConfig) int { return c.RDSSDConfig.RequestConcurrency },
	}
	wantDefault := map[string]int{"ecs": 20, "elasticache": 10, "msk": 10, "rds": 10}

	for role, want := range wantDefault {
		t.Run(role+" default", func(t *testing.T) {
			args := AWSArguments{Role: role, Region: "us-east-1", HTTPClientConfig: config.DefaultHTTPClientConfig}
			require.Equal(t, want, get[role](args.Convert().(*promaws.SDConfig)))
		})
		t.Run(role+" explicit", func(t *testing.T) {
			args := AWSArguments{Role: role, Region: "us-east-1", RequestConcurrency: 7, HTTPClientConfig: config.DefaultHTTPClientConfig}
			require.Equal(t, 7, get[role](args.Convert().(*promaws.SDConfig)))
		})
	}
}

func TestAWSConvertClusterRoles(t *testing.T) {
	tests := []struct {
		role string
		get  func(*promaws.SDConfig) (region string, clusters []string, concurrency int, ok bool)
	}{
		{"ecs", func(c *promaws.SDConfig) (string, []string, int, bool) {
			if c.ECSSDConfig == nil {
				return "", nil, 0, false
			}
			return c.ECSSDConfig.Region, c.ECSSDConfig.Clusters, c.ECSSDConfig.RequestConcurrency, true
		}},
		{"elasticache", func(c *promaws.SDConfig) (string, []string, int, bool) {
			if c.ElasticacheSDConfig == nil {
				return "", nil, 0, false
			}
			return c.ElasticacheSDConfig.Region, c.ElasticacheSDConfig.Clusters, c.ElasticacheSDConfig.RequestConcurrency, true
		}},
		{"msk", func(c *promaws.SDConfig) (string, []string, int, bool) {
			if c.MSKSDConfig == nil {
				return "", nil, 0, false
			}
			return c.MSKSDConfig.Region, c.MSKSDConfig.Clusters, c.MSKSDConfig.RequestConcurrency, true
		}},
		{"rds", func(c *promaws.SDConfig) (string, []string, int, bool) {
			if c.RDSSDConfig == nil {
				return "", nil, 0, false
			}
			return c.RDSSDConfig.Region, c.RDSSDConfig.Clusters, c.RDSSDConfig.RequestConcurrency, true
		}},
	}

	for _, tc := range tests {
		t.Run(tc.role, func(t *testing.T) {
			args := AWSArguments{
				Role:               tc.role,
				Region:             "eu-west-1",
				Clusters:           []string{"c1", "c2"},
				RequestConcurrency: 5,
				HTTPClientConfig:   config.DefaultHTTPClientConfig,
			}
			cfg := args.Convert().(*promaws.SDConfig)
			require.Equal(t, promaws.Role(tc.role), cfg.Role)
			region, clusters, concurrency, ok := tc.get(cfg)
			require.True(t, ok, "embedded sub-config must be populated")
			require.Equal(t, "eu-west-1", region)
			require.Equal(t, []string{"c1", "c2"}, clusters)
			require.Equal(t, 5, concurrency)
		})
	}
}

func TestAWSConvertLightsail(t *testing.T) {
	args := AWSArguments{
		Role:             "lightsail",
		Region:           "us-east-1",
		HTTPClientConfig: config.DefaultHTTPClientConfig,
	}
	cfg := args.Convert().(*promaws.SDConfig)
	require.Equal(t, promaws.RoleLightsail, cfg.Role)
	require.NotNil(t, cfg.LightsailSDConfig)
	require.Equal(t, "us-east-1", cfg.LightsailSDConfig.Region)
}

// TestAWSNewDiscoverer guards the embedded sub-config wiring: upstream populates
// the role-specific config only in UnmarshalYAML, which Alloy bypasses. If Convert
// doesn't set the embedded pointer, NewDiscoverer errors or nil-derefs.
func TestAWSNewDiscoverer(t *testing.T) {
	for _, role := range []string{"ec2", "ecs", "elasticache", "lightsail", "msk", "rds"} {
		t.Run(role, func(t *testing.T) {
			args := AWSArguments{Role: role, Region: "us-east-1", HTTPClientConfig: config.DefaultHTTPClientConfig}
			cfg := args.Convert().(*promaws.SDConfig)

			reg := prometheus.NewRegistry()
			metrics := cfg.NewDiscovererMetrics(reg, discovery.NewRefreshMetrics(reg))
			d, err := cfg.NewDiscoverer(discovery.DiscovererOptions{Metrics: metrics})
			require.NoError(t, err)
			require.NotNil(t, d)
		})
	}
}

// TestAWSPortRefreshDefaults confirms port and refresh_interval fall back to the
// upstream defaults when unset, and that explicit values override them.
func TestAWSPortRefreshDefaults(t *testing.T) {
	unset := AWSArguments{Role: "ec2", Region: "us-east-1", HTTPClientConfig: config.DefaultHTTPClientConfig}
	cfg := unset.Convert().(*promaws.SDConfig)
	require.Equal(t, promaws.DefaultEC2SDConfig.Port, cfg.EC2SDConfig.Port)
	require.Equal(t, promaws.DefaultEC2SDConfig.RefreshInterval, cfg.EC2SDConfig.RefreshInterval)

	set := AWSArguments{Role: "ec2", Region: "us-east-1", Port: 9090, RefreshInterval: 30 * time.Second, HTTPClientConfig: config.DefaultHTTPClientConfig}
	cfg = set.Convert().(*promaws.SDConfig)
	require.Equal(t, 9090, cfg.EC2SDConfig.Port)
	require.Equal(t, model.Duration(30*time.Second), cfg.EC2SDConfig.RefreshInterval)
}

func TestAWSValidate(t *testing.T) {
	base := func() AWSArguments {
		return AWSArguments{Region: "us-east-1", HTTPClientConfig: config.DefaultHTTPClientConfig}
	}

	t.Run("unknown role", func(t *testing.T) {
		args := base()
		args.Role = "sqs"
		require.ErrorContains(t, args.Validate(), "unsupported role")
	})

	t.Run("valid ec2", func(t *testing.T) {
		args := base()
		args.Role = "ec2"
		require.NoError(t, args.Validate())
	})

	t.Run("empty filter values", func(t *testing.T) {
		args := base()
		args.Role = "ec2"
		args.Filters = []*EC2Filter{{Name: "tag:env"}}
		require.ErrorContains(t, args.Validate(), "filter values cannot be empty")
	})

	t.Run("filter with non-ec2 role", func(t *testing.T) {
		args := base()
		args.Role = "ecs"
		args.Filters = []*EC2Filter{{Name: "tag:env", Values: []string{"prod"}}}
		require.ErrorContains(t, args.Validate(), "only supported with the ec2 role")
	})

	t.Run("clusters with ec2 role", func(t *testing.T) {
		args := base()
		args.Role = "ec2"
		args.Clusters = []string{"c1"}
		require.ErrorContains(t, args.Validate(), "clusters is only supported")
	})
}

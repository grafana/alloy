package resourcedetection_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/akamai"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/aws/ec2"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/aws/ecs"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/aws/eks"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/aws/elasticbeanstalk"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/aws/lambda"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/azure"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/azure/aks"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/consul"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/digitalocean"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/docker"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/dynatrace"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/gcp"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/heroku"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/hetzner"
	kubernetes_node "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/k8snode"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/kubeadm"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/openshift"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/openstacknova"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/oraclecloud"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/scaleway"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/system"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/upcloud"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/vultr"
	"github.com/grafana/alloy/syntax"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	var defaultArgs system.Config
	defaultArgs.SetToDefault()

	tests := []struct {
		testName string
		cfg      string
		expected map[string]any
		errorMsg string
	}{
		{
			testName: "err_no_detector",
			cfg: `
			detectors = []
			output {}
			`,
			errorMsg: "at least one detector must be specified",
		},
		{
			testName: "invalid_detector",
			cfg: `
			detectors = ["non-existent-detector"]
			output {}
			`,
			errorMsg: "invalid detector: non-existent-detector",
		},
		{
			testName: "invalid_detector_and_all_valid_ones",
			cfg: `
			detectors = ["non-existent-detector2", "env", "ec2", "ecs", "eks", "elasticbeanstalk", "lambda", "azure", "aks", "consul", "docker", "gcp", "heroku", "system", "openshift", "kubernetes_node", "dynatrace", "kubeadm"]
			output {}
			`,
			errorMsg: "invalid detector: non-existent-detector2",
		},
		{
			testName: "all_detectors_with_defaults",
			cfg: `
			detectors = ["env", "ec2", "ecs", "eks", "elasticbeanstalk", "lambda", "azure", "aks", "akamai", "consul", "digitalocean", "docker", "gcp", "heroku", "hetzner", "system", "openshift", "nova", "oraclecloud", "kubernetes_node", "dynatrace", "kubeadm", "scaleway", "upcloud", "vultr"]
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"env", "ec2", "ecs", "eks", "elasticbeanstalk", "lambda", "azure", "aks", "akamai", "consul", "digitalocean", "docker", "gcp", "heroku", "hetzner", "system", "openshift", "nova", "oraclecloud", "k8snode", "dynatrace", "kubeadm", "scaleway", "upcloud", "vultr"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "default_detector",
			cfg: `
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"env"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "ec2_defaults",
			cfg: `
			detectors = ["ec2"]
			ec2 {
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"ec2"},
				"timeout":   5 * time.Second,
				"override":  true,
				"ec2": map[string]any{
					"tags": []string{},
					"resource_attributes": map[string]any{
						"cloud.account.id":        map[string]any{"enabled": true},
						"cloud.availability_zone": map[string]any{"enabled": true},
						"cloud.platform":          map[string]any{"enabled": true},
						"cloud.provider":          map[string]any{"enabled": true},
						"cloud.region":            map[string]any{"enabled": true},
						"host.id":                 map[string]any{"enabled": true},
						"host.image.id":           map[string]any{"enabled": true},
						"host.name":               map[string]any{"enabled": true},
						"host.type":               map[string]any{"enabled": true},
					},
					"max_attempts":             3,
					"max_backoff":              20 * time.Second,
					"fail_on_missing_metadata": false,
				},
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "ec2_defaults_empty_resource_attributes",
			cfg: `
			detectors = ["ec2"]
			ec2 {
				resource_attributes {}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"ec2"},
				"timeout":   5 * time.Second,
				"override":  true,
				"ec2": map[string]any{
					"tags": []string{},
					"resource_attributes": map[string]any{
						"cloud.account.id":        map[string]any{"enabled": true},
						"cloud.availability_zone": map[string]any{"enabled": true},
						"cloud.platform":          map[string]any{"enabled": true},
						"cloud.provider":          map[string]any{"enabled": true},
						"cloud.region":            map[string]any{"enabled": true},
						"host.id":                 map[string]any{"enabled": true},
						"host.image.id":           map[string]any{"enabled": true},
						"host.name":               map[string]any{"enabled": true},
						"host.type":               map[string]any{"enabled": true},
					},
					"max_attempts":             3,
					"max_backoff":              20 * time.Second,
					"fail_on_missing_metadata": false,
				},
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "ec2_explicit",
			cfg: `
			detectors = ["ec2"]
			ec2 {
				tags = ["^tag1$", "^tag2$", "^label.*$"]
				resource_attributes {
					cloud.account.id  { enabled = true }
					cloud.availability_zone  { enabled = true }
					cloud.platform  { enabled = true }
					cloud.provider  { enabled = true }
					cloud.region  { enabled = true }
					host.id  { enabled = true }
					host.image.id  { enabled = false }
					host.name  { enabled = false }
					host.type  { enabled = false }
				}
				max_attempts = 5
				max_backoff = "10s"
				fail_on_missing_metadata = true
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"ec2"},
				"timeout":   5 * time.Second,
				"override":  true,
				"ec2": map[string]any{
					"tags": []string{"^tag1$", "^tag2$", "^label.*$"},
					"resource_attributes": map[string]any{
						"cloud.account.id":        map[string]any{"enabled": true},
						"cloud.availability_zone": map[string]any{"enabled": true},
						"cloud.platform":          map[string]any{"enabled": true},
						"cloud.provider":          map[string]any{"enabled": true},
						"cloud.region":            map[string]any{"enabled": true},
						"host.id":                 map[string]any{"enabled": true},
						"host.image.id":           map[string]any{"enabled": false},
						"host.name":               map[string]any{"enabled": false},
						"host.type":               map[string]any{"enabled": false},
					},
					"max_attempts":             5,
					"max_backoff":              10 * time.Second,
					"fail_on_missing_metadata": true,
				},
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "ecs_defaults",
			cfg: `
			detectors = ["ecs"]
			ecs {
				resource_attributes {}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"ecs"},
				"timeout":   5 * time.Second,
				"override":  true,
				"ecs": map[string]any{
					"tags": []string{},
					"resource_attributes": map[string]any{
						"aws.ecs.cluster.arn":     map[string]any{"enabled": true},
						"aws.ecs.launchtype":      map[string]any{"enabled": true},
						"aws.ecs.task.arn":        map[string]any{"enabled": true},
						"aws.ecs.task.family":     map[string]any{"enabled": true},
						"aws.ecs.task.id":         map[string]any{"enabled": true},
						"aws.ecs.task.revision":   map[string]any{"enabled": true},
						"aws.log.group.arns":      map[string]any{"enabled": true},
						"aws.log.group.names":     map[string]any{"enabled": true},
						"aws.log.stream.arns":     map[string]any{"enabled": true},
						"aws.log.stream.names":    map[string]any{"enabled": true},
						"cloud.account.id":        map[string]any{"enabled": true},
						"cloud.availability_zone": map[string]any{"enabled": true},
						"cloud.platform":          map[string]any{"enabled": true},
						"cloud.provider":          map[string]any{"enabled": true},
						"cloud.region":            map[string]any{"enabled": true},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "ecs_explicit",
			cfg: `
			detectors = ["ecs"]
			ecs {
				resource_attributes {
					aws.ecs.cluster.arn  { enabled = true }
					aws.ecs.launchtype  { enabled = true }
					aws.ecs.task.arn  { enabled = true }
					aws.ecs.task.family  { enabled = true }
					aws.ecs.task.id  { enabled = true }
					aws.ecs.task.revision  { enabled = true }
					aws.log.group.arns  { enabled = true }
					aws.log.group.names  { enabled = false }
					// aws.log.stream.arns  { enabled = true }
					// aws.log.stream.names  { enabled = true }
					// cloud.account.id  { enabled = true }
					// cloud.availability_zone  { enabled = true }
					// cloud.platform  { enabled = true }
					// cloud.provider  { enabled = true }
					// cloud.region  { enabled = true }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"ecs"},
				"timeout":   5 * time.Second,
				"override":  true,
				"ecs": map[string]any{
					"tags": []string{},
					"resource_attributes": map[string]any{
						"aws.ecs.cluster.arn":     map[string]any{"enabled": true},
						"aws.ecs.launchtype":      map[string]any{"enabled": true},
						"aws.ecs.task.arn":        map[string]any{"enabled": true},
						"aws.ecs.task.family":     map[string]any{"enabled": true},
						"aws.ecs.task.id":         map[string]any{"enabled": true},
						"aws.ecs.task.revision":   map[string]any{"enabled": true},
						"aws.log.group.arns":      map[string]any{"enabled": true},
						"aws.log.group.names":     map[string]any{"enabled": false},
						"aws.log.stream.arns":     map[string]any{"enabled": true},
						"aws.log.stream.names":    map[string]any{"enabled": true},
						"cloud.account.id":        map[string]any{"enabled": true},
						"cloud.availability_zone": map[string]any{"enabled": true},
						"cloud.platform":          map[string]any{"enabled": true},
						"cloud.provider":          map[string]any{"enabled": true},
						"cloud.region":            map[string]any{"enabled": true},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "eks_defaults",
			cfg: `
			detectors = ["eks"]
			eks {}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"eks"},
				"timeout":   5 * time.Second,
				"override":  true,
				"eks": map[string]any{
					"tags": []string{},
					"resource_attributes": map[string]any{
						"cloud.platform": map[string]any{
							"enabled": true,
						},
						"cloud.provider": map[string]any{
							"enabled": true,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "eks_explicit",
			cfg: `
			detectors = ["eks"]
			eks {
				resource_attributes {
					cloud.account.id { enabled = true }
					cloud.platform { enabled = true }
					cloud.provider { enabled = false }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"eks"},
				"timeout":   5 * time.Second,
				"override":  true,
				"eks": map[string]any{
					"tags": []string{},
					"resource_attributes": map[string]any{
						"cloud.account.id": map[string]any{
							"enabled": true,
						},
						"cloud.platform": map[string]any{
							"enabled": true,
						},
						"cloud.provider": map[string]any{
							"enabled": false,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "azure_defaults",
			cfg: `
			detectors = ["azure"]
			azure {}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"azure"},
				"timeout":   5 * time.Second,
				"override":  true,
				"azure": map[string]any{
					"resource_attributes": map[string]any{
						"tags": []string{},
						"azure.resourcegroup.name": map[string]any{
							"enabled": true,
						},
						"azure.vm.name": map[string]any{
							"enabled": true,
						},
						"azure.vm.scaleset.name": map[string]any{
							"enabled": true,
						},
						"azure.vm.size": map[string]any{
							"enabled": true,
						},
						"cloud.account.id": map[string]any{
							"enabled": true,
						},
						"cloud.platform": map[string]any{
							"enabled": true,
						},
						"cloud.provider": map[string]any{
							"enabled": true,
						},
						"cloud.region": map[string]any{
							"enabled": true,
						},
						"host.id": map[string]any{
							"enabled": true,
						},
						"host.name": map[string]any{
							"enabled": true,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "azure_explicit",
			cfg: `
			detectors = ["azure"]
			azure {
				tags = ["tag1","tag2"]
				resource_attributes {
					azure.resourcegroup.name { enabled = true }
					azure.vm.name { enabled = true }
					azure.vm.scaleset.name { enabled = true }
					azure.vm.size { enabled = true }
					cloud.account.id { enabled = false }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"azure"},
				"timeout":   5 * time.Second,
				"override":  true,
				"azure": map[string]any{
					"tags": []string{"tag1", "tag2"},
					"resource_attributes": map[string]any{
						"azure.resourcegroup.name": map[string]any{
							"enabled": true,
						},
						"azure.vm.name": map[string]any{
							"enabled": true,
						},
						"azure.vm.scaleset.name": map[string]any{
							"enabled": true,
						},
						"azure.vm.size": map[string]any{
							"enabled": true,
						},
						"cloud.account.id": map[string]any{
							"enabled": false,
						},
						"cloud.platform": map[string]any{
							"enabled": true,
						},
						"cloud.provider": map[string]any{
							"enabled": true,
						},
						"cloud.region": map[string]any{
							"enabled": true,
						},
						"host.id": map[string]any{
							"enabled": true,
						},
						"host.name": map[string]any{
							"enabled": true,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "aks_defaults",
			cfg: `
			detectors = ["aks"]
			aks {}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"aks"},
				"timeout":   5 * time.Second,
				"override":  true,
				"aks": map[string]any{
					"tags": []string{},
					"resource_attributes": map[string]any{
						"cloud.platform": map[string]any{
							"enabled": true,
						},
						"cloud.provider": map[string]any{
							"enabled": true,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "aks_explicit",
			cfg: `
			detectors = ["aks"]
			aks {
				resource_attributes {
					cloud.platform { enabled = true }
					cloud.provider { enabled = false }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"aks"},
				"timeout":   5 * time.Second,
				"override":  true,
				"aks": map[string]any{
					"tags": []string{},
					"resource_attributes": map[string]any{
						"cloud.platform": map[string]any{
							"enabled": true,
						},
						"cloud.provider": map[string]any{
							"enabled": false,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "gcp_defaults",
			cfg: `
			detectors = ["gcp"]
			gcp {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"gcp"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "gcp_explicit",
			cfg: `
			detectors = ["gcp"]
			gcp {
				resource_attributes {
					cloud.account.id { enabled = true }
					cloud.availability_zone { enabled = true }
					cloud.platform { enabled = true }
					cloud.provider { enabled = true }
					cloud.region { enabled = false }
					faas.id { enabled = false }
					gcp.gce.instance.group_manager.zone { enabled = false }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"gcp"},
				"timeout":   5 * time.Second,
				"override":  true,
				"gcp": map[string]any{
					"resource_attributes": map[string]any{
						"cloud.account.id": map[string]any{
							"enabled": true,
						},
						"cloud.availability_zone": map[string]any{
							"enabled": true,
						},
						"cloud.platform": map[string]any{
							"enabled": true,
						},
						"cloud.provider": map[string]any{
							"enabled": true,
						},
						"cloud.region": map[string]any{
							"enabled": false,
						},
						"faas.id": map[string]any{
							"enabled": false,
						},
						"faas.instance": map[string]any{
							"enabled": true,
						},
						"faas.name": map[string]any{
							"enabled": true,
						},
						"faas.version": map[string]any{
							"enabled": true,
						},
						"gcp.cloud_run.job.execution": map[string]any{
							"enabled": true,
						},
						"gcp.cloud_run.job.task_index": map[string]any{
							"enabled": true,
						},
						"gcp.gce.instance.hostname": map[string]any{
							"enabled": false,
						},
						"gcp.gce.instance.name": map[string]any{
							"enabled": false,
						},
						"gcp.gce.instance.group_manager.name": map[string]any{
							"enabled": true,
						},
						"gcp.gce.instance.group_manager.region": map[string]any{
							"enabled": true,
						},
						"gcp.gce.instance.group_manager.zone": map[string]any{
							"enabled": false,
						},
						"host.id": map[string]any{
							"enabled": true,
						},
						"host.name": map[string]any{
							"enabled": true,
						},
						"host.type": map[string]any{
							"enabled": true,
						},
						"k8s.cluster.name": map[string]any{
							"enabled": true,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "docker_defaults",
			cfg: `
			detectors = ["docker"]
			docker {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"docker"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "docker_explicit",
			cfg: `
			detectors = ["docker"]
			docker {
				resource_attributes {
					host.name { enabled = true }
					os.type { enabled = false }

				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"docker"},
				"timeout":   5 * time.Second,
				"override":  true,
				"docker": map[string]any{
					"resource_attributes": map[string]any{
						"host.name": map[string]any{
							"enabled": true,
						},
						"os.type": map[string]any{
							"enabled": false,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "lambda_defaults",
			cfg: `
			detectors = ["lambda"]
			lambda {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"lambda"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "lambda_explicit",
			cfg: `
			detectors = ["lambda"]
			lambda {
				resource_attributes {
					aws.log.group.names { enabled = true }
					aws.log.stream.names { enabled = true }
					cloud.platform { enabled = true }
					cloud.provider { enabled = false }
					cloud.region { enabled = false }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"lambda"},
				"timeout":   5 * time.Second,
				"override":  true,
				"lambda": map[string]any{
					"resource_attributes": map[string]any{
						"aws.log.group.names": map[string]any{
							"enabled": true,
						},
						"aws.log.stream.names": map[string]any{
							"enabled": true,
						},
						"cloud.platform": map[string]any{
							"enabled": true,
						},
						"cloud.provider": map[string]any{
							"enabled": false,
						},
						"cloud.region": map[string]any{
							"enabled": false,
						},
						"faas.instance": map[string]any{
							"enabled": true,
						},
						"faas.max_memory": map[string]any{
							"enabled": true,
						},
						"faas.name": map[string]any{
							"enabled": true,
						},
						"faas.version": map[string]any{
							"enabled": true,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "elasticbeanstalk_defaults",
			cfg: `
			detectors = ["elasticbeanstalk"]
			elasticbeanstalk {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"elasticbeanstalk"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "elasticbeanstalk_explicit",
			cfg: `
			detectors = ["elasticbeanstalk"]
			elasticbeanstalk {
				resource_attributes {
					cloud.platform { enabled = true }
					cloud.provider { enabled = true }
					deployment.environment { enabled = true }
					service.instance.id { enabled = false }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"elasticbeanstalk"},
				"timeout":   5 * time.Second,
				"override":  true,
				"elasticbeanstalk": map[string]any{
					"resource_attributes": map[string]any{
						"cloud.platform": map[string]any{
							"enabled": true,
						},
						"cloud.provider": map[string]any{
							"enabled": true,
						},
						"deployment.environment": map[string]any{
							"enabled": true,
						},
						"service.instance.id": map[string]any{
							"enabled": false,
						},
						"service.version": map[string]any{
							"enabled": true,
						},
					},
				},
				"ec2":          ec2.DefaultArguments.Convert(),
				"ecs":          ecs.DefaultArguments.Convert(),
				"eks":          eks.DefaultArguments.Convert(),
				"lambda":       lambda.DefaultArguments.Convert(),
				"azure":        azure.DefaultArguments.Convert(),
				"aks":          aks.DefaultArguments.Convert(),
				"consul":       consul.DefaultArguments.Convert(),
				"docker":       docker.DefaultArguments.Convert(),
				"gcp":          gcp.DefaultArguments.Convert(),
				"heroku":       heroku.DefaultArguments.Convert(),
				"system":       defaultArgs.Convert(),
				"openshift":    openshift.DefaultArguments.Convert(),
				"k8snode":      kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":      kubeadm.DefaultArguments.Convert(),
				"dynatrace":    dynatrace.DefaultArguments.Convert(),
				"akamai":       akamai.DefaultArguments.Convert(),
				"digitalocean": digitalocean.DefaultArguments.Convert(),
				"hetzner":      hetzner.DefaultArguments.Convert(),
				"nova":         openstacknova.DefaultArguments.Convert(),
				"oraclecloud":  oraclecloud.DefaultArguments.Convert(),
				"scaleway":     scaleway.DefaultArguments.Convert(),
				"upcloud":      upcloud.DefaultArguments.Convert(),
				"vultr":        vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "consul_defaults",
			cfg: `
			detectors = ["consul"]
			consul {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"consul"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "consul_explicit",
			cfg: `
			detectors = ["consul"]
			consul {
				address = "localhost:8500"
				datacenter = "dc1"
				token = "secret_token"
				namespace = "test_namespace"
				meta = ["test"]
				resource_attributes {
					cloud.region { enabled = false }
					host.id { enabled = false }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"consul"},
				"timeout":   5 * time.Second,
				"override":  true,
				"consul": map[string]any{
					"address":    "localhost:8500",
					"datacenter": "dc1",
					"token":      "secret_token",
					"namespace":  "test_namespace",
					"meta":       map[string]string{"test": ""},
					"resource_attributes": map[string]any{
						"cloud.region": map[string]any{
							"enabled": false,
						},
						"host.id": map[string]any{
							"enabled": false,
						},
						"host.name": map[string]any{
							"enabled": true,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "heroku_defaults",
			cfg: `
			detectors = ["heroku"]
			heroku {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"heroku"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "heroku_explicit",
			cfg: `
			detectors = ["heroku"]
			heroku {
				resource_attributes {
					cloud.provider { enabled = true }
					heroku.app.id { enabled = true }
					heroku.dyno.id { enabled = true }
					heroku.release.commit { enabled = true }
					heroku.release.creation_timestamp { enabled = false }
					service.instance.id { enabled = false }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"heroku"},
				"timeout":   5 * time.Second,
				"override":  true,
				"heroku": map[string]any{
					"resource_attributes": map[string]any{
						"cloud.provider": map[string]any{
							"enabled": true,
						},
						"heroku.app.id": map[string]any{
							"enabled": true,
						},
						"heroku.dyno.id": map[string]any{
							"enabled": true,
						},
						"heroku.release.commit": map[string]any{
							"enabled": true,
						},
						"heroku.release.creation_timestamp": map[string]any{
							"enabled": false,
						},
						"service.instance.id": map[string]any{
							"enabled": false,
						},
						"service.name": map[string]any{
							"enabled": true,
						},
						"service.version": map[string]any{
							"enabled": true,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "kubernetes_node_defaults",
			cfg: `
			detectors = ["kubernetes_node"]
			kubernetes_node {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"k8snode"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "kubernetes_node_explicit",
			cfg: `
			detectors = ["kubernetes_node"]
			kubernetes_node {
				auth_type = "kubeConfig"
				context = "fake_ctx"
				node_from_env_var = "MY_CUSTOM_VAR"
				resource_attributes {
					k8s.node.name { enabled = true }
					k8s.node.uid { enabled = false }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"k8snode"},
				"timeout":   5 * time.Second,
				"override":  true,
				"k8snode": map[string]any{
					"auth_type":         "kubeConfig",
					"context":           "fake_ctx",
					"node_from_env_var": "MY_CUSTOM_VAR",
					"resource_attributes": map[string]any{
						"k8s.node.name": map[string]any{
							"enabled": true,
						},
						"k8s.node.uid": map[string]any{
							"enabled": false,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		}, {
			testName: "kubeadm_defaults",
			cfg: `
			detectors = ["kubeadm"]
			kubeadm {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"kubeadm"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "kubeadm_explicit",
			cfg: `
			detectors = ["kubeadm"]
			kubeadm {
				auth_type = "kubeConfig"
				context = "fake_ctx"
				resource_attributes {
					k8s.cluster.name { enabled = true }
					k8s.cluster.uid { enabled = true }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"kubeadm"},
				"timeout":   5 * time.Second,
				"override":  true,
				"kubeadm": map[string]any{
					"auth_type": "kubeConfig",
					"context":   "fake_ctx",
					"resource_attributes": map[string]any{
						"k8s.cluster.name": map[string]any{
							"enabled": true,
						},
						"k8s.cluster.uid": map[string]any{
							"enabled": true,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "system_invalid_hostname_source",
			cfg: `
			detectors = ["system"]
			system {
				hostname_sources = ["asdf"]
				resource_attributes { }
			}
			output {}
			`,
			errorMsg: "invalid hostname source: asdf",
		},
		{
			testName: "system_defaults",
			cfg: `
			detectors = ["system"]
			system {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"system"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "system_explicit",
			cfg: `
			detectors = ["system"]
			system {
				hostname_sources = ["cname","lookup"]
				resource_attributes {
					host.arch { enabled = true }
					host.cpu.cache.l2.size { enabled = true }
					host.cpu.family { enabled = true }
					host.cpu.model.id { enabled = true }
					host.cpu.model.name { enabled = true }
					host.cpu.stepping { enabled = true }
					host.cpu.vendor.id { enabled = false }
					host.id { enabled = false }
					host.interface { enabled = true }
					host.name { enabled = false }
					// os.description { enabled = false }
					// os.type { enabled = true }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"system"},
				"timeout":   5 * time.Second,
				"override":  true,
				"system": map[string]any{
					"hostname_sources": []string{"cname", "lookup"},
					"resource_attributes": map[string]any{
						"host.arch": map[string]any{
							"enabled": true,
						},
						"host.cpu.cache.l2.size": map[string]any{
							"enabled": true,
						},
						"host.cpu.family": map[string]any{
							"enabled": true,
						},
						"host.cpu.model.id": map[string]any{
							"enabled": true,
						},
						"host.cpu.model.name": map[string]any{
							"enabled": true,
						},
						"host.cpu.stepping": map[string]any{
							"enabled": true,
						},
						"host.cpu.vendor.id": map[string]any{
							"enabled": false,
						},
						"host.id": map[string]any{
							"enabled": false,
						},
						"host.interface": map[string]any{
							"enabled": true,
						},
						"host.name": map[string]any{
							"enabled": false,
						},
						"os.build.id": map[string]any{
							"enabled": false,
						},
						"os.description": map[string]any{
							"enabled": false,
						},
						"os.name": map[string]any{
							"enabled": false,
						},
						"os.type": map[string]any{
							"enabled": true,
						},
						"os.version": map[string]any{
							"enabled": false,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "openshift_default",
			cfg: `
			detectors = ["openshift"]
			openshift {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"openshift"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "openshift_explicit",
			cfg: `
			detectors = ["openshift"]
			timeout = "7s"
			override = false
			openshift {
				address = "127.0.0.1:4444"
				token = "some_token"
				tls {
					insecure = true
				}
				resource_attributes {
					cloud.platform {
						enabled = true
					}
					cloud.provider {
						enabled = true
					}
					cloud.region {
						enabled = false
					}
					k8s.cluster.name {
						enabled = false
					}
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"openshift"},
				"timeout":   7 * time.Second,
				"override":  false,
				"openshift": map[string]any{
					"address": "127.0.0.1:4444",
					"token":   "some_token",
					"tls": map[string]any{
						"insecure": true,
					},
					"resource_attributes": map[string]any{
						"cloud.platform": map[string]any{
							"enabled": true,
						},
						"cloud.provider": map[string]any{
							"enabled": true,
						},
						"cloud.region": map[string]any{
							"enabled": false,
						},
						"k8s.cluster.name": map[string]any{
							"enabled": false,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "env",
			cfg: `
			detectors = ["env"]
			timeout = "7s"
			override = false
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"env"},
				"timeout":          7 * time.Second,
				"override":         false,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "dynatrace",
			cfg: `
			detectors = ["dynatrace"]
			timeout = "7s"
			override = false
			dynatrace {
				resource_attributes {
					host.name {
						enabled = true
					}
					dt.entity.host {
						enabled = true
					}
				}
			}
			output {}
			`,
			expected: map[string]any{
				"dynatrace": map[string]any{
					"resource_attributes": map[string]any{
						"host.name": map[string]any{
							"enabled": true,
						},
						"dt.entity.host": map[string]any{
							"enabled": true,
						},
					},
				},
				"detectors":        []string{"dynatrace"},
				"timeout":          7 * time.Second,
				"override":         false,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "akamai_defaults",
			cfg: `
			detectors = ["akamai"]
			akamai {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"akamai"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "akamai_explicit",
			cfg: `
			detectors = ["akamai"]
			akamai {
				fail_on_missing_metadata = true
				resource_attributes {
					cloud.provider { enabled = false }
					cloud.region { enabled = true }
					host.id { enabled = true }
					host.name { enabled = false }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"akamai"},
				"timeout":   5 * time.Second,
				"override":  true,
				"akamai": map[string]any{
					"fail_on_missing_metadata": true,
					"resource_attributes": map[string]any{
						"cloud.provider": map[string]any{
							"enabled": false,
						},
						"cloud.region": map[string]any{
							"enabled": true,
						},
						"host.id": map[string]any{
							"enabled": true,
						},
						"host.name": map[string]any{
							"enabled": false,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "digitalocean_defaults",
			cfg: `
			detectors = ["digitalocean"]
			digitalocean {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"digitalocean"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "digitalocean_explicit",
			cfg: `
			detectors = ["digitalocean"]
			digitalocean {
				resource_attributes {
					cloud.provider { enabled = true }
					cloud.region { enabled = false }
					host.id { enabled = false }
					host.name { enabled = true }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"digitalocean"},
				"timeout":   5 * time.Second,
				"override":  true,
				"digitalocean": map[string]any{
					"resource_attributes": map[string]any{
						"cloud.provider": map[string]any{
							"enabled": true,
						},
						"cloud.region": map[string]any{
							"enabled": false,
						},
						"host.id": map[string]any{
							"enabled": false,
						},
						"host.name": map[string]any{
							"enabled": true,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "hetzner_defaults",
			cfg: `
			detectors = ["hetzner"]
			hetzner {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"hetzner"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "hetzner_explicit",
			cfg: `
			detectors = ["hetzner"]
			hetzner {
				resource_attributes {
					cloud.availability_zone { enabled = false }
					cloud.provider { enabled = true }
					cloud.region { enabled = false }
					host.id { enabled = true }
					host.name { enabled = true }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"hetzner"},
				"timeout":   5 * time.Second,
				"override":  true,
				"hetzner": map[string]any{
					"resource_attributes": map[string]any{
						"cloud.availability_zone": map[string]any{
							"enabled": false,
						},
						"cloud.provider": map[string]any{
							"enabled": true,
						},
						"cloud.region": map[string]any{
							"enabled": false,
						},
						"host.id": map[string]any{
							"enabled": true,
						},
						"host.name": map[string]any{
							"enabled": true,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "scaleway_defaults",
			cfg: `
			detectors = ["scaleway"]
			scaleway {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"scaleway"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "scaleway_explicit",
			cfg: `
			detectors = ["scaleway"]
			scaleway {
				resource_attributes {
					cloud.account.id { enabled = false }
					cloud.availability_zone { enabled = true }
					cloud.platform { enabled = false }
					cloud.provider { enabled = true }
					cloud.region { enabled = true }
					host.id { enabled = false }
					host.image.id { enabled = true }
					host.image.name { enabled = false }
					host.name { enabled = true }
					host.type { enabled = false }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"scaleway"},
				"timeout":   5 * time.Second,
				"override":  true,
				"scaleway": map[string]any{
					"resource_attributes": map[string]any{
						"cloud.account.id": map[string]any{
							"enabled": false,
						},
						"cloud.availability_zone": map[string]any{
							"enabled": true,
						},
						"cloud.platform": map[string]any{
							"enabled": false,
						},
						"cloud.provider": map[string]any{
							"enabled": true,
						},
						"cloud.region": map[string]any{
							"enabled": true,
						},
						"host.id": map[string]any{
							"enabled": false,
						},
						"host.image.id": map[string]any{
							"enabled": true,
						},
						"host.image.name": map[string]any{
							"enabled": false,
						},
						"host.name": map[string]any{
							"enabled": true,
						},
						"host.type": map[string]any{
							"enabled": false,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "upcloud_defaults",
			cfg: `
			detectors = ["upcloud"]
			upcloud {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"upcloud"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "upcloud_explicit",
			cfg: `
			detectors = ["upcloud"]
			upcloud {
				fail_on_missing_metadata = true
				resource_attributes {
					cloud.provider { enabled = false }
					cloud.region { enabled = true }
					host.id { enabled = true }
					host.name { enabled = false }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"upcloud"},
				"timeout":   5 * time.Second,
				"override":  true,
				"upcloud": map[string]any{
					"fail_on_missing_metadata": true,
					"resource_attributes": map[string]any{
						"cloud.provider": map[string]any{
							"enabled": false,
						},
						"cloud.region": map[string]any{
							"enabled": true,
						},
						"host.id": map[string]any{
							"enabled": true,
						},
						"host.name": map[string]any{
							"enabled": false,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "vultr_defaults",
			cfg: `
			detectors = ["vultr"]
			vultr {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"vultr"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "vultr_explicit",
			cfg: `
			detectors = ["vultr"]
			vultr {
				fail_on_missing_metadata = true
				resource_attributes {
					cloud.provider { enabled = true }
					cloud.region { enabled = false }
					host.id { enabled = false }
					host.name { enabled = true }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"vultr"},
				"timeout":   5 * time.Second,
				"override":  true,
				"vultr": map[string]any{
					"fail_on_missing_metadata": true,
					"resource_attributes": map[string]any{
						"cloud.provider": map[string]any{
							"enabled": true,
						},
						"cloud.region": map[string]any{
							"enabled": false,
						},
						"host.id": map[string]any{
							"enabled": false,
						},
						"host.name": map[string]any{
							"enabled": true,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
			},
		},
		{
			testName: "nova_defaults",
			cfg: `
			detectors = ["nova"]
			nova {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"nova"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "oraclecloud_defaults",
			cfg: `
			detectors = ["oraclecloud"]
			oraclecloud {}
			output {}
			`,
			expected: map[string]any{
				"detectors":        []string{"oraclecloud"},
				"timeout":          5 * time.Second,
				"override":         true,
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "nova_explicit",
			cfg: `
			detectors = ["nova"]
			nova {
				resource_attributes {
					cloud.platform { enabled = false }
					cloud.provider { enabled = true }
					cloud.region { enabled = false }
					cloud.availability_zone { enabled = true }
					host.id { enabled = false }
					host.name { enabled = true }
					host.type { enabled = false }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"nova"},
				"timeout":   5 * time.Second,
				"override":  true,
				"nova": map[string]any{
					"fail_on_missing_metadata": false,
					"labels":                   nil,
					"resource_attributes": map[string]any{
						"cloud.platform": map[string]any{
							"enabled": false,
						},
						"cloud.provider": map[string]any{
							"enabled": true,
						},
						"cloud.region": map[string]any{
							"enabled": false,
						},
						"cloud.availability_zone": map[string]any{
							"enabled": true,
						},
						"host.id": map[string]any{
							"enabled": false,
						},
						"host.name": map[string]any{
							"enabled": true,
						},
						"host.type": map[string]any{
							"enabled": false,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"oraclecloud":      oraclecloud.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
		{
			testName: "oraclecloud_explicit",
			cfg: `
			detectors = ["oraclecloud"]
			oraclecloud {
				resource_attributes {
					cloud.platform { enabled = false }
					cloud.provider { enabled = true }
					cloud.region { enabled = false }
					cloud.availability_zone { enabled = true }
					host.id { enabled = false }
					host.name { enabled = true }
					host.type { enabled = false }
					k8s.cluster.name { enabled = true }
				}
			}
			output {}
			`,
			expected: map[string]any{
				"detectors": []string{"oraclecloud"},
				"timeout":   5 * time.Second,
				"override":  true,
				"oraclecloud": map[string]any{
					"resource_attributes": map[string]any{
						"cloud.platform": map[string]any{
							"enabled": false,
						},
						"cloud.provider": map[string]any{
							"enabled": true,
						},
						"cloud.region": map[string]any{
							"enabled": false,
						},
						"cloud.availability_zone": map[string]any{
							"enabled": true,
						},
						"host.id": map[string]any{
							"enabled": false,
						},
						"host.name": map[string]any{
							"enabled": true,
						},
						"host.type": map[string]any{
							"enabled": false,
						},
						"k8s.cluster.name": map[string]any{
							"enabled": true,
						},
					},
				},
				"ec2":              ec2.DefaultArguments.Convert(),
				"ecs":              ecs.DefaultArguments.Convert(),
				"eks":              eks.DefaultArguments.Convert(),
				"elasticbeanstalk": elasticbeanstalk.DefaultArguments.Convert(),
				"lambda":           lambda.DefaultArguments.Convert(),
				"azure":            azure.DefaultArguments.Convert(),
				"aks":              aks.DefaultArguments.Convert(),
				"akamai":           akamai.DefaultArguments.Convert(),
				"consul":           consul.DefaultArguments.Convert(),
				"digitalocean":     digitalocean.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"hetzner":          hetzner.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
				"kubeadm":          kubeadm.DefaultArguments.Convert(),
				"dynatrace":        dynatrace.DefaultArguments.Convert(),
				"nova":             openstacknova.DefaultArguments.Convert(),
				"scaleway":         scaleway.DefaultArguments.Convert(),
				"upcloud":          upcloud.DefaultArguments.Convert(),
				"vultr":            vultr.DefaultArguments.Convert(),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args resourcedetection.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			if tc.errorMsg != "" {
				require.ErrorContains(t, err, tc.errorMsg)
				return
			}

			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*resourcedetectionprocessor.Config)

			var expected resourcedetectionprocessor.Config
			err = mapstructure.Decode(tc.expected, &expected)
			require.NoError(t, err)

			require.Equal(t, expected, *actual)
		})
	}
}

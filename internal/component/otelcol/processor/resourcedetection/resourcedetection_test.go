package resourcedetection_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/aws/ec2"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/aws/ecs"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/aws/eks"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/aws/elasticbeanstalk"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/aws/lambda"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/azure"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/azure/aks"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/consul"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/docker"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/gcp"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/heroku"
	kubernetes_node "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/k8snode"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/openshift"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/system"
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
		expected map[string]interface{}
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
			detectors = ["non-existent-detector2", "env", "ec2", "ecs", "eks", "elasticbeanstalk", "lambda", "azure", "aks", "consul", "docker", "gcp", "heroku", "system", "openshift", "kubernetes_node"]
			output {}
			`,
			errorMsg: "invalid detector: non-existent-detector2",
		},
		{
			testName: "all_detectors_with_defaults",
			cfg: `
			detectors = ["env", "ec2", "ecs", "eks", "elasticbeanstalk", "lambda", "azure", "aks", "consul", "docker", "gcp", "heroku", "system", "openshift", "kubernetes_node"]
			output {}
			`,
			expected: map[string]interface{}{
				"detectors":        []string{"env", "ec2", "ecs", "eks", "elasticbeanstalk", "lambda", "azure", "aks", "consul", "docker", "gcp", "heroku", "system", "openshift", "k8snode"},
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
			},
		},
		{
			testName: "default_detector",
			cfg: `
			output {}
			`,
			expected: map[string]interface{}{
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
				"consul":           consul.DefaultArguments.Convert(),
				"docker":           docker.DefaultArguments.Convert(),
				"gcp":              gcp.DefaultArguments.Convert(),
				"heroku":           heroku.DefaultArguments.Convert(),
				"system":           defaultArgs.Convert(),
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
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
			expected: map[string]interface{}{
				"detectors": []string{"ec2"},
				"timeout":   5 * time.Second,
				"override":  true,
				"ec2": map[string]interface{}{
					"tags": []string{},
					"resource_attributes": map[string]interface{}{
						"cloud.account.id":        map[string]interface{}{"enabled": true},
						"cloud.availability_zone": map[string]interface{}{"enabled": true},
						"cloud.platform":          map[string]interface{}{"enabled": true},
						"cloud.provider":          map[string]interface{}{"enabled": true},
						"cloud.region":            map[string]interface{}{"enabled": true},
						"host.id":                 map[string]interface{}{"enabled": true},
						"host.image.id":           map[string]interface{}{"enabled": true},
						"host.name":               map[string]interface{}{"enabled": true},
						"host.type":               map[string]interface{}{"enabled": true},
					},
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
			expected: map[string]interface{}{
				"detectors": []string{"ec2"},
				"timeout":   5 * time.Second,
				"override":  true,
				"ec2": map[string]interface{}{
					"tags": []string{},
					"resource_attributes": map[string]interface{}{
						"cloud.account.id":        map[string]interface{}{"enabled": true},
						"cloud.availability_zone": map[string]interface{}{"enabled": true},
						"cloud.platform":          map[string]interface{}{"enabled": true},
						"cloud.provider":          map[string]interface{}{"enabled": true},
						"cloud.region":            map[string]interface{}{"enabled": true},
						"host.id":                 map[string]interface{}{"enabled": true},
						"host.image.id":           map[string]interface{}{"enabled": true},
						"host.name":               map[string]interface{}{"enabled": true},
						"host.type":               map[string]interface{}{"enabled": true},
					},
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
			}
			output {}
			`,
			expected: map[string]interface{}{
				"detectors": []string{"ec2"},
				"timeout":   5 * time.Second,
				"override":  true,
				"ec2": map[string]interface{}{
					"tags": []string{"^tag1$", "^tag2$", "^label.*$"},
					"resource_attributes": map[string]interface{}{
						"cloud.account.id":        map[string]interface{}{"enabled": true},
						"cloud.availability_zone": map[string]interface{}{"enabled": true},
						"cloud.platform":          map[string]interface{}{"enabled": true},
						"cloud.provider":          map[string]interface{}{"enabled": true},
						"cloud.region":            map[string]interface{}{"enabled": true},
						"host.id":                 map[string]interface{}{"enabled": true},
						"host.image.id":           map[string]interface{}{"enabled": false},
						"host.name":               map[string]interface{}{"enabled": false},
						"host.type":               map[string]interface{}{"enabled": false},
					},
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
			expected: map[string]interface{}{
				"detectors": []string{"ecs"},
				"timeout":   5 * time.Second,
				"override":  true,
				"ecs": map[string]interface{}{
					"tags": []string{},
					"resource_attributes": map[string]interface{}{
						"aws.ecs.cluster.arn":     map[string]interface{}{"enabled": true},
						"aws.ecs.launchtype":      map[string]interface{}{"enabled": true},
						"aws.ecs.task.arn":        map[string]interface{}{"enabled": true},
						"aws.ecs.task.family":     map[string]interface{}{"enabled": true},
						"aws.ecs.task.id":         map[string]interface{}{"enabled": true},
						"aws.ecs.task.revision":   map[string]interface{}{"enabled": true},
						"aws.log.group.arns":      map[string]interface{}{"enabled": true},
						"aws.log.group.names":     map[string]interface{}{"enabled": true},
						"aws.log.stream.arns":     map[string]interface{}{"enabled": true},
						"aws.log.stream.names":    map[string]interface{}{"enabled": true},
						"cloud.account.id":        map[string]interface{}{"enabled": true},
						"cloud.availability_zone": map[string]interface{}{"enabled": true},
						"cloud.platform":          map[string]interface{}{"enabled": true},
						"cloud.provider":          map[string]interface{}{"enabled": true},
						"cloud.region":            map[string]interface{}{"enabled": true},
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
			expected: map[string]interface{}{
				"detectors": []string{"ecs"},
				"timeout":   5 * time.Second,
				"override":  true,
				"ecs": map[string]interface{}{
					"tags": []string{},
					"resource_attributes": map[string]interface{}{
						"aws.ecs.cluster.arn":     map[string]interface{}{"enabled": true},
						"aws.ecs.launchtype":      map[string]interface{}{"enabled": true},
						"aws.ecs.task.arn":        map[string]interface{}{"enabled": true},
						"aws.ecs.task.family":     map[string]interface{}{"enabled": true},
						"aws.ecs.task.id":         map[string]interface{}{"enabled": true},
						"aws.ecs.task.revision":   map[string]interface{}{"enabled": true},
						"aws.log.group.arns":      map[string]interface{}{"enabled": true},
						"aws.log.group.names":     map[string]interface{}{"enabled": false},
						"aws.log.stream.arns":     map[string]interface{}{"enabled": true},
						"aws.log.stream.names":    map[string]interface{}{"enabled": true},
						"cloud.account.id":        map[string]interface{}{"enabled": true},
						"cloud.availability_zone": map[string]interface{}{"enabled": true},
						"cloud.platform":          map[string]interface{}{"enabled": true},
						"cloud.provider":          map[string]interface{}{"enabled": true},
						"cloud.region":            map[string]interface{}{"enabled": true},
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
			},
		},
		{
			testName: "eks_defaults",
			cfg: `
			detectors = ["eks"]
			eks {}
			output {}
			`,
			expected: map[string]interface{}{
				"detectors": []string{"eks"},
				"timeout":   5 * time.Second,
				"override":  true,
				"eks": map[string]interface{}{
					"tags": []string{},
					"resource_attributes": map[string]interface{}{
						"cloud.platform": map[string]interface{}{
							"enabled": true,
						},
						"cloud.provider": map[string]interface{}{
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
			},
		},
		{
			testName: "eks_explicit",
			cfg: `
			detectors = ["eks"]
			eks {
				resource_attributes {
					cloud.platform { enabled = true }
					cloud.provider { enabled = false }
				}
			}
			output {}
			`,
			expected: map[string]interface{}{
				"detectors": []string{"eks"},
				"timeout":   5 * time.Second,
				"override":  true,
				"eks": map[string]interface{}{
					"tags": []string{},
					"resource_attributes": map[string]interface{}{
						"cloud.platform": map[string]interface{}{
							"enabled": true,
						},
						"cloud.provider": map[string]interface{}{
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
			},
		},
		{
			testName: "azure_defaults",
			cfg: `
			detectors = ["azure"]
			azure {}
			output {}
			`,
			expected: map[string]interface{}{
				"detectors": []string{"azure"},
				"timeout":   5 * time.Second,
				"override":  true,
				"azure": map[string]interface{}{
					"resource_attributes": map[string]interface{}{
						"tags": []string{},
						"azure.resourcegroup.name": map[string]interface{}{
							"enabled": true,
						},
						"azure.vm.name": map[string]interface{}{
							"enabled": true,
						},
						"azure.vm.scaleset.name": map[string]interface{}{
							"enabled": true,
						},
						"azure.vm.size": map[string]interface{}{
							"enabled": true,
						},
						"cloud.account.id": map[string]interface{}{
							"enabled": true,
						},
						"cloud.platform": map[string]interface{}{
							"enabled": true,
						},
						"cloud.provider": map[string]interface{}{
							"enabled": true,
						},
						"cloud.region": map[string]interface{}{
							"enabled": true,
						},
						"host.id": map[string]interface{}{
							"enabled": true,
						},
						"host.name": map[string]interface{}{
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
			expected: map[string]interface{}{
				"detectors": []string{"azure"},
				"timeout":   5 * time.Second,
				"override":  true,
				"azure": map[string]interface{}{
					"tags": []string{"tag1", "tag2"},
					"resource_attributes": map[string]interface{}{
						"azure.resourcegroup.name": map[string]interface{}{
							"enabled": true,
						},
						"azure.vm.name": map[string]interface{}{
							"enabled": true,
						},
						"azure.vm.scaleset.name": map[string]interface{}{
							"enabled": true,
						},
						"azure.vm.size": map[string]interface{}{
							"enabled": true,
						},
						"cloud.account.id": map[string]interface{}{
							"enabled": false,
						},
						"cloud.platform": map[string]interface{}{
							"enabled": true,
						},
						"cloud.provider": map[string]interface{}{
							"enabled": true,
						},
						"cloud.region": map[string]interface{}{
							"enabled": true,
						},
						"host.id": map[string]interface{}{
							"enabled": true,
						},
						"host.name": map[string]interface{}{
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
			},
		},
		{
			testName: "aks_defaults",
			cfg: `
			detectors = ["aks"]
			aks {}
			output {}
			`,
			expected: map[string]interface{}{
				"detectors": []string{"aks"},
				"timeout":   5 * time.Second,
				"override":  true,
				"aks": map[string]interface{}{
					"tags": []string{},
					"resource_attributes": map[string]interface{}{
						"cloud.platform": map[string]interface{}{
							"enabled": true,
						},
						"cloud.provider": map[string]interface{}{
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
			expected: map[string]interface{}{
				"detectors": []string{"aks"},
				"timeout":   5 * time.Second,
				"override":  true,
				"aks": map[string]interface{}{
					"tags": []string{},
					"resource_attributes": map[string]interface{}{
						"cloud.platform": map[string]interface{}{
							"enabled": true,
						},
						"cloud.provider": map[string]interface{}{
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
			},
		},
		{
			testName: "gcp_defaults",
			cfg: `
			detectors = ["gcp"]
			gcp {}
			output {}
			`,
			expected: map[string]interface{}{
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
				}
			}
			output {}
			`,
			expected: map[string]interface{}{
				"detectors": []string{"gcp"},
				"timeout":   5 * time.Second,
				"override":  true,
				"gcp": map[string]interface{}{
					"resource_attributes": map[string]interface{}{
						"cloud.account.id": map[string]interface{}{
							"enabled": true,
						},
						"cloud.availability_zone": map[string]interface{}{
							"enabled": true,
						},
						"cloud.platform": map[string]interface{}{
							"enabled": true,
						},
						"cloud.provider": map[string]interface{}{
							"enabled": true,
						},
						"cloud.region": map[string]interface{}{
							"enabled": false,
						},
						"faas.id": map[string]interface{}{
							"enabled": false,
						},
						"faas.instance": map[string]interface{}{
							"enabled": true,
						},
						"faas.name": map[string]interface{}{
							"enabled": true,
						},
						"faas.version": map[string]interface{}{
							"enabled": true,
						},
						"gcp.cloud_run.job.execution": map[string]interface{}{
							"enabled": true,
						},
						"gcp.cloud_run.job.task_index": map[string]interface{}{
							"enabled": true,
						},
						"gcp.gce.instance.hostname": map[string]interface{}{
							"enabled": false,
						},
						"gcp.gce.instance.name": map[string]interface{}{
							"enabled": false,
						},
						"host.id": map[string]interface{}{
							"enabled": true,
						},
						"host.name": map[string]interface{}{
							"enabled": true,
						},
						"host.type": map[string]interface{}{
							"enabled": true,
						},
						"k8s.cluster.name": map[string]interface{}{
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
			},
		},
		{
			testName: "docker_defaults",
			cfg: `
			detectors = ["docker"]
			docker {}
			output {}
			`,
			expected: map[string]interface{}{
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
			expected: map[string]interface{}{
				"detectors": []string{"docker"},
				"timeout":   5 * time.Second,
				"override":  true,
				"docker": map[string]interface{}{
					"resource_attributes": map[string]interface{}{
						"host.name": map[string]interface{}{
							"enabled": true,
						},
						"os.type": map[string]interface{}{
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
			},
		},
		{
			testName: "lambda_defaults",
			cfg: `
			detectors = ["lambda"]
			lambda {}
			output {}
			`,
			expected: map[string]interface{}{
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
			expected: map[string]interface{}{
				"detectors": []string{"lambda"},
				"timeout":   5 * time.Second,
				"override":  true,
				"lambda": map[string]interface{}{
					"resource_attributes": map[string]interface{}{
						"aws.log.group.names": map[string]interface{}{
							"enabled": true,
						},
						"aws.log.stream.names": map[string]interface{}{
							"enabled": true,
						},
						"cloud.platform": map[string]interface{}{
							"enabled": true,
						},
						"cloud.provider": map[string]interface{}{
							"enabled": false,
						},
						"cloud.region": map[string]interface{}{
							"enabled": false,
						},
						"faas.instance": map[string]interface{}{
							"enabled": true,
						},
						"faas.max_memory": map[string]interface{}{
							"enabled": true,
						},
						"faas.name": map[string]interface{}{
							"enabled": true,
						},
						"faas.version": map[string]interface{}{
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
			},
		},
		{
			testName: "elasticbeanstalk_defaults",
			cfg: `
			detectors = ["elasticbeanstalk"]
			elasticbeanstalk {}
			output {}
			`,
			expected: map[string]interface{}{
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
			expected: map[string]interface{}{
				"detectors": []string{"elasticbeanstalk"},
				"timeout":   5 * time.Second,
				"override":  true,
				"elasticbeanstalk": map[string]interface{}{
					"resource_attributes": map[string]interface{}{
						"cloud.platform": map[string]interface{}{
							"enabled": true,
						},
						"cloud.provider": map[string]interface{}{
							"enabled": true,
						},
						"deployment.environment": map[string]interface{}{
							"enabled": true,
						},
						"service.instance.id": map[string]interface{}{
							"enabled": false,
						},
						"service.version": map[string]interface{}{
							"enabled": true,
						},
					},
				},
				"ec2":       ec2.DefaultArguments.Convert(),
				"ecs":       ecs.DefaultArguments.Convert(),
				"eks":       eks.DefaultArguments.Convert(),
				"lambda":    lambda.DefaultArguments.Convert(),
				"azure":     azure.DefaultArguments.Convert(),
				"aks":       aks.DefaultArguments.Convert(),
				"consul":    consul.DefaultArguments.Convert(),
				"docker":    docker.DefaultArguments.Convert(),
				"gcp":       gcp.DefaultArguments.Convert(),
				"heroku":    heroku.DefaultArguments.Convert(),
				"system":    defaultArgs.Convert(),
				"openshift": openshift.DefaultArguments.Convert(),
				"k8snode":   kubernetes_node.DefaultArguments.Convert(),
			},
		},
		{
			testName: "consul_defaults",
			cfg: `
			detectors = ["consul"]
			consul {}
			output {}
			`,
			expected: map[string]interface{}{
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
			expected: map[string]interface{}{
				"detectors": []string{"consul"},
				"timeout":   5 * time.Second,
				"override":  true,
				"consul": map[string]interface{}{
					"address":    "localhost:8500",
					"datacenter": "dc1",
					"token":      "secret_token",
					"namespace":  "test_namespace",
					"meta":       map[string]string{"test": ""},
					"resource_attributes": map[string]interface{}{
						"cloud.region": map[string]interface{}{
							"enabled": false,
						},
						"host.id": map[string]interface{}{
							"enabled": false,
						},
						"host.name": map[string]interface{}{
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
			},
		},
		{
			testName: "heroku_defaults",
			cfg: `
			detectors = ["heroku"]
			heroku {}
			output {}
			`,
			expected: map[string]interface{}{
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
			expected: map[string]interface{}{
				"detectors": []string{"heroku"},
				"timeout":   5 * time.Second,
				"override":  true,
				"heroku": map[string]interface{}{
					"resource_attributes": map[string]interface{}{
						"cloud.provider": map[string]interface{}{
							"enabled": true,
						},
						"heroku.app.id": map[string]interface{}{
							"enabled": true,
						},
						"heroku.dyno.id": map[string]interface{}{
							"enabled": true,
						},
						"heroku.release.commit": map[string]interface{}{
							"enabled": true,
						},
						"heroku.release.creation_timestamp": map[string]interface{}{
							"enabled": false,
						},
						"service.instance.id": map[string]interface{}{
							"enabled": false,
						},
						"service.name": map[string]interface{}{
							"enabled": true,
						},
						"service.version": map[string]interface{}{
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
			},
		},
		{
			testName: "kubernetes_node_defaults",
			cfg: `
			detectors = ["kubernetes_node"]
			kubernetes_node {}
			output {}
			`,
			expected: map[string]interface{}{
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
			expected: map[string]interface{}{
				"detectors": []string{"k8snode"},
				"timeout":   5 * time.Second,
				"override":  true,
				"k8snode": map[string]interface{}{
					"auth_type":         "kubeConfig",
					"context":           "fake_ctx",
					"node_from_env_var": "MY_CUSTOM_VAR",
					"resource_attributes": map[string]interface{}{
						"k8s.node.name": map[string]interface{}{
							"enabled": true,
						},
						"k8s.node.uid": map[string]interface{}{
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
			expected: map[string]interface{}{
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
					host.name { enabled = false }
					// os.description { enabled = false }
					// os.type { enabled = true }
				}
			}
			output {}
			`,
			expected: map[string]interface{}{
				"detectors": []string{"system"},
				"timeout":   5 * time.Second,
				"override":  true,
				"system": map[string]interface{}{
					"hostname_sources": []string{"cname", "lookup"},
					"resource_attributes": map[string]interface{}{
						"host.arch": map[string]interface{}{
							"enabled": true,
						},
						"host.cpu.cache.l2.size": map[string]interface{}{
							"enabled": true,
						},
						"host.cpu.family": map[string]interface{}{
							"enabled": true,
						},
						"host.cpu.model.id": map[string]interface{}{
							"enabled": true,
						},
						"host.cpu.model.name": map[string]interface{}{
							"enabled": true,
						},
						"host.cpu.stepping": map[string]interface{}{
							"enabled": true,
						},
						"host.cpu.vendor.id": map[string]interface{}{
							"enabled": false,
						},
						"host.id": map[string]interface{}{
							"enabled": false,
						},
						"host.name": map[string]interface{}{
							"enabled": false,
						},
						"os.description": map[string]interface{}{
							"enabled": false,
						},
						"os.type": map[string]interface{}{
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
				"openshift":        openshift.DefaultArguments.Convert(),
				"k8snode":          kubernetes_node.DefaultArguments.Convert(),
			},
		},
		{
			testName: "openshift_default",
			cfg: `
			detectors = ["openshift"]
			openshift {}
			output {}
			`,
			expected: map[string]interface{}{
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
			expected: map[string]interface{}{
				"detectors": []string{"openshift"},
				"timeout":   7 * time.Second,
				"override":  false,
				"openshift": map[string]interface{}{
					"address": "127.0.0.1:4444",
					"token":   "some_token",
					"tls": map[string]interface{}{
						"insecure": true,
					},
					"resource_attributes": map[string]interface{}{
						"cloud.platform": map[string]interface{}{
							"enabled": true,
						},
						"cloud.provider": map[string]interface{}{
							"enabled": true,
						},
						"cloud.region": map[string]interface{}{
							"enabled": false,
						},
						"k8s.cluster.name": map[string]interface{}{
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
			expected: map[string]interface{}{
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

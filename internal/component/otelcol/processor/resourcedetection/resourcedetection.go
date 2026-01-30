package resourcedetection

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
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
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/k8snode"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/kubeadm"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/openshift"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/openstacknova"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/oraclecloud"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/scaleway"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/system"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/upcloud"
	"github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/vultr"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.resourcedetection",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := resourcedetectionprocessor.NewFactory()
			return processor.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.processor.resourcedetection component.
type Arguments struct {
	// Detectors is an ordered list of named detectors that should be
	// run to attempt to detect resource information.
	Detectors []string `alloy:"detectors,attr,optional"`

	// Override indicates whether any existing resource attributes
	// should be overridden or preserved. Defaults to true.
	Override bool `alloy:"override,attr,optional"`

	// DetectorConfig is a list of settings specific to all detectors
	DetectorConfig DetectorConfig `alloy:",squash"`

	// HTTP client settings for the detector
	// Timeout default is 5s
	Timeout time.Duration `alloy:"timeout,attr,optional"`
	// Client otelcol.HTTPClientArguments `alloy:",squash"`
	//TODO: Uncomment this later, and remove Timeout?
	//      Can we just get away with a timeout, or do we need all the http client settings?
	//      It seems that HTTP client settings are only used in the ec2 detection via ClientFromContext.
	//      This seems like a very niche use case, so for now I won't implement it in Alloy.
	//      If we do implement it in Alloy, I am not sure how to document the HTTP client settings.
	//      We'd have to mention that they're only for a very specific use case.

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// DetectorConfig contains user-specified configurations unique to all individual detectors
type DetectorConfig struct {
	// EC2Config contains user-specified configurations for the EC2 detector
	EC2Config ec2.Config `alloy:"ec2,block,optional"`

	// ECSConfig contains user-specified configurations for the ECS detector
	ECSConfig ecs.Config `alloy:"ecs,block,optional"`

	// EKSConfig contains user-specified configurations for the EKS detector
	EKSConfig eks.Config `alloy:"eks,block,optional"`

	// Elasticbeanstalk contains user-specified configurations for the elasticbeanstalk detector
	ElasticbeanstalkConfig elasticbeanstalk.Config `alloy:"elasticbeanstalk,block,optional"`

	// Lambda contains user-specified configurations for the lambda detector
	LambdaConfig lambda.Config `alloy:"lambda,block,optional"`

	// AkamaiConfig contains user-specified configurations for the Akamai detector
	AkamaiConfig akamai.Config `alloy:"akamai,block,optional"`

	// Azure contains user-specified configurations for the azure detector
	AzureConfig azure.Config `alloy:"azure,block,optional"`

	// Aks contains user-specified configurations for the aks detector
	AksConfig aks.Config `alloy:"aks,block,optional"`

	// ConsulConfig contains user-specified configurations for the Consul detector
	ConsulConfig consul.Config `alloy:"consul,block,optional"`

	// DockerConfig contains user-specified configurations for the docker detector
	DockerConfig docker.Config `alloy:"docker,block,optional"`

	// DigitalOceanConfig contains user-specified configurations for the DigitalOcean detector
	DigitalOceanConfig digitalocean.Config `alloy:"digitalocean,block,optional"`

	// GcpConfig contains user-specified configurations for the gcp detector
	GcpConfig gcp.Config `alloy:"gcp,block,optional"`

	// HerokuConfig contains user-specified configurations for the heroku detector
	HerokuConfig heroku.Config `alloy:"heroku,block,optional"`

	// HetznerConfig contains user-specified configurations for the Hetzner detector
	HetznerConfig hetzner.Config `alloy:"hetzner,block,optional"`

	// SystemConfig contains user-specified configurations for the System detector
	SystemConfig system.Config `alloy:"system,block,optional"`

	// OpenShift contains user-specified configurations for the Openshift detector
	OpenShiftConfig openshift.Config `alloy:"openshift,block,optional"`

	// KubernetesNode contains user-specified configurations for the K8SNode detector
	KubernetesNodeConfig k8snode.Config `alloy:"kubernetes_node,block,optional"`

	// KubeADMConfig contains user-specified configurations for the KubeADM detector
	KubeADMConfig kubeadm.Config `alloy:"kubeadm,block,optional"`

	// Dynatrace contains user-specified configurations for the Dynatrace detector
	DynatraceConfig dynatrace.Config `alloy:"dynatrace,block,optional"`

	// OpenStackNovaConfig contains user-specified configurations for the OpenStack Nova detector
	OpenStackNovaConfig openstacknova.Config `alloy:"nova,block,optional"`

	// OracleCloudConfig contains user-specified configurations for the Oracle Cloud detector
	OracleCloudConfig oraclecloud.Config `alloy:"oraclecloud,block,optional"`

	// ScalewayConfig contains user-specified configurations for the Scaleway detector
	ScalewayConfig scaleway.Config `alloy:"scaleway,block,optional"`

	// UpCloudConfig contains user-specified configurations for the UpCloud detector
	UpCloudConfig upcloud.Config `alloy:"upcloud,block,optional"`

	// VultrConfig contains user-specified configurations for the Vultr detector
	VultrConfig vultr.Config `alloy:"vultr,block,optional"`
}

func (dc *DetectorConfig) SetToDefault() {
	*dc = DetectorConfig{
		EC2Config:              ec2.DefaultArguments,
		ECSConfig:              ecs.DefaultArguments,
		EKSConfig:              eks.DefaultArguments,
		ElasticbeanstalkConfig: elasticbeanstalk.DefaultArguments,
		LambdaConfig:           lambda.DefaultArguments,
		AkamaiConfig:           akamai.DefaultArguments,
		AzureConfig:            azure.DefaultArguments,
		AksConfig:              aks.DefaultArguments,
		ConsulConfig:           consul.DefaultArguments,
		DockerConfig:           docker.DefaultArguments,
		DigitalOceanConfig:     digitalocean.DefaultArguments,
		GcpConfig:              gcp.DefaultArguments,
		HerokuConfig:           heroku.DefaultArguments,
		HetznerConfig:          hetzner.DefaultArguments,
		OpenShiftConfig:        openshift.DefaultArguments,
		KubernetesNodeConfig:   k8snode.DefaultArguments,
		KubeADMConfig:          kubeadm.DefaultArguments,
		DynatraceConfig:        dynatrace.DefaultArguments,
		OpenStackNovaConfig:    openstacknova.DefaultArguments,
		OracleCloudConfig:      oraclecloud.DefaultArguments,
		ScalewayConfig:         scaleway.DefaultArguments,
		UpCloudConfig:          upcloud.DefaultArguments,
		VultrConfig:            vultr.DefaultArguments,
	}
	dc.SystemConfig.SetToDefault()
}

var (
	_ processor.Arguments = Arguments{}
	_ syntax.Validator    = (*Arguments)(nil)
	_ syntax.Defaulter    = (*Arguments)(nil)
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Detectors: []string{"env"},
		Override:  true,
		Timeout:   5 * time.Second,
	}
	args.DetectorConfig.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if len(args.Detectors) == 0 {
		return fmt.Errorf("at least one detector must be specified")
	}

	for _, detector := range args.Detectors {
		switch detector {
		case "env",
			ec2.Name,
			ecs.Name,
			eks.Name,
			elasticbeanstalk.Name,
			lambda.Name,
			akamai.Name,
			azure.Name,
			aks.Name,
			consul.Name,
			digitalocean.Name,
			docker.Name,
			gcp.Name,
			heroku.Name,
			hetzner.Name,
			system.Name,
			openshift.Name,
			k8snode.Name,
			kubeadm.Name,
			dynatrace.Name,
			openstacknova.Name,
			oraclecloud.Name,
			scaleway.Name,
			upcloud.Name,
			vultr.Name:
		// Valid option - nothing to do
		default:
			return fmt.Errorf("invalid detector: %s", detector)
		}
	}

	return nil
}

func (args Arguments) ConvertDetectors() []string {
	if args.Detectors == nil {
		return nil
	}

	res := make([]string, 0, len(args.Detectors))
	for _, detector := range args.Detectors {
		switch detector {
		case k8snode.Name:
			res = append(res, "k8snode")
		default:
			res = append(res, detector)
		}
	}
	return res
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	input := make(map[string]any)

	input["detectors"] = args.ConvertDetectors()
	input["override"] = args.Override
	input["timeout"] = args.Timeout

	input["ec2"] = args.DetectorConfig.EC2Config.Convert()
	input["ecs"] = args.DetectorConfig.ECSConfig.Convert()
	input["eks"] = args.DetectorConfig.EKSConfig.Convert()
	input["elasticbeanstalk"] = args.DetectorConfig.ElasticbeanstalkConfig.Convert()
	input["lambda"] = args.DetectorConfig.LambdaConfig.Convert()
	input["akamai"] = args.DetectorConfig.AkamaiConfig.Convert()
	input["azure"] = args.DetectorConfig.AzureConfig.Convert()
	input["aks"] = args.DetectorConfig.AksConfig.Convert()
	input["consul"] = args.DetectorConfig.ConsulConfig.Convert()
	input["docker"] = args.DetectorConfig.DockerConfig.Convert()
	input["digitalocean"] = args.DetectorConfig.DigitalOceanConfig.Convert()
	input["gcp"] = args.DetectorConfig.GcpConfig.Convert()
	input["heroku"] = args.DetectorConfig.HerokuConfig.Convert()
	input["hetzner"] = args.DetectorConfig.HetznerConfig.Convert()
	input["system"] = args.DetectorConfig.SystemConfig.Convert()
	input["openshift"] = args.DetectorConfig.OpenShiftConfig.Convert()
	input["k8snode"] = args.DetectorConfig.KubernetesNodeConfig.Convert()
	input["kubeadm"] = args.DetectorConfig.KubeADMConfig.Convert()
	input["dynatrace"] = args.DetectorConfig.DynatraceConfig.Convert()
	input["nova"] = args.DetectorConfig.OpenStackNovaConfig.Convert()
	input["oraclecloud"] = args.DetectorConfig.OracleCloudConfig.Convert()
	input["scaleway"] = args.DetectorConfig.ScalewayConfig.Convert()
	input["upcloud"] = args.DetectorConfig.UpCloudConfig.Convert()
	input["vultr"] = args.DetectorConfig.VultrConfig.Convert()
	var result resourcedetectionprocessor.Config
	err := mapstructure.Decode(input, &result)

	if err != nil {
		return nil, err
	}

	return &result, nil
}

// Extensions implements processor.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements processor.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements processor.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// DebugMetricsConfig implements processor.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

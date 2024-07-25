package beyla

import (
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/otelcol"
)

// Arguments configures the Beyla component.
type Arguments struct {
	Port           string                     `alloy:"open_port,attr,optional"`
	ExecutableName string                     `alloy:"executable_name,attr,optional"`
	Debug          bool                       `alloy:"debug,attr,optional"`
	Routes         Routes                     `alloy:"routes,block,optional"`
	Attributes     Attributes                 `alloy:"attributes,block,optional"`
	Discovery      Discovery                  `alloy:"discovery,block,optional"`
	Prometheus     Prometheus                 `alloy:"prometheus,block,optional"`
	Network        Network                    `alloy:"network,block,optional"`
	Output         *otelcol.ConsumerArguments `alloy:"output,block,optional"`
}

type Exports struct {
	Targets []discovery.Target `alloy:"targets,attr"`
}

type Routes struct {
	Unmatch        string   `alloy:"unmatched,attr,optional"`
	Patterns       []string `alloy:"patterns,attr,optional"`
	IgnorePatterns []string `alloy:"ignored_patterns,attr,optional"`
	IgnoredEvents  string   `alloy:"ignore_mode,attr,optional"`
}

type Attributes struct {
	Kubernetes KubernetesDecorator `alloy:"kubernetes,block"`
}

type KubernetesDecorator struct {
	Enable      string `alloy:"enable,attr"`
	ClusterName string `alloy:"cluster_name,attr,optional"`
}

type Services []Service

type Service struct {
	Name       string            `alloy:"name,attr,optional"`
	Namespace  string            `alloy:"namespace,attr,optional"`
	OpenPorts  string            `alloy:"open_ports,attr,optional"`
	Path       string            `alloy:"exe_path,attr,optional"`
	Kubernetes KubernetesService `alloy:"kubernetes,block,optional"`
}

type KubernetesService struct {
	Namespace       string            `alloy:"namespace,attr,optional"`
	PodName         string            `alloy:"pod_name,attr,optional"`
	DeploymentName  string            `alloy:"deployment_name,attr,optional"`
	ReplicaSetName  string            `alloy:"replicaset_name,attr,optional"`
	StatefulSetName string            `alloy:"statefulset_name,attr,optional"`
	DaemonSetName   string            `alloy:"daemonset_name,attr,optional"`
	OwnerName       string            `alloy:"owner_name,attr,optional"`
	PodLabels       map[string]string `alloy:"pod_labels,attr,optional"`
}

type Discovery struct {
	Services        Services `alloy:"services,block"`
	ExcludeServices Services `alloy:"exclude_services,block"`
}

type Prometheus struct {
	Features         []string `alloy:"features,attr,optional"`
	Instrumentations []string `alloy:"instrumentations,attr,optional"`
}

type Network struct {
	Enable bool `alloy:"enable,attr"`
}

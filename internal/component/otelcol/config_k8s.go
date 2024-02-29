package otelcol

import "fmt"

const (
	KubernetesAPIConfig_AuthType_None           = "none"
	KubernetesAPIConfig_AuthType_ServiceAccount = "serviceAccount"
	KubernetesAPIConfig_AuthType_KubeConfig     = "kubeConfig"
	KubernetesAPIConfig_AuthType_TLS            = "tls"
)

// KubernetesAPIConfig contains options relevant to connecting to the K8s API
type KubernetesAPIConfig struct {
	// How to authenticate to the K8s API server.  This can be one of `none`
	// (for no auth), `serviceAccount` (to use the standard service account
	// token provided to the agent pod), or `kubeConfig` to use credentials
	// from `~/.kube/config`.
	AuthType string `river:"auth_type,attr,optional"`

	// When using auth_type `kubeConfig`, override the current context.
	Context string `river:"context,attr,optional"`
}

// Validate returns an error if the config is invalid.
func (c *KubernetesAPIConfig) Validate() error {
	switch c.AuthType {
	case KubernetesAPIConfig_AuthType_None,
		KubernetesAPIConfig_AuthType_ServiceAccount,
		KubernetesAPIConfig_AuthType_KubeConfig,
		KubernetesAPIConfig_AuthType_TLS:
		return nil
	default:
		return fmt.Errorf("invalid auth_type %q", c.AuthType)
	}
}

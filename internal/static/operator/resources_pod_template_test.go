package operator

import (
	"testing"

	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_generatePodTemplate(t *testing.T) {
	var (
		cfg  = &Config{}
		name = "example"
	)

	t.Run("image should have version", func(t *testing.T) {
		deploy := gragent.Deployment{
			Agent: &gragent.GrafanaAgent{
				ObjectMeta: v1.ObjectMeta{Name: name, Namespace: name},
			},
		}

		tmpl, _, err := generatePodTemplate(cfg, "agent", deploy, podTemplateOptions{})
		require.NoError(t, err)
		require.Equal(t, DefaultAgentImage, tmpl.Spec.Containers[1].Image)
	})

	t.Run("reloader image should have version", func(t *testing.T) {
		deploy := gragent.Deployment{
			Agent: &gragent.GrafanaAgent{
				ObjectMeta: v1.ObjectMeta{Name: name, Namespace: name},
			},
		}

		tmpl, _, err := generatePodTemplate(cfg, "agent", deploy, podTemplateOptions{})
		require.NoError(t, err)
		require.Equal(t, DefaultConfigReloaderImage, tmpl.Spec.Containers[0].Image)
	})

	t.Run("allow custom version", func(t *testing.T) {
		deploy := gragent.Deployment{
			Agent: &gragent.GrafanaAgent{
				ObjectMeta: v1.ObjectMeta{Name: name, Namespace: name},
				Spec: gragent.GrafanaAgentSpec{
					Version: "vX.Y.Z",
				},
			},
		}

		tmpl, _, err := generatePodTemplate(cfg, "agent", deploy, podTemplateOptions{})
		require.NoError(t, err)
		require.Equal(t, DefaultAgentBaseImage+":vX.Y.Z", tmpl.Spec.Containers[1].Image)
	})

	t.Run("allow custom version for reloader", func(t *testing.T) {
		deploy := gragent.Deployment{
			Agent: &gragent.GrafanaAgent{
				ObjectMeta: v1.ObjectMeta{Name: name, Namespace: name},
				Spec: gragent.GrafanaAgentSpec{
					ConfigReloaderVersion: "vX.Y.Z",
				},
			},
		}

		tmpl, _, err := generatePodTemplate(cfg, "agent", deploy, podTemplateOptions{})
		require.NoError(t, err)
		require.Equal(t, DefaultConfigReloaderBaseImage+":vX.Y.Z", tmpl.Spec.Containers[0].Image)
	})

	t.Run("does not set version label in spec selector", func(t *testing.T) {
		deploy := gragent.Deployment{
			Agent: &gragent.GrafanaAgent{
				ObjectMeta: v1.ObjectMeta{Name: name, Namespace: name},
			},
		}

		tmpl, selectors, err := generatePodTemplate(cfg, "agent", deploy, podTemplateOptions{})
		require.NoError(t, err)

		// version label should not be set in selectors, since that is immutable
		require.NotContains(t, selectors.MatchLabels, versionLabelName)
		require.Contains(t, tmpl.ObjectMeta.Labels, versionLabelName)
	})

	t.Run("security ctx does not contain privileged", func(t *testing.T) {
		deploy := gragent.Deployment{
			Agent: &gragent.GrafanaAgent{
				ObjectMeta: v1.ObjectMeta{Name: name, Namespace: name},
			},
		}

		tmpl, _, err := generatePodTemplate(cfg, "agent", deploy, podTemplateOptions{})
		require.NoError(t, err)
		require.Equal(t, "config-reloader", tmpl.Spec.Containers[0].Name)
		for i := range tmpl.Spec.Containers {
			assert.False(t, tmpl.Spec.Containers[i].SecurityContext.Privileged != nil &&
				*tmpl.Spec.Containers[i].SecurityContext.Privileged,
				"privileged is not required. Fargate cannot schedule privileged containers.")
			assert.False(t, tmpl.Spec.Containers[i].SecurityContext.RunAsUser != nil &&
				*tmpl.Spec.Containers[i].SecurityContext.RunAsUser == int64(0),
				"force the container to run as root is not required.")
			assert.False(t, tmpl.Spec.Containers[i].SecurityContext.AllowPrivilegeEscalation != nil &&
				*tmpl.Spec.Containers[i].SecurityContext.AllowPrivilegeEscalation,
				"allow privilege escalation is not required.")
		}
	})

	t.Run("security ctx does contain privilege for logs daemonset", func(t *testing.T) {
		deploy := gragent.Deployment{
			Agent: &gragent.GrafanaAgent{
				ObjectMeta: v1.ObjectMeta{Name: name, Namespace: name},
			},
		}

		tmpl, _, err := generatePodTemplate(cfg, "agent", deploy, podTemplateOptions{Privileged: true})
		require.NoError(t, err)
		for i := range tmpl.Spec.Containers {
			// only grafana-agent container is supposed to be privileged
			if tmpl.Spec.Containers[i].Name == "grafana-agent" {
				assert.True(t, tmpl.Spec.Containers[i].SecurityContext.Privileged != nil &&
					*tmpl.Spec.Containers[i].SecurityContext.Privileged,
					"privileged is needed for grafana-agent if pod options say so.")
			} else {
				assert.False(t, tmpl.Spec.Containers[i].SecurityContext.Privileged != nil &&
					*tmpl.Spec.Containers[i].SecurityContext.Privileged,
					"privileged is not required for other containers.")
				assert.False(t, tmpl.Spec.Containers[i].SecurityContext.RunAsUser != nil &&
					*tmpl.Spec.Containers[i].SecurityContext.RunAsUser == int64(0),
					"force the container to run as root is not required for other containers.")
				assert.False(t, tmpl.Spec.Containers[i].SecurityContext.AllowPrivilegeEscalation != nil &&
					*tmpl.Spec.Containers[i].SecurityContext.AllowPrivilegeEscalation,
					"allow privilege escalation is not required for other containers.")
			}
		}
	})

	t.Run("runtimeclassname set if passed in", func(t *testing.T) {
		name := "test123"
		deploy := gragent.Deployment{
			Agent: &gragent.GrafanaAgent{
				ObjectMeta: v1.ObjectMeta{Name: name, Namespace: name},
				Spec: gragent.GrafanaAgentSpec{
					RuntimeClassName: &name,
				},
			},
		}
		tmpl, _, err := generatePodTemplate(cfg, "agent", deploy, podTemplateOptions{})
		require.NoError(t, err)
		assert.Equal(t, name, *tmpl.Spec.RuntimeClassName)

		deploy.Agent.Spec.RuntimeClassName = nil
		tmpl, _, err = generatePodTemplate(cfg, "agent", deploy, podTemplateOptions{})
		require.NoError(t, err)
		assert.Nil(t, tmpl.Spec.RuntimeClassName)
	})

	t.Run("AGENT_DEPLOY_MODE env ser", func(t *testing.T) {
		deploy := gragent.Deployment{
			Agent: &gragent.GrafanaAgent{
				ObjectMeta: v1.ObjectMeta{Name: name, Namespace: name},
			},
		}

		tmpl, _, err := generatePodTemplate(cfg, "agent", deploy, podTemplateOptions{})
		require.NoError(t, err)
		require.Equal(t, "operator", tmpl.Spec.Containers[1].Env[1].Value)
		require.Equal(t, "AGENT_DEPLOY_MODE", tmpl.Spec.Containers[1].Env[1].Name)
	})
}

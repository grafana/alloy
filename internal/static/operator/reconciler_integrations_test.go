package operator

import (
	"testing"

	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	"github.com/stretchr/testify/require"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_deploymentIntegrationSubset(t *testing.T) {
	var (
		nodeExporter = &gragent.Integration{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      "node_exporter",
				Namespace: "default",
			},
			Spec: gragent.IntegrationSpec{
				Name: "node_exporter",
				Type: gragent.IntegrationType{AllNodes: true},
			},
		}
		process = &gragent.Integration{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      "process",
				Namespace: "default",
			},
			Spec: gragent.IntegrationSpec{
				Name: "process",
				Type: gragent.IntegrationType{AllNodes: true},
			},
		}
		redis = &gragent.Integration{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      "redis",
				Namespace: "default",
			},
			Spec: gragent.IntegrationSpec{
				Name: "redis",
				Type: gragent.IntegrationType{AllNodes: false},
			},
		}

		deploy = gragent.Deployment{
			Integrations: []gragent.IntegrationsDeployment{
				{Instance: nodeExporter},
				{Instance: process},
				{Instance: redis},
			},
		}
	)

	tt := []struct {
		name     string
		allNodes bool
		expect   []*gragent.Integration
	}{
		{
			name:     "allNodes=false",
			allNodes: false,
			expect:   []*gragent.Integration{redis},
		},
		{
			name:     "allNodes=true",
			allNodes: true,
			expect:   []*gragent.Integration{nodeExporter, process},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			res := deploymentIntegrationSubset(deploy, tc.allNodes)

			integrations := make([]*gragent.Integration, 0, len(res.Integrations))
			for _, i := range res.Integrations {
				integrations = append(integrations, i.Instance)
			}

			require.Equal(t, tc.expect, integrations)
		})
	}
}

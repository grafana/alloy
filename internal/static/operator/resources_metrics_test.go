package operator

import (
	"testing"

	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	"github.com/stretchr/testify/require"
	core_v1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetadataFromPodTemplate(t *testing.T) {
	t.Run("Should not include version label in statefulset metadata", func(t *testing.T) {
		meta := metadataFromPodTemplate("foo",
			gragent.Deployment{
				Agent: &gragent.GrafanaAgent{},
			},
			core_v1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{versionLabelName: "v1.2.3"},
				},
			})
		require.NotContains(t, meta.Labels, versionLabelName)
	})
}

package config

import (
	"testing"

	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/agent/internal/util/subset"
	"github.com/stretchr/testify/require"
	apiext_v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

func TestIntegration(t *testing.T) {
	toJSON := func(in string) apiext_v1.JSON {
		t.Helper()
		out, err := yaml.YAMLToJSONStrict([]byte(in))
		require.NoError(t, err)
		return apiext_v1.JSON{Raw: out}
	}

	tt := []struct {
		name   string
		input  map[string]interface{}
		expect string
	}{
		{
			name: "configured integration",
			input: map[string]interface{}{
				"integration": &gragent.Integration{
					Spec: gragent.IntegrationSpec{
						Name: "mysqld_exporter",
						Config: toJSON(`
              data_source_names: root@(server-a:3306)/
            `),
					},
				},
			},
			expect: util.Untab(`
				data_source_names: root@(server-a:3306)/
      `),
		},
		{
			name: "integration no config",
			input: map[string]interface{}{
				"integration": &gragent.Integration{
					Spec: gragent.IntegrationSpec{
						Name: "mysqld_exporter",
					},
				},
			},
			expect: util.Untab(`{}`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			vm, err := createVM(testStore())
			require.NoError(t, err)

			actual, err := runSnippetTLA(t, vm, "./integrations.libsonnet", tc.input)
			require.NoError(t, err)
			require.NoError(t, subset.YAMLAssert([]byte(tc.expect), []byte(actual)), "incomplete yaml\n%s", actual)
		})
	}
}

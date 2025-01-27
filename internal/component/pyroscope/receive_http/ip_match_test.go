package receive_http

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/util"
)

func toJson(t testing.TB, o any) string {
	t.Helper()
	x, err := json.Marshal(o)
	require.NoError(t, err)
	return string(x)
}

func TestBuildIPLookupMap(t *testing.T) {
	logger := util.TestAlloyLogger(t)

	for _, tc := range []struct {
		name    string
		targets []discovery.Target
		result  string
	}{
		{name: "empty targets"},
		{
			name: "valid ips, no overlap",
			targets: []discovery.Target{
				map[string]string{
					labelMetaDockerNetworkIP: "1.2.3.4",
					"my-label":               "value",
					"my-label2":              "value2",
				},
				map[string]string{
					labelMetaKubernetesPodIP: "1.2.3.5",
					"my-pod":                 "pod2",
					"my-namespace":           "namespace2",
				},
			},
			result: `{
            "1.2.3.4":{"` + labelMetaDockerNetworkIP + `":"1.2.3.4","my-label":"value","my-label2":"value2"}, 
            "1.2.3.5":{"` + labelMetaKubernetesPodIP + `":"1.2.3.5","my-pod":"pod2","my-namespace":"namespace2"}
            }`,
		},
		{
			name: "valid overlapping ips, pod label overlaps",
			targets: []discovery.Target{
				map[string]string{
					labelMetaKubernetesPodIP: "1.2.3.4",
					"my-pod":                 "pod1",
					"my-namespace":           "namespace1",
				},
				map[string]string{
					labelMetaKubernetesPodIP: "1.2.3.4",
					"my-pod":                 "pod2",
					"my-namespace":           "namespace1",
				},
			},
			result: `{
            "1.2.3.4":{"` + labelMetaKubernetesPodIP + `":"1.2.3.4","my-namespace":"namespace1"}
            }`,
		},
		{
			name: "valid overlapping ipv6s, pod label overlaps",
			targets: []discovery.Target{
				map[string]string{
					labelMetaKubernetesPodIP: "cafe::",
					"my-pod":                 "pod1",
					"my-namespace":           "namespace1",
				},
				map[string]string{
					labelMetaKubernetesPodIP: "cafe::0", // note: string is not overlapping
					"my-pod":                 "pod2",
					"my-namespace":           "namespace1",
				},
			},
			result: `{
            "cafe::":{"my-namespace":"namespace1"}
            }`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.result == "" {
				tc.result = "{}"
			}
			got := buildIPLookupMap(logger, tc.targets)
			require.JSONEq(t, tc.result, toJson(t, got))
		})
	}
}

package kubelet

import (
	"net/url"
	"testing"

	"github.com/prometheus/prometheus/discovery/targetgroup"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	bearer_token_file = "/path/to/file.token"
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

func TestBadAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	bearer_token = "token"
	bearer_token_file = "/path/to/file.token"
`

	// Make sure the squashed HTTPClientConfig Validate function is being utilized correctly
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.ErrorContains(t, err, "at most one of basic_auth, authorization, oauth2, bearer_token & bearer_token_file must be configured")

	// Make sure that URL defaults to https://localhost:10250
	var args2 Arguments
	err = syntax.Unmarshal([]byte{}, &args2)
	require.NoError(t, err)
	require.Equal(t, args2.URL.String(), "https://localhost:10250")
}

func TestPodDeletion(t *testing.T) {
	pod1 := newPod("pod-1", "namespace-1")
	pod2 := newPod("pod-2", "namespace-2")
	podList1 := v1.PodList{
		Items: []v1.Pod{pod1, pod2},
	}
	podList2 := v1.PodList{
		Items: []v1.Pod{pod2},
	}

	var args Arguments
	args.SetToDefault()
	kubeletDiscovery, err := NewKubeletDiscovery(args)
	require.NoError(t, err)

	kubeletDiscovery.refresh(podList1)
	require.Len(t, kubeletDiscovery.discoveredPodSources, 2)

	tg2 := kubeletDiscovery.refresh(podList2)
	require.Len(t, kubeletDiscovery.discoveredPodSources, 1)
	// should contain a target group for pod 1 with an empty target list as it has
	// been deleted
	require.Contains(t, tg2, &targetgroup.Group{Source: podSource(pod1)})
}

func newPod(name, namespace string) v1.Pod {
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: "container-1",
					Ports: []v1.ContainerPort{
						{
							Name:          "port-1",
							ContainerPort: 443,
						},
					},
				},
			},
		},
		Status: v1.PodStatus{
			PodIP: "1.2.3.4",
		},
	}
}

func TestDiscoveryPodWithoutPod(t *testing.T) {
	pod1 := newPod("pod-1", "namespace-1")
	pod2 := newPod("pod-2", "namespace-2")
	pod1.Spec.Containers[0].Ports = []v1.ContainerPort{}

	podList1 := v1.PodList{
		Items: []v1.Pod{pod1, pod2},
	}

	var args Arguments
	args.SetToDefault()
	kubeletDiscovery, err := NewKubeletDiscovery(args)
	require.NoError(t, err)

	kubeletDiscovery.refresh(podList1)
	require.Len(t, kubeletDiscovery.discoveredPodSources, 2)
}

func TestWithDefaultKubeletHost(t *testing.T) {
	var args Arguments
	args.SetToDefault()
	kubeletDiscovery, err := NewKubeletDiscovery(args)
	require.NoError(t, err)
	require.Equal(t, "https://localhost:10250/pods", kubeletDiscovery.url)
}

func TestWithCustomPath(t *testing.T) {
	kubeletProxyUrl, _ := url.Parse("https://kubernetes.default.svc.cluster.local:443/api/v1/nodes/cluster-node-1/proxy")
	kubeletDiscovery, err := NewKubeletDiscovery(Arguments{
		URL: config.URL{
			URL: kubeletProxyUrl,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://kubernetes.default.svc.cluster.local:443/api/v1/nodes/cluster-node-1/proxy/pods", kubeletDiscovery.url)
}

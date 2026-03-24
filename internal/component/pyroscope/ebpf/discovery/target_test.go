package discovery

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTargetFinder(t *testing.T) {
	options := TargetsOptions{
		Targets: []DiscoveredTarget{
			map[string]string{
				//nolint:lll
				"__meta_kubernetes_pod_container_id":   "containerd://9a7c72f122922fe3445ba85ce72c507c8976c0f3d919403fda7c22dfe516f66f",
				"__meta_kubernetes_namespace":          "foo",
				"__meta_kubernetes_pod_container_name": "bar",
			},
			map[string]string{
				//nolint:lll
				"__container_id__":                     "57ac76ffc93d7e7735ca186bc67115656967fc8aecbe1f65526c4c48b033e6a5",
				"__meta_kubernetes_namespace":          "qwe",
				"__meta_kubernetes_pod_container_name": "asd",
			},
		},
		TargetsOnly:   true,
		DefaultTarget: nil,
	}
	//cgroups.Add(1801264, "9a7c72f122922fe3445ba85ce72c507c8976c0f3d919403fda7c22dfe516f66f")
	//cgroups.Add(489323, "57ac76ffc93d7e7735ca186bc67115656967fc8aecbe1f65526c4c48b033e6a5")
	tf := NewTargetProducer(options)

	target := tf.FindTarget(1801264, "9a7c72f122922fe3445ba85ce72c507c8976c0f3d919403fda7c22dfe516f66f")
	require.NotNil(t, target)
	require.Equal(t, "ebpf/foo/bar", target.labels.Get("service_name"))

	target = tf.FindTarget(489323, "57ac76ffc93d7e7735ca186bc67115656967fc8aecbe1f65526c4c48b033e6a5")
	require.NotNil(t, target)
	require.Equal(t, "ebpf/qwe/asd", target.labels.Get("service_name"))

	tf.Update(options)

	target2 := tf.FindTarget(489323, "57ac76ffc93d7e7735ca186bc67115656967fc8aecbe1f65526c4c48b033e6a5")
	require.NotNil(t, target2)
	require.Same(t, target2, target)

	target = tf.FindTarget(239, "")
	require.Nil(t, target)
}

func TestPreferPIDOverContainerID(t *testing.T) {
	options := TargetsOptions{
		Targets: []DiscoveredTarget{
			map[string]string{
				//nolint:lll
				"__meta_kubernetes_pod_container_id":   "containerd://9a7c72f122922fe3445ba85ce72c507c8976c0f3d919403fda7c22dfe516f66f",
				"__meta_kubernetes_namespace":          "foo",
				"__meta_kubernetes_pod_container_name": "bar",
				"__process_pid__":                      "1801264",
				"exe":                                  "/bin/bash",
			},
			map[string]string{
				//nolint:lll
				"__meta_kubernetes_pod_container_id":   "containerd://9a7c72f122922fe3445ba85ce72c507c8976c0f3d919403fda7c22dfe516f66f",
				"__meta_kubernetes_namespace":          "foo",
				"__meta_kubernetes_pod_container_name": "bar",
				"__process_pid__":                      "1801265",
				"exe":                                  "/bin/dash",
			},
		},
		TargetsOnly:   true,
		DefaultTarget: nil,
	}

	tf := NewTargetProducer(options)

	target := tf.FindTarget(1801264, "")
	require.NotNil(t, target)
	require.Equal(t, "ebpf/foo/bar", target.labels.Get("service_name"))
	require.Equal(t, "/bin/bash", target.labels.Get("exe"))

	target = tf.FindTarget(1801265, "")
	require.NotNil(t, target)
	require.Equal(t, "ebpf/foo/bar", target.labels.Get("service_name"))
	require.Equal(t, "/bin/dash", target.labels.Get("exe"))

	tf.Update(options)

	target2 := tf.FindTarget(1801265, "")
	require.NotNil(t, target2)
	require.Same(t, target2, target)
}

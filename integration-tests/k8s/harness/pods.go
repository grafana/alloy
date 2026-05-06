package harness

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	timeout       = 5 * time.Minute
	retryInterval = 500 * time.Millisecond
)

func (ctx *TestContext) WaitForAllPodsRunning(t *testing.T, namespace, labelSelector string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pods, err := ctx.client.CoreV1().Pods(namespace).List(t.Context(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		require.NoError(c, err)
		require.NotEmpty(c, pods.Items, "no pods for namespace=%s selector=%s", namespace, labelSelector)
		for _, pod := range pods.Items {
			require.Nil(c, pod.DeletionTimestamp, "pod %s is deleting", pod.Name)
			require.Equal(c, corev1.PodRunning, pod.Status.Phase, "pod %s is not running", pod.Name)
		}
	}, timeout, retryInterval)
}

package util

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesTester struct {
	t      *testing.T
	client *kubernetes.Clientset
}

func NewKubernetesTester(t *testing.T) *KubernetesTester {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	require.NoError(t, err)

	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

	return &KubernetesTester{
		t:      t,
		client: clientset,
	}
}

func (t *KubernetesTester) WaitForPodRunning(namespace, labelSelector string) {
	require.EventuallyWithT(t.t, func(c *assert.CollectT) {
		// Get Alloy pods in the testing namespace
		pods, err := t.client.CoreV1().Pods(namespace).List(t.t.Context(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		require.NoError(c, err)
		require.NotEmpty(c, pods.Items, fmt.Sprintf("No pods found for namespace %s and label selector %s", namespace, labelSelector))

		// Check if all pods are running and ready
		for _, pod := range pods.Items {
			// Check if pod is being deleted
			require.Nil(c, pod.DeletionTimestamp, fmt.Sprintf("Pod %s in namespace %s is being deleted", pod.Name, namespace))
			// Check if pod is running
			require.Equal(c, pod.Status.Phase, corev1.PodRunning, fmt.Sprintf("Pod %s in namespace %s is not running", pod.Name, namespace))
		}
	}, 5*time.Minute, 2*time.Second)
}

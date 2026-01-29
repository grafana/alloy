package util

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
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
	logger *slog.Logger
	client *kubernetes.Clientset
}

func NewKubernetesTester(t *testing.T) *KubernetesTester {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	require.NoError(t, err)

	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

	return &KubernetesTester{
		logger: getLogger(),
		client: clientset,
	}
}

// NewTestLogger creates a logger configured based on ALLOY_K8S_TEST_LOGGING env var.
// Set ALLOY_K8S_TEST_LOGGING=debug to enable debug logging.
// TODO: Use the logger from the test package?
func getLogger() *slog.Logger {
	logLevel := slog.LevelInfo
	if os.Getenv(envVarLogLevel) == "debug" {
		logLevel = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
}

func (kt *KubernetesTester) WaitForPodRunning(t *testing.T, namespace, labelSelector string) {
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		// Get Alloy pods in the testing namespace
		kt.logger.Debug("Listing pods", "namespace", namespace, "labelSelector", labelSelector)
		pods, err := kt.client.CoreV1().Pods(namespace).List(t.Context(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		require.NoError(c, err)
		require.NotEmpty(c, pods.Items, fmt.Sprintf("No pods found for namespace %s and label selector %s", namespace, labelSelector))

		// Check if all pods are running and ready
		for _, pod := range pods.Items {
			kt.logger.Debug("Checking pod status", "pod", pod.Name, "namespace", namespace, "phase", pod.Status.Phase, "deletionTimestamp", pod.DeletionTimestamp)
			// Check if pod is being deleted
			require.Nil(c, pod.DeletionTimestamp, fmt.Sprintf("Pod %s in namespace %s is being deleted", pod.Name, namespace))
			// Check if pod is running
			require.Equal(c, corev1.PodRunning, pod.Status.Phase, fmt.Sprintf("Pod %s in namespace %s is not running", pod.Name, namespace))
		}
	}, 5*time.Minute, 2*time.Second)
}

func (k *KubernetesTester) Curl(c *assert.CollectT, url string) string {
	resp, err := http.Get(url)
	require.NoError(c, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(c, err)

	k.logger.Debug("Curl", "url", url, "status", resp.StatusCode, "response", string(body))
	return string(body)
}

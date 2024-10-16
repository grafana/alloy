package kubetail

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

// fakecorev1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"
func Test_parseKubernetesLog(t *testing.T) {
	tt := []struct {
		inputLine  string
		expectTS   time.Time
		expectLine string
	}{
		{
			// Test normal RFC3339Nano log line.
			inputLine:  `2023-01-23T17:00:10.000000001Z hello, world!`,
			expectTS:   time.Date(2023, time.January, 23, 17, 0, 10, 1, time.UTC),
			expectLine: "hello, world!",
		},
		{
			// Test normal RFC3339 log line.
			inputLine:  `2023-01-23T17:00:10Z hello, world!`,
			expectTS:   time.Date(2023, time.January, 23, 17, 0, 10, 0, time.UTC),
			expectLine: "hello, world!",
		},
		{
			// Test empty log line. There will always be a space prepended by
			// Kubernetes.
			inputLine:  `2023-01-23T17:00:10.000000001Z `,
			expectTS:   time.Date(2023, time.January, 23, 17, 0, 10, 1, time.UTC),
			expectLine: "",
		},
	}

	for _, tc := range tt {
		t.Run(tc.inputLine, func(t *testing.T) {
			actualTS, actualLine := parseKubernetesLog(tc.inputLine)
			require.Equal(t, tc.expectTS, actualTS)
			require.Equal(t, tc.expectLine, actualLine)
		})
	}
}

// Test context cancellation.
func TestContextCancel(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stdout)

	target := NewTarget(
		labels.FromStrings("t1", "t1", "t2", "t2"),
		labels.FromStrings("p1", "p1", "p2", "p2"),
	)

	pos, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: t.TempDir() + "/positions.yml",
	})
	require.NoError(t, err)

	namespace := "alloy-test-ns"
	podName := "alloy-pod1"
	kubeObjects := []runtime.Object{
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
				Labels: map[string]string{
					"label1": "value1",
				},
			},
		},
	}
	fakeClientset := fake.NewClientset(kubeObjects...)
	entries := make(chan loki.Entry)
	handler := loki.NewEntryHandler(entries, func() {})

	task := &tailerTask{
		Options: &Options{
			Client:    fakeClientset,
			Handler:   handler,
			Positions: pos,
		},
		Target: target,
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	tailer := newTailer(logger, task)
	go tailer.Run(ctx)

	// Let the tailer run for a bit.
	time.Sleep(10 * time.Second)

	// deletionOption := metav1.NewDeleteOptions(0)
	// err = fakeClientset.CoreV1().Pods(namespace).Delete(ctx, podName, *deletionOption)
	// require.NoError(t, err)

	// fakeClientset.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("get", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
	// 	return true, &v1.Pod{}, errors.New("Error creating secret")
	// })

	// fakeClientset.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("logs", "alloy-pod1", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
	// 	return true, &v1.Pod{}, errors.New("Error creating logs")
	// })

	// Let the tailer run for a bit.
	time.Sleep(10 * time.Second)

	for entry := range entries {
		// k8s.io/client-go@v0.31.0/kubernetes/typed/core/v1/fake/fake_pod_expansion.go#GetLogs
		require.Equal(t, entry.Line, "fake logs")
		//TODO: Also check the targets array
	}

	cancelFunc()
}

// Test tailer restart.

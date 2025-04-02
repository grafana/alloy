package podlogs

import (
	"fmt"
	"testing"

	"github.com/go-kit/log"
	monitoringv1alpha2 "github.com/grafana/alloy/internal/component/loki/source/podlogs/internal/apis/monitoring/v1alpha2"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/util/strutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuildPodLogsTargetLabels(t *testing.T) {
	tests := []struct {
		name           string
		podLogs        *monitoringv1alpha2.PodLogs
		expectedLabels map[string]string
	}{
		{
			name: "with labels and annotations",
			podLogs: &monitoringv1alpha2.PodLogs{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test",
					Labels: map[string]string{
						"app":         "myapp",
						"complex-key": "value1",
					},
					Annotations: map[string]string{
						"note":        "important",
						"another-key": "value2",
					},
				},
			},
			expectedLabels: map[string]string{
				kubePodlogsNamespace: "default",
				kubePodlogsName:      "test",
				kubePodlogsLabel + strutil.SanitizeLabelName("app"):                     "myapp",
				kubePodlogsLabelPresent + strutil.SanitizeLabelName("app"):              "true",
				kubePodlogsLabel + strutil.SanitizeLabelName("complex-key"):             "value1",
				kubePodlogsLabelPresent + strutil.SanitizeLabelName("complex-key"):      "true",
				kubePodlogsAnnotation + strutil.SanitizeLabelName("note"):               "important",
				kubePodlogsAnnotationPresent + strutil.SanitizeLabelName("note"):        "true",
				kubePodlogsAnnotation + strutil.SanitizeLabelName("another-key"):        "value2",
				kubePodlogsAnnotationPresent + strutil.SanitizeLabelName("another-key"): "true",
			},
		},
		{
			name: "empty labels and annotations",
			podLogs: &monitoringv1alpha2.PodLogs{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "myns",
					Name:      "noprops",
				},
			},
			expectedLabels: map[string]string{
				kubePodlogsNamespace: "myns",
				kubePodlogsName:      "noprops",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildPodLogsTargetLabels(tc.podLogs)
			gotMap := make(map[string]string, len(got))
			for _, lbl := range got {
				gotMap[lbl.Name] = lbl.Value
			}

			// Verify each expected key is present with its value.
			for k, v := range tc.expectedLabels {
				if val, ok := gotMap[k]; !ok {
					t.Errorf("missing key %q in output", k)
				} else if val != v {
					t.Errorf("for key %q, expected %q, got %q", k, v, val)
				}
			}

			// Ensure no extra keys are present.
			if len(gotMap) != len(tc.expectedLabels) {
				t.Errorf("expected %d labels, got %d: %v", len(tc.expectedLabels), len(gotMap), gotMap)
			}
		})
	}
}

func TestReconcilePodLogs_DefaultLabels(t *testing.T) {
	// Create a PodLogs object with empty selectors.
	podLogs := &monitoringv1alpha2.PodLogs{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "testlogs",
		},
		Spec: monitoringv1alpha2.PodLogsSpec{
			Selector:          metav1.LabelSelector{}, // matches all Pods
			NamespaceSelector: metav1.LabelSelector{}, // matches all Namespaces
		},
	}

	// Create a Namespace with some dummy labels.
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "default",
			Labels: map[string]string{"env": "test"},
		},
	}

	// Create a Pod that should match the selectors.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "mypod",
			UID:       "12345",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "container1",
					Image: "nginx",
				},
			},
			NodeName: "node1",
		},
		Status: corev1.PodStatus{
			PodIP:  "10.0.0.1",
			Phase:  corev1.PodRunning,
			HostIP: "192.168.1.1",
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme,
		monitoringv1alpha2.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatalf("unable to register scheme: %v", err)
		}
	}

	// Build a fake client with the PodLogs, Namespace, and Pod.
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(podLogs, ns, pod).Build()

	// Create a reconciler. The tailer and cluster are not used by reconcilePodLogs,
	// so we can pass nil.
	r := newReconciler(log.NewNopLogger(), nil, nil)

	// Call reconcilePodLogs.
	targets, _ := r.reconcilePodLogs(t.Context(), cl, podLogs)

	// Verify that one target was discovered.
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	target := targets[0]
	discoveryLabelsMap := target.DiscoveryLabels().Map()
	labelsMap := target.Labels().Map()

	// Expected default labels.
	expectedInstance := fmt.Sprintf("%s/%s:%s", pod.Namespace, pod.Name, "container1")
	expectedJob := fmt.Sprintf("%s/%s", podLogs.Namespace, podLogs.Name)

	// Check expected pod logs labels.
	if discoveryLabelsMap[kubePodlogsNamespace] != podLogs.Namespace {
		t.Errorf("expected pod logs namespace %q, got %q", podLogs.Namespace, labelsMap[kubePodlogsNamespace])
	}
	if discoveryLabelsMap[kubePodlogsName] != podLogs.Name {
		t.Errorf("expected pod logs name %q, got %q", podLogs.Name, labelsMap[kubePodlogsName])
	}
	if discoveryLabelsMap[kubeNamespaceLabel+strutil.SanitizeLabelName("env")] != "test" {
		t.Errorf("expected namespace label 'env' to be 'test', got %q", labelsMap[kubeNamespaceLabel+strutil.SanitizeLabelName("env")])
	}

	if labelsMap[model.InstanceLabel] != expectedInstance {
		t.Errorf("expected instance label %q, got %q", expectedInstance, labelsMap[model.InstanceLabel])
	}
	if labelsMap[model.JobLabel] != expectedJob {
		t.Errorf("expected job label %q, got %q", expectedJob, labelsMap[model.JobLabel])
	}
	if discoveryLabelsMap[kubePodContainerInit] != "false" {
		t.Errorf("expected %s to be false, got %q", kubePodContainerInit, labelsMap[kubePodContainerInit])
	}
	if discoveryLabelsMap[kubePodContainerName] != "container1" {
		t.Errorf("expected %s to be 'container1', got %q", kubePodContainerName, labelsMap[kubePodContainerName])
	}
	if discoveryLabelsMap[kubePodContainerImage] != "nginx" {
		t.Errorf("expected %s to be 'nginx', got %q", kubePodContainerImage, labelsMap[kubePodContainerImage])
	}
	if discoveryLabelsMap[kubePodReady] != "true" {
		t.Errorf("expected %s to be 'true', got %q", kubePodReady, labelsMap[kubePodReady])
	}
	if discoveryLabelsMap[kubePodPhase] != string(corev1.PodRunning) {
		t.Errorf("expected %s to be %q, got %q", kubePodPhase, string(corev1.PodRunning), labelsMap[kubePodPhase])
	}
	if discoveryLabelsMap[kubePodNodeName] != "node1" {
		t.Errorf("expected %s to be 'node1', got %q", kubePodNodeName, labelsMap[kubePodNodeName])
	}
	if discoveryLabelsMap[kubePodHostIP] != "192.168.1.1" {
		t.Errorf("expected %s to be '192.168.1.1', got %q", kubePodHostIP, labelsMap[kubePodHostIP])
	}
	if discoveryLabelsMap[kubePodUID] != "12345" {
		t.Errorf("expected %s to be '12345', got %q", kubePodUID, labelsMap[kubePodUID])
	}
}

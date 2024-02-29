package kubetail

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"k8s.io/apimachinery/pkg/types"
)

// Internal labels which indicate what container to tail logs from.
const (
	LabelPodNamespace     = "__pod_namespace__"
	LabelPodName          = "__pod_name__"
	LabelPodContainerName = "__pod_container_name__"
	LabelPodUID           = "__pod_uid__"

	kubePodNamespace     = "__meta_kubernetes_namespace"
	kubePodName          = "__meta_kubernetes_pod_name"
	kubePodContainerName = "__meta_kubernetes_pod_container_name"
	kubePodUID           = "__meta_kubernetes_pod_uid"
)

// Target represents an individual container being tailed for logs.
type Target struct {
	origLabels   labels.Labels // Original discovery labels
	publicLabels labels.Labels // Labels with private labels omitted

	namespacedName types.NamespacedName
	containerName  string
	id             string // String representation of "namespace/pod:container"; not fully unique
	uid            string // UID from pod
	hash           uint64 // Hash of public labels and id

	mut       sync.RWMutex
	lastError error
	lastEntry time.Time
}

// NewTarget creates a new Target which can be passed to a tailer.
func NewTarget(origLabels labels.Labels, lset labels.Labels) *Target {
	// Precompute some values based on labels so we don't have to continually
	// search them.
	var (
		namespacedName = types.NamespacedName{
			Namespace: lset.Get(LabelPodNamespace),
			Name:      lset.Get(LabelPodName),
		}

		containerName = lset.Get(LabelPodContainerName)
		uid           = lset.Get(LabelPodUID)

		id           = fmt.Sprintf("%s:%s", namespacedName, containerName)
		publicLabels = publicLabels(lset)
	)

	// Precompute the hash of the target from the public labels and the ID of the
	// target.
	hasher := xxhash.New()
	fmt.Fprintf(hasher, "%016d", publicLabels.Hash())
	fmt.Fprint(hasher, id)
	fmt.Fprint(hasher, uid)
	hash := hasher.Sum64()

	return &Target{
		origLabels:   origLabels,
		publicLabels: publicLabels,

		namespacedName: namespacedName,
		containerName:  containerName,
		id:             id,
		uid:            uid,
		hash:           hash,
	}
}

func publicLabels(lset labels.Labels) labels.Labels {
	lb := labels.NewBuilder(lset)

	for _, l := range lset {
		if strings.HasPrefix(l.Name, model.ReservedLabelPrefix) {
			lb.Del(l.Name)
		}
	}

	return lb.Labels()
}

// NamespacedName returns the key of the Pod being targeted.
func (t *Target) NamespacedName() types.NamespacedName { return t.namespacedName }

// ContainerName returns the container name being targeted.
func (t *Target) ContainerName() string { return t.containerName }

// String returns a string representing the target in the form
// "namespace/name:container".
func (t *Target) String() string { return t.id }

// DiscoveryLabels returns the set of original labels prior to processing or
// relabeling.
func (t *Target) DiscoveryLabels() labels.Labels { return t.origLabels }

// Labels returns the set of public labels for the target.
func (t *Target) Labels() labels.Labels { return t.publicLabels }

// Hash returns an identifying hash for the target.
func (t *Target) Hash() uint64 { return t.hash }

// UID returns the UID for this target, based on the pod's UID.
func (t *Target) UID() string { return t.uid }

// Report reports information about the target.
func (t *Target) Report(time time.Time, err error) {
	t.mut.Lock()
	defer t.mut.Unlock()

	t.lastError = err
	t.lastEntry = time
}

// LastError returns the most recent error if the target is unhealthy.
func (t *Target) LastError() error {
	t.mut.RLock()
	defer t.mut.RUnlock()

	return t.lastError
}

// LastEntry returns the time the most recent log line was read or when the
// most recent error occurred.
func (t *Target) LastEntry() time.Time {
	t.mut.RLock()
	defer t.mut.RUnlock()

	return t.lastEntry
}

// PrepareLabels builds a label set with default labels applied from the
// default label set. It validates that the input label set is valid.
//
// The namespace of the pod to tail logs from is determined by the
// [LabelPodNamespace] label. If this label isn't present, PrepareLabels falls
// back to __meta_kubernetes_namespace.
//
// The name of the pod to tail logs from is determined by the [LabelPodName]
// label. If this label isn't present, PrepareLabels falls back to
// __meta_kubernetes_pod_name.
//
// The name of the container to tail logs from is determined by the
// [LabelPodContainerName] label. If this label isn't present, PrepareLabels
// falls back to __meta_kubernetes_pod_container_name.
//
// Validation of lset fails if there is no label indicating the pod namespace,
// pod name, or container name.
func PrepareLabels(lset labels.Labels, defaultJob string) (res labels.Labels, err error) {
	tailLabels := []labels.Label{
		{Name: model.JobLabel, Value: defaultJob},
	}
	lb := labels.NewBuilder(lset)

	// Add default labels to lb if they're not in lset.
	for _, l := range tailLabels {
		if !lset.Has(l.Name) {
			lb.Set(l.Name, l.Value)
		}
	}

	firstLabelValue := func(labelNames ...string) string {
		for _, labelName := range labelNames {
			if lv := lset.Get(labelName); lv != "" {
				return lv
			}
		}
		return ""
	}

	var (
		podNamespace     = firstLabelValue(LabelPodNamespace, kubePodNamespace)
		podName          = firstLabelValue(LabelPodName, kubePodName)
		podContainerName = firstLabelValue(LabelPodContainerName, kubePodContainerName)
		podUID           = firstLabelValue(LabelPodUID, kubePodUID)
	)

	switch {
	case podNamespace == "":
		return nil, fmt.Errorf("missing pod namespace label")
	case podName == "":
		return nil, fmt.Errorf("missing pod name label")
	case podContainerName == "":
		return nil, fmt.Errorf("missing pod container name label")
	case podUID == "":
		return nil, fmt.Errorf("missing pod UID label")
	}

	// Make sure that LabelPodNamespace, LabelPodName, LabelPodContainerName, and
	// LabelPodUID are set on the final target.
	if !lset.Has(LabelPodNamespace) {
		lb.Set(LabelPodNamespace, podNamespace)
	}
	if !lset.Has(LabelPodName) {
		lb.Set(LabelPodName, podName)
	}
	if !lset.Has(LabelPodContainerName) {
		lb.Set(LabelPodContainerName, podContainerName)
	}
	if !lset.Has(LabelPodUID) {
		lb.Set(LabelPodUID, podUID)
	}

	// Meta labels are deleted after relabelling. Other internal labels propagate
	// to the target which decides whether they will be part of their label set.
	for _, l := range lset {
		if strings.HasPrefix(l.Name, model.MetaLabelPrefix) {
			lb.Del(l.Name)
		}
	}

	// Default the instance label to the target address.
	if !lset.Has(model.InstanceLabel) {
		defaultInstance := fmt.Sprintf("%s/%s:%s", podNamespace, podName, podContainerName)
		lb.Set(model.InstanceLabel, defaultInstance)
	}

	res = lb.Labels()
	for _, l := range res {
		// Check label values are valid, drop the target if not.
		if !model.LabelValue(l.Value).IsValid() {
			return nil, fmt.Errorf("invalid label value for %q: %q", l.Name, l.Value)
		}
	}
	return res, nil
}

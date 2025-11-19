package discovery // import "go.opentelemetry.io/ebpf-profiler/pyroscope/discovery"

import (
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/sirupsen/logrus"
)

type DiscoveredTarget map[string]string

const (
	labelContainerID      = "__container_id__"
	labelPID              = "__process_pid__"
	labelServiceName      = "service_name"
	labelServiceNameK8s   = "__meta_kubernetes_pod_annotation_pyroscope_io_service_name"
	MetricValueProcessCPU = "process_cpu"
	MetricValueOffCPU     = "offcpu"
)

type Target struct {
	labels      labels.Labels
	serviceName string
	fingerprint uint64
}

func NewTarget(cid containerID, pid uint32, target DiscoveredTarget) *Target {
	serviceName := target[labelServiceName]
	if serviceName == "" {
		serviceName = inferServiceName(target)
	}

	lset := make(map[string]string, len(target))
	for k, v := range target {
		if strings.HasPrefix(k, model.ReservedLabelPrefix) &&
			k != labels.MetricName {
			continue
		}
		lset[k] = v
	}
	if lset[labelServiceName] == "" {
		lset[labelServiceName] = serviceName
	}
	if cid != "" {
		lset[labelContainerID] = string(cid)
	}
	if pid != 0 {
		lset[labelPID] = strconv.Itoa(int(pid))
	}
	ls := labels.FromMap(lset)
	return &Target{
		labels:      ls,
		fingerprint: ls.Hash(),
		serviceName: serviceName,
	}
}

func (t *Target) ServiceName() string {
	return t.serviceName
}

func inferServiceName(target DiscoveredTarget) string {
	k8sServiceName := target[labelServiceNameK8s]
	if k8sServiceName != "" {
		return k8sServiceName
	}
	k8sNamespace := target["__meta_kubernetes_namespace"]
	k8sContainer := target["__meta_kubernetes_pod_container_name"]
	if k8sNamespace != "" && k8sContainer != "" {
		return "ebpf/" + k8sNamespace + "/" + k8sContainer
	}
	dockerContainer := target["__meta_docker_container_name"]
	if dockerContainer != "" {
		return dockerContainer
	}
	swarmServiceKey := "__meta_dockerswarm_container_label_service_name"
	if swarmService := target[swarmServiceKey]; swarmService != "" {
		return swarmService
	}
	if swarmService := target["__meta_dockerswarm_service_name"]; swarmService != "" {
		return swarmService
	}
	return "unspecified"
}

func (t *Target) Labels() (uint64, labels.Labels) {
	return t.fingerprint, t.labels
}

func (t *Target) String() string {
	return t.labels.String()
}

func (t *Target) Get(k string) (string, bool) {
	v := t.labels.Get(k)
	return v, v != ""
}

type containerID string

// TargetProducer ( ex TargetFinder)
type TargetProducer interface {
	FindTarget(pid uint32, containerID string) *Target
	Update(args TargetsOptions)
}
type TargetsOptions struct {
	Targets       []DiscoveredTarget
	TargetsOnly   bool
	DefaultTarget DiscoveredTarget
}

type targetProducer struct {
	cid2target    map[containerID]*Target
	pid2target    map[uint32]*Target
	defaultTarget *Target
	sync          sync.Mutex
}

func NewTargetProducer(
	options TargetsOptions,
) TargetProducer {
	res := &targetProducer{}
	res.setTargets(options)
	return res
}

func (tf *targetProducer) FindTarget(pid uint32, cid string) *Target {
	tf.sync.Lock()
	res := tf.findTarget(pid, cid)
	if res != nil {
		tf.sync.Unlock()
		return res
	}
	res = tf.defaultTarget
	tf.sync.Unlock()
	return res
}

func (tf *targetProducer) Update(args TargetsOptions) {
	tf.sync.Lock()
	defer tf.sync.Unlock()
	tf.setTargets(args)
}

func (tf *targetProducer) setTargets(opts TargetsOptions) {
	containerID2Target := make(map[containerID]*Target)
	pid2Target := make(map[uint32]*Target)
	oco := func(prev, next *Target) *Target {
		if prev != nil && prev.fingerprint == next.fingerprint {
			return prev
		}
		return next
	}
	for _, target := range opts.Targets {
		if pid := pidFromTarget(target); pid != 0 {
			t := NewTarget("", pid, target)
			pid2Target[pid] = oco(tf.pid2target[pid], t)
		} else if cid := containerIDFromTarget(target); cid != "" {
			t := NewTarget(cid, 0, target)
			containerID2Target[cid] = oco(tf.cid2target[cid], t)
		}
	}
	if len(opts.Targets) > 0 && len(containerID2Target) == 0 && len(pid2Target) == 0 {
		logrus.Warn("targetProducer: No targets found")
	}
	tf.cid2target = containerID2Target
	tf.pid2target = pid2Target
	if opts.TargetsOnly {
		tf.defaultTarget = nil
	} else {
		t := NewTarget("", 0, opts.DefaultTarget)
		tf.defaultTarget = oco(tf.defaultTarget, t)
	}
	logrus.Debugf("targetProducer: created targets cids %d pids %d",
		len(tf.cid2target), len(tf.pid2target))
}

func (tf *targetProducer) findTarget(pid uint32, cid string) *Target {
	if target, ok := tf.pid2target[pid]; ok {
		return target
	}

	return tf.cid2target[containerID(cid)]
}

func pidFromTarget(target DiscoveredTarget) uint32 {
	t, ok := target[labelPID]
	if !ok {
		return 0
	}
	var pid uint64
	var err error
	pid, err = strconv.ParseUint(t, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(pid)
}

func containerIDFromTarget(target DiscoveredTarget) containerID {
	cid, ok := target[labelContainerID]
	if ok && cid != "" {
		return containerID(cid)
	}
	cid, ok = target["__meta_kubernetes_pod_container_id"]
	if ok && cid != "" {
		return getContainerIDFromK8S(cid)
	}
	cid, ok = target["__meta_docker_container_id"]
	if ok && cid != "" {
		return containerID(cid)
	}
	if cid, ok = target["__meta_dockerswarm_task_container_id"]; ok && cid != "" {
		return containerID(cid)
	}
	return ""
}

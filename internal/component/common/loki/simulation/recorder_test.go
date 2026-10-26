package simulation

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// recorders holds one recorder per simulation, keyed by the run ID that every
// fixture gets as an argument.
var recorders sync.Map

func recorderFor(run string) *recorder {
	r, ok := recorders.Load(run)
	if !ok {
		panic(fmt.Sprintf("no recorder for run %q", run))
	}
	return r.(*recorder)
}

// recorder tracks every entry from the moment a source hands it off until a
// sink receives it, plus the exclusive resources that sources hold.
type recorder struct {
	nextID atomic.Int64
	paused atomic.Bool
	// reached counts entries that reached at least one sink. It's cheap to
	// read while entries flow, unlike account.
	reached atomic.Int64
	// abort unblocks every fixture. Only used to unwind a deadlocked runtime.
	abort chan struct{}

	mut        sync.Mutex
	produced   map[int64]string           // id -> source that handed it off
	delivered  map[int64]map[string]int   // id -> sink and path -> times received
	dropped    map[int64]string           // id -> component that dropped it
	resources  map[string]string          // resource -> holder
	starts     map[string]int             // source/generation -> Start calls
	running    map[string]runningProducer // source -> its running producer
	violations []string
}

type runningProducer struct {
	holder     string
	generation int
}

func newRecorder() *recorder {
	return &recorder{
		abort:     make(chan struct{}),
		produced:  map[int64]string{},
		delivered: map[int64]map[string]int{},
		dropped:   map[int64]string{},
		resources: map[string]string{},
		starts:    map[string]int{},
		running:   map[string]runningProducer{},
	}
}

func (r *recorder) newEntry(source string) (loki.Entry, int64) {
	id := r.nextID.Add(1)
	return loki.Entry{
		Labels: model.LabelSet{"source": model.LabelValue(source)},
		Entry:  push.Entry{Timestamp: time.Now(), Line: strconv.FormatInt(id, 10)},
	}, id
}

func entryID(e loki.Entry) int64 {
	id, err := strconv.ParseInt(e.Line, 10, 64)
	if err != nil {
		panic(err)
	}
	return id
}

func (r *recorder) markProduced(id int64, source string) {
	r.mut.Lock()
	defer r.mut.Unlock()
	r.produced[id] = source
}

// markDelivered records that sink received e. Entries are counted per path,
// see relay.
func (r *recorder) markDelivered(e loki.Entry, sink string) {
	id := entryID(e)
	if via := e.Labels["via"]; via != "" {
		sink += " via " + string(via)
	}
	r.mut.Lock()
	defer r.mut.Unlock()
	if r.delivered[id] == nil {
		r.delivered[id] = map[string]int{}
		r.reached.Inc()
	}
	r.delivered[id][sink]++
}

func (r *recorder) markDropped(e loki.Entry, by string) {
	r.mut.Lock()
	defer r.mut.Unlock()
	r.dropped[entryID(e)] = by
}

// acquire takes an exclusive resource, like binding a port.
func (r *recorder) acquire(resource, holder string) error {
	r.mut.Lock()
	defer r.mut.Unlock()
	if h, held := r.resources[resource]; held {
		return fmt.Errorf("resource %q is held by %s", resource, h)
	}
	r.resources[resource] = holder
	return nil
}

func (r *recorder) release(resource, holder string) {
	r.mut.Lock()
	defer r.mut.Unlock()
	if h := r.resources[resource]; h != holder {
		r.violations = append(r.violations, fmt.Sprintf("%s released %q held by %q", holder, resource, h))
		return
	}
	delete(r.resources, resource)
}

// countStart counts Start calls for a source generation and returns the count
// including this call.
func (r *recorder) countStart(key string) int {
	r.mut.Lock()
	defer r.mut.Unlock()
	r.starts[key]++
	return r.starts[key]
}

func (r *recorder) markRunning(source, holder string, generation int) {
	r.mut.Lock()
	defer r.mut.Unlock()
	if prev, ok := r.running[source]; ok {
		r.violations = append(r.violations, fmt.Sprintf("%s started while %s was still running", holder, prev.holder))
	}
	r.running[source] = runningProducer{holder: holder, generation: generation}
}

func (r *recorder) markStopped(source, holder string) {
	r.mut.Lock()
	defer r.mut.Unlock()
	if r.running[source].holder == holder {
		delete(r.running, source)
	}
}

// runningGeneration returns the generation of the running producer of
// source, or -1 if none is running.
func (r *recorder) runningGeneration(source string) int {
	r.mut.Lock()
	defer r.mut.Unlock()
	if p, ok := r.running[source]; ok {
		return p.generation
	}
	return -1
}

// deliveries returns where each entry produced after epoch was delivered,
// keyed by the source that produced it.
func (r *recorder) deliveries(epoch int64) map[int64]delivery {
	r.mut.Lock()
	defer r.mut.Unlock()
	d := map[int64]delivery{}
	for id, source := range r.produced {
		if id > epoch {
			d[id] = delivery{source: source, sinks: maps.Clone(r.delivered[id])}
		}
	}
	return d
}

type delivery struct {
	source string
	sinks  map[string]int
}

// accounting summarizes what happened to every produced entry.
type accounting struct {
	produced, delivered, dropped int
	missing                      []int64 // produced but neither delivered nor dropped
	duplicates                   []string
	droppedBy                    map[string]int
	violations                   []string
}

func (r *recorder) account() accounting {
	r.mut.Lock()
	defer r.mut.Unlock()

	a := accounting{produced: len(r.produced), droppedBy: map[string]int{}, violations: slices.Clone(r.violations)}
	for id := range r.produced {
		sinks, ok := r.delivered[id]
		_, dropped := r.dropped[id]
		switch {
		case ok:
			a.delivered++
		case dropped:
			a.dropped++
			a.droppedBy[r.dropped[id]]++
		default:
			a.missing = append(a.missing, id)
		}
		for sink, n := range sinks {
			if n > 1 {
				a.duplicates = append(a.duplicates, fmt.Sprintf("entry %d reached %s %d times", id, sink, n))
			}
		}
	}
	slices.Sort(a.missing)
	return a
}

func (a accounting) String() string {
	missing := a.missing
	if len(missing) > 10 {
		missing = missing[:10]
	}
	return fmt.Sprintf("produced=%d delivered=%d dropped=%d %v missing=%d (first: %v) duplicates=%d violations=%v",
		a.produced, a.delivered, a.dropped, a.droppedBy, len(a.missing), missing, len(a.duplicates), a.violations)
}

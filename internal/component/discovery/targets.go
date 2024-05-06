package discovery

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/grafana/alloy/syntax"
	"github.com/prometheus/prometheus/model/labels"
)

var (
	_ syntax.Capsule                     = (*LabelArray)(nil)
	_ syntax.ConvertibleFromArrayCapsule = (*LabelArray)(nil)
)

type Labels struct {
	labels map[string]string
}

func (tc Labels) LabelsCopy() labels.Labels {
	return labels.FromMap(tc.labels)
}

type LabelArray struct {
	Lbls []*Labels
}

func (t *LabelArray) ConvertFromItem(src interface{}) error {
	if t == nil {
		t = &LabelArray{
			Lbls: make([]*Labels, 0),
		}
	}
	switch src := src.(type) {
	case map[string]string:
		t.Lbls = append(t.Lbls, CapsulePool.CreateLabelCache(src))
		return nil
	}
	return fmt.Errorf("unable to convert %T into TargetCapsule", src)
}

func (t *LabelArray) AlloyCapsule() {
}

var CapsulePool = &LabelCache{
	cache:    NewInterner(),
	capsules: make(map[*Labels]int),
}

type LabelCache struct {
	mut      sync.Mutex
	capsules map[*Labels]int
	cache    Interner
}

func (tcc *LabelCache) CreateLabelCache(labels map[string]string) *Labels {
	tcc.mut.Lock()
	defer tcc.mut.Unlock()

	var c *Labels

	lbls := make(map[string]string)
	for k, v := range labels {
		lbls[tcc.cache.Intern(k)] = tcc.cache.Intern(v)
	}
	tc := &Labels{
		labels: lbls,
	}

	// SetFinalizer will allow us to automatically clean up the TargetCapsule.
	runtime.SetFinalizer(tc, func(tc *Labels) {
		for k, v := range tc.labels {
			tcc.cache.Release(k)
			tcc.cache.Release(v)
		}
	})
	return c
}

func (tcc *LabelCache) FromLabels(labels labels.Labels) *Labels {
	tcc.mut.Lock()
	defer tcc.mut.Unlock()

	lbls := make(map[string]string)
	for _, v := range labels {
		lbls[tcc.cache.Intern(v.Name)] = tcc.cache.Intern(v.Value)
	}
	c := &Labels{
		labels: lbls,
	}

	// SetFinalizer will allow us to automatically clean up the string interning.
	// It's fine if it takes some time to trigger.
	runtime.SetFinalizer(c, func(tc *Labels) {
		for k, v := range tc.labels {
			tcc.cache.Release(k)
			tcc.cache.Release(v)
		}
	})
	return c
}

package consumer

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"

	"github.com/grafana/alloy/internal/component/kafka/tenant/kafkaclient"
)

// ownership fails the test if two consumers process the same partition at
// the same time.
type ownership struct {
	mut        sync.Mutex
	active     map[int32]string
	violations []string
}

func (o *ownership) hook(id string) func(int32, bool) {
	return func(p int32, start bool) {
		o.mut.Lock()
		defer o.mut.Unlock()
		if !start {
			delete(o.active, p)
			return
		}
		if other, ok := o.active[p]; ok && other != id {
			o.violations = append(o.violations, fmt.Sprintf("partition %d processed by %s and %s at once", p, other, id))
		}
		o.active[p] = id
	}
}

func assigned(c *Component) int {
	return int(testutil.ToFloat64(c.metrics.assignedPartitions))
}

// tenantFormats groups deliveries by tenant into the set of formats seen.
func tenantFormats(ds []delivery) map[string]map[string]int {
	out := map[string]map[string]int{}
	for _, d := range ds {
		if out[d.Tenant] == nil {
			out[d.Tenant] = map[string]int{}
		}
		out[d.Tenant][d.Format]++
	}
	return out
}

func TestAffinity(t *testing.T) {
	const rounds = 5

	cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopic))
	defer cluster.Close()

	own := &ownership{active: map[int32]string{}}
	rec1, rec2 := newRecorder(t), newRecorder(t)
	c1, stop1 := startConsumer(t, "consumer-1", rec1.args(cluster.ListenAddrs()), own.hook("consumer-1"))
	c2, _ := startConsumer(t, "consumer-2", rec2.args(cluster.ListenAddrs()), own.hook("consumer-2"))

	require.Eventually(t, func() bool {
		return assigned(c1) == 2 && assigned(c2) == 2
	}, 30*time.Second, 50*time.Millisecond, "partitions should be balanced across both consumers")

	cl := newProducer(t, cluster.ListenAddrs())
	produceRounds := func() {
		for range rounds {
			for tenant := range testTenants {
				produce(t, cl, allSignals(t, tenant)...)
			}
		}
	}
	perTenant := rounds * 4
	total := perTenant * len(testTenants)

	// Phase 1: both consumers run. Every tenant is handled by exactly one.
	produceRounds()
	require.Eventually(t, func() bool {
		return len(rec1.deliveries())+len(rec2.deliveries()) == total
	}, 30*time.Second, 50*time.Millisecond)

	t1, t2 := tenantFormats(rec1.deliveries()), tenantFormats(rec2.deliveries())
	for tenant := range testTenants {
		_, in1 := t1[tenant]
		_, in2 := t2[tenant]
		require.True(t, in1 != in2, "tenant %s must be processed by exactly one consumer (1: %v, 2: %v)", tenant, in1, in2)

		formats := t1[tenant]
		if in2 {
			formats = t2[tenant]
		}
		for _, f := range []string{kafkaclient.FormatPromRWv1, kafkaclient.FormatLokiPush, kafkaclient.FormatPyroscopeIngest, kafkaclient.FormatOTLP} {
			require.Equal(t, rounds, formats[f], "tenant %s format %s", tenant, f)
		}
	}
	require.NotEmpty(t, t1, "consumer-1 should own at least one tenant")
	require.NotEmpty(t, t2, "consumer-2 should own at least one tenant")

	// Phase 2: consumer-1 stops. Its partitions move whole to consumer-2.
	before1, before2 := len(rec1.deliveries()), len(rec2.deliveries())
	stop1()
	require.Eventually(t, func() bool { return assigned(c2) == 4 }, 30*time.Second, 50*time.Millisecond)

	produceRounds()
	require.Eventually(t, func() bool {
		return len(rec2.deliveries()) == before2+total
	}, 30*time.Second, 50*time.Millisecond)
	require.Len(t, rec1.deliveries(), before1, "a stopped consumer must not process records")

	phase2 := tenantFormats(rec2.deliveries()[before2:])
	for tenant := range testTenants {
		for _, f := range []string{kafkaclient.FormatPromRWv1, kafkaclient.FormatLokiPush, kafkaclient.FormatPyroscopeIngest, kafkaclient.FormatOTLP} {
			require.Equal(t, rounds, phase2[tenant][f], "tenant %s format %s after handoff", tenant, f)
		}
	}

	own.mut.Lock()
	defer own.mut.Unlock()
	require.Empty(t, own.violations)
}

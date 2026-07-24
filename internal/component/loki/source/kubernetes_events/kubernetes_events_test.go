package kubernetes_events

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	cachetools "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/syntax"
)

func TestUpdate(t *testing.T) {
	fixture := newTestFixture(t)
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		forward_to = []
		job_name   = "events-before"
		log_format = "logfmt"

		client {
			api_server = "http://127.0.0.1:1"
		}
	`), &args))
	c := fixture.New(&args)

	eventTime := time.Now()
	fixture.SendEvent("alloy-reloaded-before", eventTime)
	entry := fixture.ReceiveEntry()
	require.Equal(t, model.LabelValue("events-before"), entry.Labels["job"])
	require.Equal(t, `name=alloy msg="configuration reloaded" `, entry.Line)

	args.JobName = "events-after"
	args.LogFormat = logFormatJson
	require.NoError(t, c.Update(args))

	fixture.SendEvent("alloy-reloaded-after", eventTime.Add(time.Second))
	entry = fixture.ReceiveEntry()
	require.Equal(t, model.LabelValue("events-after"), entry.Labels["job"])
	require.JSONEq(t, `{"msg":"configuration reloaded","name":"alloy"}`, entry.Line)
}

type testFixture struct {
	t          *testing.T
	ctx        context.Context
	fakeClient *fake.Clientset
	receiver   loki.LogsReceiver
}

func newTestFixture(t *testing.T) *testFixture {
	t.Helper()

	fakeClient := fake.NewClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	fakeCache := &eventCache{informer: informerFactory.Core().V1().Events().Informer()}
	originalNewCache := newCache
	newCache = func(*rest.Config, cache.Options) (cache.Cache, error) {
		return fakeCache, nil
	}
	t.Cleanup(func() { newCache = originalNewCache })

	return &testFixture{
		t:          t,
		ctx:        context.Background(),
		fakeClient: fakeClient,
		receiver:   loki.NewLogsReceiver(loki.WithChannel(make(chan loki.Entry, 1))),
	}
}

func (f *testFixture) New(args *Arguments) *Component {
	f.t.Helper()
	args.ForwardTo = []loki.LogsReceiver{f.receiver}

	c, err := New(component.Options{
		ID:       "loki.source.kubernetes_events.test",
		Logger:   logging.NewSlogNop(),
		DataPath: f.t.TempDir(),
		GetServiceData: func(name string) (any, error) {
			if name != cluster.ServiceName {
				return nil, fmt.Errorf("unexpected service %q", name)
			}
			return cluster.Mock(), nil
		},
	}, *args)
	require.NoError(f.t, err)

	ctx, cancel := context.WithCancel(f.ctx)
	runDone := make(chan error, 1)
	go func() { runDone <- c.Run(ctx) }()
	f.t.Cleanup(func() {
		cancel()
		require.NoError(f.t, <-runDone)
	})
	return c
}

func (f *testFixture) SendEvent(name string, eventTime time.Time) {
	f.t.Helper()
	_, err := f.fakeClient.CoreV1().Events("default").Create(f.ctx, &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		EventTime: metav1.MicroTime{Time: eventTime},
		InvolvedObject: corev1.ObjectReference{
			Name:      "alloy",
			Namespace: "default",
		},
		Message: "configuration reloaded",
	}, metav1.CreateOptions{})
	require.NoError(f.t, err)
}

func (f *testFixture) ReceiveEntry() loki.Entry {
	f.t.Helper()
	select {
	case entry := <-f.receiver.Chan():
		return entry
	case <-time.After(time.Second):
		f.t.Fatal("timed out waiting for event")
		return loki.Entry{}
	}
}

type eventCache struct {
	cache.Cache
	informer cachetools.SharedIndexInformer
}

func (c *eventCache) GetInformer(context.Context, client.Object, ...cache.InformerGetOption) (cache.Informer, error) {
	return c.informer, nil
}

func (c *eventCache) Start(ctx context.Context) error {
	c.informer.Run(ctx.Done())
	return nil
}

func (c *eventCache) WaitForCacheSync(ctx context.Context) bool {
	return cachetools.WaitForCacheSync(ctx.Done(), c.informer.HasSynced)
}

package rules

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promListers "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/mimir/client"
)

type fakeMimirClient struct {
	rulesMut sync.RWMutex
	rules    map[string][]client.MimirRuleGroup
}

var _ client.RulerInterface = &fakeMimirClient{}

func newFakeMimirClient() *fakeMimirClient {
	return &fakeMimirClient{
		rules: make(map[string][]client.MimirRuleGroup),
	}
}

func (m *fakeMimirClient) CreateRuleGroup(_ context.Context, namespace string, rule client.MimirRuleGroup) error {
	m.rulesMut.Lock()
	defer m.rulesMut.Unlock()
	m.deleteLocked(namespace, rule.Name)
	m.rules[namespace] = append(m.rules[namespace], rule)
	return nil
}

func (m *fakeMimirClient) DeleteRuleGroup(_ context.Context, namespace, group string) error {
	m.rulesMut.Lock()
	defer m.rulesMut.Unlock()
	m.deleteLocked(namespace, group)
	return nil
}

func (m *fakeMimirClient) deleteLocked(namespace, group string) {
	for ns, v := range m.rules {
		if namespace != "" && namespace != ns {
			continue
		}
		for i, g := range v {
			if g.Name == group {
				m.rules[ns] = append(m.rules[ns][:i], m.rules[ns][i+1:]...)

				if len(m.rules[ns]) == 0 {
					delete(m.rules, ns)
				}

				return
			}
		}
	}
}

func (m *fakeMimirClient) ListRules(_ context.Context, namespace string) (map[string][]client.MimirRuleGroup, error) {
	m.rulesMut.RLock()
	defer m.rulesMut.RUnlock()
	output := make(map[string][]client.MimirRuleGroup)
	for ns, v := range m.rules {
		if namespace != "" && namespace != ns {
			continue
		}
		output[ns] = v
	}
	return output, nil
}

func TestEventLoop(t *testing.T) {
	nsIndexer := testNamespaceIndexer()
	ruleIndexer := testRuleIndexer()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace",
			UID:  types.UID("33f8860c-bd06-4c0d-a0b1-a114d6b9937b"),
		},
	}

	rule := &v1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
			UID:       types.UID("64aab764-c95e-4ee9-a932-cd63ba57e6cf"),
		},
		Spec: v1.PrometheusRuleSpec{
			Groups: []v1.RuleGroup{
				{
					Name: "group",
					Rules: []v1.Rule{
						{
							Alert: "alert",
							Expr:  intstr.FromString("expr"),
						},
					},
				},
			},
		},
	}

	processor := &eventProcessor{
		queue:             workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[kubernetes.Event]()),
		stopChan:          make(chan struct{}),
		health:            &fakeHealthReporter{},
		mimirClient:       newFakeMimirClient(),
		namespaceLister:   coreListers.NewNamespaceLister(nsIndexer),
		ruleLister:        promListers.NewPrometheusRuleLister(ruleIndexer),
		namespaceSelector: labels.Everything(),
		ruleSelector:      labels.Everything(),
		namespacePrefix:   "alloy",
		metrics:           newMetrics(),
		logger:            log.With(log.NewLogfmtLogger(os.Stdout), "ts", log.DefaultTimestampUTC),
	}

	ctx := t.Context()

	// Do an initial sync of the Mimir ruler state before starting the event processing loop.
	require.NoError(t, processor.syncMimir(ctx))
	go processor.run(ctx)
	defer processor.stop()

	eventHandler := kubernetes.NewQueuedEventHandler(processor.logger, processor.queue)

	// Add a namespace and rule to kubernetes
	require.NoError(t, nsIndexer.Add(ns))
	require.NoError(t, ruleIndexer.Add(rule))
	eventHandler.OnAdd(rule, false)

	// Wait for the rule to be added to mimir
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		rules, err := processor.mimirClient.ListRules(ctx, "")
		assert.NoError(c, err)
		assert.Len(c, rules, 1)
	}, time.Second, 10*time.Millisecond)

	// Update the rule in kubernetes
	rule.Spec.Groups[0].Rules = append(rule.Spec.Groups[0].Rules, v1.Rule{
		Alert: "alert2",
		Expr:  intstr.FromString("expr2"),
	})
	require.NoError(t, ruleIndexer.Update(rule))
	eventHandler.OnUpdate(rule, rule)

	// Wait for the rule to be updated in mimir
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		allRules, err := processor.mimirClient.ListRules(ctx, "")
		assert.NoError(c, err)
		rules := allRules[mimirNamespaceForRuleCRD("alloy", rule)][0].Rules
		assert.Len(c, rules, 2)
	}, time.Second, 10*time.Millisecond)

	// Remove the rule from kubernetes
	require.NoError(t, ruleIndexer.Delete(rule))
	eventHandler.OnDelete(rule)

	// Wait for the rule to be removed from mimir
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		rules, err := processor.mimirClient.ListRules(ctx, "")
		assert.NoError(c, err)
		assert.Empty(c, rules)
	}, time.Second, 10*time.Millisecond)
}

func TestAdditionalLabels(t *testing.T) {
	nsIndexer := testNamespaceIndexer()
	ruleIndexer := testRuleIndexer()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace",
			UID:  types.UID("33f8860c-bd06-4c0d-a0b1-a114d6b9937b"),
		},
	}

	rule := &v1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
			UID:       types.UID("64aab764-c95e-4ee9-a932-cd63ba57e6cf"),
		},
		Spec: v1.PrometheusRuleSpec{
			Groups: []v1.RuleGroup{
				{
					Name: "group1",
					Rules: []v1.Rule{
						{
							Alert: "alert1",
							Expr:  intstr.FromString("expr1"),
						},
						{
							Alert: "alert2",
							Expr:  intstr.FromString("expr2"),
							Labels: map[string]string{
								// This label should get overridden.
								"foo": "lalalala",
							},
						},
					},
				},
			},
		},
	}

	processor := &eventProcessor{
		queue:             workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[kubernetes.Event]()),
		stopChan:          make(chan struct{}),
		health:            &fakeHealthReporter{},
		mimirClient:       newFakeMimirClient(),
		namespaceLister:   coreListers.NewNamespaceLister(nsIndexer),
		ruleLister:        promListers.NewPrometheusRuleLister(ruleIndexer),
		namespaceSelector: labels.Everything(),
		ruleSelector:      labels.Everything(),
		namespacePrefix:   "alloy",
		metrics:           newMetrics(),
		logger:            log.With(log.NewLogfmtLogger(os.Stdout), "ts", log.DefaultTimestampUTC),
		externalLabels:    map[string]string{"foo": "bar"},
	}

	ctx := t.Context()

	// Do an initial sync of the Mimir ruler state before starting the event processing loop.
	require.NoError(t, processor.syncMimir(ctx))
	go processor.run(ctx)
	defer processor.stop()

	eventHandler := kubernetes.NewQueuedEventHandler(processor.logger, processor.queue)

	// Add a namespace and rule to kubernetes
	require.NoError(t, nsIndexer.Add(ns))
	require.NoError(t, ruleIndexer.Add(rule))
	eventHandler.OnAdd(rule, false)

	// Wait for the rule to be added to mimir
	rules := map[string][]client.MimirRuleGroup{}
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var err error
		rules, err = processor.mimirClient.ListRules(ctx, "")
		assert.NoError(c, err)
		assert.Len(c, rules, 1)
	}, 3*time.Second, 10*time.Millisecond)

	// The map of rules has only one element.
	for ruleName, rule := range rules {
		require.Equal(t, "alloy/namespace/name/64aab764-c95e-4ee9-a932-cd63ba57e6cf", ruleName)

		ruleBuf, err := yaml.Marshal(rule)
		require.NoError(t, err)

		expectedRule := `- name: group1
  rules:
  - alert: alert1
    expr: expr1
    labels:
      foo: bar
  - alert: alert2
    expr: expr2
    labels:
      foo: bar
`
		require.YAMLEq(t, expectedRule, string(ruleBuf))
	}
}

func TestExtraQueryMatchers(t *testing.T) {
	nsIndexer := testNamespaceIndexer()
	ruleIndexer := testRuleIndexer()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace",
			UID:  types.UID("33f8860c-bd06-4c0d-a0b1-a114d6b9937b"),
		},
	}

	rule := &v1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
			UID:       types.UID("64aab764-c95e-4ee9-a932-cd63ba57e6cf"),
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Spec: v1.PrometheusRuleSpec{
			Groups: []v1.RuleGroup{
				{
					Name: "group1",
					Rules: []v1.Rule{
						{
							Record: "record_rule_1",
							Expr:   intstr.FromString("sum by (namespace) (rate(success{\"job\"=\"bad\"}[10m]) / rate(total{}[10m]))"),
						},
						{
							Alert: "alert_1",
							Expr:  intstr.FromString("sum by (namespace) (rate(success{\"foo\"=\"bar\"}[10m]) / (rate(success{\"job\"!~\"bad\"}[10m]) + rate(failure[10m]))) < 0.995"),
						},
					},
				},
			},
		},
	}

	processor := &eventProcessor{
		queue:             workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[kubernetes.Event]()),
		stopChan:          make(chan struct{}),
		health:            &fakeHealthReporter{},
		mimirClient:       newFakeMimirClient(),
		namespaceLister:   coreListers.NewNamespaceLister(nsIndexer),
		ruleLister:        promListers.NewPrometheusRuleLister(ruleIndexer),
		namespaceSelector: labels.Everything(),
		ruleSelector:      labels.Everything(),
		namespacePrefix:   "alloy",
		metrics:           newMetrics(),
		logger:            log.With(log.NewLogfmtLogger(os.Stdout), "ts", log.DefaultTimestampUTC),
		extraQueryMatchers: &ExtraQueryMatchers{Matchers: []Matcher{
			{
				Name:      "cluster",
				MatchType: "=~",
				Value:     "prod-.*",
			},
			{
				Name:      "job",
				MatchType: "=",
				Value:     "good",
			},
			{
				Name:           "label",
				MatchType:      "=",
				ValueFromLabel: "foo",
			},
		}},
	}

	ctx := t.Context()

	// Do an initial sync of the Mimir ruler state before starting the event processing loop.
	require.NoError(t, processor.syncMimir(ctx))
	go processor.run(ctx)
	defer processor.stop()

	eventHandler := kubernetes.NewQueuedEventHandler(processor.logger, processor.queue)

	// Add a namespace and rule to kubernetes
	require.NoError(t, nsIndexer.Add(ns))
	require.NoError(t, ruleIndexer.Add(rule))
	eventHandler.OnAdd(rule, false)

	// Wait for the rule to be added to mimir
	rules := map[string][]client.MimirRuleGroup{}
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var err error
		rules, err = processor.mimirClient.ListRules(ctx, "")
		assert.NoError(c, err)
		assert.Len(c, rules, 1)
	}, 10*time.Second, 10*time.Millisecond)

	// The map of rules has only one element.
	for ruleName, rule := range rules {
		require.Equal(t, "alloy/namespace/name/64aab764-c95e-4ee9-a932-cd63ba57e6cf", ruleName)

		ruleBuf, err := yaml.Marshal(rule)
		require.NoError(t, err)

		expectedRule := `- name: group1
  rules:
    - expr: "sum by (namespace) (rate(success{cluster=~\"prod-.*\",job=\"good\",label=\"bar\"}[10m]) / rate(total{cluster=~\"prod-.*\",job=\"good\",label=\"bar\"}[10m]))"
      record: record_rule_1
    - alert: alert_1
      expr: "sum by (namespace) (rate(success{cluster=~\"prod-.*\",foo=\"bar\",job=\"good\",label=\"bar\"}[10m]) / (rate(success{cluster=~\"prod-.*\",job=\"good\",label=\"bar\"}[10m]) + rate(failure{cluster=~\"prod-.*\",job=\"good\",label=\"bar\"}[10m]))) < 0.995"
`
		require.YAMLEq(t, expectedRule, string(ruleBuf))
	}
}

func TestSourceTenants(t *testing.T) {
	nsIndexer := testNamespaceIndexer()
	ruleIndexer := testRuleIndexer()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace",
			UID:  types.UID("33f8860c-bd06-4c0d-a0b1-a114d6b9937b"),
		},
	}

	rule := &v1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
			UID:       types.UID("64aab764-c95e-4ee9-a932-cd63ba57e6cf"),
			Annotations: map[string]string{
				AnnotationsSourceTenants: "tenant1,tenant2",
			},
		},
		Spec: v1.PrometheusRuleSpec{
			Groups: []v1.RuleGroup{
				{
					Name: "group1",
					Rules: []v1.Rule{
						{
							Alert: "alert1",
							Expr:  intstr.FromString("expr1"),
						},
						{
							Alert: "alert2",
							Expr:  intstr.FromString("expr2"),
							Labels: map[string]string{
								// This label should get overridden.
								"foo": "lalalala",
							},
						},
					},
				},
			},
		},
	}

	processor := &eventProcessor{
		queue:             workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[kubernetes.Event]()),
		stopChan:          make(chan struct{}),
		health:            &fakeHealthReporter{},
		mimirClient:       newFakeMimirClient(),
		namespaceLister:   coreListers.NewNamespaceLister(nsIndexer),
		ruleLister:        promListers.NewPrometheusRuleLister(ruleIndexer),
		namespaceSelector: labels.Everything(),
		ruleSelector:      labels.Everything(),
		namespacePrefix:   "alloy",
		metrics:           newMetrics(),
		logger:            log.With(log.NewLogfmtLogger(os.Stdout), "ts", log.DefaultTimestampUTC),
	}

	ctx := t.Context()

	// Do an initial sync of the Mimir ruler state before starting the event processing loop.
	require.NoError(t, processor.syncMimir(ctx))
	go processor.run(ctx)
	defer processor.stop()

	eventHandler := kubernetes.NewQueuedEventHandler(processor.logger, processor.queue)

	// Add a namespace and rule to kubernetes
	require.NoError(t, nsIndexer.Add(ns))
	require.NoError(t, ruleIndexer.Add(rule))
	eventHandler.OnAdd(rule, false)

	// Wait for the rule to be added to mimir
	rules := map[string][]client.MimirRuleGroup{}
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var err error
		rules, err = processor.mimirClient.ListRules(ctx, "")
		assert.NoError(c, err)
		assert.Len(c, rules, 1)
	}, 10*time.Second, 10*time.Millisecond)

	// The map of rules has only one element.
	for ruleName, rule := range rules {
		require.Equal(t, "alloy/namespace/name/64aab764-c95e-4ee9-a932-cd63ba57e6cf", ruleName)

		ruleBuf, err := yaml.Marshal(rule)
		require.NoError(t, err)

		expectedRule := `- name: group1
  rules:
  - alert: alert1
    expr: expr1
  - alert: alert2
    expr: expr2
    labels:
      foo: lalalala
  source_tenants: ["tenant1","tenant2"]
`
		require.YAMLEq(t, expectedRule, string(ruleBuf))
	}
}

func testRuleIndexer() cache.Indexer {
	ruleIndexer := cache.NewIndexer(
		cache.DeletionHandlingMetaNamespaceKeyFunc,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	return ruleIndexer
}

func testNamespaceIndexer() cache.Indexer {
	nsIndexer := cache.NewIndexer(
		cache.DeletionHandlingMetaNamespaceKeyFunc,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	return nsIndexer
}

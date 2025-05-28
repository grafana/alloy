package alerts

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/go-kit/log"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	promListers_v1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1alpha1"
	"github.com/prometheus/alertmanager/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/grafana/alloy/internal/component/common/kubernetes"
	mimirClient "github.com/grafana/alloy/internal/mimir/client"
	"github.com/grafana/alloy/syntax"
)

type fakeMimirClient struct {
	rulesMut sync.RWMutex
	rules    map[string][]mimirClient.MimirRuleGroup

	alertMgrConfigsMut sync.RWMutex
	alertMgrConfig     config.Config
}

var _ mimirClient.Interface = &fakeMimirClient{}

func newFakeMimirClient() *fakeMimirClient {
	return &fakeMimirClient{
		rules: make(map[string][]mimirClient.MimirRuleGroup),
	}
}

func (m *fakeMimirClient) CreateRuleGroup(_ context.Context, namespace string, rule mimirClient.MimirRuleGroup) error {
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

func (m *fakeMimirClient) ListRules(_ context.Context, namespace string) (map[string][]mimirClient.MimirRuleGroup, error) {
	m.rulesMut.RLock()
	defer m.rulesMut.RUnlock()
	output := make(map[string][]mimirClient.MimirRuleGroup)
	for ns, v := range m.rules {
		if namespace != "" && namespace != ns {
			continue
		}
		output[ns] = v
	}
	return output, nil
}

func (m *fakeMimirClient) CreateAlertmanagerConfigs(ctx context.Context, conf config.Config, templateFiles map[string]string) error {
	m.alertMgrConfigsMut.Lock()
	defer m.alertMgrConfigsMut.Unlock()
	m.alertMgrConfig = conf
	// TODO: Check templateFiles
	return nil
}

func (m *fakeMimirClient) getAlertmanagerConfig() config.Config {
	m.alertMgrConfigsMut.RLock()
	defer m.alertMgrConfigsMut.RUnlock()
	return m.alertMgrConfig
}

func convertToAlertmanagerType(t *testing.T, alertmanagerConf string) config.Config {
	var res config.Config
	err := yaml.Unmarshal([]byte(alertmanagerConf), &res)
	assert.NoError(t, err)
	return res
}

func TestEventLoop(t *testing.T) {
	emptyCfg := "templates: []\n"

	baseCfg := `
global:
  resolve_timeout: 5m
  http_config:
    follow_redirects: true
    enable_http2: true
  smtp_hello: localhost
  smtp_require_tls: true
route:
  receiver: "null"
  continue: false
receivers:
- name: "null"
templates: []`

	amConfCrd1 := `apiVersion: monitoring.coreos.com/v1alpha1
kind: AlertmanagerConfig
metadata:
  name: alertmgr-config1
  namespace: %s
  labels: %s
spec:
  route:
    receiver: "null"
    routes:
    - receiver: myamc
      continue: true
  receivers:
  - name: "null"
  - name: myamc
    webhookConfigs:
    - url: http://test.url
      httpConfig:
        followRedirects: true`

	amConfCrd1_mynamespace := fmt.Sprintf(amConfCrd1, "mynamespace", "")
	amConfCrd1_mynamespace_alloyLabel := fmt.Sprintf(amConfCrd1, "mynamespace", `{alloy: "yes"}`)

	amConfCrd2 := `apiVersion: monitoring.coreos.com/v1alpha1
kind: AlertmanagerConfig
metadata:
  name: alertmgr-config2
  namespace: %s
spec:
  route:
    receiver: "null"
    routes:
    - receiver: 'database-pager'
      group_wait: 10s
      matchers:
      - name: service
        value: webapp
  receivers:
  - name: "null"
  - name: "database-pager"`

	amConfCrd2_mynamespace := fmt.Sprintf(amConfCrd2, "mynamespace")
	amConfCrd2_myOtherNamespace := fmt.Sprintf(amConfCrd2, "myOtherNamespace")

	final_amConf_1 := `global:
  resolve_timeout: 5m
  http_config:
    follow_redirects: true
    enable_http2: true
  smtp_hello: localhost
  smtp_require_tls: true
route:
  receiver: "null"
  continue: false
  routes:
  - receiver: mynamespace/alertmgr-config1/null
    matchers:
    - namespace="mynamespace"
    continue: true
    routes:
    - receiver: mynamespace/alertmgr-config1/myamc
      continue: true
receivers:
- name: "null"
- name: mynamespace/alertmgr-config1/null
- name: mynamespace/alertmgr-config1/myamc
  webhook_configs:
  - send_resolved: false
    http_config:
      follow_redirects: true
      enable_http2: true
    url: <secret>
    url_file: ""
    max_alerts: 0
templates: []`

	final_amConf_1_and_2 := `global:
  resolve_timeout: 5m
  http_config:
    follow_redirects: true
    enable_http2: true
  smtp_hello: localhost
  smtp_require_tls: true
route:
  receiver: "null"
  continue: false
  routes:
  - receiver: mynamespace/alertmgr-config1/null
    matchers:
    - namespace="mynamespace"
    continue: true
    routes:
    - receiver: mynamespace/alertmgr-config1/myamc
      continue: true
  - receiver: mynamespace/alertmgr-config2/null
    matchers:
    - namespace="mynamespace"
    continue: true
    routes:
    - receiver: mynamespace/alertmgr-config2/database-pager
      match:
        service: webapp
      continue: false
receivers:
- name: "null"
- name: mynamespace/alertmgr-config1/null
- name: mynamespace/alertmgr-config1/myamc
  webhook_configs:
  - send_resolved: false
    http_config:
      follow_redirects: true
      enable_http2: true
    url: <secret>
    url_file: ""
    max_alerts: 0
- name: mynamespace/alertmgr-config2/null
- name: mynamespace/alertmgr-config2/database-pager
templates: []`

	tests := []struct {
		name              string
		baseCfgStr        string
		matcherStrategy   monitoringv1.AlertmanagerConfigMatcherStrategy
		amConfig          []string
		namespaceSelector string
		cfgSelector       string
		want              string
		wantErr           bool
	}{
		{
			name:       "Np AlertmanagerConfig CRDs",
			baseCfgStr: baseCfg,
			amConfig:   []string{},
			matcherStrategy: monitoringv1.AlertmanagerConfigMatcherStrategy{
				Type: "OnNamespace",
			},
			want:    baseCfg,
			wantErr: false,
		},
		{
			name:       "2 AlertmanagerConfig CRDs",
			baseCfgStr: baseCfg,
			amConfig: []string{
				amConfCrd1_mynamespace,
				amConfCrd2_mynamespace,
			},
			matcherStrategy: monitoringv1.AlertmanagerConfigMatcherStrategy{
				Type: "OnNamespace",
			},
			want:    final_amConf_1_and_2,
			wantErr: false,
		},
		{
			name:       "2 AlertmanagerConfig CRDs and no base config",
			baseCfgStr: "",
			amConfig: []string{
				amConfCrd1_mynamespace,
				amConfCrd2_mynamespace,
			},
			matcherStrategy: monitoringv1.AlertmanagerConfigMatcherStrategy{
				Type: "OnNamespace",
			},
			want:    emptyCfg,
			wantErr: false,
		},
		{
			name:       "2 AlertmanagerConfig CRDs - 1 in another namespace",
			baseCfgStr: baseCfg,
			amConfig: []string{
				amConfCrd1_mynamespace,
				amConfCrd2_myOtherNamespace,
			},
			matcherStrategy: monitoringv1.AlertmanagerConfigMatcherStrategy{
				Type: "OnNamespace",
			},
			want:    final_amConf_1,
			wantErr: false,
		},
		{
			name:       "With selectors",
			baseCfgStr: baseCfg,
			amConfig: []string{
				amConfCrd1_mynamespace_alloyLabel,
				amConfCrd2_mynamespace,
			},
			namespaceSelector: `
		match_labels = {
			alloy = "yes",
		}`,
			cfgSelector: `
		match_labels = {
			alloy = "yes",
		}`,
			matcherStrategy: monitoringv1.AlertmanagerConfigMatcherStrategy{
				Type: "OnNamespace",
			},
			want:    final_amConf_1,
			wantErr: false,
		},
		{
			name: "1 invalid AlertmanagerConfig CRD",
			// TODO: Check if the logs contain an error like this:
			// level=error ts=2025-05-01T16:14:01.724378Z msg="got an invalid AlertmanagerConfig CRD from Kubernetes" namespace=mynamespace name=alertmgr-config3 err="route[0]: json: cannot unmarshal string into Go struct field Route.matchers of type v1alpha1.Matcher"
			baseCfgStr: baseCfg,
			amConfig: []string{
				`apiVersion: monitoring.coreos.com/v1alpha1
kind: AlertmanagerConfig
metadata:
  name: alertmgr-config3
  # This config should be in a namespace which
  # mimir.alerts.kubernetes is not watching.
  namespace: mynamespace
  labels:
    alloy: "yes"
spec:
  route:
    receiver: "null"
    routes:
    - receiver: team-X-mails
      matchers:
      - service=~"foo1|foo2|baz"
  receivers:
  - name: "null"
  - name: "team-X-mails"
    emailConfigs:
    - to: 'team-X+alerts@example.org'`,
			},
			matcherStrategy: monitoringv1.AlertmanagerConfigMatcherStrategy{
				Type: "OnNamespace",
			},
			want:    baseCfg,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nsIndexer := testNamespaceIndexer()
			ruleIndexer := testRuleIndexer()

			mimirClient := newFakeMimirClient()

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mynamespace",
					UID:  types.UID("33f8860c-bd06-4c0d-a0b1-a114d6b9937b"),
					Labels: map[string]string{
						"alloy": "yes",
					},
				},
			}

			namespaceSelector := labels.Everything()
			if tt.namespaceSelector != "" {
				namespaceSelector = convertStringToSelector(t, tt.namespaceSelector)
			}

			cfgSelector := labels.Everything()
			if tt.cfgSelector != "" {
				cfgSelector = convertStringToSelector(t, tt.cfgSelector)
			}

			processor := &eventProcessor{
				queue:             workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[kubernetes.Event]()),
				stopChan:          make(chan struct{}),
				health:            &fakeHealthReporter{},
				mimirClient:       mimirClient,
				baseCfg:           convertToAlertmanagerType(t, tt.baseCfgStr),
				namespaceLister:   coreListers.NewNamespaceLister(nsIndexer),
				cfgLister:         promListers_v1alpha1.NewAlertmanagerConfigLister(ruleIndexer),
				namespaceSelector: namespaceSelector,
				cfgSelector:       cfgSelector,
				metrics:           newMetrics(),
				logger:            log.With(log.NewLogfmtLogger(os.Stdout), "ts", log.DefaultTimestampUTC),
			}

			go processor.run(t.Context())
			defer processor.stop()

			eventHandler := kubernetes.NewQueuedEventHandler(processor.logger, processor.queue)

			// Add a namespace to kubernetes
			require.NoError(t, nsIndexer.Add(ns))

			// Add a AlertmanagerConfigs to kubernetes
			for _, amConfigStr := range tt.amConfig {
				var amConfig monitoringv1alpha1.AlertmanagerConfig
				err := yaml.Unmarshal([]byte(amConfigStr), &amConfig)
				assert.NoError(t, err)

				require.NoError(t, ruleIndexer.Add(&amConfig))
				eventHandler.OnAdd(&amConfig, false)
			}

			// Wait for the configs to be added to mimir
			require.EventuallyWithT(t, func(c *assert.CollectT) {
				actual := mimirClient.getAlertmanagerConfig().String()
				assert.YAMLEq(t, tt.want, actual, "want", tt.want, "actual", actual)
			}, 2*time.Second, 10*time.Millisecond)
			// TODO: Check the component health
		})
	}
}

func convertStringToSelector(t *testing.T, labelSelector string) labels.Selector {
	var alloySelector kubernetes.LabelSelector
	err := syntax.Unmarshal([]byte(labelSelector), &alloySelector)
	assert.NoError(t, err)

	selector, err := kubernetes.ConvertSelectorToListOptions(alloySelector)
	assert.NoError(t, err)
	return selector
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

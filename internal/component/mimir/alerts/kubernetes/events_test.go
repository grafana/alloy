package alerts

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/mimir/alertmanager"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/prometheus-operator/prometheus-operator/pkg/assets"
	promListers_v1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/grafana/alloy/internal/component/common/kubernetes"
	mimirClient "github.com/grafana/alloy/internal/mimir/client"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/internal/util/syncbuffer"
	"github.com/grafana/alloy/syntax"
)

type fakeMimirClient struct {
	alertMgrConfigsMut sync.RWMutex
	alertMgrConfig     alertmanager.Config
	templateFiles      map[string]string
}

var _ mimirClient.AlertmanagerInterface = &fakeMimirClient{}

func newFakeMimirClient() *fakeMimirClient {
	return &fakeMimirClient{}
}

func (m *fakeMimirClient) CreateAlertmanagerConfigs(ctx context.Context, conf *alertmanager.Config, templateFiles map[string]string) error {
	m.alertMgrConfigsMut.Lock()
	defer m.alertMgrConfigsMut.Unlock()
	// These are just shallow copies, but it should be sufficient.
	m.alertMgrConfig = *conf
	m.templateFiles = templateFiles
	return nil
}

func (m *fakeMimirClient) getAlertmanagerConfig() alertmanager.Config {
	m.alertMgrConfigsMut.RLock()
	defer m.alertMgrConfigsMut.RUnlock()
	return m.alertMgrConfig
}

func convertToAlertmanagerType(t *testing.T, alertmanagerConf string) alertmanager.Config {
	cfg, err := alertmanager.Unmarshal([]byte(alertmanagerConf))
	assert.NoError(t, err)
	return *cfg
}

// createTestLoggerWithBuffer creates a logger that writes to a thread-safe buffer for testing
func createTestLoggerWithBuffer(t *testing.T) (log.Logger, *syncbuffer.Buffer) {
	t.Helper()

	logBuffer := &syncbuffer.Buffer{}
	logger := log.NewLogfmtLogger(log.NewSyncWriter(logBuffer))
	logger = log.WithPrefix(logger,
		"test", t.Name(),
		"ts", log.DefaultTimestampUTC,
	)

	return logger, logBuffer
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
        followRedirects: true
    slackConfigs:
    - apiURL:
        key: api-url
        name: "s-receiver-api-url"
      actions:
      - type: type
        text: text
        name: my-action
        confirm:
          text: text
      fields:
      - title: title
        value: value`

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
      groupWait: 10s
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
  slack_configs:
  - api_url: https://val1.com
    fields:
    - title: title
      value: value
    actions:
    - type: type
      text: text
      name: my-action
      confirm:
        text: text
  webhook_configs:
  - http_config:
      follow_redirects: true
    url: http://test.url
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
      matchers:
      - "service=\"webapp\""
      group_wait: 10s
receivers:
- name: "null"
- name: mynamespace/alertmgr-config1/null
- name: mynamespace/alertmgr-config1/myamc
  slack_configs:
  - api_url: https://val1.com
    fields:
    - title: title
      value: value
    actions:
    - type: type
      text: text
      name: my-action
      confirm:
        text: text
  webhook_configs:
  - http_config:
      follow_redirects: true
    url: http://test.url
- name: mynamespace/alertmgr-config2/null
- name: mynamespace/alertmgr-config2/database-pager
templates: []`

	final_amConf_1_and_2_other_namespace := `global:
  resolve_timeout: 5m
  http_config:
    follow_redirects: true
    enable_http2: true
  smtp_hello: localhost
  smtp_require_tls: true
route:
  receiver: "null"
  routes:
  - receiver: myOtherNamespace/alertmgr-config2/null
    matchers:
    - namespace="myOtherNamespace"
    continue: true
    routes:
    - receiver: myOtherNamespace/alertmgr-config2/database-pager
      matchers:
      - "service=\"webapp\""
      group_wait: 10s
  - receiver: mynamespace/alertmgr-config1/null
    matchers:
    - namespace="mynamespace"
    continue: true
    routes:
    - receiver: mynamespace/alertmgr-config1/myamc
      continue: true
receivers:
- name: "null"
- name: myOtherNamespace/alertmgr-config2/null
- name: myOtherNamespace/alertmgr-config2/database-pager
- name: mynamespace/alertmgr-config1/null
- name: mynamespace/alertmgr-config1/myamc
  slack_configs:
  - api_url: https://val1.com
    fields:
    - title: title
      value: value
    actions:
    - type: type
      text: text
      name: my-action
      confirm:
        text: text
  webhook_configs:
  - http_config:
      follow_redirects: true
    url: http://test.url
templates: []`

	final_amConf_1_and_2_matcher_none := `global:
  resolve_timeout: 5m
  http_config:
    follow_redirects: true
    enable_http2: true
  smtp_hello: localhost
  smtp_require_tls: true
route:
  receiver: "null"
  routes:
  - receiver: myOtherNamespace/alertmgr-config2/null
    continue: true
    routes:
    - receiver: myOtherNamespace/alertmgr-config2/database-pager
      matchers:
      - "service=\"webapp\""
      group_wait: 10s
  - receiver: mynamespace/alertmgr-config1/null
    continue: true
    routes:
    - receiver: mynamespace/alertmgr-config1/myamc
      continue: true
receivers:
- name: "null"
- name: myOtherNamespace/alertmgr-config2/null
- name: myOtherNamespace/alertmgr-config2/database-pager
- name: mynamespace/alertmgr-config1/null
- name: mynamespace/alertmgr-config1/myamc
  slack_configs:
  - api_url: https://val1.com
    fields:
    - title: title
      value: value
    actions:
    - type: type
      text: text
      name: my-action
      confirm:
        text: text
  webhook_configs:
  - http_config:
      follow_redirects: true
    url: http://test.url
templates: []`

	final_amConf_1_and_2_matcher_on_namespace_except_for_alertmanager_namespace := `global:
  resolve_timeout: 5m
  http_config:
    follow_redirects: true
    enable_http2: true
  smtp_hello: localhost
  smtp_require_tls: true
route:
  receiver: "null"
  routes:
  - receiver: myOtherNamespace/alertmgr-config2/null
    matchers:
    - namespace="myOtherNamespace"
    continue: true
    routes:
    - receiver: myOtherNamespace/alertmgr-config2/database-pager
      matchers:
      - "service=\"webapp\""
      group_wait: 10s
  - receiver: mynamespace/alertmgr-config1/null
    continue: true
    routes:
    - receiver: mynamespace/alertmgr-config1/myamc
      continue: true
receivers:
- name: "null"
- name: myOtherNamespace/alertmgr-config2/null
- name: myOtherNamespace/alertmgr-config2/database-pager
- name: mynamespace/alertmgr-config1/null
- name: mynamespace/alertmgr-config1/myamc
  slack_configs:
  - api_url: https://val1.com
    fields:
    - title: title
      value: value
    actions:
    - type: type
      text: text
      name: my-action
      confirm:
        text: text
  webhook_configs:
  - http_config:
      follow_redirects: true
    url: http://test.url
templates: []`

	tests := []struct {
		name              string
		baseCfgStr        string
		matcherStrategy   monitoringv1.AlertmanagerConfigMatcherStrategy
		amConfig          []string
		namespaceSelector string
		cfgSelector       string
		want              string
		expectLogMessage  string // Expected error log message to check for
	}{
		{
			name:       "No AlertmanagerConfig CRDs",
			baseCfgStr: baseCfg,
			amConfig:   []string{},
			matcherStrategy: monitoringv1.AlertmanagerConfigMatcherStrategy{
				Type: "OnNamespace",
			},
			want: baseCfg,
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
			want: final_amConf_1_and_2,
		},
		{
			name:       "2 AlertmanagerConfig CRDs - with NoneConfigMatcherStrategy",
			baseCfgStr: baseCfg,
			amConfig: []string{
				amConfCrd1_mynamespace,
				amConfCrd2_myOtherNamespace,
			},
			matcherStrategy: monitoringv1.AlertmanagerConfigMatcherStrategy{
				Type: "None",
			},
			want: final_amConf_1_and_2_matcher_none,
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
			want:             emptyCfg,
			expectLogMessage: "failed to initialize from global AlertmangerConfig",
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
			want: final_amConf_1_and_2_other_namespace,
		},
		{
			name:       "2 AlertmanagerConfig CRDs - with OnNamespaceExceptForAlertmanagerNamespace",
			baseCfgStr: baseCfg,
			amConfig: []string{
				amConfCrd1_mynamespace,
				amConfCrd2_myOtherNamespace,
			},
			matcherStrategy: monitoringv1.AlertmanagerConfigMatcherStrategy{
				Type: "OnNamespaceExceptForAlertmanagerNamespace",
			},
			want: final_amConf_1_and_2_matcher_on_namespace_except_for_alertmanager_namespace,
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
			want: final_amConf_1,
		},
		{
			name:       "1 invalid AlertmanagerConfig CRD",
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
			want:             baseCfg,
			expectLogMessage: "got an invalid AlertmanagerConfig CRD from Kubernetes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nsIndexer := testNamespaceIndexer()
			amConfigsIndexer := testAmConfigsIndexer()

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
			ns_other := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myOtherNamespace",
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

			// Create logger with buffer to capture log messages
			var testLogger log.Logger
			var logBuffer *syncbuffer.Buffer
			if tt.expectLogMessage != "" {
				testLogger, logBuffer = createTestLoggerWithBuffer(t)
			} else {
				testLogger = util.TestLogger(t)
			}

			c := fake.NewSimpleClientset(
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "s-receiver-api-url",
						Namespace: "mynamespace",
					},
					Data: map[string][]byte{
						"api-url": []byte("https://val1.com"),
					},
				},
			)
			store := assets.NewStoreBuilder(c.CoreV1(), c.CoreV1())

			processor := &eventProcessor{
				queue:                 workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[kubernetes.Event]()),
				stopChan:              make(chan struct{}),
				health:                &fakeHealthReporter{},
				mimirClient:           mimirClient,
				baseCfg:               convertToAlertmanagerType(t, tt.baseCfgStr),
				namespaceLister:       coreListers.NewNamespaceLister(nsIndexer),
				cfgLister:             promListers_v1alpha1.NewAlertmanagerConfigLister(amConfigsIndexer),
				namespaceSelector:     namespaceSelector,
				matcherStrategy:       tt.matcherStrategy.Type,
				alertmanagerNamespace: ns.Name,
				cfgSelector:           cfgSelector,
				metrics:               newMetrics(),
				logger:                testLogger,
				storeBuilder:          store,
			}

			ctx := t.Context()

			// Do an initial sync of the Mimir ruler state before starting the event processing loop.
			processor.enqueueSyncMimir()

			go processor.run(ctx)
			defer processor.stop()

			eventHandler := kubernetes.NewQueuedEventHandler(processor.logger, processor.queue)

			// Add the namespaces to kubernetes
			require.NoError(t, nsIndexer.Add(ns))
			require.NoError(t, nsIndexer.Add(ns_other))

			// Add AlertmanagerConfigs to kubernetes
			for _, amConfigStr := range tt.amConfig {
				var amConfig monitoringv1alpha1.AlertmanagerConfig
				err := yaml.Unmarshal([]byte(amConfigStr), &amConfig)
				assert.NoError(t, err)

				require.NoError(t, amConfigsIndexer.Add(&amConfig))
				eventHandler.OnAdd(&amConfig, false)
			}

			// Wait for the configs to be added to mimir
			require.EventuallyWithT(t, func(c *assert.CollectT) {
				actual, err := mimirClient.getAlertmanagerConfig().String()
				require.NoError(c, err)
				require.YAMLEq(c, tt.want, actual, "want", tt.want, "actual", actual)
			}, 10*time.Second, 100*time.Millisecond)

			// Check for expected log messages if specified
			if tt.expectLogMessage != "" && logBuffer != nil {
				require.EventuallyWithT(t, func(c *assert.CollectT) {
					logOutput := logBuffer.String()
					require.Contains(c, logOutput, tt.expectLogMessage,
						"Expected log message not found in output: %s", logOutput)
				}, 5*time.Second, 100*time.Millisecond)
			}

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

func testAmConfigsIndexer() cache.Indexer {
	amConfigsIndexer := cache.NewIndexer(
		cache.DeletionHandlingMetaNamespaceKeyFunc,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	return amConfigsIndexer
}

func testNamespaceIndexer() cache.Indexer {
	nsIndexer := cache.NewIndexer(
		cache.DeletionHandlingMetaNamespaceKeyFunc,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	return nsIndexer
}

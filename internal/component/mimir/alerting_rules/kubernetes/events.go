package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus-operator/prometheus-operator/pkg/alertmanager"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promv1beta1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1beta1"
	promListers_v1beta1 "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1beta1"
	alertmgr_cfg "github.com/prometheus/alertmanager/config"
	promlabels "github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/rulefmt"
	"github.com/prometheus/prometheus/promql/parser"
	"k8s.io/apimachinery/pkg/labels"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/yaml" // Used for CRD compatibility instead of gopkg.in/yaml.v2

	"github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/mimir/client"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	eventTypeSyncMimir kubernetes.EventType = "sync-mimir"
)

type eventProcessor struct {
	queue    workqueue.RateLimitingInterface
	stopChan chan struct{}
	health   healthReporter

	mimirClient        client.Interface
	namespaceLister    coreListers.NamespaceLister
	ruleLister         promListers_v1beta1.AlertmanagerConfigLister
	namespaceSelector  labels.Selector
	ruleSelector       labels.Selector
	namespacePrefix    string
	externalLabels     map[string]string
	extraQueryMatchers *ExtraQueryMatchers

	configBuilder alertmanager.ConfigBuilder

	metrics *metrics
	logger  log.Logger

	currentState    kubernetes.AlertManagerConfigsByNamespace
	currentStateMtx sync.RWMutex
}

// run processes events added to the queue until the queue is shutdown.
func (e *eventProcessor) run(ctx context.Context) {
	for {
		eventInterface, shutdown := e.queue.Get()
		if shutdown {
			level.Info(e.logger).Log("msg", "shutting down event loop")
			return
		}

		evt := eventInterface.(kubernetes.Event)
		e.metrics.eventsTotal.WithLabelValues(string(evt.Typ)).Inc()
		err := e.processEvent(ctx, evt)

		if err != nil {
			retries := e.queue.NumRequeues(evt)
			if retries < 5 && client.IsRecoverable(err) {
				e.metrics.eventsRetried.WithLabelValues(string(evt.Typ)).Inc()
				e.queue.AddRateLimited(evt)
				level.Error(e.logger).Log(
					"msg", "failed to process event, will retry",
					"retries", fmt.Sprintf("%d/5", retries),
					"err", err,
				)
				continue
			} else {
				e.metrics.eventsFailed.WithLabelValues(string(evt.Typ)).Inc()
				level.Error(e.logger).Log(
					"msg", "failed to process event, unrecoverable error or max retries exceeded",
					"retries", fmt.Sprintf("%d/5", retries),
					"err", err,
				)
				e.health.reportUnhealthy(err)
			}
		} else {
			e.health.reportHealthy()
		}

		e.queue.Forget(evt)
	}
}

// stop stops adding new Kubernetes events to the queue and blocks until all existing
// events have been processed by the run loop.
func (e *eventProcessor) stop() {
	close(e.stopChan)
	// Because this method blocks until the queue is empty, it's important that we don't
	// stop the run loop and let it continue to process existing items in the queue.
	e.queue.ShutDownWithDrain()
}

func (e *eventProcessor) processEvent(ctx context.Context, event kubernetes.Event) error {
	defer e.queue.Done(event)
	return e.reconcileState(ctx)
}

func (e *eventProcessor) enqueueSyncMimir() {
	e.queue.Add(kubernetes.Event{
		Typ: eventTypeSyncMimir,
	})
}

func (e *eventProcessor) reconcileState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	desiredState, err := e.desiredStateFromKubernetes()
	if err != nil {
		return err
	}

	currentState := e.getMimirState()
	diffs := kubernetes.DiffAlertManagerConfigs(desiredState, currentState)

	var result error
	for ns, diff := range diffs {
		err = e.applyChanges(ctx, ns, diff)
		if err != nil {
			result = multierror.Append(result, err)
			continue
		}
	}

	return result
}

// desiredStateFromKubernetes loads PrometheusRule resources from Kubernetes and converts
// them to corresponding Mimir rule groups, indexed by Mimir namespace.
func (e *eventProcessor) desiredStateFromKubernetes() (kubernetes.AlertManagerConfigsByNamespace, error) {
	kubernetesState, err := e.getKubernetesState()
	if err != nil {
		return nil, err
	}

	desiredState := make(kubernetes.AlertManagerConfigsByNamespace)
	for _, configs := range kubernetesState {
		for _, config := range configs {
			mimirNs := mimirNamespaceForAlertmanagerConfigCRD(e.namespacePrefix, config)

			configBuf, err := json.Marshal(config)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal rule: %w", err)
			}

			level.Info(e.logger).Log("msg", "desiredStateFromKubernetes", "mimirNs", mimirNs, "config", string(configBuf))

			cfg, err := convertCRD(config.Spec)
			if err != nil {
				return nil, fmt.Errorf("failed to convert CRD: %w", err)
			}

			// TODO: Avoid a copy?
			desiredState[mimirNs] = []alertmgr_cfg.Config{*cfg}
		}
	}

	return desiredState, nil
}

func convertCRD(crd promv1beta1.AlertmanagerConfigSpec) (*alertmgr_cfg.Config, error) {
	buf, err := yaml.Marshal(crd)
	if err != nil {
		return nil, err
	}

	var cfg alertmgr_cfg.Config
	err = yaml.Unmarshal(buf, cfg)
	if err != nil {
		return nil, err
	}

	// TODO: See if we can do Marshal/Unmarshal instead:
	// https://github.com/prometheus-operator/prometheus-operator/issues/7386
	// Alternatively, we will need to write a better conversion function.
	for _, recv := range crd.Receivers {
		newRecv := alertmgr_cfg.Receiver{
			Name:         recv.Name,
			EmailConfigs: convertEmailConfig(recv.EmailConfigs),
			SlackConfigs: convertSlackConfig(recv.SlackConfigs),
		}
		cfg.Receivers = append(cfg.Receivers, newRecv)
	}

	return &cfg, nil
}

// TODO: Delete this later. It's just an example conversion function.
func convertEmailConfig(cfgs []promv1beta1.EmailConfig) []*alertmgr_cfg.EmailConfig {
	if cfgs == nil {
		return nil
	}

	var out []*alertmgr_cfg.EmailConfig
	for _, cfg := range cfgs {
		out = append(out, &alertmgr_cfg.EmailConfig{
			To: cfg.To,
		})
	}
	return out
}

// TODO: Delete this later. It's just an example conversion function.
func convertSlackConfig(cfgs []promv1beta1.SlackConfig) []*alertmgr_cfg.SlackConfig {
	if cfgs == nil {
		return nil
	}

	var out []*alertmgr_cfg.SlackConfig
	for _, cfg := range cfgs {
		out = append(out, &alertmgr_cfg.SlackConfig{
			Username: cfg.Username,
		})
	}
	return out
}

func addMatchersToQuery(query string, matchers []Matcher) (string, error) {
	var err error
	for _, s := range matchers {
		query, err = labelsSetPromQL(query, s.MatchType, s.Name, s.Value)
		if err != nil {
			return "", err
		}
	}
	return query, nil
}

// Lifted from: https://github.com/prometheus/prometheus/blob/79a6238e195ecc1c20937036c1e3b4e3bdaddc49/cmd/promtool/main.go#L1242
func labelsSetPromQL(query, labelMatchType, name, value string) (string, error) {
	expr, err := parser.ParseExpr(query)
	if err != nil {
		return query, err
	}

	var matchType promlabels.MatchType
	switch labelMatchType {
	case parser.ItemType(parser.EQL).String():
		matchType = promlabels.MatchEqual
	case parser.ItemType(parser.NEQ).String():
		matchType = promlabels.MatchNotEqual
	case parser.ItemType(parser.EQL_REGEX).String():
		matchType = promlabels.MatchRegexp
	case parser.ItemType(parser.NEQ_REGEX).String():
		matchType = promlabels.MatchNotRegexp
	default:
		return query, fmt.Errorf("invalid label match type: %s", labelMatchType)
	}

	parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
		if n, ok := node.(*parser.VectorSelector); ok {
			var found bool
			for i, l := range n.LabelMatchers {
				if l.Name == name {
					n.LabelMatchers[i].Type = matchType
					n.LabelMatchers[i].Value = value
					found = true
				}
			}
			if !found {
				n.LabelMatchers = append(n.LabelMatchers, &promlabels.Matcher{
					Type:  matchType,
					Name:  name,
					Value: value,
				})
			}
		}
		return nil
	})

	return expr.String(), nil
}

func convertCRDRuleGroupToRuleGroup(crd promv1.PrometheusRuleSpec) ([]rulefmt.RuleGroup, error) {
	buf, err := yaml.Marshal(crd)
	if err != nil {
		return nil, err
	}

	groups, errs := rulefmt.Parse(buf)
	if len(errs) > 0 {
		return nil, multierror.Append(nil, errs...)
	}

	return groups.Groups, nil
}

func (e *eventProcessor) applyChanges(ctx context.Context, namespace string, diffs []kubernetes.AlertManagerConfigDiff) error {
	if len(diffs) == 0 {
		return nil
	}

	for _, diff := range diffs {
		switch diff.Kind {
		case kubernetes.AlertManagerConfigDiffKindAdd:
			err := e.mimirClient.CreateAlertmanagerConfigs(ctx, namespace, diff.Desired)
			if err != nil {
				return err
			}
			// TODO: Change this "group"
			level.Info(e.logger).Log("msg", "added rule group", "namespace", namespace, "group", diff.Desired.String())
		//TODO: Support removal of alert mgr configs?
		case kubernetes.AlertManagerConfigDiffKindRemove:
			// err := e.mimirClient.DeleteRuleGroup(ctx, namespace, diff.Actual.Name)
			// if err != nil {
			// 	return err
			// }
			// TODO: Change this "group"
			level.Info(e.logger).Log("msg", "removed rule group", "namespace", namespace, "group", diff.Actual.String())
		case kubernetes.AlertManagerConfigDiffKindUpdate:
			err := e.mimirClient.CreateAlertmanagerConfigs(ctx, namespace, diff.Desired)
			if err != nil {
				return err
			}
			// TODO: Change this "group"
			level.Info(e.logger).Log("msg", "updated rule group", "namespace", namespace, "group", diff.Desired.String())
		default:
			level.Error(e.logger).Log("msg", "unknown rule group diff kind", "kind", diff.Kind)
		}
	}

	return nil
}

// getMimirState returns the cached Mimir ruler state, rule groups indexed by Mimir namespace.
func (e *eventProcessor) getMimirState() kubernetes.AlertManagerConfigsByNamespace {
	e.currentStateMtx.RLock()
	defer e.currentStateMtx.RUnlock()

	out := make(kubernetes.AlertManagerConfigsByNamespace, len(e.currentState))
	for ns, groups := range e.currentState {
		out[ns] = groups
	}

	return out
}

// getKubernetesState returns PrometheusRule resources indexed by Kubernetes namespace.
func (e *eventProcessor) getKubernetesState() (map[string][]*promv1beta1.AlertmanagerConfig, error) {
	namespaces, err := e.namespaceLister.List(e.namespaceSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	out := make(map[string][]*promv1beta1.AlertmanagerConfig)
	for _, namespace := range namespaces {
		rules, err := e.ruleLister.AlertmanagerConfigs(namespace.Name).List(e.ruleSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to list rules: %w", err)
		}

		out[namespace.Name] = append(out[namespace.Name], rules...)
	}

	return out, nil
}

// mimirNamespaceForRuleCRD returns the namespace that the rule CRD should be
// stored in mimir. This function, along with isManagedNamespace, is used to
// determine if a rule CRD is managed by Alloy.
func mimirNamespaceForAlertmanagerConfigCRD(prefix string, pr *promv1beta1.AlertmanagerConfig) string {
	return fmt.Sprintf("%s/%s/%s/%s", prefix, pr.Namespace, pr.Name, pr.UID)
}

// isManagedMimirNamespace returns true if the namespace is managed by Alloy.
// Unmanaged namespaces are left as is by the operator.
func isManagedMimirNamespace(prefix, namespace string) bool {
	prefixPart := regexp.QuoteMeta(prefix)
	namespacePart := `.+`
	namePart := `.+`
	uuidPart := `[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}`
	managedNamespaceRegex := regexp.MustCompile(
		fmt.Sprintf("^%s/%s/%s/%s$", prefixPart, namespacePart, namePart, uuidPart),
	)
	return managedNamespaceRegex.MatchString(namespace)
}

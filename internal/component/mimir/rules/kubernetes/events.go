package rules

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/mimir/client"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/hashicorp/go-multierror"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promListers "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1"
	"github.com/prometheus/prometheus/model/rulefmt"
	"k8s.io/apimachinery/pkg/labels"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/yaml" // Used for CRD compatibility instead of gopkg.in/yaml.v2
)

const (
	eventTypeSyncMimir kubernetes.EventType = "sync-mimir"
)

type eventProcessor struct {
	queue    workqueue.RateLimitingInterface
	stopChan chan struct{}
	health   healthReporter

	mimirClient       client.Interface
	namespaceLister   coreListers.NamespaceLister
	ruleLister        promListers.PrometheusRuleLister
	namespaceSelector labels.Selector
	ruleSelector      labels.Selector
	namespacePrefix   string
	externalLabels    map[string]string

	metrics *metrics
	logger  log.Logger

	currentState    kubernetes.RuleGroupsByNamespace
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

	switch event.Typ {
	case kubernetes.EventTypeResourceChanged:
		level.Info(e.logger).Log("msg", "processing event", "type", event.Typ, "key", event.ObjectKey)
	case eventTypeSyncMimir:
		level.Debug(e.logger).Log("msg", "syncing current state from ruler")
		err := e.syncMimir(ctx)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown event type: %s", event.Typ)
	}

	return e.reconcileState(ctx)
}

func (e *eventProcessor) enqueueSyncMimir() {
	e.queue.Add(kubernetes.Event{
		Typ: eventTypeSyncMimir,
	})
}

func (e *eventProcessor) syncMimir(ctx context.Context) error {
	rulesByNamespace, err := e.mimirClient.ListRules(ctx, "")
	if err != nil {
		level.Error(e.logger).Log("msg", "failed to list rules from mimir", "err", err)
		return err
	}

	for ns := range rulesByNamespace {
		if !isManagedMimirNamespace(e.namespacePrefix, ns) {
			delete(rulesByNamespace, ns)
		}
	}

	e.currentStateMtx.Lock()
	e.currentState = rulesByNamespace
	e.currentStateMtx.Unlock()

	return nil
}

func (e *eventProcessor) reconcileState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	desiredState, err := e.desiredStateFromKubernetes()
	if err != nil {
		return err
	}

	currentState := e.getMimirState()
	diffs := kubernetes.DiffRuleState(desiredState, currentState)

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
func (e *eventProcessor) desiredStateFromKubernetes() (kubernetes.RuleGroupsByNamespace, error) {
	kubernetesState, err := e.getKubernetesState()
	if err != nil {
		return nil, err
	}

	desiredState := make(kubernetes.RuleGroupsByNamespace)
	for _, rules := range kubernetesState {
		for _, rule := range rules {
			mimirNs := mimirNamespaceForRuleCRD(e.namespacePrefix, rule)
			groups, err := convertCRDRuleGroupToRuleGroup(rule.Spec)
			if err != nil {
				return nil, fmt.Errorf("failed to convert rule group: %w", err)
			}

			if len(e.externalLabels) > 0 {
				for _, rule_group := range groups {
					// Refer to the slice element via its index,
					// to make sure we mutate on the original and not a copy.
					for i := range rule_group.Rules {
						if rule_group.Rules[i].Labels == nil {
							rule_group.Rules[i].Labels = make(map[string]string, len(e.externalLabels))
						}
						maps.Copy(rule_group.Rules[i].Labels, e.externalLabels)
					}
				}
			}

			desiredState[mimirNs] = groups
		}
	}

	return desiredState, nil
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

func (e *eventProcessor) applyChanges(ctx context.Context, namespace string, diffs []kubernetes.RuleGroupDiff) error {
	if len(diffs) == 0 {
		return nil
	}

	for _, diff := range diffs {
		switch diff.Kind {
		case kubernetes.RuleGroupDiffKindAdd:
			err := e.mimirClient.CreateRuleGroup(ctx, namespace, diff.Desired)
			if err != nil {
				return err
			}
			level.Info(e.logger).Log("msg", "added rule group", "namespace", namespace, "group", diff.Desired.Name)
		case kubernetes.RuleGroupDiffKindRemove:
			err := e.mimirClient.DeleteRuleGroup(ctx, namespace, diff.Actual.Name)
			if err != nil {
				return err
			}
			level.Info(e.logger).Log("msg", "removed rule group", "namespace", namespace, "group", diff.Actual.Name)
		case kubernetes.RuleGroupDiffKindUpdate:
			err := e.mimirClient.CreateRuleGroup(ctx, namespace, diff.Desired)
			if err != nil {
				return err
			}
			level.Info(e.logger).Log("msg", "updated rule group", "namespace", namespace, "group", diff.Desired.Name)
		default:
			level.Error(e.logger).Log("msg", "unknown rule group diff kind", "kind", diff.Kind)
		}
	}

	// resync mimir state after applying changes
	return e.syncMimir(ctx)
}

// getMimirState returns the cached Mimir ruler state, rule groups indexed by Mimir namespace.
func (e *eventProcessor) getMimirState() kubernetes.RuleGroupsByNamespace {
	e.currentStateMtx.RLock()
	defer e.currentStateMtx.RUnlock()

	out := make(kubernetes.RuleGroupsByNamespace, len(e.currentState))
	for ns, groups := range e.currentState {
		out[ns] = groups
	}

	return out
}

// getKubernetesState returns PrometheusRule resources indexed by Kubernetes namespace.
func (e *eventProcessor) getKubernetesState() (map[string][]*promv1.PrometheusRule, error) {
	namespaces, err := e.namespaceLister.List(e.namespaceSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	out := make(map[string][]*promv1.PrometheusRule)
	for _, namespace := range namespaces {
		rules, err := e.ruleLister.PrometheusRules(namespace.Name).List(e.ruleSelector)
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
func mimirNamespaceForRuleCRD(prefix string, pr *promv1.PrometheusRule) string {
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

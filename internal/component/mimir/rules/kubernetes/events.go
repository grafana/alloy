package rules

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/agent/internal/flow/logging/level"
	"github.com/hashicorp/go-multierror"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/prometheus/model/rulefmt"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/yaml" // Used for CRD compatibility instead of gopkg.in/yaml.v2
)

// This type must be hashable, so it is kept simple. The indexer will maintain a
// cache of current state, so this is mostly used for logging.
type event struct {
	typ       eventType
	objectKey string
}

type eventType string

const (
	eventTypeResourceChanged eventType = "resource-changed"
	eventTypeSyncMimir       eventType = "sync-mimir"
)

type queuedEventHandler struct {
	log   log.Logger
	queue workqueue.RateLimitingInterface
}

func newQueuedEventHandler(log log.Logger, queue workqueue.RateLimitingInterface) *queuedEventHandler {
	return &queuedEventHandler{
		log:   log,
		queue: queue,
	}
}

// OnAdd implements the cache.ResourceEventHandler interface.
func (c *queuedEventHandler) OnAdd(obj interface{}, _ bool) {
	c.publishEvent(obj)
}

// OnUpdate implements the cache.ResourceEventHandler interface.
func (c *queuedEventHandler) OnUpdate(oldObj, newObj interface{}) {
	c.publishEvent(newObj)
}

// OnDelete implements the cache.ResourceEventHandler interface.
func (c *queuedEventHandler) OnDelete(obj interface{}) {
	c.publishEvent(obj)
}

func (c *queuedEventHandler) publishEvent(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		level.Error(c.log).Log("msg", "failed to get key for object", "err", err)
		return
	}

	c.queue.AddRateLimited(event{
		typ:       eventTypeResourceChanged,
		objectKey: key,
	})
}

func (c *Component) eventLoop(ctx context.Context) {
	for {
		eventInterface, shutdown := c.queue.Get()
		if shutdown {
			level.Info(c.log).Log("msg", "shutting down event loop")
			return
		}

		evt := eventInterface.(event)
		c.metrics.eventsTotal.WithLabelValues(string(evt.typ)).Inc()
		err := c.processEvent(ctx, evt)

		if err != nil {
			retries := c.queue.NumRequeues(evt)
			if retries < 5 {
				c.metrics.eventsRetried.WithLabelValues(string(evt.typ)).Inc()
				c.queue.AddRateLimited(evt)
				level.Error(c.log).Log(
					"msg", "failed to process event, will retry",
					"retries", fmt.Sprintf("%d/5", retries),
					"err", err,
				)
				continue
			} else {
				c.metrics.eventsFailed.WithLabelValues(string(evt.typ)).Inc()
				level.Error(c.log).Log(
					"msg", "failed to process event, max retries exceeded",
					"retries", fmt.Sprintf("%d/5", retries),
					"err", err,
				)
				c.reportUnhealthy(err)
			}
		} else {
			c.reportHealthy()
		}

		c.queue.Forget(evt)
	}
}

func (c *Component) processEvent(ctx context.Context, e event) error {
	defer c.queue.Done(e)

	switch e.typ {
	case eventTypeResourceChanged:
		level.Info(c.log).Log("msg", "processing event", "type", e.typ, "key", e.objectKey)
	case eventTypeSyncMimir:
		level.Debug(c.log).Log("msg", "syncing current state from ruler")
		err := c.syncMimir(ctx)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown event type: %s", e.typ)
	}

	return c.reconcileState(ctx)
}

func (c *Component) syncMimir(ctx context.Context) error {
	rulesByNamespace, err := c.mimirClient.ListRules(ctx, "")
	if err != nil {
		level.Error(c.log).Log("msg", "failed to list rules from mimir", "err", err)
		return err
	}

	for ns := range rulesByNamespace {
		if !isManagedMimirNamespace(c.args.MimirNameSpacePrefix, ns) {
			delete(rulesByNamespace, ns)
		}
	}

	c.currentState = rulesByNamespace

	return nil
}

func (c *Component) reconcileState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	desiredState, err := c.loadStateFromK8s()
	if err != nil {
		return err
	}

	diffs := diffRuleState(desiredState, c.currentState)
	var result error
	for ns, diff := range diffs {
		err = c.applyChanges(ctx, ns, diff)
		if err != nil {
			result = multierror.Append(result, err)
			continue
		}
	}

	return result
}

func (c *Component) loadStateFromK8s() (ruleGroupsByNamespace, error) {
	matchedNamespaces, err := c.namespaceLister.List(c.namespaceSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	desiredState := make(ruleGroupsByNamespace)
	for _, ns := range matchedNamespaces {
		crdState, err := c.ruleLister.PrometheusRules(ns.Name).List(c.ruleSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to list rules: %w", err)
		}

		for _, pr := range crdState {
			mimirNs := mimirNamespaceForRuleCRD(c.args.MimirNameSpacePrefix, pr)

			groups, err := convertCRDRuleGroupToRuleGroup(pr.Spec)
			if err != nil {
				return nil, fmt.Errorf("failed to convert rule group: %w", err)
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

func (c *Component) applyChanges(ctx context.Context, namespace string, diffs []ruleGroupDiff) error {
	if len(diffs) == 0 {
		return nil
	}

	for _, diff := range diffs {
		switch diff.Kind {
		case ruleGroupDiffKindAdd:
			err := c.mimirClient.CreateRuleGroup(ctx, namespace, diff.Desired)
			if err != nil {
				return err
			}
			level.Info(c.log).Log("msg", "added rule group", "namespace", namespace, "group", diff.Desired.Name)
		case ruleGroupDiffKindRemove:
			err := c.mimirClient.DeleteRuleGroup(ctx, namespace, diff.Actual.Name)
			if err != nil {
				return err
			}
			level.Info(c.log).Log("msg", "removed rule group", "namespace", namespace, "group", diff.Actual.Name)
		case ruleGroupDiffKindUpdate:
			err := c.mimirClient.CreateRuleGroup(ctx, namespace, diff.Desired)
			if err != nil {
				return err
			}
			level.Info(c.log).Log("msg", "updated rule group", "namespace", namespace, "group", diff.Desired.Name)
		default:
			level.Error(c.log).Log("msg", "unknown rule group diff kind", "kind", diff.Kind)
		}
	}

	// resync mimir state after applying changes
	return c.syncMimir(ctx)
}

// mimirNamespaceForRuleCRD returns the namespace that the rule CRD should be
// stored in mimir. This function, along with isManagedNamespace, is used to
// determine if a rule CRD is managed by the agent.
func mimirNamespaceForRuleCRD(prefix string, pr *promv1.PrometheusRule) string {
	return fmt.Sprintf("%s/%s/%s/%s", prefix, pr.Namespace, pr.Name, pr.UID)
}

// isManagedMimirNamespace returns true if the namespace is managed by the agent.
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

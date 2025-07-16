package rules

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/grafana/loki/v3/pkg/logql/syntax"
	"github.com/hashicorp/go-multierror"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/rulefmt"
	"github.com/prometheus/prometheus/promql/parser"
	"sigs.k8s.io/yaml" // Used for CRD compatibility instead of gopkg.in/yaml.v2

	"github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const eventTypeSyncLoki kubernetes.EventType = "sync-loki"

func (c *Component) eventLoop(ctx context.Context) {
	for {
		evt, shutdown := c.queue.Get()
		if shutdown {
			level.Info(c.log).Log("msg", "shutting down event loop")
			return
		}

		c.metrics.eventsTotal.WithLabelValues(string(evt.Typ)).Inc()
		err := c.processEvent(ctx, evt)

		if err != nil {
			retries := c.queue.NumRequeues(evt)
			if retries < 5 {
				c.metrics.eventsRetried.WithLabelValues(string(evt.Typ)).Inc()
				c.queue.AddRateLimited(evt)
				level.Error(c.log).Log(
					"msg", "failed to process event, will retry",
					"retries", fmt.Sprintf("%d/5", retries),
					"err", err,
				)
				continue
			} else {
				c.metrics.eventsFailed.WithLabelValues(string(evt.Typ)).Inc()
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

func (c *Component) processEvent(ctx context.Context, e kubernetes.Event) error {
	defer c.queue.Done(e)

	switch e.Typ {
	case kubernetes.EventTypeResourceChanged:
		level.Info(c.log).Log("msg", "processing event", "type", e.Typ, "key", e.ObjectKey)
	case eventTypeSyncLoki:
		level.Debug(c.log).Log("msg", "syncing current state from ruler")
		err := c.syncLoki(ctx)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown event type: %s", e.Typ)
	}

	return c.reconcileState(ctx)
}

func (c *Component) syncLoki(ctx context.Context) error {
	rulesByNamespace, err := c.lokiClient.ListRules(ctx, "")
	if err != nil {
		level.Error(c.log).Log("msg", "failed to list rules from loki", "err", err)
		return err
	}

	for ns := range rulesByNamespace {
		if !isManagedLokiNamespace(c.args.LokiNameSpacePrefix, ns) {
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

	diffs := kubernetes.DiffPrometheusRuleGroupState(desiredState, c.currentState)
	var errs error
	for ns, diff := range diffs {
		err = c.applyChanges(ctx, ns, diff)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
	}

	return errs
}

func (c *Component) loadStateFromK8s() (kubernetes.PrometheusRuleGroupsByNamespace, error) {
	matchedNamespaces, err := c.namespaceLister.List(c.namespaceSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	desiredState := make(kubernetes.PrometheusRuleGroupsByNamespace)
	for _, ns := range matchedNamespaces {
		crdState, err := c.ruleLister.PrometheusRules(ns.Name).List(c.ruleSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to list rules: %w", err)
		}

		for _, rule := range crdState {
			lokiNs := lokiNamespaceForRuleCRD(c.args.LokiNameSpacePrefix, rule)
			groups, err := convertCRDRuleGroupToRuleGroup(rule.Spec)
			if err != nil {
				return nil, fmt.Errorf("failed to convert rule group: %w", err)
			}

			if c.args.ExtraQueryMatchers != nil {
				for _, ruleGroup := range groups {
					for i := range ruleGroup.Rules {
						query := ruleGroup.Rules[i].Expr
						newQuery, err := addMatchersToQuery(query, c.args.ExtraQueryMatchers.Matchers)
						if err != nil {
							level.Error(c.log).Log("msg", "failed to add labels to PrometheusRule query", "query", query, "err", err)
						}
						ruleGroup.Rules[i].Expr = newQuery
					}
				}
			}

			desiredState[lokiNs] = groups
		}
	}

	return desiredState, nil
}

func addMatchersToQuery(query string, matchers []Matcher) (string, error) {
	var err error
	for _, s := range matchers {
		query, err = labelsSetLogQL(query, s.MatchType, s.Name, s.Value)
		if err != nil {
			return "", err
		}
	}
	return query, nil
}

// Inspired from the labelsSetPromQL function from the mimir.rules.kubernetes component
// this function was modified to use the logql parser instead
func labelsSetLogQL(query, labelMatchType, name, value string) (string, error) {
	expr, err := syntax.ParseExpr(query)
	if err != nil {
		return query, err
	}

	var matchType labels.MatchType
	switch labelMatchType {
	case parser.ItemType(parser.EQL).String():
		matchType = labels.MatchEqual
	case parser.ItemType(parser.NEQ).String():
		matchType = labels.MatchNotEqual
	case parser.ItemType(parser.EQL_REGEX).String():
		matchType = labels.MatchRegexp
	case parser.ItemType(parser.NEQ_REGEX).String():
		matchType = labels.MatchNotRegexp
	default:
		return query, fmt.Errorf("invalid label match type: %s", labelMatchType)
	}
	expr.Walk(func(e syntax.Expr) bool {
		switch concrete := e.(type) {
		case *syntax.MatchersExpr:
			var found bool
			for _, l := range concrete.Mts {
				if l.Name == name {
					l.Type = matchType
					l.Value = value
					found = true
				}
			}
			if !found {
				concrete.Mts = append(concrete.Mts, &labels.Matcher{
					Type:  matchType,
					Name:  name,
					Value: value,
				})
			}
		}
		return true
	})

	return expr.String(), nil
}

func convertCRDRuleGroupToRuleGroup(crd promv1.PrometheusRuleSpec) ([]rulefmt.RuleGroup, error) {
	buf, err := yaml.Marshal(crd)
	if err != nil {
		return nil, err
	}

	var errs error
	groups, _ := rulefmt.Parse(buf, false)
	for _, group := range groups.Groups {
		for _, rule := range group.Rules {
			if _, err := syntax.ParseExpr(rule.Expr); err != nil {
				if rule.Record != "" {
					errs = multierror.Append(errs, fmt.Errorf("could not parse expression for record '%s' in group '%s': %w", rule.Record, group.Name, err))
				} else {
					errs = multierror.Append(errs, fmt.Errorf("could not parse expression for alert '%s' in group '%s': %w", rule.Alert, group.Name, err))
				}
			}
		}
	}
	if errs != nil {
		return nil, errs
	}

	return groups.Groups, nil
}

func (c *Component) applyChanges(ctx context.Context, namespace string, diffs []kubernetes.PrometheusRuleGroupDiff) error {
	if len(diffs) == 0 {
		return nil
	}

	for _, diff := range diffs {
		switch diff.Kind {
		case kubernetes.RuleGroupDiffKindAdd:
			err := c.lokiClient.CreateRuleGroup(ctx, namespace, diff.Desired)
			if err != nil {
				return err
			}
			level.Info(c.log).Log("msg", "added rule group", "namespace", namespace, "group", diff.Desired.Name)
		case kubernetes.RuleGroupDiffKindRemove:
			err := c.lokiClient.DeleteRuleGroup(ctx, namespace, diff.Actual.Name)
			if err != nil {
				return err
			}
			level.Info(c.log).Log("msg", "removed rule group", "namespace", namespace, "group", diff.Actual.Name)
		case kubernetes.RuleGroupDiffKindUpdate:
			err := c.lokiClient.CreateRuleGroup(ctx, namespace, diff.Desired)
			if err != nil {
				return err
			}
			level.Info(c.log).Log("msg", "updated rule group", "namespace", namespace, "group", diff.Desired.Name)
		default:
			level.Error(c.log).Log("msg", "unknown rule group diff kind", "kind", diff.Kind)
		}
	}

	// resync loki state after applying changes
	return c.syncLoki(ctx)
}

// lokiNamespaceForRuleCRD returns the namespace that the rule CRD should be
// stored in loki. This function, along with isManagedNamespace, is used to
// determine if a rule CRD is managed by Alloy.
func lokiNamespaceForRuleCRD(prefix string, pr *promv1.PrometheusRule) string {
	// Set to - to separate, loki doesn't support prefixpath like mimir ruler does
	return fmt.Sprintf("%s-%s-%s-%s", prefix, pr.Namespace, pr.Name, pr.UID)
}

// isManagedLokiNamespace returns true if the namespace is managed by Alloy.
// Unmanaged namespaces are left as is by the operator.
func isManagedLokiNamespace(prefix, namespace string) bool {
	prefixPart := regexp.QuoteMeta(prefix)
	namespacePart := `.+`
	namePart := `.+`
	uuidPart := `[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}`
	managedNamespaceRegex := regexp.MustCompile(
		// Set to - to separate, loki doesn't support prefixpath like mimir ruler does
		fmt.Sprintf("^%s-%s-%s-%s$", prefixPart, namespacePart, namePart, uuidPart),
	)
	return managedNamespaceRegex.MatchString(namespace)
}

package database_observability

import (
	"fmt"
	"slices"
	"strings"

	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/relabel"
)

const (
	ProviderNameLabel    = "provider_name"
	ProviderRegionLabel  = "provider_region"
	ProviderAccountLabel = "provider_account"
)

// ReservedLabels are the labels the components set themselves on metrics
// targets and on log entries. The per-database relabel rules of a
// `database_instance` block must not overwrite or remove them: correlating
// metrics with logs, and the db-o11y dashboards, depend on them.
//
// The set is the union of both signal paths, so that a rule set which is valid
// is valid for metrics and logs alike:
//
//   - `job` and `server_id` are set on both.
//   - `engine` and `op` are only set on log entries.
//   - the `provider_*` and `__`-prefixed labels are only set on targets.
//
// `instance` is deliberately absent, because rewriting it per database to a
// friendly name is the documented reason to relabel these components at all. A
// `database_instance` rule is the safest place to do it, because one rule
// applies to the metrics and the logs of that database together. Doing it
// downstream instead takes a `discovery.relabel` and a `loki.relabel` that
// nothing cross-checks for agreement. Nothing in the components reads
// `instance` back off a target: ownership and the metrics path both derive it
// from the DSN. It's still protected from removal, by
// labelsProtectedFromRemoval.
var ReservedLabels = []string{
	"job",
	"server_id",
	"engine",
	"op",
	ProviderNameLabel,
	ProviderRegionLabel,
	ProviderAccountLabel,
	model.AddressLabel,
	model.SchemeLabel,
	model.MetricsPathLabel,
}

// InstanceLabel identifies the monitored database. A rule may rewrite its
// value, unlike the ReservedLabels, but may not remove it: metrics and logs
// are correlated on it.
const InstanceLabel = "instance"

// labelsProtectedFromRemoval returns the labels a `labeldrop` rule must not
// match. It's the reserved labels plus `instance`, which a rule may rewrite but
// must not delete.
func labelsProtectedFromRemoval() []string {
	return append(slices.Clone(ReservedLabels), InstanceLabel)
}

// allowedRuleActions are the relabel actions a `database_instance` rule may
// use.
//
// The filtering actions, `keep`, `drop`, `keepequal`, and `dropequal`, are
// excluded because they mean different things on the two signal paths: on the
// metrics path a filtered target means the database exports no metrics at all,
// while on the logs path it means a single entry is discarded. A downstream
// `loki.relabel` still covers log filtering.
//
// `labelmap` is excluded because its `replacement` is a label *name* template,
// which can't be checked against ReservedLabels at config time.
//
// `labelkeep` is excluded because it inverts: keeping a label set that still
// contains every reserved label means the regex has to match all of them, so
// the only rule that passes validation is the no-op `(.*)`. `labeldrop`
// expresses the same intent without the inversion.
var allowedRuleActions = []relabel.Action{
	relabel.Replace,
	relabel.Lowercase,
	relabel.Uppercase,
	relabel.HashMod,
	relabel.LabelDrop,
}

// ValidateRelabelRules checks the relabel rules of one `database_instance`
// block: only the labelling actions are allowed, and no rule may overwrite or
// remove a label in ReservedLabels.
//
// The per-rule arguments themselves are validated by relabel.Config, which the
// syntax decoder calls for each decoded block.
func ValidateRelabelRules(rules []*relabel.Config) error {
	for i, r := range rules {
		if r == nil {
			continue
		}

		if !slices.Contains(allowedRuleActions, r.Action) {
			return fmt.Errorf("rule %d: action %q isn't supported in a database_instance block, use one of: %s", i+1, r.Action, joinActions(allowedRuleActions))
		}

		// relabel.Config applies the `(.*)` default when the syntax decoder
		// decodes a block, so a rule from a configuration file always has a
		// regex. One built programmatically might not, and the relabel engine
		// doesn't substitute a default: it calls the embedded *regexp.Regexp
		// directly and panics on nil. Reject it here instead.
		if r.Regex.Regexp == nil {
			return fmt.Errorf("rule %d: regex isn't set: build rules from relabel.DefaultRelabelConfig so that the relabel defaults apply", i+1)
		}

		switch r.Action {
		case relabel.LabelDrop:
			for _, label := range labelsProtectedFromRemoval() {
				if !r.Regex.MatchString(label) {
					continue
				}
				if label == InstanceLabel {
					return fmt.Errorf("rule %d: labeldrop regex %q matches %q, which correlates a database's metrics with its logs: a rule may rewrite instance, but not remove it", i+1, r.Regex, label)
				}
				return fmt.Errorf("rule %d: labeldrop regex %q matches reserved label %q, which database_observability sets itself", i+1, r.Regex, label)
			}
		default:
			if strings.Contains(r.TargetLabel, "$") {
				return fmt.Errorf("rule %d: target_label %q is templated, which isn't supported in a database_instance block: use a literal target_label so that it can be checked against the reserved labels", i+1, r.TargetLabel)
			}
			if slices.Contains(ReservedLabels, r.TargetLabel) {
				return fmt.Errorf("rule %d: target_label %q is reserved by database_observability and can't be overwritten", i+1, r.TargetLabel)
			}

			// hashmod maps its input onto `modulus` buckets, so writing it to
			// the label that identifies the database destroys that identity:
			// several databases land in the same bucket by design, and a source
			// label that's absent at runtime puts every database in one bucket,
			// because they all hash the empty string. ApplyRules can't catch
			// that, since the value hashmod writes is never empty.
			if r.Action == relabel.HashMod && r.TargetLabel == InstanceLabel {
				return fmt.Errorf("rule %d: hashmod can't write to target_label %q, because it maps several databases onto the same value: metrics and logs are correlated on instance", i+1, r.TargetLabel)
			}

			if err := checkComputedValue(i, r); err != nil {
				return err
			}
		}
	}

	return nil
}

// checkComputedValue rejects a rule whose computed value makes it useless or
// destructive, where that's certain when the configuration loads.
//
// A rule with no `source_labels` extracts the empty string, so its result
// depends only on the rule itself. Two outcomes are never what the author
// wanted: an empty value, which removes the target label rather than setting
// it, and a value derived only from that constant input, which is the same for
// every database and so collapses them onto one series.
//
// For `replace` this runs the same expansion the relabel engine runs, rather
// than guessing from the shape of `replacement`: a capture group reference
// doesn't imply an empty result, since `db-$1` yields `db-` and `$$` yields a
// literal `$`.
//
// A rule with `source_labels` depends on the labels present at runtime and
// can't be checked here. ProtectLabels stops the destructive half of that at
// runtime; the collapsing half is only reachable by pointing several databases
// at a source label they share, which is documented rather than enforced.
func checkComputedValue(i int, r *relabel.Config) error {
	if len(r.SourceLabels) > 0 {
		return nil
	}

	switch r.Action {
	case relabel.Lowercase, relabel.Uppercase:
		return fmt.Errorf("rule %d: %s without source_labels always computes an empty value, which removes target_label %q instead of setting it: add source_labels", i+1, r.Action, r.TargetLabel)
	case relabel.HashMod:
		return fmt.Errorf("rule %d: hashmod without source_labels always computes the same value, so every database would get the same target_label %q: add source_labels", i+1, r.TargetLabel)
	case relabel.Replace:
		indexes := r.Regex.FindStringSubmatchIndex("")
		if indexes == nil {
			return fmt.Errorf("rule %d: regex %q doesn't match the empty value this rule extracts, so the rule never does anything: add source_labels, or a regex that matches", i+1, r.Regex)
		}
		if len(r.Regex.ExpandString(nil, r.Replacement, "", indexes)) == 0 {
			return fmt.Errorf("rule %d: replacement %q expands to an empty value, which removes target_label %q instead of setting it: set a literal replacement, or use labeldrop to remove a label", i+1, r.Replacement, r.TargetLabel)
		}
	}

	return nil
}

// StaticInstanceLabel returns the value the rules always write to the
// `instance` label, and reports whether that value is known when the
// configuration loads.
//
// Only a `replace` rule with no `source_labels` and a `replacement` that
// references no capture group has a value known this early. A later rule that
// writes `instance` from a source label makes the result unknown again.
func StaticInstanceLabel(rules []*relabel.Config) (string, bool) {
	var (
		value string
		known bool
	)

	for _, r := range rules {
		if r == nil || r.TargetLabel != InstanceLabel {
			continue
		}

		if r.Action == relabel.Replace && len(r.SourceLabels) == 0 && !strings.Contains(r.Replacement, "$") {
			value, known = r.Replacement, true
			continue
		}

		value, known = "", false
	}

	return value, known
}

func joinActions(actions []relabel.Action) string {
	names := make([]string, 0, len(actions))
	for _, a := range actions {
		names = append(names, string(a))
	}
	return strings.Join(names, ", ")
}

func GetRelabelingRules(serverID string, cp *CloudProvider) []*relabel.Config {
	r := relabel.DefaultRelabelConfig // use default to avoid defining all fields
	r.Replacement = serverID
	r.TargetLabel = "server_id"
	r.Action = relabel.Replace

	rs := []*relabel.Config{&r}

	if cp != nil {
		if cp.AWS != nil {
			providerName := relabel.DefaultRelabelConfig
			providerName.Replacement = "aws"
			providerName.TargetLabel = ProviderNameLabel
			providerName.Action = relabel.Replace

			providerRegion := relabel.DefaultRelabelConfig
			providerRegion.Replacement = cp.AWS.ARN.Region
			providerRegion.TargetLabel = ProviderRegionLabel
			providerRegion.Action = relabel.Replace

			providerAccount := relabel.DefaultRelabelConfig
			providerAccount.Replacement = cp.AWS.ARN.AccountID
			providerAccount.TargetLabel = ProviderAccountLabel
			providerAccount.Action = relabel.Replace

			rs = append(rs, &providerName, &providerRegion, &providerAccount)
		}
		if cp.Azure != nil {
			providerName := relabel.DefaultRelabelConfig
			providerName.Replacement = "azure"
			providerName.TargetLabel = ProviderNameLabel
			providerName.Action = relabel.Replace

			providerRegion := relabel.DefaultRelabelConfig
			providerRegion.Replacement = cp.Azure.ResourceGroup
			providerRegion.TargetLabel = ProviderRegionLabel
			providerRegion.Action = relabel.Replace

			providerAccount := relabel.DefaultRelabelConfig
			providerAccount.Replacement = cp.Azure.SubscriptionID
			providerAccount.TargetLabel = ProviderAccountLabel
			providerAccount.Action = relabel.Replace

			rs = append(rs, &providerName, &providerRegion, &providerAccount)
		}
		if cp.GCP != nil {
			providerName := relabel.DefaultRelabelConfig
			providerName.Replacement = "gcp"
			providerName.TargetLabel = ProviderNameLabel
			providerName.Action = relabel.Replace

			providerRegion := relabel.DefaultRelabelConfig
			providerRegion.Replacement = cp.GCP.Region
			providerRegion.TargetLabel = ProviderRegionLabel
			providerRegion.Action = relabel.Replace

			providerAccount := relabel.DefaultRelabelConfig
			providerAccount.Replacement = cp.GCP.ProjectID
			providerAccount.TargetLabel = ProviderAccountLabel
			providerAccount.Action = relabel.Replace

			rs = append(rs, &providerName, &providerRegion, &providerAccount)
		}
	}

	return rs
}

// ApplyRules runs the per-database relabel rules of a `database_instance` block
// against lb, and reports whether the target or entry should be kept.
//
// The rules can't remove the labels in labelsProtectedFromRemoval. A rule that
// computes an empty value removes its target label, and ValidateRelabelRules
// rejects the rules that provably do so, but a rule whose value is only empty
// at runtime can't be caught when the configuration loads: a source label that
// doesn't exist on this signal path, a typo in a source label name, or a
// capture group that doesn't participate in the match. Those keep the previous
// value instead.
//
// Keeping a stale value is always safer than the alternative. An empty
// `instance` on a metrics target makes Prometheus fill it in from `__address__`
// instead, and every database in one component shares that address, so a single
// rule would otherwise collapse every database onto one series while their logs
// lost the label entirely.
//
// Only the per-database rules go through this. The rules from GetRelabelingRules
// set these labels in the first place, so callers run those against lb directly.
func ApplyRules(lb relabel.LabelBuilder, rules []*relabel.Config) bool {
	if len(rules) == 0 {
		return true
	}
	return relabel.ProcessBuilder(protectedLabelBuilder{inner: lb, protected: labelsProtectedFromRemoval()}, rules...)
}

// protectedLabelBuilder is the relabel.LabelBuilder ApplyRules runs the rules
// through. It passes everything to inner except a removal of a protected label.
type protectedLabelBuilder struct {
	inner     relabel.LabelBuilder
	protected []string
}

var _ relabel.LabelBuilder = protectedLabelBuilder{}

func (b protectedLabelBuilder) Get(label string) string {
	return b.inner.Get(label)
}

func (b protectedLabelBuilder) Range(f func(label string, value string)) {
	b.inner.Range(f)
}

// Set drops the write when it would remove a protected label, because both
// builders this wraps treat an empty value as a delete.
func (b protectedLabelBuilder) Set(label string, val string) {
	if val == "" && slices.Contains(b.protected, label) {
		return
	}
	b.inner.Set(label, val)
}

func (b protectedLabelBuilder) Del(labels ...string) {
	keep := make([]string, 0, len(labels))
	for _, label := range labels {
		if slices.Contains(b.protected, label) {
			continue
		}
		keep = append(keep, label)
	}

	if len(keep) > 0 {
		b.inner.Del(keep...)
	}
}

// labelSetBuilder adapts a model.LabelSet to relabel.LabelBuilder, so that the
// per-database rules can be applied to the labels of a log entry with the same
// rule engine as the metrics targets, whose discovery.TargetBuilder already
// implements the interface.
//
// It mutates the label set in place, matching loki.AddLabelsMiddleware.
type labelSetBuilder struct {
	labels model.LabelSet
}

var _ relabel.LabelBuilder = labelSetBuilder{}

func (b labelSetBuilder) Get(label string) string {
	return string(b.labels[model.LabelName(label)])
}

// Range iterates over a snapshot of the label names, because the relabel
// action that calls it, `labeldrop`, deletes from the label set inside the
// callback.
func (b labelSetBuilder) Range(f func(label string, value string)) {
	names := make([]model.LabelName, 0, len(b.labels))
	for name := range b.labels {
		names = append(names, name)
	}

	for _, name := range names {
		if value, ok := b.labels[name]; ok {
			f(string(name), string(value))
		}
	}
}

// Set sets a label, or deletes it when val is empty, following the semantics
// documented on relabel.LabelBuilder.
func (b labelSetBuilder) Set(label string, val string) {
	if val == "" {
		delete(b.labels, model.LabelName(label))
		return
	}
	b.labels[model.LabelName(label)] = model.LabelValue(val)
}

func (b labelSetBuilder) Del(labels ...string) {
	for _, label := range labels {
		delete(b.labels, model.LabelName(label))
	}
}

// AddLabelsAndRelabelMiddleware returns an EntryMiddleware that adds labels to
// every log entry and then applies the per-database relabel rules of a
// `database_instance` block to the result.
//
// The two steps share one middleware, rather than chaining
// loki.AddLabelsMiddleware with a second one, for two reasons. Each
// loki.NewEntryMutatorHandler runs a goroutine that only its Stop closure ends,
// and the handler chain the components build is never stopped, so every chained
// middleware costs a leaked goroutine per instance start. It also makes the
// ordering explicit: the rules run after the labels, so a rule can read them
// with `source_labels`, which mirrors the metrics path where the per-database
// rules follow the rules from GetRelabelingRules.
//
// As in loki.AddLabelsMiddleware, a label is only added when the entry doesn't
// already carry it.
func AddLabelsAndRelabelMiddleware(labels model.LabelSet, rules []*relabel.Config) loki.EntryMiddleware {
	return loki.EntryMiddlewareFunc(func(next loki.EntryHandler) loki.EntryHandler {
		if len(labels) == 0 && len(rules) == 0 {
			return next
		}

		return loki.NewEntryMutatorHandler(next, func(e loki.Entry) loki.Entry {
			if e.Labels == nil {
				e.Labels = make(model.LabelSet, len(labels))
			}

			for name, value := range labels {
				if _, ok := e.Labels[name]; !ok {
					e.Labels[name] = value
				}
			}

			// The return value reports whether the entry should be kept, which
			// only the filtering actions can make false. ValidateRelabelRules
			// rejects those, so every rule reaching this point can only mutate
			// labels. Allowing them would mean replacing this mutator with a
			// handler that can drop an entry.
			ApplyRules(labelSetBuilder{labels: e.Labels}, rules)

			return e
		})
	})
}

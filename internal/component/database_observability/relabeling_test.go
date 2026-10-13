package database_observability

import (
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/loki/pkg/push"
)

func Test_GetRelabelingRules(t *testing.T) {
	t.Run("return relabeling rules", func(t *testing.T) {
		rr := GetRelabelingRules("some-server-id", nil)

		require.Equal(t, 1, len(rr))
		require.Equal(t, "some-server-id", rr[0].Replacement)
		require.Equal(t, "server_id", rr[0].TargetLabel)
		require.Equal(t, relabel.Replace, rr[0].Action)
	})

	t.Run("return relabeling rules with AWS config", func(t *testing.T) {
		rr := GetRelabelingRules("some-server-id", &CloudProvider{
			AWS: &AWSCloudProviderInfo{
				ARN: arn.ARN{
					Region:    "some-region",
					AccountID: "some-account",
				},
			},
		})

		require.Equal(t, 4, len(rr))
		require.Equal(t, "some-server-id", rr[0].Replacement)
		require.Equal(t, "server_id", rr[0].TargetLabel)
		require.Equal(t, relabel.Replace, rr[0].Action)

		require.Equal(t, "aws", rr[1].Replacement)
		require.Equal(t, "provider_name", rr[1].TargetLabel)
		require.Equal(t, relabel.Replace, rr[1].Action)

		require.Equal(t, "some-region", rr[2].Replacement)
		require.Equal(t, "provider_region", rr[2].TargetLabel)
		require.Equal(t, relabel.Replace, rr[2].Action)

		require.Equal(t, "some-account", rr[3].Replacement)
		require.Equal(t, "provider_account", rr[3].TargetLabel)
		require.Equal(t, relabel.Replace, rr[3].Action)
	})

	t.Run("return relabeling rules with Azure config", func(t *testing.T) {
		rr := GetRelabelingRules("some-server-id", &CloudProvider{
			Azure: &AzureCloudProviderInfo{
				ServerName:     "some-resource",
				ResourceGroup:  "some-resource-group",
				SubscriptionID: "some-subscription-id",
			},
		})

		require.Equal(t, 4, len(rr))
		require.Equal(t, "some-server-id", rr[0].Replacement)
		require.Equal(t, "server_id", rr[0].TargetLabel)
		require.Equal(t, relabel.Replace, rr[0].Action)

		require.Equal(t, "azure", rr[1].Replacement)
		require.Equal(t, "provider_name", rr[1].TargetLabel)
		require.Equal(t, relabel.Replace, rr[1].Action)

		require.Equal(t, "some-resource-group", rr[2].Replacement)
		require.Equal(t, "provider_region", rr[2].TargetLabel)
		require.Equal(t, relabel.Replace, rr[2].Action)

		require.Equal(t, "some-subscription-id", rr[3].Replacement)
		require.Equal(t, "provider_account", rr[3].TargetLabel)
		require.Equal(t, relabel.Replace, rr[3].Action)
	})

	t.Run("return relabeling rules with GCP config", func(t *testing.T) {
		rr := GetRelabelingRules("some-server-id", &CloudProvider{
			GCP: &GCPCloudProviderInfo{
				ProjectID:  "some-project-id",
				Region:     "some-region",
				InstanceID: "some-instance-id",
			},
		})

		require.Equal(t, 4, len(rr))
		require.Equal(t, "some-server-id", rr[0].Replacement)
		require.Equal(t, "server_id", rr[0].TargetLabel)
		require.Equal(t, relabel.Replace, rr[0].Action)

		require.Equal(t, "gcp", rr[1].Replacement)
		require.Equal(t, "provider_name", rr[1].TargetLabel)
		require.Equal(t, relabel.Replace, rr[1].Action)

		require.Equal(t, "some-region", rr[2].Replacement)
		require.Equal(t, "provider_region", rr[2].TargetLabel)
		require.Equal(t, relabel.Replace, rr[2].Action)

		require.Equal(t, "some-project-id", rr[3].Replacement)
		require.Equal(t, "provider_account", rr[3].TargetLabel)
		require.Equal(t, relabel.Replace, rr[3].Action)
	})
}

// testRule builds a rule the way the syntax decoder would: starting from the
// relabel defaults, so that an unset regex is `(.*)` rather than nil.
//
// The actions that write a value get source_labels and a literal replacement,
// so that they compute a non-empty value. Without that, ValidateRelabelRules
// rejects them for only ever removing their target label, which
// Test_ValidateRelabelRules_WritesAValue covers on its own.
func testRule(t *testing.T, action relabel.Action, targetLabel, regex string) *relabel.Config {
	t.Helper()

	r := relabel.DefaultRelabelConfig
	r.Action = action
	r.TargetLabel = targetLabel

	switch action {
	case relabel.LabelDrop, relabel.LabelKeep, relabel.LabelMap:
	default:
		r.SourceLabels = []string{"job"}
		r.Replacement = "some-value"
	}

	if regex != "" {
		var re relabel.Regexp
		require.NoError(t, re.UnmarshalText([]byte(regex)))
		r.Regex = re
	}

	return &r
}

// protect builds the builder ApplyRules runs the rules through, for the tests
// that exercise its methods directly rather than through a rule.
func protect(lb relabel.LabelBuilder) relabel.LabelBuilder {
	return protectedLabelBuilder{inner: lb, protected: labelsProtectedFromRemoval()}
}

// staticRule builds a replace rule with no source_labels, the form a static
// label uses.
func staticRule(t *testing.T, targetLabel, replacement string) *relabel.Config {
	t.Helper()

	r := relabel.DefaultRelabelConfig
	r.Action = relabel.Replace
	r.TargetLabel = targetLabel
	r.Replacement = replacement

	return &r
}

// Test_ValidateRelabelRules_WritesAValue covers the check that rejects a rule
// which can only remove its target label. With no source_labels the extracted
// value is always empty, so the outcome is known when the configuration loads.
func Test_ValidateRelabelRules_WritesAValue(t *testing.T) {
	tests := []struct {
		name      string
		rule      *relabel.Config
		expectErr string
	}{
		{
			// The documented idiom for a static label.
			name: "literal replacement",
			rule: staticRule(t, "environment", "production"),
		},
		{
			// A capture group reference doesn't imply an empty expansion: the
			// literal text around it survives.
			name: "literal text around a capture group",
			rule: staticRule(t, "environment", "db-$1"),
		},
		{
			name: "escaped dollar sign expands to a literal dollar",
			rule: staticRule(t, "cost_centre", "$$"),
		},
		{
			name: "escaped dollar sign with trailing text",
			rule: staticRule(t, "cost_centre", "$$USD"),
		},
		{
			// The default replacement, which is the footgun this check closes.
			name:      "default replacement removes the label",
			rule:      testRuleDefaults(t, relabel.Replace, "environment"),
			expectErr: `rule 1: replacement "$1" expands to an empty value, which removes target_label "environment"`,
		},
		{
			name:      "empty replacement removes the label",
			rule:      staticRule(t, "environment", ""),
			expectErr: `rule 1: replacement "" expands to an empty value, which removes target_label "environment"`,
		},
		{
			name:      "unmatched capture group removes the label",
			rule:      staticRule(t, "environment", "$2"),
			expectErr: `rule 1: replacement "$2" expands to an empty value, which removes target_label "environment"`,
		},
		{
			name:      "lowercase without source_labels removes the label",
			rule:      testRuleDefaults(t, relabel.Lowercase, "environment"),
			expectErr: `rule 1: lowercase without source_labels always computes an empty value`,
		},
		{
			name:      "uppercase without source_labels removes the label",
			rule:      testRuleDefaults(t, relabel.Uppercase, "environment"),
			expectErr: `rule 1: uppercase without source_labels always computes an empty value`,
		},
		{
			// hashmod over the empty string is a constant, so every database
			// carrying the rule would get the same value.
			name:      "hashmod without source_labels computes a constant",
			rule:      hashModRule(t, "shard", 10),
			expectErr: `rule 1: hashmod without source_labels always computes the same value, so every database would get the same target_label "shard"`,
		},
		{
			// Rejected by the hashmod/instance check rather than the
			// empty-value one, since hashmod can never write to instance.
			name:      "hashmod into instance would collapse every database",
			rule:      hashModRule(t, "instance", 10),
			expectErr: `rule 1: hashmod can't write to target_label "instance"`,
		},
		{
			// Protects `instance` from the same mistakes.
			name:      "default replacement into instance removes it",
			rule:      testRuleDefaults(t, relabel.Replace, "instance"),
			expectErr: `removes target_label "instance"`,
		},
		{
			name:      "lowercase into instance removes it",
			rule:      testRuleDefaults(t, relabel.Lowercase, "instance"),
			expectErr: `removes target_label "instance"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRelabelRules([]*relabel.Config{tt.rule})
			if tt.expectErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.expectErr)
		})
	}

	// A rule with source_labels can't be checked at load time, because whether
	// the value is empty depends on the labels present at runtime. ProtectLabels
	// keeps those from removing a protected label.
	t.Run("a typo'd source label passes validation", func(t *testing.T) {
		r := relabel.DefaultRelabelConfig
		r.SourceLabels = []string{"typpo"}
		r.TargetLabel = "instance"

		require.NoError(t, ValidateRelabelRules([]*relabel.Config{&r}))
	})

	// hashmod is the exception: it writes a non-empty value, so no runtime
	// guard can stop it, and a source label that's absent at runtime makes
	// every database hash the empty string into the same bucket. It's rejected
	// whether or not source_labels is set.
	t.Run("hashmod into instance is rejected even with source_labels", func(t *testing.T) {
		r := relabel.DefaultRelabelConfig
		r.Action = relabel.HashMod
		r.SourceLabels = []string{"typpo"}
		r.TargetLabel = "instance"
		r.Modulus = 10

		require.ErrorContains(t,
			ValidateRelabelRules([]*relabel.Config{&r}),
			`rule 1: hashmod can't write to target_label "instance"`,
		)
	})

	t.Run("hashmod into another label with source_labels is allowed", func(t *testing.T) {
		r := relabel.DefaultRelabelConfig
		r.Action = relabel.HashMod
		r.SourceLabels = []string{"server_id"}
		r.TargetLabel = "shard"
		r.Modulus = 10

		require.NoError(t, ValidateRelabelRules([]*relabel.Config{&r}))
	})
}

// testRuleDefaults builds a rule with no source_labels and the default
// replacement of `$1`, as a user would get by omitting both.
func testRuleDefaults(t *testing.T, action relabel.Action, targetLabel string) *relabel.Config {
	t.Helper()

	r := relabel.DefaultRelabelConfig
	r.Action = action
	r.TargetLabel = targetLabel

	return &r
}

// hashModRule builds a hashmod rule with no source_labels, which hashes the
// empty string and so computes the same value for every database.
func hashModRule(t *testing.T, targetLabel string, modulus uint64) *relabel.Config {
	t.Helper()

	r := relabel.DefaultRelabelConfig
	r.Action = relabel.HashMod
	r.TargetLabel = targetLabel
	r.Modulus = modulus

	return &r
}

func Test_ValidateRelabelRules(t *testing.T) {
	tests := []struct {
		name  string
		rules []*relabel.Config
		// expectErr is a substring of the expected error, or empty when the
		// rules are valid.
		expectErr string
	}{
		{
			name:  "no rules",
			rules: nil,
		},
		{
			name:  "replace into a new label",
			rules: []*relabel.Config{testRule(t, relabel.Replace, "environment", "")},
		},
		{
			name:  "lowercase and uppercase into new labels",
			rules: []*relabel.Config{testRule(t, relabel.Lowercase, "env_lower", ""), testRule(t, relabel.Uppercase, "env_upper", "")},
		},
		{
			name:  "nil rule is skipped",
			rules: []*relabel.Config{nil},
		},
		{
			name:      "replace into a reserved label set on both paths",
			rules:     []*relabel.Config{testRule(t, relabel.Replace, "server_id", "")},
			expectErr: `rule 1: target_label "server_id" is reserved`,
		},
		{
			name:      "replace into a logs-only reserved label",
			rules:     []*relabel.Config{testRule(t, relabel.Replace, "op", "")},
			expectErr: `rule 1: target_label "op" is reserved`,
		},
		{
			name:      "replace into a metrics-only reserved label",
			rules:     []*relabel.Config{testRule(t, relabel.Replace, "provider_region", "")},
			expectErr: `rule 1: target_label "provider_region" is reserved`,
		},
		{
			name:      "replace into a prometheus meta label",
			rules:     []*relabel.Config{testRule(t, relabel.Replace, "__address__", "")},
			expectErr: `rule 1: target_label "__address__" is reserved`,
		},
		{
			name:      "lowercase into a reserved label",
			rules:     []*relabel.Config{testRule(t, relabel.Lowercase, "engine", "")},
			expectErr: `rule 1: target_label "engine" is reserved`,
		},
		{
			// Rewriting instance per database is the documented reason to
			// relabel these components, so it's deliberately not reserved.
			name:  "replace into instance is allowed",
			rules: []*relabel.Config{testRule(t, relabel.Replace, "instance", "")},
		},
		{
			// But removing it would break metrics/logs correlation, so
			// labeldrop still has to protect it.
			name:      "labeldrop of instance is rejected",
			rules:     []*relabel.Config{testRule(t, relabel.LabelDrop, "", "instance")},
			expectErr: `rule 1: labeldrop regex "instance" matches "instance", which correlates a database's metrics with its logs`,
		},
		{
			name:      "hashmod into a reserved label",
			rules:     []*relabel.Config{testRule(t, relabel.HashMod, "job", "")},
			expectErr: `rule 1: target_label "job" is reserved`,
		},
		{
			name:      "templated target_label",
			rules:     []*relabel.Config{testRule(t, relabel.Replace, "${1}_environment", "")},
			expectErr: `rule 1: target_label "${1}_environment" is templated`,
		},
		{
			name:      "error names the offending rule",
			rules:     []*relabel.Config{testRule(t, relabel.Replace, "environment", ""), testRule(t, relabel.Replace, "engine", "")},
			expectErr: `rule 2: target_label "engine" is reserved`,
		},
		{
			name:      "keep is rejected",
			rules:     []*relabel.Config{testRule(t, relabel.Keep, "", "")},
			expectErr: `rule 1: action "keep" isn't supported`,
		},
		{
			name:      "drop is rejected",
			rules:     []*relabel.Config{testRule(t, relabel.Drop, "", "")},
			expectErr: `rule 1: action "drop" isn't supported`,
		},
		{
			name:      "keepequal is rejected",
			rules:     []*relabel.Config{testRule(t, relabel.KeepEqual, "environment", "")},
			expectErr: `rule 1: action "keepequal" isn't supported`,
		},
		{
			name:      "dropequal is rejected",
			rules:     []*relabel.Config{testRule(t, relabel.DropEqual, "environment", "")},
			expectErr: `rule 1: action "dropequal" isn't supported`,
		},
		{
			name:      "labelmap is rejected",
			rules:     []*relabel.Config{testRule(t, relabel.LabelMap, "", "db_(.*)")},
			expectErr: `rule 1: action "labelmap" isn't supported`,
		},
		{
			name:  "labeldrop of a non-reserved label",
			rules: []*relabel.Config{testRule(t, relabel.LabelDrop, "", "environment")},
		},
		{
			name:      "labeldrop with the default regex drops everything",
			rules:     []*relabel.Config{testRule(t, relabel.LabelDrop, "", "")},
			expectErr: `rule 1: labeldrop regex "(.*)" matches reserved label "job"`,
		},
		{
			name:      "labeldrop matching a reserved label",
			rules:     []*relabel.Config{testRule(t, relabel.LabelDrop, "", "server_.*")},
			expectErr: `rule 1: labeldrop regex "server_.*" matches reserved label "server_id"`,
		},
		{
			// labelkeep inverts: a passing regex has to match every reserved
			// label, so the only valid rule would be the no-op `(.*)`.
			name:      "labelkeep is rejected",
			rules:     []*relabel.Config{testRule(t, relabel.LabelKeep, "", "(.*)")},
			expectErr: `rule 1: action "labelkeep" isn't supported`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRelabelRules(tt.rules)
			if tt.expectErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.expectErr)
		})
	}
}

// Test_ValidateRelabelRules_NilRegex covers Arguments built programmatically
// rather than decoded from a config file, where a rule can have no regex at
// all. The relabel engine doesn't substitute the `(.*)` default: it calls the
// embedded *regexp.Regexp directly and panics on nil, so such a rule has to be
// rejected rather than accepted.
func Test_ValidateRelabelRules_NilRegex(t *testing.T) {
	for _, action := range allowedRuleActions {
		t.Run(string(action), func(t *testing.T) {
			rules := []*relabel.Config{{Action: action, TargetLabel: "environment", Modulus: 1}}
			require.ErrorContains(t, ValidateRelabelRules(rules), "rule 1: regex isn't set")
		})
	}
}

// Test_ValidateRelabelRules_CoversEveryAction guards against a new relabel
// action being added upstream without a decision about whether a per-database
// rule may use it.
func Test_ValidateRelabelRules_CoversEveryAction(t *testing.T) {
	allActions := []relabel.Action{
		relabel.Replace, relabel.Keep, relabel.Drop, relabel.HashMod,
		relabel.LabelMap, relabel.LabelDrop, relabel.LabelKeep,
		relabel.Lowercase, relabel.Uppercase, relabel.KeepEqual, relabel.DropEqual,
	}

	for _, action := range allActions {
		t.Run(string(action), func(t *testing.T) {
			// A regex that matches no reserved label, so for labeldrop the
			// rule is harmless. For the other allowed actions the regex is
			// matched against source values rather than label names, so it
			// doesn't affect the outcome either.
			err := ValidateRelabelRules([]*relabel.Config{testRule(t, action, "environment", "environment")})

			if slices.Contains(allowedRuleActions, action) {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, "isn't supported in a database_instance block")
		})
	}
}

func Test_labelSetBuilder(t *testing.T) {
	t.Run("get returns empty for an absent label", func(t *testing.T) {
		b := labelSetBuilder{labels: model.LabelSet{"job": "some-job"}}

		require.Equal(t, "some-job", b.Get("job"))
		require.Equal(t, "", b.Get("missing"))
	})

	t.Run("set adds and overwrites", func(t *testing.T) {
		labels := model.LabelSet{"job": "some-job"}
		b := labelSetBuilder{labels: labels}

		b.Set("environment", "production")
		b.Set("job", "other-job")

		require.Equal(t, model.LabelSet{"job": "other-job", "environment": "production"}, labels)
	})

	t.Run("set to an empty value deletes", func(t *testing.T) {
		labels := model.LabelSet{"job": "some-job", "environment": "production"}
		b := labelSetBuilder{labels: labels}

		b.Set("environment", "")

		require.Equal(t, model.LabelSet{"job": "some-job"}, labels)
	})

	t.Run("del removes several labels", func(t *testing.T) {
		labels := model.LabelSet{"a": "1", "b": "2", "c": "3"}
		b := labelSetBuilder{labels: labels}

		b.Del("a", "c", "missing")

		require.Equal(t, model.LabelSet{"b": "2"}, labels)
	})

	t.Run("range visits every label", func(t *testing.T) {
		b := labelSetBuilder{labels: model.LabelSet{"a": "1", "b": "2"}}

		seen := map[string]string{}
		b.Range(func(label, value string) { seen[label] = value })

		require.Equal(t, map[string]string{"a": "1", "b": "2"}, seen)
	})

	// Range iterates a snapshot, so deleting from inside the callback - which
	// is what the labeldrop and labelkeep actions do - stays well defined.
	t.Run("range tolerates deletes from the callback", func(t *testing.T) {
		labels := model.LabelSet{"a": "1", "b": "2", "c": "3"}
		b := labelSetBuilder{labels: labels}

		var visited int
		b.Range(func(label, value string) {
			visited++
			b.Del(label)
		})

		require.Equal(t, 3, visited)
		require.Empty(t, labels)
	})

	t.Run("range on an empty label set", func(t *testing.T) {
		b := labelSetBuilder{labels: model.LabelSet{}}

		b.Range(func(label, value string) { t.Fatalf("unexpected label %q", label) })
	})
}

// Test_labelSetBuilder_ProcessBuilder checks the builder against the real rule
// engine, rather than only its methods in isolation.
func Test_labelSetBuilder_ProcessBuilder(t *testing.T) {
	t.Run("replace derives a label from an existing one", func(t *testing.T) {
		var re relabel.Regexp
		require.NoError(t, re.UnmarshalText([]byte("(.{4}).*")))

		rule := relabel.DefaultRelabelConfig
		rule.SourceLabels = []string{"server_id"}
		rule.Regex = re
		rule.TargetLabel = "short_id"
		rule.Replacement = "$1"

		labels := model.LabelSet{"server_id": "abcdefgh"}
		require.True(t, relabel.ProcessBuilder(labelSetBuilder{labels: labels}, &rule))

		require.Equal(t, model.LabelSet{"server_id": "abcdefgh", "short_id": "abcd"}, labels)
	})

	t.Run("labeldrop removes the matching labels", func(t *testing.T) {
		var re relabel.Regexp
		require.NoError(t, re.UnmarshalText([]byte("temp_.*")))

		rule := relabel.DefaultRelabelConfig
		rule.Action = relabel.LabelDrop
		rule.Regex = re

		labels := model.LabelSet{"job": "some-job", "temp_a": "1", "temp_b": "2"}
		require.True(t, relabel.ProcessBuilder(labelSetBuilder{labels: labels}, &rule))

		require.Equal(t, model.LabelSet{"job": "some-job"}, labels)
	})

	t.Run("uppercase writes the transformed value", func(t *testing.T) {
		rule := relabel.DefaultRelabelConfig
		rule.Action = relabel.Uppercase
		rule.SourceLabels = []string{"environment"}
		rule.TargetLabel = "environment_upper"

		labels := model.LabelSet{"environment": "production"}
		require.True(t, relabel.ProcessBuilder(labelSetBuilder{labels: labels}, &rule))

		require.Equal(t, model.LabelValue("PRODUCTION"), labels["environment_upper"])
	})
}

// sendAndCollect pushes one entry carrying the given labels through a
// rules-only middleware and returns the labels it comes out with.
func sendAndCollect(t *testing.T, rules []*relabel.Config, labels model.LabelSet) model.LabelSet {
	t.Helper()

	collector := loki.NewCollectingHandler()
	defer collector.Stop()

	entryHandler := AddLabelsAndRelabelMiddleware(nil, rules).Wrap(collector)

	go func() {
		entryHandler.Chan() <- loki.Entry{
			Labels: labels,
			Entry:  push.Entry{Timestamp: time.Now(), Line: "some-message"},
		}
	}()

	require.Eventually(t, func() bool {
		return len(collector.Received()) == 1
	}, 5*time.Second, 10*time.Millisecond)

	return collector.Received()[0].Labels
}

func Test_AddLabelsAndRelabelMiddleware(t *testing.T) {
	t.Run("nothing to do returns the next handler unchanged", func(t *testing.T) {
		collector := loki.NewCollectingHandler()
		defer collector.Stop()

		require.Same(t, collector, AddLabelsAndRelabelMiddleware(nil, nil).Wrap(collector))
		require.Same(t, collector, AddLabelsAndRelabelMiddleware(model.LabelSet{}, []*relabel.Config{}).Wrap(collector))
	})

	// The labels and the rules share one mutator, so that the chain the
	// components build costs one goroutine rather than one per step.
	t.Run("labels are added before the rules run", func(t *testing.T) {
		var re relabel.Regexp
		require.NoError(t, re.UnmarshalText([]byte("(.{4}).*")))

		rule := relabel.DefaultRelabelConfig
		rule.SourceLabels = []string{"server_id"}
		rule.Regex = re
		rule.TargetLabel = "short_id"
		rule.Replacement = "$1"

		collector := loki.NewCollectingHandler()
		defer collector.Stop()

		entryHandler := AddLabelsAndRelabelMiddleware(
			model.LabelSet{"server_id": "abcdefgh", "job": "some-job"},
			[]*relabel.Config{&rule},
		).Wrap(collector)

		go func() {
			entryHandler.Chan() <- loki.Entry{
				Entry: push.Entry{Timestamp: time.Now(), Line: "some-message"},
			}
		}()

		require.Eventually(t, func() bool {
			return len(collector.Received()) == 1
		}, 5*time.Second, 10*time.Millisecond)

		assert.Equal(t, model.LabelSet{
			"server_id": "abcdefgh",
			"job":       "some-job",
			"short_id":  "abcd",
		}, collector.Received()[0].Labels)
	})

	t.Run("an existing label is not overwritten by the added labels", func(t *testing.T) {
		collector := loki.NewCollectingHandler()
		defer collector.Stop()

		entryHandler := AddLabelsAndRelabelMiddleware(model.LabelSet{"op": "added"}, nil).Wrap(collector)

		go func() {
			entryHandler.Chan() <- loki.Entry{
				Labels: model.LabelSet{"op": "original"},
				Entry:  push.Entry{Timestamp: time.Now(), Line: "some-message"},
			}
		}()

		require.Eventually(t, func() bool {
			return len(collector.Received()) == 1
		}, 5*time.Second, 10*time.Millisecond)

		assert.Equal(t, model.LabelSet{"op": "original"}, collector.Received()[0].Labels)
	})

	t.Run("a static rule adds a label", func(t *testing.T) {
		rule := relabel.DefaultRelabelConfig
		rule.TargetLabel = "environment"
		rule.Replacement = "production"

		got := sendAndCollect(t, []*relabel.Config{&rule}, model.LabelSet{"job": "some-job"})

		assert.Equal(t, model.LabelSet{"job": "some-job", "environment": "production"}, got)
	})

	t.Run("a rule can read the labels already on the entry", func(t *testing.T) {
		var re relabel.Regexp
		require.NoError(t, re.UnmarshalText([]byte("(.{4}).*")))

		rule := relabel.DefaultRelabelConfig
		rule.SourceLabels = []string{"server_id"}
		rule.Regex = re
		rule.TargetLabel = "short_id"
		rule.Replacement = "$1"

		got := sendAndCollect(t, []*relabel.Config{&rule}, model.LabelSet{"server_id": "abcdefgh"})

		assert.Equal(t, model.LabelSet{"server_id": "abcdefgh", "short_id": "abcd"}, got)
	})

	t.Run("rules are applied in order", func(t *testing.T) {
		first := relabel.DefaultRelabelConfig
		first.TargetLabel = "environment"
		first.Replacement = "production"

		second := relabel.DefaultRelabelConfig
		second.Action = relabel.Uppercase
		second.SourceLabels = []string{"environment"}
		second.TargetLabel = "environment_upper"

		got := sendAndCollect(t, []*relabel.Config{&first, &second}, model.LabelSet{})

		assert.Equal(t, model.LabelSet{"environment": "production", "environment_upper": "PRODUCTION"}, got)
	})

	t.Run("an entry with no labels gets a label set", func(t *testing.T) {
		rule := relabel.DefaultRelabelConfig
		rule.TargetLabel = "environment"
		rule.Replacement = "production"

		got := sendAndCollect(t, []*relabel.Config{&rule}, nil)

		assert.Equal(t, model.LabelSet{"environment": "production"}, got)
	})
}

func Test_protectedLabelBuilder(t *testing.T) {
	t.Run("set to an empty value keeps a protected label", func(t *testing.T) {
		labels := model.LabelSet{"instance": "orders_db", "server_id": "abc"}
		b := protect(labelSetBuilder{labels: labels})

		b.Set("instance", "")
		b.Set("server_id", "")

		assert.Equal(t, model.LabelSet{"instance": "orders_db", "server_id": "abc"}, labels)
	})

	t.Run("set to an empty value still removes an unprotected label", func(t *testing.T) {
		labels := model.LabelSet{"environment": "production"}
		b := protect(labelSetBuilder{labels: labels})

		b.Set("environment", "")

		assert.Empty(t, labels)
	})

	t.Run("a protected label can still be rewritten", func(t *testing.T) {
		labels := model.LabelSet{"instance": "tcp(127.0.0.1:3306)/orders"}
		b := protect(labelSetBuilder{labels: labels})

		b.Set("instance", "orders_db")

		assert.Equal(t, model.LabelValue("orders_db"), labels["instance"])
	})

	t.Run("del skips the protected labels only", func(t *testing.T) {
		labels := model.LabelSet{"instance": "orders_db", "job": "j", "environment": "production"}
		b := protect(labelSetBuilder{labels: labels})

		b.Del("instance", "job", "environment")

		assert.Equal(t, model.LabelSet{"instance": "orders_db", "job": "j"}, labels)
	})

	t.Run("get and range delegate to the wrapped builder", func(t *testing.T) {
		b := protect(labelSetBuilder{labels: model.LabelSet{"instance": "orders_db"}})

		assert.Equal(t, "orders_db", b.Get("instance"))

		seen := map[string]string{}
		b.Range(func(label, value string) { seen[label] = value })
		assert.Equal(t, map[string]string{"instance": "orders_db"}, seen)
	})
}

// Test_ProtectLabels_SourceLabelAbsent covers the case validation can't reach:
// a rule whose source label doesn't exist, so its computed value is empty only
// at runtime. Without the guard it would remove the target label.
func Test_ProtectLabels_SourceLabelAbsent(t *testing.T) {
	rule := relabel.DefaultRelabelConfig
	rule.SourceLabels = []string{"typpo"}
	rule.TargetLabel = "instance"

	t.Run("on a log entry", func(t *testing.T) {
		labels := model.LabelSet{"instance": "orders_db", "job": "j"}

		require.True(t, ApplyRules(labelSetBuilder{labels: labels}, []*relabel.Config{&rule}))

		assert.Equal(t, model.LabelValue("orders_db"), labels["instance"], "instance should survive")
	})

	// On a metrics target an empty instance is worse than absent: Prometheus
	// fills it in from __address__, which every database in the component
	// shares, so all of them would collapse onto one series.
	t.Run("on a metrics target", func(t *testing.T) {
		target := discovery.NewTargetFromMap(map[string]string{
			"__address__": "127.0.0.1:12345",
			"instance":    "orders_db",
			"job":         "j",
		})

		builder := discovery.NewTargetBuilderFrom(target)
		require.True(t, ApplyRules(builder, []*relabel.Config{&rule}))

		got, ok := builder.Target().Get("instance")
		require.True(t, ok, "instance should survive")
		assert.Equal(t, "orders_db", got)
	})
}

// Test_InstanceSurvivesEveryKnownRoute pins the three rules a review proved
// could delete the instance label, each of which has to be stopped either when
// the configuration loads or when the rule runs. An empty instance on a metrics
// target makes Prometheus fill it in from __address__, which every database in
// a component shares, so losing it collapses them onto one series.
func Test_InstanceSurvivesEveryKnownRoute(t *testing.T) {
	// Rejected when the configuration loads: the outcome is certain.
	t.Run("empty replacement is rejected", func(t *testing.T) {
		rule := relabel.DefaultRelabelConfig
		rule.TargetLabel = "instance"
		rule.Replacement = ""

		require.ErrorContains(t,
			ValidateRelabelRules([]*relabel.Config{&rule}),
			`removes target_label "instance"`,
		)
	})

	t.Run("default replacement with no source_labels is rejected", func(t *testing.T) {
		rule := relabel.DefaultRelabelConfig
		rule.TargetLabel = "instance"

		require.ErrorContains(t,
			ValidateRelabelRules([]*relabel.Config{&rule}),
			`removes target_label "instance"`,
		)
	})

	t.Run("lowercase with no source_labels is rejected", func(t *testing.T) {
		rule := relabel.DefaultRelabelConfig
		rule.Action = relabel.Lowercase
		rule.TargetLabel = "instance"

		require.ErrorContains(t,
			ValidateRelabelRules([]*relabel.Config{&rule}),
			`removes target_label "instance"`,
		)
	})

	// Accepted when the configuration loads, because emptiness depends on the
	// labels present at runtime, so the builder has to hold the line instead.
	runtimeCases := []struct {
		name string
		rule func() relabel.Config
	}{
		{
			name: "replace from a missing source label",
			rule: func() relabel.Config {
				r := relabel.DefaultRelabelConfig
				r.SourceLabels = []string{"missing"}
				r.TargetLabel = "instance"
				return r
			},
		},
		{
			name: "lowercase from a missing source label",
			rule: func() relabel.Config {
				r := relabel.DefaultRelabelConfig
				r.Action = relabel.Lowercase
				r.SourceLabels = []string{"missing"}
				r.TargetLabel = "instance"
				return r
			},
		},
		{
			name: "uppercase from a missing source label",
			rule: func() relabel.Config {
				r := relabel.DefaultRelabelConfig
				r.Action = relabel.Uppercase
				r.SourceLabels = []string{"missing"}
				r.TargetLabel = "instance"
				return r
			},
		},
	}

	for _, tc := range runtimeCases {
		t.Run(tc.name, func(t *testing.T) {
			rule := tc.rule()
			require.NoError(t, ValidateRelabelRules([]*relabel.Config{&rule}))

			labels := model.LabelSet{"instance": "orders_db"}
			require.True(t, ApplyRules(labelSetBuilder{labels: labels}, []*relabel.Config{&rule}))
			assert.Equal(t, model.LabelValue("orders_db"), labels["instance"], "instance should survive on the logs path")

			builder := discovery.NewTargetBuilderFrom(discovery.NewTargetFromMap(map[string]string{
				"__address__": "127.0.0.1:12345",
				"instance":    "orders_db",
			}))
			require.True(t, ApplyRules(builder, []*relabel.Config{&rule}))
			got, ok := builder.Target().Get("instance")
			require.True(t, ok, "instance should survive on the metrics path")
			assert.Equal(t, "orders_db", got)
		})
	}
}

func Test_StaticInstanceLabel(t *testing.T) {
	tests := []struct {
		name      string
		rules     []*relabel.Config
		wantValue string
		wantKnown bool
	}{
		{
			name: "no rules",
		},
		{
			name:  "a rule targeting another label is ignored",
			rules: []*relabel.Config{staticRule(t, "environment", "production")},
		},
		{
			name:      "a static replace into instance is known",
			rules:     []*relabel.Config{staticRule(t, "instance", "orders_db")},
			wantValue: "orders_db",
			wantKnown: true,
		},
		{
			name:  "a replacement referencing a capture group isn't known",
			rules: []*relabel.Config{staticRule(t, "instance", "db-$1")},
		},
		{
			name:  "a rule reading a source label isn't known",
			rules: []*relabel.Config{testRule(t, relabel.Replace, "instance", "")},
		},
		{
			name: "a later dynamic rule makes the value unknown again",
			rules: []*relabel.Config{
				staticRule(t, "instance", "orders_db"),
				testRule(t, relabel.Replace, "instance", ""),
			},
		},
		{
			name: "the last static rule wins",
			rules: []*relabel.Config{
				staticRule(t, "instance", "first"),
				staticRule(t, "instance", "second"),
			},
			wantValue: "second",
			wantKnown: true,
		},
		{
			name:  "a nil rule is skipped",
			rules: []*relabel.Config{nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, known := StaticInstanceLabel(tt.rules)
			assert.Equal(t, tt.wantKnown, known)
			assert.Equal(t, tt.wantValue, value)
		})
	}
}

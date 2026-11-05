package mimirruleskubernetes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

func TestMimirRules(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-mimir-rules-kubernetes",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	promOp := deps.NewPrometheusOperator(deps.PrometheusOperatorOptions{})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	extraManifests := deps.NewCustomWorkloads(deps.CustomWorkloadsOptions{
		Path: "./config/workloads.yaml",
		Vars: map[string]string{"NAMESPACE": ns.Name()},
	})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-mimir-rules",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, promOp, extraManifests, mimir, alloy},
	})

	t.Run("Rules with leading whitespace are synced", func(t *testing.T) {
		mimir.QueryRules(t, func(c *assert.CollectT, rules map[string][]deps.RuleGroup) {
			assert.Equal(c, map[string]string{
				"control_rule": "vector(1)",
				// Whitespace before the first token is dropped, the rest of the
				// expression is kept as written.
				"leading_whitespace_rule": "# The blank line and nested indentation are intentional.\n  (\n    up\n  )",
			}, recordedExprs(rules))
		})
	})

	t.Run("Changed rule is synced", func(t *testing.T) {
		// Alloy lists every rule group from Mimir before it applies a change, so
		// this fails if Mimir returns a listing that Alloy can't parse.
		require.NoError(t, harness.RunCommand("kubectl", "patch", "prometheusrule", "control",
			"--namespace", ns.Name(), "--type", "json",
			"--patch", `[{"op": "replace", "path": "/spec/groups/0/rules/0/expr", "value": "vector(2)"}]`))

		mimir.QueryRules(t, func(c *assert.CollectT, rules map[string][]deps.RuleGroup) {
			assert.Equal(c, "vector(2)", recordedExprs(rules)["control_rule"])
		})
	})
}

// recordedExprs returns the expression of every recording rule in Mimir,
// indexed by the name of the recorded series.
func recordedExprs(rules map[string][]deps.RuleGroup) map[string]string {
	exprs := map[string]string{}
	for _, groups := range rules {
		for _, group := range groups {
			for _, rule := range group.Rules {
				exprs[rule.Record] = rule.Expr
			}
		}
	}
	return exprs
}

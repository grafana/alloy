package statictest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const testConfig = `
discovery.kubernetes "pods" {
  role = "pod"
}

prometheus.scrape "pods" {
  targets    = discovery.kubernetes.pods.targets
  forward_to = [prometheus.remote_write.default.receiver]
  clustering {
    enabled = true
  }
}

prometheus.scrape "self" {
  targets    = [{__address__ = "localhost:12345"}]
  forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  endpoint {
    url = "https://prom.example.com/api/prom/push"
  }
}

loki.source.file "app" {
  targets    = [{__path__ = "/var/log/app.log"}]
  forward_to = [loki.secretfilter.default.receiver]
}

loki.source.journal "node" {
  forward_to = [loki.process.default.receiver]
}

loki.secretfilter "default" {
  forward_to = [loki.process.default.receiver]
}

loki.process "default" {
  stage.static_labels {
    values = {env = "prod"}
  }
  forward_to = [loki.write.default.receiver]
}

loki.write "default" {
  endpoint {
    url = "https://logs-gateway.example.com/loki/api/v1/push"
  }
  endpoint {
    url = sys.env("BACKUP_URL")
  }
}
`

func TestRules(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rule    string
		passed  bool
		message string
	}{
		{
			name: "all-or-none fails when only some components have it",
			rule: `
type: all-or-none
select: [prometheus.scrape, prometheus.operator.*]
path: clustering.enabled
equals: true`,
			message: "doesn't: prometheus.scrape.self (config.alloy:14): clustering.enabled is not set",
		},
		{
			name: "all-or-none passes when none have it",
			rule: `
type: all-or-none
select: prometheus.scrape
path: honor_labels
equals: true`,
			passed: true,
		},
		{
			name: "attribute checks every block",
			rule: `
type: attribute
select: loki.write
path: endpoint[*].url
matches: 'https://logs-gateway\.example\.com/.*'`,
			message: "loki.write.default (config.alloy:45): endpoint[*].url is not a constant",
		},
		{
			name: "attribute with an index",
			rule: `
type: attribute
select: loki.write
path: endpoint[0].url
matches: 'https://logs-gateway\.example\.com/.*'`,
			passed: true,
		},
		{
			name: "attribute with equals",
			rule: `
type: attribute
select: prometheus.scrape.pods
path: clustering.enabled
equals: false`,
			message: "clustering.enabled is true, want equals false",
		},
		{
			name: "attribute in a block with a dotted name",
			rule: `
type: attribute
select: loki.process
path: stage.static_labels.values
equals: {env: prod}`,
			passed: true,
		},
		{
			name: "no matching component passes, with a note",
			rule: `
type: attribute
select: otelcol.exporter.*
path: endpoint
equals: x`,
			passed:  true,
			message: "no component matches otelcol.exporter.*",
		},
		{
			name: "count of blocks",
			rule: `
type: count
select: "*"
path: clustering
max: 0`,
			message: "found 1 of clustering in *, want at most 0\n  prometheus.scrape.pods (config.alloy:6)",
		},
		{
			name: "count of components",
			rule: `
type: count
select: loki.write
min: 1
max: 1`,
			passed: true,
		},
		{
			name: "when skips a rule",
			rule: `
type: count
select: "*"
path: clustering
max: 0
when:
  select: prometheus.scrape
  max: 0`,
			passed:  true,
			message: "skipped: the when condition doesn't hold",
		},
		{
			name: "flows-through finds the path around the filter",
			rule: `
type: flows-through
from: loki.source.*
to: loki.write
through: loki.secretfilter`,
			message: "loki.source.journal.node (config.alloy:30) reaches loki.write.default without loki.secretfilter: loki.source.journal.node -> loki.process.default -> loki.write.default",
		},
		{
			name: "flows-through with no sources passes, with a note",
			rule: `
type: flows-through
from: otelcol.receiver.*
to: loki.write
through: loki.secretfilter`,
			passed:  true,
			message: "no component matches from: otelcol.receiver.*",
		},
		{
			name: "flows-through follows pulled exports",
			rule: `
type: flows-through
from: discovery.kubernetes
to: prometheus.remote_write
through: prometheus.scrape`,
			passed: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var chk Check
			require.NoError(t, yaml.Unmarshal([]byte(tc.rule), &chk))
			require.NoError(t, chk.Validate())
			res, err := Run("config.alloy", []byte(testConfig), []Rule{{Name: "r", Check: chk}})
			require.NoError(t, err)
			got := res.Rules[0]
			require.Equal(t, tc.passed, got.Passed, got.Message)
			if tc.message != "" {
				require.Contains(t, got.Message, tc.message)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	for rule, want := range map[string]string{
		"type: attribute\nselect: a\npath: b":                           "needs one of equals or matches",
		"type: attribute\nselect: a\npath: b\nequals: 1\nmatches: x":    "needs one of equals or matches",
		"type: attribute\nselect: a\npath: b\nmatches: '('":             "matches:",
		"type: count\nselect: a":                                        "count needs select, and min or max",
		"type: flows-through\nfrom: a\nto: b":                           "flows-through needs from, to and through",
		"type: cel":                                                     `unknown type "cel"`,
		"select: a":                                                     "type is missing",
		"type: count\nselect: a\nmax: 0\nwhen: {select: b}":             "when needs select, and min or max",
		"type: all-or-none\nselect: [a, b]\npath: x.y\nequals: false\n": "",
	} {
		var chk Check
		require.NoError(t, yaml.Unmarshal([]byte(rule), &chk))
		err := chk.Validate()
		if want == "" {
			require.NoError(t, err, rule)
		} else {
			require.ErrorContains(t, err, want, rule)
		}
	}
}

const moduleConfig = `
declare "node_logs" {
  argument "logs_destinations" {}

  loki.source.journal "node" {
    forward_to = argument.logs_destinations.value
  }

  export "receiver" {
    value = loki.secretfilter.default.receiver
  }

  loki.secretfilter "default" {
    forward_to = argument.logs_destinations.value
  }
}

declare "cluster_metrics" {
  argument "metrics_destinations" {}
  argument "targets" {}

  prometheus.scrape "nodes" {
    targets    = argument.targets.value
    forward_to = argument.metrics_destinations.value
  }
}

node_logs "feature" {
  logs_destinations = [loki.write.default.receiver]
}

loki.source.file "app" {
  targets    = []
  forward_to = [node_logs.feature.receiver]
}

discovery.kubernetes "nodes" {
  role = "node"
}

cluster_metrics "feature" {
  targets              = discovery.kubernetes.nodes.targets
  metrics_destinations = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  endpoint {
    url = "https://prom.example.com/api/prom/push"
  }
}

loki.write "default" {
  endpoint {
    url = "https://logs.example.com/loki/api/v1/push"
  }
}
`

func TestRulesInModules(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rule    string
		passed  bool
		message string
	}{
		{
			name: "a name selects components in modules",
			rule: `
type: attribute
select: prometheus.scrape
path: clustering.enabled
equals: true`,
			message: "cluster_metrics/prometheus.scrape.nodes (config.alloy:22): clustering.enabled is not set",
		},
		{
			name: "an ID selects one component in a module",
			rule: `
type: count
select: cluster_metrics/prometheus.scrape.nodes
min: 1`,
			passed: true,
		},
		{
			name: "a module pattern selects every component in it",
			rule: `
type: count
select: "node_logs/*"
min: 2
max: 2`,
			passed: true,
		},
		{
			name: "* doesn't cross the module path in an ID",
			rule: `
type: count
select: "*.nodes"
max: 1`,
			passed: true,
		},
		{
			name: "flows-through follows a module argument",
			rule: `
type: flows-through
from: loki.source.*
to: loki.write
through: loki.secretfilter`,
			message: "node_logs/loki.source.journal.node (config.alloy:5) reaches loki.write.default without loki.secretfilter: node_logs/loki.source.journal.node -> node_logs/argument.logs_destinations -> loki.write.default",
		},
		{
			name: "flows-through follows a module export",
			rule: `
type: flows-through
from: loki.source.file
to: loki.write
through: loki.secretfilter`,
			passed: true,
		},
		{
			name: "flows-through follows pulled exports into a module",
			rule: `
type: flows-through
from: discovery.kubernetes
to: prometheus.remote_write
through: prometheus.scrape`,
			passed: true,
		},
		{
			name: "flows-through finds a path into a module",
			rule: `
type: flows-through
from: discovery.kubernetes
to: prometheus.remote_write
through: prometheus.relabel`,
			message: "discovery.kubernetes.nodes -> cluster_metrics/argument.targets -> cluster_metrics/prometheus.scrape.nodes -> cluster_metrics/argument.metrics_destinations -> prometheus.remote_write.default",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var chk Check
			require.NoError(t, yaml.Unmarshal([]byte(tc.rule), &chk))
			require.NoError(t, chk.Validate())
			res, err := Run("config.alloy", []byte(moduleConfig), []Rule{{Name: "r", Check: chk}})
			require.NoError(t, err)
			got := res.Rules[0]
			require.Equal(t, tc.passed, got.Passed, got.Message)
			if tc.message != "" {
				require.Contains(t, got.Message, tc.message)
			}
		})
	}
}

package diagnosis

import (
	"context"
	"strings"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type diagnosis struct {
	logger     log.Logger
	registerer prometheus.Registerer
	metrics    *metrics
	enabled    bool
	tree       map[string]map[string]*node // key is the component name, second key is the component id
	nodes      []*node
	roots      []*node
}

type metrics struct {
	errors   prometheus.Gauge
	warnings prometheus.Gauge
	tips     prometheus.Gauge
}

func newDiagnosis(l log.Logger, r prometheus.Registerer) *diagnosis {
	return &diagnosis{
		logger:     l,
		registerer: r,
	}
}

func (d *diagnosis) run(ctx context.Context, host service.Host) error {
	// TODO: handle modules
	components, err := host.ListComponents("", component.InfoOptions{GetArguments: true})
	if err != nil {
		return err
	}

	d.buildGraph(components)

	insights := d.applyRules()
	errors, warnings, tips := 0, 0, 0
	for _, insight := range insights {
		switch insight.Level {
		case LevelError:
			level.Error(d.logger).Log("msg", insight.Msg)
			errors++
		case LevelWarning:
			level.Warn(d.logger).Log("msg", insight.Msg)
			warnings++
		case LevelTips:
			level.Info(d.logger).Log("msg", insight.Msg)
			tips++
		}
	}
	d.metrics.errors.Set(float64(errors))
	d.metrics.warnings.Set(float64(warnings))
	d.metrics.tips.Set(float64(tips))
	<-ctx.Done()
	return nil
}

func (d *diagnosis) applyRules() []insight {
	insights := make([]insight, 0)
	for _, rule := range rules {
		insights = rule(d, insights)
	}
	return insights
}

func (d *diagnosis) registerMetrics() {
	prom := promauto.With(d.registerer)
	d.metrics = &metrics{
		errors:   prom.NewGauge(prometheus.GaugeOpts{Name: "diagnosis_errors_total"}),
		warnings: prom.NewGauge(prometheus.GaugeOpts{Name: "diagnosis_warnings_total"}),
		tips:     prom.NewGauge(prometheus.GaugeOpts{Name: "diagnosis_tips_total"}),
	}
}

func (d *diagnosis) buildGraph(components []*component.Info) {
	d.tree = make(map[string]map[string]*node, 0)
	d.nodes = make([]*node, 0)
	d.roots = make([]*node, 0)
	for _, c := range components {
		if _, ok := d.tree[c.ComponentName]; !ok {
			d.tree[c.ComponentName] = make(map[string]*node, 0)
		}
		node := &node{
			info:        c,
			connections: make([]*node, 0),
		}
		d.tree[c.ComponentName][c.ID.LocalID] = node
		d.nodes = append(d.nodes, node)
	}

	destNode := make(map[string]struct{})
	for _, c := range components {
		if strings.HasPrefix(c.ID.LocalID, "prometheus.exporter") || strings.HasPrefix(c.ID.LocalID, "discovery") {
			for _, ref := range c.ReferencedBy {
				refCpName := getNameFromID(ref)
				d.tree[c.ComponentName][c.ID.LocalID].connections = append(d.tree[c.ComponentName][c.ID.LocalID].connections, d.tree[refCpName][ref])
				destNode[ref] = struct{}{}
			}
		} else {
			for _, ref := range c.References {
				if strings.HasPrefix(ref, "prometheus.exporter") || strings.HasPrefix(ref, "discovery") {
					continue
				}
				refCpName := getNameFromID(ref)
				d.tree[c.ComponentName][c.ID.LocalID].connections = append(d.tree[c.ComponentName][c.ID.LocalID].connections, d.tree[refCpName][ref])
				destNode[ref] = struct{}{}
			}
		}
	}

	for _, node := range d.nodes {
		if _, ok := destNode[node.info.ID.LocalID]; !ok {
			d.roots = append(d.roots, node)
		}
	}
}

// TODO: should we unregister the metrics when disabled?
func (d *diagnosis) SetEnabled(enabled bool) {
	d.enabled = enabled
	if enabled {
		d.registerMetrics()
	}
}

// TODO: remove this
func (d *diagnosis) printGraph() {
	var printNode func(node *node, depth int)
	printNode = func(node *node, depth int) {
		indent := strings.Repeat("  ", depth)
		componentID := node.info.ID.LocalID
		componentName := node.info.ComponentName

		println(indent + "- " + componentName + " (" + componentID + ")")

		for _, conn := range node.connections {
			printNode(conn, depth+1)
		}
	}

	// Print all root nodes
	println("Component Graph:")
	for _, root := range d.roots {
		printNode(root, 0)
	}
}

func getNameFromID(s string) string {
	lastDotIndex := strings.LastIndex(s, ".")
	if lastDotIndex == -1 {
		return s
	}
	return s[:lastDotIndex]
}

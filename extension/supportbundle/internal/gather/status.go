package gather

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
	"gopkg.in/yaml.v3"
)

// componentStatus is the recorded status of one component instance.
type componentStatus struct {
	Component string    `yaml:"component"`
	Kind      string    `yaml:"kind"`
	Pipelines []string  `yaml:"pipelines,omitempty"`
	Status    string    `yaml:"status"`
	Error     string    `yaml:"error,omitempty"`
	Timestamp time.Time `yaml:"timestamp"`
}

// Status records component status events and writes them to the bundle. The
// collector delivers events through the extension, which forwards them to
// Record. The gatherer keeps the latest status for each component instance.
type Status struct {
	mu       sync.Mutex
	statuses map[string]componentStatus
}

func NewStatus() *Status {
	return &Status{statuses: make(map[string]componentStatus)}
}

func (*Status) Name() string { return "component-status" }

// Record stores the latest status for the source component.
func (g *Status) Record(source *componentstatus.InstanceID, event *componentstatus.Event) {
	var pipelines []string
	source.AllPipelineIDs(func(id pipeline.ID) bool {
		pipelines = append(pipelines, id.String())
		return true
	})
	sort.Strings(pipelines)

	cs := componentStatus{
		Component: source.ComponentID().String(),
		Kind:      source.Kind().String(),
		Pipelines: pipelines,
		Status:    event.Status().String(),
		Timestamp: event.Timestamp(),
	}
	if err := event.Err(); err != nil {
		cs.Error = err.Error()
	}

	key := cs.Kind + "|" + cs.Component + "|" + strings.Join(pipelines, ",")

	g.mu.Lock()
	g.statuses[key] = cs
	g.mu.Unlock()
}

func (g *Status) Gather(_ context.Context, _ Options) ([]File, error) {
	g.mu.Lock()
	list := make([]componentStatus, 0, len(g.statuses))
	for _, cs := range g.statuses {
		list = append(list, cs)
	}
	g.mu.Unlock()

	if len(list) == 0 {
		// The collector has not reported any component status yet.
		return nil, nil
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].Kind != list[j].Kind {
			return list[i].Kind < list[j].Kind
		}
		if list[i].Component != list[j].Component {
			return list[i].Component < list[j].Component
		}
		return strings.Join(list[i].Pipelines, ",") < strings.Join(list[j].Pipelines, ",")
	})

	data, err := yaml.Marshal(list)
	if err != nil {
		return nil, err
	}
	return []File{{Path: "component-status.yaml", Content: data}}, nil
}

package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/grafana/alloy/internal/alloy/logging/level"
	"github.com/grafana/alloy/internal/featuregate"
	httpgo "net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service/http"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.test.metrics",
		Args:      Arguments{},
		Exports:   Exports{},
		Stability: featuregate.StabilityExperimental,
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewComponent(opts, args.(Arguments))
		},
	})
}

type Component struct {
	mut        sync.Mutex
	args       Arguments
	o          component.Options
	instances  []*instance
	argsUpdate chan Arguments
	path       string
	handler    http.Data
}

// Handler should return a valid HTTP handler for the component.
// All requests to the component will have the path trimmed such that the component is at the root.
func (c *Component) Handler() httpgo.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/discovery", c.discovery)
	r.HandleFunc("/instance/{id}/metrics", c.serveMetrics)
	return r
}

func (c *Component) discovery(w httpgo.ResponseWriter, r *httpgo.Request) {
	w.Header().Set("Content-Type", "application/json")
	instances := make([]target, len(c.instances))
	for x := range c.instances {
		instances[x] = createTarget(c.handler.HTTPListenAddr, fmt.Sprintf("%sinstance/%d/metrics", c.path, x))
	}
	marshalledBytes, err := json.Marshal(instances)
	if err != nil {
		level.Error(c.o.Logger).Log("msg", "error marshalling discovery", "err", err)
		return
	}
	_, err = w.Write(marshalledBytes)
	if err != nil {
		level.Error(c.o.Logger).Log("msg", "error writing discovery", "err", err)
		return
	}
}

func (c *Component) serveMetrics(w httpgo.ResponseWriter, r *httpgo.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		level.Error(c.o.Logger).Log("msg", "id not found", "id", vars["id"], "err", err)
		w.WriteHeader(httpgo.StatusNotFound)
		return
	}
	_, _ = w.Write(c.instances[id].buffer())
}

func NewComponent(o component.Options, c Arguments) (*Component, error) {
	data, err := o.GetServiceData(http.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get HTTP information: %w", err)
	}
	httpData := data.(http.Data)
	fullpath := httpData.HTTPPathForComponent(o.ID)
	return &Component{
		o:          o,
		args:       c,
		path:       fullpath,
		argsUpdate: make(chan Arguments),
		instances:  make([]*instance, 0),
		handler:    httpData,
	}, nil
}

func (c *Component) Run(ctx context.Context) error {
	forceNewSet := true
	c.generateNewSet(forceNewSet)
	for {
		c.mut.Lock()
		t := time.NewTicker(c.args.MetricsRefresh)
		c.mut.Unlock()
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			{
				c.generateNewSet(forceNewSet)
				forceNewSet = false
			}
		case a := <-c.argsUpdate:
			{
				// If instance count changed we need to force a new update
				forceNewSet = a.NumberOfInstances != c.args.NumberOfInstances
				c.args = a
			}
		}
	}
}

func (c *Component) Update(args component.Arguments) error {
	c.argsUpdate <- args.(Arguments)
	return nil
}

type Arguments struct {
	NumberOfInstances int           `alloy:"number_of_instances,attr"`
	NumberOfMetrics   int           `alloy:"number_of_metrics,attr"`
	NumberOfSeries    int           `alloy:"number_of_labels,attr"`
	MetricsRefresh    time.Duration `alloy:"metrics_refresh,attr"`
	ChurnPercent      float32       `alloy:"churn_percent,attr"`
}

func getDefault() Arguments {
	return Arguments{
		NumberOfInstances: 1,
		NumberOfMetrics:   1,
		NumberOfSeries:    1,
		MetricsRefresh:    1 * time.Minute,
		ChurnPercent:      0,
	}
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = getDefault()
}

// Validate returns whether args is valid.
func (a *Arguments) Validate() error {
	if a.NumberOfInstances <= 0 {
		return fmt.Errorf("number_of_instances must be positive and is %d", a.NumberOfInstances)
	}
	if a.NumberOfMetrics <= 0 {
		return fmt.Errorf("number_of_metrics must be positive and is %d", a.NumberOfMetrics)
	}
	if a.NumberOfSeries < 0 {
		return fmt.Errorf("number_of_seires must not be negative and is %d", a.NumberOfSeries)
	}
	if a.ChurnPercent < 0 || a.ChurnPercent > 1 {
		return fmt.Errorf("churn_percent must be between 0 and 1 and is %f", a.ChurnPercent)
	}

	return nil
}

type Exports struct {
	Targets []map[string]string `alloy:"targets,attr,optional"`
}

// generateNewSet  creates the buffers of data. forceNewInstances instantiates all new buffers.
func (c *Component) generateNewSet(forceNewInstances bool) {
	c.mut.Lock()
	defer c.mut.Unlock()

	if len(c.instances) == 0 || forceNewInstances {
		c.instances = make([]*instance, c.args.NumberOfInstances)
		for i := 0; i < len(c.instances); i++ {
			c.instances[i] = &instance{
				start: 0,
				end:   c.args.NumberOfMetrics,
				id:    i,
			}
		}
	} else {
		for _, i := range c.instances {
			i.churn(c.args.ChurnPercent)
		}
	}
	for _, i := range c.instances {
		i.generateData(c.args.NumberOfSeries)
	}
	if forceNewInstances {
		targets := make([]map[string]string, len(c.instances))
		for x, _ := range c.instances {
			targets[x] = make(map[string]string)
			targets[x]["__address__"] = c.handler.HTTPListenAddr
			targets[x]["__metrics_path__"] = c.path + fmt.Sprintf("instance/%d/metrics", x)
		}
		c.o.OnStateChange(Exports{Targets: targets})
	}
}

func (c *Component) data() [][]byte {
	c.mut.Lock()
	defer c.mut.Unlock()

	bb := make([][]byte, len(c.instances))
	for x, i := range c.instances {
		bb[x] = make([]byte, len(i.buf))
		copy(bb[x], i.buf)
	}
	return bb
}

type instance struct {
	mut   sync.RWMutex
	start int
	end   int
	id    int
	buf   []byte
}

func (i *instance) buffer() []byte {
	i.mut.RLock()
	defer i.mut.RUnlock()

	retBuf := make([]byte, len(i.buf))
	copy(retBuf, i.buf)
	return retBuf
}

func (i *instance) churn(churn float32) {
	i.mut.Lock()
	defer i.mut.Unlock()

	if churn == 0 || churn > 1 {
		return
	}
	// This adjusts the ids by the churn rate, so if there was 10 ids and 10% churn it would move them forward one.
	adjust := int(float32(i.end-i.start) * churn)
	i.start = i.start + adjust
	i.end = i.end + adjust
}

func (i *instance) generateData(seriesCount int) {
	i.mut.Lock()
	defer i.mut.Unlock()

	buf := bytes.NewBuffer(nil)
	for j := i.start; j < i.end; j++ {
		buf.WriteString(fmt.Sprintf("# TYPE agent_metric_test_%d counter\n", j))
		buf.WriteString(fmt.Sprintf("agent_metric_test_%d{", j))
		series := make([]string, 0)
		for s := 0; s < seriesCount; s++ {
			series = append(series, fmt.Sprintf("series_%d=\"value_%d\"", s, s))
		}
		series = append(series, fmt.Sprintf("instance_id=\"%d\"", i.id))
		lblstring := strings.Join(series, ",")
		buf.WriteString(lblstring)
		buf.WriteString("} 1\n")
	}
	i.buf = buf.Bytes()
}

func createTarget(host, path string) target {
	return target{
		Host: []string{host},
		Labels: map[string]string{
			"__metrics_path__": path,
		},
	}
}

type target struct {
	Host   []string          `json:"targets"`
	Labels map[string]string `json:"labels"`
}

var _ http.Component = (*Component)(nil)

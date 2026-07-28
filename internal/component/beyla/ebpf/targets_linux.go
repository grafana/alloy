//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"fmt"
	"os"
	"path"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/discovery"
	http_service "github.com/grafana/alloy/internal/service/http"
)

func (c *Component) publishExports() error {
	baseTarget, err := c.baseTarget()

	if err != nil {
		return err
	}

	c.opts.OnStateChange(Exports{
		Targets: []discovery.Target{baseTarget},
	})

	return nil
}

func (c *Component) registerMetrics(reg prometheus.Registerer) error {
	subReg := prometheus.WrapRegistererWith(prometheus.Labels{"subprocess": "beyla"}, reg)

	opts := collectors.ProcessCollectorOpts{
		PidFn:        c.subprocessPid,
		Namespace:    "alloy_resources",
		ReportErrors: false,
	}

	return subReg.Register(collectors.NewProcessCollector(opts))
}

func (c *Component) subprocessPid() (int, error) {
	if pid, ok := c.subprocess.Pid(); ok {
		return pid, nil
	}

	return 0, fmt.Errorf("subprocess not running")
}

func (c *Component) baseTarget() (discovery.Target, error) {
	data, err := c.opts.GetServiceData(http_service.ServiceName)

	if err != nil {
		return discovery.EmptyTarget, fmt.Errorf("failed to get HTTP information: %w", err)
	}

	httpData := data.(http_service.Data)

	return discovery.NewTargetFromMap(map[string]string{
		model.AddressLabel:     httpData.MemoryListenAddr,
		model.SchemeLabel:      "http",
		model.MetricsPathLabel: path.Join(httpData.HTTPPathForComponent(c.opts.ID), "metrics"),
		"instance":             defaultInstance(),
		"job":                  "beyla",
	}), nil
}

func defaultInstance() string {
	hostname := os.Getenv("HOSTNAME")

	if hostname != "" {
		return hostname
	}

	hostname, err := os.Hostname()

	if err != nil {
		return "unknown"
	}

	return hostname
}

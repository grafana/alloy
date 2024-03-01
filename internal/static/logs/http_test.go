package logs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grafana/agent/internal/util"
	"github.com/grafana/loki/clients/pkg/promtail/targets/target"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestAgent_ListInstancesHandler(t *testing.T) {
	cfgText := util.Untab(`
configs:
- name: instance-a
  positions:
    filename: /tmp/positions.yaml
  clients:
	- url: http://127.0.0.1:80/loki/api/v1/push
	`)

	var cfg Config

	logger := util.TestLogger(t)
	l, err := New(prometheus.NewRegistry(), &cfg, logger, false)
	require.NoError(t, err)
	defer l.Stop()

	r := httptest.NewRequest("GET", "/agent/api/v1/logs/instances", nil)

	t.Run("no instances", func(t *testing.T) {
		rr := httptest.NewRecorder()
		l.ListInstancesHandler(rr, r)
		expect := `{"status":"success","data":[]}`
		require.Equal(t, expect, rr.Body.String())
	})

	dec := yaml.NewDecoder(strings.NewReader(cfgText))
	dec.SetStrict(true)
	require.NoError(t, dec.Decode(&cfg))
	t.Run("non-empty", func(t *testing.T) {
		require.NoError(t, l.ApplyConfig(&cfg, false))

		expect := `{"status":"success","data":["instance-a"]}`

		util.Eventually(t, func(t require.TestingT) {
			rr := httptest.NewRecorder()
			l.ListInstancesHandler(rr, r)
			require.Equal(t, expect, rr.Body.String())
		})
	})
}

func TestAgent_ListTargetsHandler(t *testing.T) {
	cfgText := util.Untab(`
configs:
- name: instance-a
  positions:
    filename: /tmp/positions.yaml
  clients:
	- url: http://127.0.0.1:80/loki/api/v1/push
	`)

	var cfg Config
	dec := yaml.NewDecoder(strings.NewReader(cfgText))
	dec.SetStrict(true)
	require.NoError(t, dec.Decode(&cfg))

	logger := util.TestLogger(t)
	l, err := New(prometheus.NewRegistry(), &cfg, logger, false)
	require.NoError(t, err)
	defer l.Stop()

	r := httptest.NewRequest("GET", "/agent/api/v1/logs/targets", nil)

	t.Run("scrape manager not ready", func(t *testing.T) {
		rr := httptest.NewRecorder()
		l.ListTargetsHandler(rr, r)
		expect := `{"status": "success", "data": []}`
		require.JSONEq(t, expect, rr.Body.String())
		require.Equal(t, http.StatusOK, rr.Result().StatusCode)
	})

	t.Run("scrape manager targets", func(t *testing.T) {
		rr := httptest.NewRecorder()
		targets := map[string]TargetSet{
			"instance-a": mockActiveTargets(),
		}
		listTargetsHandler(targets).ServeHTTP(rr, r)
		expect := `{
			"status": "success",
			"data": [
				{
				  "instance": "instance-a",
				  "target_group": "varlogs",
				  "type": "File",
				  "labels": {
					"job": "varlogs"
				  },
				  "discovered_labels": {
					"__address__": "localhost",
					"__path__": "/var/log/*log",
					"job": "varlogs"
				  },
				  "ready": true,
				  "details": {
					"/var/log/alternatives.log": 13386,
					"/var/log/apport.log": 0,
					"/var/log/auth.log": 37009,
					"/var/log/bootstrap.log": 107347,
					"/var/log/dpkg.log": 374420,
					"/var/log/faillog": 0,
					"/var/log/fontconfig.log": 11629,
					"/var/log/gpu-manager.log": 1541,
					"/var/log/kern.log": 782582,
					"/var/log/lastlog": 0,
					"/var/log/syslog": 788450
				  }
				}
			]  
		}`
		require.JSONEq(t, expect, rr.Body.String())
		require.Equal(t, http.StatusOK, rr.Result().StatusCode)
	})
}

func mockActiveTargets() map[string][]target.Target {
	return map[string][]target.Target{
		"varlogs": {&mockTarget{}},
	}
}

type mockTarget struct {
}

func (mt *mockTarget) Type() target.TargetType {
	return target.TargetType("File")
}

func (mt *mockTarget) DiscoveredLabels() model.LabelSet {
	return map[model.LabelName]model.LabelValue{
		"__address__": "localhost",
		"__path__":    "/var/log/*log",
		"job":         "varlogs",
	}
}

func (mt *mockTarget) Labels() model.LabelSet {
	return map[model.LabelName]model.LabelValue{
		"job": "varlogs",
	}
}

func (mt *mockTarget) Ready() bool {
	return true
}

func (mt *mockTarget) Details() interface{} {
	return map[string]int{
		"/var/log/alternatives.log": 13386,
		"/var/log/apport.log":       0,
		"/var/log/auth.log":         37009,
		"/var/log/bootstrap.log":    107347,
		"/var/log/dpkg.log":         374420,
		"/var/log/faillog":          0,
		"/var/log/fontconfig.log":   11629,
		"/var/log/gpu-manager.log":  1541,
		"/var/log/kern.log":         782582,
		"/var/log/lastlog":          0,
		"/var/log/syslog":           788450,
	}
}

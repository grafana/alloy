package tailscale_exporter

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kit/log"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	tsclient "tailscale.com/client/tailscale"
)

func TestNew_validation(t *testing.T) {
	logger := log.NewNopLogger()

	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "missing tailnet",
			cfg:     Config{APIKey: "key", AuthKey: "authkey"},
			wantErr: "tailnet is required",
		},
		{
			name:    "missing api_key",
			cfg:     Config{Tailnet: "example.com", AuthKey: "authkey"},
			wantErr: "api_key is required",
		},
		{
			name:    "missing auth_key",
			cfg:     Config{Tailnet: "example.com", APIKey: "key"},
			wantErr: "auth_key is required",
		},
		{
			name:    "valid config",
			cfg:     Config{Tailnet: "example.com", APIKey: "key", AuthKey: "authkey"},
			wantErr: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(logger, tc.cfg)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNew_defaults(t *testing.T) {
	i, err := New(log.NewNopLogger(), Config{
		Tailnet: "example.com",
		APIKey:  "key",
		AuthKey: "authkey",
	})
	require.NoError(t, err)
	require.Equal(t, defaultAPIBaseURL, i.cfg.APIBaseURL)
	require.Equal(t, defaultRefreshInterval, i.cfg.RefreshInterval)
	require.Equal(t, defaultPeerMetricsPort, i.cfg.PeerMetricsPort)
	require.Equal(t, defaultPeerMetricsPath, i.cfg.PeerMetricsPath)
	require.Equal(t, defaultPeerScrapeTimeout, i.cfg.PeerScrapeTimeout)
	require.Equal(t, defaultTSNetHostname, i.cfg.TSNetHostname)
}

func TestMetricsHandler_beforeFirstRefresh(t *testing.T) {
	i, err := New(log.NewNopLogger(), Config{
		Tailnet: "example.com",
		APIKey:  "key",
		AuthKey: "authkey",
	})
	require.NoError(t, err)

	h, err := i.MetricsHandler()
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestScrapeConfigs(t *testing.T) {
	i, _ := New(log.NewNopLogger(), Config{
		Tailnet: "example.com",
		APIKey:  "key",
		AuthKey: "authkey",
	})
	cfgs := i.ScrapeConfigs()
	require.Len(t, cfgs, 1)
	require.Equal(t, "tailscale", cfgs[0].JobName)
}

func TestPeerMetricsGatherer_empty(t *testing.T) {
	g := &peerMetricsGatherer{cache: map[string][]byte{}}
	families, err := g.Gather()
	require.NoError(t, err)
	require.Empty(t, families)
}

func TestParsePeerMetrics_counter(t *testing.T) {
	raw := []byte(`# HELP tailscaled_inbound_packets_total Inbound packets.
# TYPE tailscaled_inbound_packets_total counter
tailscaled_inbound_packets_total{path="derp"} 42
`)
	families, err := parsePeerMetrics(raw, "my-node")
	require.NoError(t, err)
	require.Len(t, families, 1)

	mf := families[0]
	require.Equal(t, "tailscaled_inbound_packets_total", mf.GetName())
	require.Len(t, mf.Metric, 1)

	// "node" label must be injected.
	nodeLabel := labelValue(mf.Metric[0].Label, "node")
	require.Equal(t, "my-node", nodeLabel)

	// Existing labels are preserved.
	pathLabel := labelValue(mf.Metric[0].Label, "path")
	require.Equal(t, "derp", pathLabel)
}

func TestParsePeerMetrics_multipleNodes(t *testing.T) {
	raw := []byte(`# HELP metric_a A gauge.
# TYPE metric_a gauge
metric_a 1
`)
	g := &peerMetricsGatherer{
		cache: map[string][]byte{
			"node-a": raw,
			"node-b": raw,
		},
	}
	families, err := g.Gather()
	require.NoError(t, err)
	require.Len(t, families, 2)

	seen := map[string]bool{}
	for _, mf := range families {
		for _, m := range mf.Metric {
			seen[labelValue(m.Label, "node")] = true
		}
	}
	require.True(t, seen["node-a"], "node-a label not found")
	require.True(t, seen["node-b"], "node-b label not found")
}

func TestParsePeerMetrics_malformed(t *testing.T) {
	// The key guarantee is that invalid Prometheus text does not panic and
	// that any metric families returned still have the node label injected.
	raw := []byte(`not valid prometheus text !!!`)
	families, _ := parsePeerMetrics(raw, "bad-node")
	for _, mf := range families {
		for _, m := range mf.Metric {
			require.Equal(t, "bad-node", labelValue(m.Label, "node"))
		}
	}
}

func TestParsePeerMetrics_preservesHelpAndType(t *testing.T) {
	raw := []byte(`# HELP my_gauge A test gauge.
# TYPE my_gauge gauge
my_gauge 3.14
`)
	families, err := parsePeerMetrics(raw, "n1")
	require.NoError(t, err)
	require.Len(t, families, 1)
	require.Equal(t, "my_gauge", families[0].GetName())
	require.Equal(t, "A test gauge.", families[0].GetHelp())
}

func TestParseTime(t *testing.T) {
	require.True(t, parseTime("").IsZero())
	require.True(t, parseTime("not-a-time").IsZero())

	ts := parseTime("2024-01-15T10:30:00Z")
	require.False(t, ts.IsZero())
	require.Equal(t, 2024, ts.Year())
}

func TestBoolToFloat(t *testing.T) {
	require.Equal(t, 1.0, boolToFloat(true))
	require.Equal(t, 0.0, boolToFloat(false))
}

func TestCopyPeerCache(t *testing.T) {
	orig := map[string][]byte{"a": []byte("data")}
	cp := copyPeerCache(orig)
	require.Equal(t, orig, cp)

	cp["b"] = []byte("extra")
	require.NotContains(t, orig, "b")
}

func TestRegisterAPIMetrics_emptyDevices(t *testing.T) {
	i, _ := New(log.NewNopLogger(), Config{
		Tailnet: "example.com",
		APIKey:  "key",
		AuthKey: "authkey",
	})
	reg := prometheus.NewRegistry()
	err := i.registerAPIMetrics(reg, nil)
	require.NoError(t, err)

	mfs, err := reg.Gather()
	require.NoError(t, err)

	total := familyValue(mfs, "tailscale_devices_total")
	require.Equal(t, 0.0, total)
}

func TestRegisterAPIMetrics_onlineDevice(t *testing.T) {
	i, _ := New(log.NewNopLogger(), Config{
		Tailnet: "example.com",
		APIKey:  "key",
		AuthKey: "authkey",
	})

	devices := []*tsclient.Device{
		{
			DeviceID:  "dev1",
			Name:      "mynode",
			Hostname:  "mynode",
			OS:        "linux",
			Addresses: []string{"100.64.1.1"},
			LastSeen:  time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339),
			Authorized: true,
		},
	}

	reg := prometheus.NewRegistry()
	err := i.registerAPIMetrics(reg, devices)
	require.NoError(t, err)

	mfs, err := reg.Gather()
	require.NoError(t, err)

	require.Equal(t, 1.0, familyMetricValue(mfs, "tailscale_device_online", "id", "dev1"))
	require.Equal(t, 1.0, familyValue(mfs, "tailscale_devices_online_total"))
	require.Equal(t, 1.0, familyValue(mfs, "tailscale_devices_authorized_total"))
	require.Equal(t, 1.0, familyValue(mfs, "tailscale_devices_total"))
}

func TestRegisterAPIMetrics_offlineDevice(t *testing.T) {
	i, _ := New(log.NewNopLogger(), Config{
		Tailnet: "example.com",
		APIKey:  "key",
		AuthKey: "authkey",
	})

	devices := []*tsclient.Device{
		{
			DeviceID:  "dev2",
			Name:      "oldnode",
			Hostname:  "oldnode",
			LastSeen:  time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339),
			Authorized: false,
		},
	}

	reg := prometheus.NewRegistry()
	err := i.registerAPIMetrics(reg, devices)
	require.NoError(t, err)

	mfs, err := reg.Gather()
	require.NoError(t, err)

	require.Equal(t, 0.0, familyMetricValue(mfs, "tailscale_device_online", "id", "dev2"))
	require.Equal(t, 0.0, familyValue(mfs, "tailscale_devices_online_total"))
	require.Equal(t, 0.0, familyValue(mfs, "tailscale_devices_authorized_total"))
}

// --- test helpers ---

// labelValue returns the value of the label with the given name from a slice
// of label pairs. Returns "" if not found.
func labelValue(labels []*dto.LabelPair, name string) string {
	for _, l := range labels {
		if l.GetName() == name {
			return l.GetValue()
		}
	}
	return ""
}

// familyValue returns the gauge value of the first metric in the named family.
func familyValue(mfs []*dto.MetricFamily, name string) float64 {
	for _, mf := range mfs {
		if mf.GetName() == name && len(mf.Metric) > 0 {
			if g := mf.Metric[0].Gauge; g != nil {
				return g.GetValue()
			}
		}
	}
	return -1
}

// familyMetricValue finds the first metric in family name where labelName=labelValue
// and returns its gauge value. Returns -1 if not found.
func familyMetricValue(mfs []*dto.MetricFamily, name, labelName, labelVal string) float64 {
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.Metric {
			if labelValue(m.Label, labelName) == labelVal {
				if g := m.Gauge; g != nil {
					return g.GetValue()
				}
			}
		}
	}
	return -1
}

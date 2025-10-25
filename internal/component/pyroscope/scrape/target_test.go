package scrape

import (
	"net/url"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func Test_targetsFromGroup(t *testing.T) {
	args := NewDefaultArguments()
	args.ProfilingConfig.Block.Enabled = false
	args.ProfilingConfig.Goroutine.Enabled = false
	args.ProfilingConfig.Mutex.Enabled = false

	active, err := targetsFromGroup(&targetgroup.Group{
		Targets: []model.LabelSet{
			{model.AddressLabel: "localhost:9090"},
			{model.AddressLabel: "localhost:9091", serviceNameLabel: "svc"},
			{model.AddressLabel: "localhost:9092", serviceNameK8SLabel: "k8s-svc"},
			{model.AddressLabel: "localhost:9093", "__meta_kubernetes_namespace": "ns", "__meta_kubernetes_pod_container_name": "container"},
			{model.AddressLabel: "localhost:9094", "__meta_docker_container_name": "docker-container"},
		},
		Labels: model.LabelSet{
			"foo": "bar",
		},
	}, args, args.ProfilingConfig.AllTargets())
	expected := []*Target{
		// unspecified
		NewTarget(labels.FromMap(map[string]string{
			model.AddressLabel:    "localhost:9090",
			serviceNameLabel:      "unspecified",
			model.MetricNameLabel: pprofMemory,
			ProfilePath:           "/debug/pprof/allocs",
			model.SchemeLabel:     "http",
			"foo":                 "bar",
			"instance":            "localhost:9090",
		}), url.Values{}),
		NewTarget(labels.FromMap(map[string]string{
			model.AddressLabel:    "localhost:9090",
			serviceNameLabel:      "unspecified",
			model.MetricNameLabel: pprofProcessCPU,
			ProfilePath:           "/debug/pprof/profile",
			model.SchemeLabel:     "http",
			"foo":                 "bar",
			"instance":            "localhost:9090",
		}), url.Values{"seconds": []string{"14"}}),

		// specified
		NewTarget(labels.FromMap(map[string]string{
			model.AddressLabel:    "localhost:9091",
			serviceNameLabel:      "svc",
			model.MetricNameLabel: pprofMemory,
			ProfilePath:           "/debug/pprof/allocs",
			model.SchemeLabel:     "http",
			"foo":                 "bar",
			"instance":            "localhost:9091",
		}), url.Values{}),
		NewTarget(labels.FromMap(map[string]string{
			model.AddressLabel:    "localhost:9091",
			serviceNameLabel:      "svc",
			model.MetricNameLabel: pprofProcessCPU,
			ProfilePath:           "/debug/pprof/profile",
			model.SchemeLabel:     "http",
			"foo":                 "bar",
			"instance":            "localhost:9091",
		}), url.Values{"seconds": []string{"14"}}),

		// k8s annotation specified
		NewTarget(labels.FromMap(map[string]string{
			model.AddressLabel:    "localhost:9092",
			serviceNameLabel:      "k8s-svc",
			model.MetricNameLabel: pprofMemory,
			ProfilePath:           "/debug/pprof/allocs",
			model.SchemeLabel:     "http",
			"foo":                 "bar",
			"instance":            "localhost:9092",
			serviceNameK8SLabel:   "k8s-svc",
		}), url.Values{}),
		NewTarget(labels.FromMap(map[string]string{
			model.AddressLabel:    "localhost:9092",
			serviceNameLabel:      "k8s-svc",
			model.MetricNameLabel: pprofProcessCPU,
			ProfilePath:           "/debug/pprof/profile",
			model.SchemeLabel:     "http",
			"foo":                 "bar",
			"instance":            "localhost:9092",
			serviceNameK8SLabel:   "k8s-svc",
		}), url.Values{"seconds": []string{"14"}}),

		// unspecified, infer from k8s
		NewTarget(labels.FromMap(map[string]string{
			model.AddressLabel:                     "localhost:9093",
			serviceNameLabel:                       "ns/container",
			model.MetricNameLabel:                  pprofMemory,
			ProfilePath:                            "/debug/pprof/allocs",
			model.SchemeLabel:                      "http",
			"foo":                                  "bar",
			"instance":                             "localhost:9093",
			"__meta_kubernetes_namespace":          "ns",
			"__meta_kubernetes_pod_container_name": "container",
		}), url.Values{}),
		NewTarget(labels.FromMap(map[string]string{
			model.AddressLabel:                     "localhost:9093",
			serviceNameLabel:                       "ns/container",
			model.MetricNameLabel:                  pprofProcessCPU,
			ProfilePath:                            "/debug/pprof/profile",
			model.SchemeLabel:                      "http",
			"foo":                                  "bar",
			"instance":                             "localhost:9093",
			"__meta_kubernetes_namespace":          "ns",
			"__meta_kubernetes_pod_container_name": "container",
		}), url.Values{"seconds": []string{"14"}}),

		// unspecified, infer from docker
		NewTarget(labels.FromMap(map[string]string{
			model.AddressLabel:             "localhost:9094",
			serviceNameLabel:               "docker-container",
			model.MetricNameLabel:          pprofMemory,
			ProfilePath:                    "/debug/pprof/allocs",
			model.SchemeLabel:              "http",
			"foo":                          "bar",
			"instance":                     "localhost:9094",
			"__meta_docker_container_name": "docker-container",
		}), url.Values{}),
		NewTarget(labels.FromMap(map[string]string{
			model.AddressLabel:             "localhost:9094",
			serviceNameLabel:               "docker-container",
			model.MetricNameLabel:          pprofProcessCPU,
			ProfilePath:                    "/debug/pprof/profile",
			model.SchemeLabel:              "http",
			"foo":                          "bar",
			"instance":                     "localhost:9094",
			"__meta_docker_container_name": "docker-container",
		}), url.Values{"seconds": []string{"14"}}),
	}
	require.NoError(t, err)
	sort.Sort(Targets(active))
	sort.Sort(Targets(expected))
	require.Equal(t, expected, active)
}

// Test that the godeltaprof is not surfaced publicly
func Test_NewTarget_godeltaprof(t *testing.T) {
	withGodeltaprof := NewTarget(labels.FromMap(map[string]string{
		model.AddressLabel:    "localhost:9094",
		serviceNameLabel:      "docker-container",
		model.MetricNameLabel: pprofGoDeltaProfMemory,
		ProfilePath:           "/debug/pprof/delta_heap",
		model.SchemeLabel:     "http",
		"foo":                 "bar",
		"instance":            "localhost:9094",
	}), url.Values{})
	withoutGodeltaprof := NewTarget(labels.FromMap(map[string]string{
		model.AddressLabel:    "localhost:9094",
		serviceNameLabel:      "docker-container",
		model.MetricNameLabel: pprofMemory,
		ProfilePath:           "/debug/pprof/heap",
		model.SchemeLabel:     "http",
		"foo":                 "bar",
		"instance":            "localhost:9094",
	}), url.Values{})

	require.NotEqual(t, withGodeltaprof.allLabels, withoutGodeltaprof.allLabels)
	assert.Equal(t, pprofMemory, withGodeltaprof.allLabels.Get(model.MetricNameLabel))
	assert.Equal(t, pprofMemory, withoutGodeltaprof.allLabels.Get(model.MetricNameLabel))
	assert.Equal(t, "/debug/pprof/heap", withoutGodeltaprof.allLabels.Get(ProfilePath))
	assert.Equal(t, "/debug/pprof/delta_heap", withGodeltaprof.allLabels.Get(ProfilePath))
}

func Test_targetsFromGroup_withSpecifiedDeltaProfilingDuration(t *testing.T) {
	args := NewDefaultArguments()
	args.ProfilingConfig.Block.Enabled = false
	args.ProfilingConfig.Goroutine.Enabled = false
	args.ProfilingConfig.Mutex.Enabled = false
	args.DeltaProfilingDuration = 20 * time.Second

	active, err := targetsFromGroup(&targetgroup.Group{
		Targets: []model.LabelSet{
			{model.AddressLabel: "localhost:9090"},
		},
		Labels: model.LabelSet{
			"foo": "bar",
		},
	}, args, args.ProfilingConfig.AllTargets())
	expected := []*Target{
		// unspecified
		NewTarget(labels.FromMap(map[string]string{
			model.AddressLabel:    "localhost:9090",
			serviceNameLabel:      "unspecified",
			model.MetricNameLabel: pprofMemory,
			ProfilePath:           "/debug/pprof/allocs",
			model.SchemeLabel:     "http",
			"foo":                 "bar",
			"instance":            "localhost:9090",
		}), url.Values{}),
		NewTarget(labels.FromMap(map[string]string{
			model.AddressLabel:    "localhost:9090",
			serviceNameLabel:      "unspecified",
			model.MetricNameLabel: pprofProcessCPU,
			ProfilePath:           "/debug/pprof/profile",
			model.SchemeLabel:     "http",
			"foo":                 "bar",
			"instance":            "localhost:9090",
		}), url.Values{"seconds": []string{"20"}}),
	}

	require.NoError(t, err)
	sort.Sort(Targets(active))
	sort.Sort(Targets(expected))
	require.Equal(t, expected, active)
}

func TestProfileURL(t *testing.T) {
	targets := func(t *testing.T, args Arguments, ls []model.LabelSet) []*Target {
		active, err := targetsFromGroup(&targetgroup.Group{
			Targets: ls,
		}, args, args.ProfilingConfig.AllTargets())
		require.NoError(t, err)
		require.NotEmpty(t, active)
		return active
	}
	testdata := []struct {
		name         string
		args         func() Arguments
		targets      []model.LabelSet
		expectedUrls []string
	}{
		{
			name: "single cpu only target",
			args: func() Arguments {
				args := NewDefaultArguments()
				args.ProfilingConfig = ProfilingConfig{
					ProcessCPU: ProfilingTarget{
						Enabled: true,
						Path:    "/debug/pprof/profile",
						Delta:   true,
					},
				}
				return args
			},
			targets:      []model.LabelSet{{model.AddressLabel: "localhost:9090"}},
			expectedUrls: []string{"http://localhost:9090/debug/pprof/profile?seconds=14"},
		},
		{
			name:    "default profiling config targets from single target",
			args:    NewDefaultArguments,
			targets: []model.LabelSet{{model.AddressLabel: "localhost:9090"}},
			expectedUrls: []string{
				"http://localhost:9090/debug/pprof/allocs",
				"http://localhost:9090/debug/pprof/block",
				"http://localhost:9090/debug/pprof/goroutine",
				"http://localhost:9090/debug/pprof/mutex",
				"http://localhost:9090/debug/pprof/profile?seconds=14",
			},
		},
		{
			name: "default config, https scheme label",
			args: NewDefaultArguments,
			targets: []model.LabelSet{{
				model.AddressLabel: "localhost:9090",
				model.SchemeLabel:  "https",
			}},
			expectedUrls: []string{
				"https://localhost:9090/debug/pprof/allocs",
				"https://localhost:9090/debug/pprof/block",
				"https://localhost:9090/debug/pprof/goroutine",
				"https://localhost:9090/debug/pprof/mutex",
				"https://localhost:9090/debug/pprof/profile?seconds=14",
			},
		},
		{
			name: "default config, https scheme label, no port",
			args: NewDefaultArguments,
			targets: []model.LabelSet{{
				model.AddressLabel: "localhost",
				model.SchemeLabel:  "https",
			}},
			expectedUrls: []string{
				"https://localhost:443/debug/pprof/allocs",
				"https://localhost:443/debug/pprof/block",
				"https://localhost:443/debug/pprof/goroutine",
				"https://localhost:443/debug/pprof/mutex",
				"https://localhost:443/debug/pprof/profile?seconds=14",
			},
		},
		{
			name: "default config, http scheme label, no port",
			args: NewDefaultArguments,
			targets: []model.LabelSet{{
				model.AddressLabel: "localhost",
				model.SchemeLabel:  "http",
			}},
			expectedUrls: []string{
				"http://localhost:80/debug/pprof/allocs",
				"http://localhost:80/debug/pprof/block",
				"http://localhost:80/debug/pprof/goroutine",
				"http://localhost:80/debug/pprof/mutex",
				"http://localhost:80/debug/pprof/profile?seconds=14",
			},
		},
		{
			name: "default config, no port",
			args: NewDefaultArguments,
			targets: []model.LabelSet{{
				model.AddressLabel: "localhost",
			}},
			expectedUrls: []string{
				"http://localhost:80/debug/pprof/allocs",
				"http://localhost:80/debug/pprof/block",
				"http://localhost:80/debug/pprof/goroutine",
				"http://localhost:80/debug/pprof/mutex",
				"http://localhost:80/debug/pprof/profile?seconds=14",
			},
		},
		{
			name: "config with https scheme, no port",
			args: func() Arguments {
				args := NewDefaultArguments()
				args.Scheme = "https"
				return args
			},
			targets: []model.LabelSet{{
				model.AddressLabel: "localhost",
			}},
			expectedUrls: []string{
				"https://localhost:443/debug/pprof/allocs",
				"https://localhost:443/debug/pprof/block",
				"https://localhost:443/debug/pprof/goroutine",
				"https://localhost:443/debug/pprof/mutex",
				"https://localhost:443/debug/pprof/profile?seconds=14",
			},
		},
		{
			name: "all enabled",
			args: func() Arguments {
				args := NewDefaultArguments()
				args.ProfilingConfig.GoDeltaProfBlock.Enabled = true
				args.ProfilingConfig.GoDeltaProfMutex.Enabled = true
				args.ProfilingConfig.GoDeltaProfMemory.Enabled = true
				args.ProfilingConfig.FGProf.Enabled = true
				args.ProfilingConfig.Custom = []CustomProfilingTarget{{
					Enabled: true,
					Path:    "/foo239",
					Name:    "custom",
				}}
				args.Scheme = "https"
				return args
			},
			targets: []model.LabelSet{{
				model.AddressLabel: "127.0.0.1:4100",
			}},
			expectedUrls: []string{
				"https://127.0.0.1:4100/debug/fgprof?seconds=14",
				"https://127.0.0.1:4100/debug/pprof/allocs",
				"https://127.0.0.1:4100/debug/pprof/block",
				"https://127.0.0.1:4100/debug/pprof/delta_block",
				"https://127.0.0.1:4100/debug/pprof/delta_heap",
				"https://127.0.0.1:4100/debug/pprof/delta_mutex",
				"https://127.0.0.1:4100/debug/pprof/goroutine",
				"https://127.0.0.1:4100/debug/pprof/mutex",
				"https://127.0.0.1:4100/debug/pprof/profile?seconds=14",
				"https://127.0.0.1:4100/foo239",
			},
		},
		{
			name: "all enabled with prefix",
			args: func() Arguments {
				args := NewDefaultArguments()
				args.ProfilingConfig.GoDeltaProfBlock.Enabled = true
				args.ProfilingConfig.GoDeltaProfMutex.Enabled = true
				args.ProfilingConfig.GoDeltaProfMemory.Enabled = true
				args.ProfilingConfig.FGProf.Enabled = true
				args.ProfilingConfig.Custom = []CustomProfilingTarget{{
					Enabled: true,
					Path:    "/foo239",
					Name:    "custom",
				}}
				args.Scheme = "https"
				return args
			},
			targets: []model.LabelSet{{
				model.AddressLabel: "127.0.0.1:4100",
				ProfilePathPrefix:  "/mimir-prometheus",
			}},
			expectedUrls: []string{
				"https://127.0.0.1:4100/mimir-prometheus/debug/fgprof?seconds=14",
				"https://127.0.0.1:4100/mimir-prometheus/debug/pprof/allocs",
				"https://127.0.0.1:4100/mimir-prometheus/debug/pprof/block",
				"https://127.0.0.1:4100/mimir-prometheus/debug/pprof/delta_block",
				"https://127.0.0.1:4100/mimir-prometheus/debug/pprof/delta_heap",
				"https://127.0.0.1:4100/mimir-prometheus/debug/pprof/delta_mutex",
				"https://127.0.0.1:4100/mimir-prometheus/debug/pprof/goroutine",
				"https://127.0.0.1:4100/mimir-prometheus/debug/pprof/mutex",
				"https://127.0.0.1:4100/mimir-prometheus/debug/pprof/profile?seconds=14",
				"https://127.0.0.1:4100/mimir-prometheus/foo239",
			},
		},
		{
			name: "path prefix argument",
			args: func() Arguments {
				args := NewDefaultArguments()
				args.ProfilingConfig = ProfilingConfig{
					ProcessCPU: ProfilingTarget{
						Enabled: true,
						Path:    "/debug/pprof/profile",
						Delta:   true,
					},
					PathPrefix: "/foo",
				}

				return args
			},
			targets:      []model.LabelSet{{model.AddressLabel: "localhost:9090"}},
			expectedUrls: []string{"http://localhost:9090/foo/debug/pprof/profile?seconds=14"},
		},
		{
			name: "path prefix argument overridden by target label",
			args: func() Arguments {
				args := NewDefaultArguments()
				args.ProfilingConfig = ProfilingConfig{
					ProcessCPU: ProfilingTarget{
						Enabled: true,
						Path:    "/debug/pprof/profile",
						Delta:   true,
					},
					PathPrefix: "/foo",
				}

				return args
			},
			targets: []model.LabelSet{
				{
					model.AddressLabel: "localhost:9090",
				},
				{
					model.AddressLabel: "localhost:4242",
					ProfilePathPrefix:  "/bar",
				},
			},
			expectedUrls: []string{
				"http://localhost:4242/bar/debug/pprof/profile?seconds=14",
				"http://localhost:9090/foo/debug/pprof/profile?seconds=14",
			},
		},
	}
	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			var actualURLs []string
			for _, tt := range targets(t, td.args(), td.targets) {
				actualURLs = append(actualURLs, tt.URL())
			}
			slices.Sort(td.expectedUrls)
			slices.Sort(actualURLs)
			require.Equal(t, td.expectedUrls, actualURLs)
		})
	}
}

func TestLabelsByProfiles(t *testing.T) {
	testdata := []struct {
		name     string
		target   labels.Labels
		cfg      *ProfilingConfig
		expected []labels.Labels
	}{
		{
			name: "single target",
			target: labels.FromMap(map[string]string{
				model.AddressLabel: "localhost:9090",
			}),
			cfg: &ProfilingConfig{
				ProcessCPU: ProfilingTarget{
					Enabled: true,
					Path:    "/debug/pprof/profile",
					Delta:   true,
				},
			},
			expected: []labels.Labels{
				labels.FromMap(map[string]string{
					ProfilePath:        "/debug/pprof/profile",
					ProfileName:        "process_cpu",
					model.AddressLabel: "localhost:9090",
				}),
			},
		},
		{
			name: "path prefix from args",
			target: labels.FromMap(map[string]string{
				model.AddressLabel: "localhost:9090",
			}),
			cfg: &ProfilingConfig{
				ProcessCPU: ProfilingTarget{
					Enabled: true,
					Path:    "/debug/pprof/profile",
					Delta:   true,
				},
				PathPrefix: "/foo",
			},
			expected: []labels.Labels{
				labels.FromMap(map[string]string{
					ProfilePath:        "/debug/pprof/profile",
					ProfileName:        "process_cpu",
					model.AddressLabel: "localhost:9090",
					ProfilePathPrefix:  "/foo",
				}),
			},
		},
		{
			name: "path prefix from label",
			target: labels.FromMap(map[string]string{
				model.AddressLabel: "localhost:9090",
				ProfilePathPrefix:  "/bar",
			}),
			cfg: &ProfilingConfig{
				ProcessCPU: ProfilingTarget{
					Enabled: true,
					Path:    "/debug/pprof/profile",
					Delta:   true,
				},
				PathPrefix: "/foo",
			},
			expected: []labels.Labels{
				labels.FromMap(map[string]string{
					ProfilePath:        "/debug/pprof/profile",
					ProfileName:        "process_cpu",
					model.AddressLabel: "localhost:9090",
					ProfilePathPrefix:  "/bar",
				}),
			},
		},
		{
			name: "no duplicates",
			target: labels.FromMap(map[string]string{
				model.AddressLabel: "localhost:9090",
				ProfilePath:        "/debug/pprof/custom_profile",
				ProfileName:        "custom_process_cpu",
				ProfilePathPrefix:  "/prefix",
			}),
			cfg: &ProfilingConfig{
				ProcessCPU: ProfilingTarget{
					Enabled: true,
					Path:    "/debug/pprof/profile",
					Delta:   true,
				},
				PathPrefix: "/foo",
			},
			expected: []labels.Labels{
				labels.FromMap(map[string]string{
					model.AddressLabel: "localhost:9090",
					ProfilePath:        "/debug/pprof/custom_profile",
					ProfileName:        "custom_process_cpu",
					ProfilePathPrefix:  "/prefix",
				}),
			},
		},
	}
	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			actualBuilders := labelsByProfiles(labels.NewBuilder(td.target).Labels(), td.cfg)
			actual := make([]labels.Labels, len(actualBuilders))
			for i, b := range actualBuilders {
				actual[i] = b.Labels()
			}
			require.Equal(t, td.expected, actual)
		})
	}
}

func BenchmarkPopulateLabels(b *testing.B) {
	args := NewDefaultArguments()
	tg := &targetgroup.Group{
		Targets: []model.LabelSet{
			{model.AddressLabel: "localhost:9090"},
			{model.AddressLabel: "localhost:9091", serviceNameLabel: "svc"},
			{model.AddressLabel: "localhost:9092", serviceNameK8SLabel: "k8s-svc"},
			{model.AddressLabel: "localhost:9093", "__meta_kubernetes_namespace": "ns", "__meta_kubernetes_pod_container_name": "container"},
			{model.AddressLabel: "localhost:9094", "__meta_docker_container_name": "docker-container"},
		},
		Labels: model.LabelSet{
			"foo": "bar",
		},
	}
	for i := 0; i < b.N; i++ {
		active, err := targetsFromGroup(tg, args, args.ProfilingConfig.AllTargets())
		if err != nil || len(active) == 0 {
			b.Fail()
		}
	}
}

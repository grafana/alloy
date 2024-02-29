package config

import (
	"fmt"
	"strings"
	"testing"

	jsonnet "github.com/google/go-jsonnet"
	prom "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prom_v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	"github.com/grafana/agent/internal/static/operator/assets"
	"github.com/grafana/agent/internal/util"
)

func TestLogsClientConfig(t *testing.T) {
	agent := &gragent.GrafanaAgent{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "telemetry",
			Name:      "grafana-agent",
		},
	}

	tt := []struct {
		name   string
		input  map[string]interface{}
		expect string
	}{
		{
			name: "all-in-one URL",
			input: map[string]interface{}{
				"agent":     agent,
				"namespace": "operator",
				"spec": &gragent.LogsClientSpec{
					URL: "http://username:password@localhost:3100/loki/api/v1/push",
				},
			},
			expect: util.Untab(`
				url: http://username:password@localhost:3100/loki/api/v1/push
				external_labels:
					cluster: telemetry/grafana-agent
			`),
		},
		{
			name: "full basic config",
			input: map[string]interface{}{
				"agent":     agent,
				"namespace": "operator",
				"spec": &gragent.LogsClientSpec{
					URL:       "http://localhost:3100/loki/api/v1/push",
					TenantID:  "tenant",
					BatchWait: "5m",
					BatchSize: 500,
					Timeout:   "5m",
					ExternalLabels: map[string]string{
						"foo":  "bar",
						"fizz": "buzz",
					},
					ProxyURL: "http://proxy:3100/",
					BackoffConfig: &gragent.LogsBackoffConfigSpec{
						MinPeriod:  "500ms",
						MaxPeriod:  "5m",
						MaxRetries: 100,
					},
				},
			},
			expect: util.Untab(`
				url: http://localhost:3100/loki/api/v1/push
				tenant_id: tenant
				batchwait: 5m
				batchsize: 500
				proxy_url: http://proxy:3100/
				backoff_config:
					min_period: 500ms
					max_period: 5m
					max_retries: 100
				external_labels:
					cluster: telemetry/grafana-agent
					foo: bar
					fizz: buzz
				timeout: 5m
			`),
		},
		{
			name: "tls config",
			input: map[string]interface{}{
				"agent":     agent,
				"namespace": "operator",
				"spec": &gragent.LogsClientSpec{
					URL: "http://localhost:3100/loki/api/v1/push",
					TLSConfig: &prom.TLSConfig{
						CAFile:   "ca",
						KeyFile:  "key",
						CertFile: "cert",
					},
				},
			},
			expect: util.Untab(`
				url: http://localhost:3100/loki/api/v1/push
				tls_config:
					ca_file: ca
					key_file: key
					cert_file: cert
				external_labels:
					cluster: telemetry/grafana-agent
			`),
		},
		{
			name: "bearer tokens",
			input: map[string]interface{}{
				"agent":     agent,
				"namespace": "operator",
				"spec": &gragent.LogsClientSpec{
					URL:             "http://localhost:3100/loki/api/v1/push",
					BearerToken:     "tok",
					BearerTokenFile: "tokfile",
				},
			},
			expect: util.Untab(`
				url: http://localhost:3100/loki/api/v1/push
				bearer_token: tok
				bearer_token_file: tokfile
				external_labels:
					cluster: telemetry/grafana-agent
			`),
		},
		{
			name: "basic auth",
			input: map[string]interface{}{
				"agent":     agent,
				"namespace": "operator",
				"spec": &gragent.LogsClientSpec{
					URL: "http://localhost:3100/loki/api/v1/push",
					BasicAuth: &prom.BasicAuth{
						Username: v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
							Key:                  "key",
						},
						Password: v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
							Key:                  "key",
						},
					},
				},
			},
			expect: util.Untab(`
				url: http://localhost:3100/loki/api/v1/push
				basic_auth:
					username: secretkey
					password: secretkey
				external_labels:
					cluster: telemetry/grafana-agent
			`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			vm, err := createVM(testStore())
			require.NoError(t, err)

			actual, err := runSnippetTLA(t, vm, "./component/logs/client.libsonnet", tc.input)
			require.NoError(t, err)
			require.YAMLEq(t, tc.expect, actual)
		})
	}
}

func TestLogsStages(t *testing.T) {
	tt := []struct {
		name   string
		input  map[string]interface{}
		expect string
	}{
		{
			name: "docker",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				Docker: &gragent.DockerStageSpec{},
			}},
			expect: util.Untab(`docker: {}`),
		},
		{
			name: "cri",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				CRI: &gragent.CRIStageSpec{},
			}},
			expect: util.Untab(`cri: {}`),
		},
		{
			name: "regex",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				Regex: &gragent.RegexStageSpec{
					Source:     "time",
					Expression: "^(?P<year>\\d+)",
				},
			}},
			expect: util.Untab(`
				regex:
					expression: '^(?P<year>\d+)'
					source: time
			`),
		},
		{
			name: "json",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				JSON: &gragent.JSONStageSpec{
					Expressions: map[string]string{"user": ""},
					Source:      "extra",
				},
			}},
			expect: util.Untab(`
				json:
					expressions:
						user: ""
					source: extra
			`),
		},
		{
			name: "labelallow",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				LabelAllow: []string{"foo", "bar"},
			}},
			expect: util.Untab(`
				labelallow: [foo, bar]
			`),
		},
		{
			name: "labeldrop",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				LabelDrop: []string{"foo", "bar"},
			}},
			expect: util.Untab(`
				labeldrop: [foo, bar]
			`),
		},
		{
			name: "labels",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				Labels: map[string]string{
					"foo":  "",
					"fizz": "buzz",
				},
			}},
			expect: util.Untab(`
				labels:
					foo: ""
					fizz: buzz
			`),
		},
		{
			name: "limit",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				Limit: &gragent.LimitStageSpec{
					Rate:  10,
					Burst: 20,
					Drop:  false,
				},
			}},
			expect: util.Untab(`
				limit:
					rate: 10
					burst: 20
					drop: false
			`),
		},
		{
			name: "match",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				Match: &gragent.MatchStageSpec{
					PipelineName:      "app2",
					Selector:          `{app="pokey"}`,
					Action:            "keep",
					DropCounterReason: "no_pokey",
					Stages: util.Untab(`
					- json:
  			      expressions:
							  msg: msg
					`),
				},
			}},
			expect: util.Untab(`
				match:
					pipeline_name: app2
					selector: '{app="pokey"}'
					action: keep
					drop_counter_reason: no_pokey
					stages:
					- json:
							expressions:
								msg: msg
			`),
		},
		{
			name: "metrics",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				Metrics: map[string]gragent.MetricsStageSpec{
					"logs_line_total": {
						Type:            "counter",
						Description:     "total number of log lines",
						Prefix:          "my_promtail_custom_",
						MaxIdleDuration: "24h",
						MatchAll:        boolPtr(true),
						Action:          "inc",
					},
					"queue_elements": {
						Type:        "gauge",
						Description: "elements in queue",
						Action:      "add",
					},
					"http_response_time_seconds": {
						Type:    "histogram",
						Source:  "response_time",
						Action:  "inc",
						Buckets: []string{"0.001", "0.0025", "0.050"},
					},
				},
			}},
			expect: util.Untab(`
				metrics:
					logs_line_total:
						type: Counter
						description: total number of log lines
						prefix: my_promtail_custom_
						max_idle_duration: 24h
						config:
							match_all: true
							action: inc
					queue_elements:
						type: Gauge
						description: elements in queue
						config:
							action: add
					http_response_time_seconds:
						type: Histogram
						source: response_time
						config:
							action: inc
							buckets: [0.001, 0.0025, 0.050]
			`),
		},
		{
			name: "multiline",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				Multiline: &gragent.MultilineStageSpec{
					FirstLine:   "first",
					MaxWaitTime: "5m",
					MaxLines:    5,
				},
			}},
			expect: util.Untab(`
				multiline:
					firstline: first
					max_wait_time: 5m
					max_lines: 5
			`),
		},
		{
			name: "output",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				Output: &gragent.OutputStageSpec{Source: "message"},
			}},
			expect: util.Untab(`
				output:
					source: message
			`),
		},
		{
			name: "pack",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				Pack: &gragent.PackStageSpec{
					Labels:          []string{"foo", "bar"},
					IngestTimestamp: true,
				},
			}},
			expect: util.Untab(`
				pack:
					labels: [foo, bar]
					ingest_timestamp: true
			`),
		},
		{
			name: "regex",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				Regex: &gragent.RegexStageSpec{
					Source:     "msg",
					Expression: "some regex",
				},
			}},
			expect: util.Untab(`
				regex:
					source: msg
					expression: some regex
			`),
		},
		{
			name: "replace",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				Replace: &gragent.ReplaceStageSpec{
					Expression: "password (\\S+)",
					Replace:    "****",
					Source:     "msg",
				},
			}},
			expect: util.Untab(`
				replace:
					expression: 'password (\S+)'
					replace: '****'
					source: msg
			`),
		},
		{
			name: "template",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				Template: &gragent.TemplateStageSpec{
					Source:   "new_key",
					Template: "hello world!",
				},
			}},
			expect: util.Untab(`
				template:
					source: new_key
					template: "hello world!"
			`),
		},
		{
			name: "tenant",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				Tenant: &gragent.TenantStageSpec{
					Label:  "__meta_kubernetes_pod_label_fake",
					Source: "customer_id",
					Value:  "fake",
				},
			}},
			expect: util.Untab(`
				tenant:
					label: __meta_kubernetes_pod_label_fake
					source: customer_id
					value: fake
			`),
		},
		{
			name: "timestamp",
			input: map[string]interface{}{"spec": &gragent.PipelineStageSpec{
				Timestamp: &gragent.TimestampStageSpec{
					Source:          "time",
					Format:          "RFC3339Nano",
					FallbackFormats: []string{"UnixNs"},
					Location:        "America/New_York",
					ActionOnFailure: "fudge",
				},
			}},
			expect: util.Untab(`
				timestamp:
					source: time
					format: RFC3339Nano
					fallback_formats: [UnixNs]
					location: America/New_York
					action_on_failure: fudge
			`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			vm, err := createVM(testStore())
			require.NoError(t, err)

			actual, err := runSnippetTLA(t, vm, "./component/logs/stages.libsonnet", tc.input)
			require.NoError(t, err)
			require.YAMLEq(t, tc.expect, actual)
		})
	}
}

func TestPodLogsConfig(t *testing.T) {
	tt := []struct {
		name   string
		input  map[string]interface{}
		expect string
	}{
		{
			name: "default",
			input: map[string]interface{}{
				"agentNamespace": "operator",
				"podLogs": gragent.PodLogs{
					ObjectMeta: meta_v1.ObjectMeta{
						Namespace: "operator",
						Name:      "podlogs",
					},
					Spec: gragent.PodLogsSpec{
						RelabelConfigs: []*prom_v1.RelabelConfig{{
							SourceLabels: []prom.LabelName{"input_a", "input_b"},
							Separator:    ";",
							TargetLabel:  "target_a",
							Regex:        "regex",
							Modulus:      1234,
							Replacement:  "foobar",
							Action:       "replace",
						}},
					},
				},
				"apiServer":                prom_v1.APIServerConfig{},
				"ignoreNamespaceSelectors": false,
				"enforcedNamespaceLabel":   "",
			},
			expect: util.Untab(`
				job_name: podLogs/operator/podlogs
				kubernetes_sd_configs:
				- role: pod
				  namespaces:
						names: [operator]
				relabel_configs:
				- source_labels: [job]
					target_label: __tmp_prometheus_job_name
				- source_labels: [__meta_kubernetes_namespace]
					target_label: namespace
				- source_labels: [__meta_kubernetes_service_name]
					target_label: service
				- source_labels: [__meta_kubernetes_pod_name]
					target_label: pod
				- source_labels: [__meta_kubernetes_pod_container_name]
					target_label: container
				- target_label: job
					replacement: operator/podlogs
				- source_labels: ['__meta_kubernetes_pod_uid', '__meta_kubernetes_pod_container_name']
					target_label: __path__
					separator: /
					replacement: /var/log/pods/*$1/*.log
				- source_labels: ["input_a", "input_b"]
					separator: ";"
					target_label: "target_a"
					regex: regex
					modulus: 1234
					replacement: foobar
					action: replace
			`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			vm, err := createVM(testStore())
			require.NoError(t, err)

			actual, err := runSnippetTLA(t, vm, "./component/logs/pod_logs.libsonnet", tc.input)
			require.NoError(t, err)
			require.YAMLEq(t, tc.expect, actual)
		})
	}
}

func TestLogsConfig(t *testing.T) {
	agent := &gragent.GrafanaAgent{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "operator",
			Name:      "grafana-agent",
		},
	}

	tt := []struct {
		name   string
		input  map[string]interface{}
		expect string
	}{
		{
			name: "global clients",
			input: map[string]interface{}{
				"agent": agent,
				"global": &gragent.LogsSubsystemSpec{
					Clients: []gragent.LogsClientSpec{{URL: "global"}},
				},
				"instance": &gragent.LogsDeployment{
					Instance: &gragent.LogsInstance{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "inst",
							Name:      "default",
						},
						Spec: gragent.LogsInstanceSpec{},
					},
				},
				"apiServer": &prom.APIServerConfig{},

				"ignoreNamespaceSelectors": false,
				"enforcedNamespaceLabel":   "",
			},
			expect: util.Untab(`
				name: inst/default
				clients:
				- url: global
				  external_labels:
					  cluster: operator/grafana-agent
			`),
		},
		{
			name: "local clients",
			input: map[string]interface{}{
				"agent": agent,
				"global": &gragent.LogsSubsystemSpec{
					Clients: []gragent.LogsClientSpec{{URL: "global"}},
				},
				"instance": &gragent.LogsDeployment{
					Instance: &gragent.LogsInstance{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "inst",
							Name:      "default",
						},
						Spec: gragent.LogsInstanceSpec{
							Clients: []gragent.LogsClientSpec{{URL: "local"}},
						},
					},
				},
				"apiServer": &prom.APIServerConfig{},

				"ignoreNamespaceSelectors": false,
				"enforcedNamespaceLabel":   "",
			},
			expect: util.Untab(`
				name: inst/default
				clients:
				- url: local
				  external_labels:
					  cluster: operator/grafana-agent
			`),
		},
		{
			name: "pod logs",
			input: map[string]interface{}{
				"agent":  agent,
				"global": &gragent.LogsSubsystemSpec{},
				"instance": &gragent.LogsDeployment{
					Instance: &gragent.LogsInstance{
						ObjectMeta: metav1.ObjectMeta{Namespace: "inst", Name: "default"},
					},
					PodLogs: []*gragent.PodLogs{{
						ObjectMeta: metav1.ObjectMeta{Namespace: "app", Name: "pod"},
					}},
				},
				"apiServer": &prom.APIServerConfig{},

				"ignoreNamespaceSelectors": false,
				"enforcedNamespaceLabel":   "",
			},
			expect: util.Untab(`
				name: inst/default
				scrape_configs:
				- job_name: podLogs/app/pod
					kubernetes_sd_configs:
					- namespaces:
							names:
							- app
						role: pod
					relabel_configs:
					- source_labels:
						- job
						target_label: __tmp_prometheus_job_name
					- source_labels:
						- __meta_kubernetes_namespace
						target_label: namespace
					- source_labels:
						- __meta_kubernetes_service_name
						target_label: service
					- source_labels:
						- __meta_kubernetes_pod_name
						target_label: pod
					- source_labels:
						- __meta_kubernetes_pod_container_name
						target_label: container
					- replacement: app/pod
						target_label: job
					- source_labels: ['__meta_kubernetes_pod_uid', '__meta_kubernetes_pod_container_name']
						target_label: __path__
						separator: /
						replacement: /var/log/pods/*$1/*.log
			`),
		},
		{
			name: "additional scrape configs",
			input: map[string]interface{}{
				"agent":  agent,
				"global": &gragent.LogsSubsystemSpec{},
				"instance": &gragent.LogsDeployment{
					Instance: &gragent.LogsInstance{
						ObjectMeta: metav1.ObjectMeta{Namespace: "inst", Name: "default"},
						Spec: gragent.LogsInstanceSpec{
							AdditionalScrapeConfigs: &v1.SecretKeySelector{
								LocalObjectReference: v1.LocalObjectReference{Name: "additional"},
								Key:                  "configs",
							},
						},
					},
				},
				"apiServer": &prom.APIServerConfig{},

				"ignoreNamespaceSelectors": false,
				"enforcedNamespaceLabel":   "",
			},
			expect: util.Untab(`
				name: inst/default
				scrape_configs:
					- job_name: extra
			`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := testStore()

			s[assets.KeyForSecret("inst", &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "additional",
				},
				Key: "configs",
			})] = `[{ "job_name": "extra" }]`

			vm, err := createVM(s)
			require.NoError(t, err)

			actual, err := runSnippetTLA(t, vm, "./logs.libsonnet", tc.input)
			require.NoError(t, err)
			require.YAMLEq(t, tc.expect, actual)
		})
	}
}

func runSnippetTLA(t *testing.T, vm *jsonnet.VM, filename string, tla map[string]interface{}) (string, error) {
	t.Helper()

	args := make([]string, 0, len(tla))
	for arg := range tla {
		args = append(args, arg)
	}

	boundArgs := make([]string, len(args))
	for i := range args {
		boundArgs[i] = fmt.Sprintf("%[1]s=%[1]s", args[i])
	}

	// Bind argument to TLA.
	for arg, value := range tla {
		bb, err := jsonnetMarshal(value)
		require.NoError(t, err)
		vm.TLACode(arg, string(bb))
	}

	return vm.EvaluateAnonymousSnippet(
		filename,
		fmt.Sprintf(`
			local marshal = import './ext/marshal.libsonnet';
			local optionals = import './ext/optionals.libsonnet';
			local eval = import '%s';
			function(%s) marshal.YAML(optionals.trim(eval(%s)))
		`, filename, strings.Join(args, ","), strings.Join(boundArgs, ",")),
	)
}

func boolPtr(v bool) *bool { return &v }

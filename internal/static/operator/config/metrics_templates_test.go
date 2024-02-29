package config

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/go-jsonnet"
	prom_v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	"github.com/grafana/agent/internal/static/operator/assets"
	"github.com/grafana/agent/internal/util"
)

func TestExternalLabels(t *testing.T) {
	tt := []struct {
		name       string
		input      interface{}
		addReplica bool
		expect     string
	}{
		{
			name:       "no replica",
			addReplica: false,
			input: gragent.Deployment{
				Agent: &gragent.GrafanaAgent{
					ObjectMeta: meta_v1.ObjectMeta{
						Namespace: "operator",
						Name:      "agent",
					},
				},
			},
			expect: util.Untab(`
				cluster: operator/agent
			`),
		},
		{
			name:       "defaults",
			addReplica: true,
			input: gragent.Deployment{
				Agent: &gragent.GrafanaAgent{
					ObjectMeta: meta_v1.ObjectMeta{
						Namespace: "operator",
						Name:      "agent",
					},
				},
			},
			expect: util.Untab(`
				cluster: operator/agent
				__replica__: replica-$(STATEFULSET_ORDINAL_NUMBER)
			`),
		},
		{
			name:       "external_labels",
			addReplica: true,
			input: gragent.Deployment{
				Agent: &gragent.GrafanaAgent{
					ObjectMeta: meta_v1.ObjectMeta{
						Namespace: "operator",
						Name:      "agent",
					},
					Spec: gragent.GrafanaAgentSpec{
						Metrics: gragent.MetricsSubsystemSpec{
							ExternalLabels: map[string]string{"foo": "bar"},
						},
					},
				},
			},
			expect: util.Untab(`
				cluster: operator/agent
				foo: bar
				__replica__: replica-$(STATEFULSET_ORDINAL_NUMBER)
			`),
		},
		{
			name:       "custom labels",
			addReplica: true,
			input: gragent.Deployment{
				Agent: &gragent.GrafanaAgent{
					ObjectMeta: meta_v1.ObjectMeta{
						Namespace: "operator",
						Name:      "agent",
					},
					Spec: gragent.GrafanaAgentSpec{
						Metrics: gragent.MetricsSubsystemSpec{
							MetricsExternalLabelName: ptr.To("deployment"),
							ReplicaExternalLabelName: ptr.To("replica"),
							ExternalLabels:           map[string]string{"foo": "bar"},
						},
					},
				},
			},
			expect: util.Untab(`
				deployment: operator/agent
				foo: bar
				replica: replica-$(STATEFULSET_ORDINAL_NUMBER)
			`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			vm, err := createVM(nil)
			require.NoError(t, err)
			bb, err := jsonnetMarshal(tc.input)
			require.NoError(t, err)

			vm.TLACode("ctx", string(bb))
			vm.TLACode("addReplica", fmt.Sprintf("%v", tc.addReplica))
			actual, err := runSnippet(vm, "./component/metrics/external_labels.libsonnet", "ctx", "addReplica")
			require.NoError(t, err)
			require.YAMLEq(t, tc.expect, actual)
		})
	}
}

func TestKubeSDConfig(t *testing.T) {
	tt := []struct {
		name   string
		input  map[string]interface{}
		expect string
	}{
		{
			name: "defaults",
			input: map[string]interface{}{
				"namespace": "operator",
				"role":      "pod",
			},
			expect: util.Untab(`
				role: pod
			`),
		},
		{
			name: "defaults",
			input: map[string]interface{}{
				"namespace":  "operator",
				"namespaces": []string{"operator"},
				"role":       "pod",
			},
			expect: util.Untab(`
				role: pod
				namespaces:
					names: [operator]
			`),
		},
		{
			name: "host",
			input: map[string]interface{}{
				"namespace": "operator",
				"apiServer": &prom_v1.APIServerConfig{Host: "host"},
				"role":      "pod",
			},
			expect: util.Untab(`
				role: pod
				api_server: host
			`),
		},
		{
			name: "basic auth",
			input: map[string]interface{}{
				"namespace": "operator",
				"apiServer": &prom_v1.APIServerConfig{
					BasicAuth: &prom_v1.BasicAuth{
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
				"role": "pod",
			},
			expect: util.Untab(`
				role: pod
				basic_auth:
					username: secretkey
					password: secretkey
			`),
		},
		{
			name: "bearer auth",
			input: map[string]interface{}{
				"namespace": "operator",
				"apiServer": &prom_v1.APIServerConfig{
					BearerToken:     "bearer",
					BearerTokenFile: "file",
				},
				"role": "pod",
			},
			expect: util.Untab(`
				role: pod
				authorization:
					type: Bearer
					credentials: bearer
					credentials_file: file
			`),
		},
		{
			name: "tls_config",
			input: map[string]interface{}{
				"namespace": "operator",
				"apiServer": &prom_v1.APIServerConfig{
					TLSConfig: &prom_v1.TLSConfig{
						CAFile: "ca",
					},
				},
				"role": "pod",
			},
			expect: util.Untab(`
				role: pod
				tls_config:
					ca_file: ca
			`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			vm, err := createVM(testStore())
			require.NoError(t, err)

			args := []string{"namespace", "namespaces", "apiServer", "role"}
			for _, arg := range args {
				bb, err := jsonnetMarshal(tc.input[arg])
				require.NoError(t, err)
				vm.TLACode(arg, string(bb))
			}

			actual, err := runSnippet(vm, "./component/metrics/kube_sd_config.libsonnet", args...)
			require.NoError(t, err)
			require.YAMLEq(t, tc.expect, actual)
		})
	}
}

func TestPodMonitor(t *testing.T) {
	var falseVal = false
	var trueVal = true
	tt := []struct {
		name   string
		input  map[string]interface{}
		expect string
	}{
		{
			name: "default",
			input: map[string]interface{}{
				"agentNamespace": "operator",
				"monitor": prom_v1.PodMonitor{
					ObjectMeta: meta_v1.ObjectMeta{
						Namespace: "operator",
						Name:      "podmonitor",
					},
				},
				"endpoint": prom_v1.PodMetricsEndpoint{
					Port:          "metrics",
					EnableHttp2:   &falseVal,
					FilterRunning: &trueVal,
				},
				"index":                    0,
				"apiServer":                prom_v1.APIServerConfig{},
				"overrideHonorLabels":      false,
				"overrideHonorTimestamps":  false,
				"ignoreNamespaceSelectors": false,
				"enforcedNamespaceLabel":   "",
				"enforcedSampleLimit":      nil,
				"enforcedTargetLimit":      nil,
				"shards":                   1,
			},
			expect: util.Untab(`
				job_name: podMonitor/operator/podmonitor/0
				enable_http2: false
				honor_labels: false
				kubernetes_sd_configs:
				- role: pod
				  namespaces:
						names: [operator]
				relabel_configs:
				- source_labels: [job]
					target_label: __tmp_prometheus_job_name
				- source_labels: [__meta_kubernetes_pod_phase]
				  regex: (Failed|Succeeded)
				  action: drop
				- source_labels: [__meta_kubernetes_pod_container_port_name]
					regex: metrics
					action: keep
				- source_labels: [__meta_kubernetes_namespace]
					target_label: namespace
				- source_labels: [__meta_kubernetes_service_name]
					target_label: service
				- source_labels: [__meta_kubernetes_pod_name]
					target_label: pod
				- source_labels: [__meta_kubernetes_pod_container_name]
					target_label: container
				- target_label: job
					replacement: operator/podmonitor
				- target_label: endpoint
					replacement: metrics
				- source_labels: [__address__]
					target_label: __tmp_hash
					action: hashmod
					modulus: 1
				- source_labels: [__tmp_hash]
					action: keep
					regex: $(SHARD)
			`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			vm, err := createVM(testStore())
			require.NoError(t, err)

			args := []string{
				"agentNamespace", "monitor", "endpoint", "index", "apiServer", "overrideHonorLabels",
				"overrideHonorTimestamps", "ignoreNamespaceSelectors", "enforcedNamespaceLabel",
				"enforcedSampleLimit", "enforcedTargetLimit", "shards",
			}
			for _, arg := range args {
				bb, err := jsonnetMarshal(tc.input[arg])
				require.NoError(t, err)
				vm.TLACode(arg, string(bb))
			}

			actual, err := runSnippet(vm, "./component/metrics/pod_monitor.libsonnet", args...)
			require.NoError(t, err)
			if !assert.YAMLEq(t, tc.expect, actual) {
				fmt.Fprintln(os.Stderr, actual)
			}
		})
	}
}

func TestProbe(t *testing.T) {
	tt := []struct {
		name   string
		input  map[string]interface{}
		expect string
	}{
		{
			name: "default",
			input: map[string]interface{}{
				"agentNamespace": "operator",
				"probe": prom_v1.Probe{
					ObjectMeta: meta_v1.ObjectMeta{
						Namespace: "operator",
						Name:      "probe",
					},
					Spec: prom_v1.ProbeSpec{
						Module: "mod",
						Targets: prom_v1.ProbeTargets{
							Ingress: &prom_v1.ProbeTargetIngress{
								Selector: meta_v1.LabelSelector{
									MatchLabels: map[string]string{"foo": "bar"},
								},
							},
						},
						TLSConfig: &prom_v1.ProbeTLSConfig{
							SafeTLSConfig: prom_v1.SafeTLSConfig{
								InsecureSkipVerify: true,
							},
						},
					},
				},
				"apiServer":                prom_v1.APIServerConfig{},
				"overrideHonorTimestamps":  false,
				"ignoreNamespaceSelectors": false,
				"enforcedNamespaceLabel":   "",
				"enforcedSampleLimit":      nil,
				"enforcedTargetLimit":      nil,
				"shards":                   1,
			},
			expect: util.Untab(`
				job_name: probe/operator/probe
				honor_timestamps: true
				kubernetes_sd_configs:
				- role: ingress
					namespaces:
						names: [operator]
				metrics_path: /probe
				params:
					module: ["mod"]
				relabel_configs:
				- source_labels: [job]
					target_label: __tmp_prometheus_job_name
				- action: keep
					regex: bar
					source_labels: [__meta_kubernetes_ingress_label_foo]
				- action: replace
					regex: (.+);(.+);(.+)
					replacement: $1://$2$3
					separator: ;
					source_labels:
						- __meta_kubernetes_ingress_scheme
						- __address__
						- __meta_kubernetes_ingress_path
					target_label: __param_target
				- source_labels: [__meta_kubernetes_namespace]
					target_label: namespace
				- source_labels: [__meta_kubernetes_ingress_name]
					target_label: ingress
				- source_labels: [__param_target]
					target_label: instance
				- replacement: ""
					target_label: __address__
				tls_config:
					insecure_skip_verify: true
			`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			vm, err := createVM(testStore())
			require.NoError(t, err)

			args := []string{
				"agentNamespace", "probe", "apiServer", "overrideHonorTimestamps",
				"ignoreNamespaceSelectors", "enforcedNamespaceLabel",
				"enforcedSampleLimit", "enforcedTargetLimit", "shards",
			}
			for _, arg := range args {
				bb, err := jsonnetMarshal(tc.input[arg])
				require.NoError(t, err)
				vm.TLACode(arg, string(bb))
			}

			actual, err := runSnippet(vm, "./component/metrics/probe.libsonnet", args...)
			require.NoError(t, err)
			if !assert.YAMLEq(t, tc.expect, actual) {
				fmt.Fprintln(os.Stderr, actual)
			}
		})
	}
}

func TestRelabelConfig(t *testing.T) {
	tt := []struct {
		name   string
		input  interface{}
		expect string
	}{
		{
			name: "full",
			input: prom_v1.RelabelConfig{
				SourceLabels: []prom_v1.LabelName{"input_a", "input_b"},
				Separator:    ";",
				TargetLabel:  "target_a",
				Regex:        "regex",
				Modulus:      1234,
				Replacement:  "foobar",
				Action:       "replace",
			},
			expect: util.Untab(`
				source_labels: ["input_a", "input_b"]
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
			vm, err := createVM(nil)
			require.NoError(t, err)
			bb, err := jsonnetMarshal(tc.input)
			require.NoError(t, err)

			vm.TLACode("cfg", string(bb))
			actual, err := runSnippet(vm, "./component/metrics/relabel_config.libsonnet", "cfg")
			require.NoError(t, err)
			require.YAMLEq(t, tc.expect, actual)
		})
	}
}

func TestRemoteWrite(t *testing.T) {
	tt := []struct {
		name   string
		input  map[string]interface{}
		expect string
	}{
		{
			name: "bare",
			input: map[string]interface{}{
				"namespace": "operator",
				"rw": gragent.RemoteWriteSpec{
					URL: "http://cortex/api/prom/push",
				},
			},
			expect: util.Untab(`
				url: http://cortex/api/prom/push
			`),
		},
		{
			name: "base configs",
			input: map[string]interface{}{
				"namespace": "operator",
				"rw": gragent.RemoteWriteSpec{
					Name:          "cortex",
					URL:           "http://cortex/api/prom/push",
					RemoteTimeout: "5m",
					Headers:       map[string]string{"foo": "bar"},
				},
			},
			expect: util.Untab(`
				name: cortex
				url: http://cortex/api/prom/push
				remote_timeout: 5m
				headers:
					foo: bar
			`),
		},
		{
			name: "write_relabel_configs",
			input: map[string]interface{}{
				"namespace": "operator",
				"rw": gragent.RemoteWriteSpec{
					URL: "http://cortex/api/prom/push",
					WriteRelabelConfigs: []prom_v1.RelabelConfig{{
						SourceLabels: []prom_v1.LabelName{"__name__"},
						Action:       "drop",
					}},
				},
			},
			expect: util.Untab(`
				url: http://cortex/api/prom/push
				write_relabel_configs:
				- source_labels: [__name__]
					action: drop
			`),
		},
		{
			name: "tls_config",
			input: map[string]interface{}{
				"namespace": "operator",
				"rw": gragent.RemoteWriteSpec{
					URL: "http://cortex/api/prom/push",
					TLSConfig: &prom_v1.TLSConfig{
						CAFile:   "ca",
						CertFile: "cert",
					},
				},
			},
			expect: util.Untab(`
				url: http://cortex/api/prom/push
				tls_config:
					ca_file: ca
					cert_file: cert
			`),
		},
		{
			name: "basic_auth",
			input: map[string]interface{}{
				"namespace": "operator",
				"rw": gragent.RemoteWriteSpec{
					URL: "http://cortex/api/prom/push",
					BasicAuth: &prom_v1.BasicAuth{
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
				url: http://cortex/api/prom/push
				basic_auth:
					username: secretkey
					password_file: /var/lib/grafana-agent/secrets/_secrets_operator_obj_key
			`),
		},
		{
			name: "bearer_token",
			input: map[string]interface{}{
				"namespace": "operator",
				"rw": gragent.RemoteWriteSpec{
					URL:         "http://cortex/api/prom/push",
					BearerToken: "my-token",
				},
			},
			expect: util.Untab(`
				url: http://cortex/api/prom/push
				authorization:
					type: Bearer
					credentials: my-token
			`),
		},
		{
			name: "bearer_token_file",
			input: map[string]interface{}{
				"namespace": "operator",
				"rw": gragent.RemoteWriteSpec{
					URL:             "http://cortex/api/prom/push",
					BearerTokenFile: "/path/to/file",
				},
			},
			expect: util.Untab(`
				url: http://cortex/api/prom/push
				authorization:
					type: Bearer
					credentials_file: /path/to/file
			`),
		},
		{
			name: "sigv4",
			input: map[string]interface{}{
				"namespace": "operator",
				"rw": gragent.RemoteWriteSpec{
					URL: "http://cortex/api/prom/push",
					SigV4: &gragent.SigV4Config{
						Region: "region",
						AccessKey: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
							Key:                  "key",
						},
						SecretKey: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
							Key:                  "key",
						},
						Profile: "profile",
						RoleARN: "arn",
					},
				},
			},
			expect: util.Untab(`
				url: http://cortex/api/prom/push
				sigv4:
					region: region
					access_key: secretkey
					secret_key: secretkey
					profile: profile
					role_arn: arn
			`),
		},
		{
			name: "queue_config",
			input: map[string]interface{}{
				"namespace": "operator",
				"rw": gragent.RemoteWriteSpec{
					URL: "http://cortex/api/prom/push",
					QueueConfig: &gragent.QueueConfig{
						Capacity:          1000,
						MinShards:         1,
						MaxShards:         100,
						MaxSamplesPerSend: 500,
						BatchSendDeadline: "5m",
						MinBackoff:        "1m",
						MaxBackoff:        "5m",
					},
				},
			},
			expect: util.Untab(`
				url: http://cortex/api/prom/push
				queue_config:
					capacity: 1000
					min_shards: 1
					max_shards: 100
					max_samples_per_send: 500
					batch_send_deadline: 5m
					min_backoff: 1m
					max_backoff: 5m
			`),
		},
		{
			name: "metadata_config",
			input: map[string]interface{}{
				"namespace": "operator",
				"rw": gragent.RemoteWriteSpec{
					URL: "http://cortex/api/prom/push",
					MetadataConfig: &gragent.MetadataConfig{
						Send:         true,
						SendInterval: "5m",
					},
				},
			},
			expect: util.Untab(`
				url: http://cortex/api/prom/push
				metadata_config:
					send: true
					send_interval: 5m
			`),
		},
		{
			name: "proxy_url",
			input: map[string]interface{}{
				"namespace": "operator",
				"rw": gragent.RemoteWriteSpec{
					URL:      "http://cortex/api/prom/push",
					ProxyURL: "http://proxy",
				},
			},
			expect: util.Untab(`
        url: http://cortex/api/prom/push
        proxy_url: http://proxy
			`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			vm, err := createVM(testStore())
			require.NoError(t, err)

			args := []string{"namespace", "rw"}
			for _, arg := range args {
				bb, err := jsonnetMarshal(tc.input[arg])
				require.NoError(t, err)
				vm.TLACode(arg, string(bb))
			}

			actual, err := runSnippet(vm, "./component/metrics/remote_write.libsonnet", args...)
			require.NoError(t, err)
			require.YAMLEq(t, tc.expect, actual)
		})
	}
}

func TestSafeTLSConfig(t *testing.T) {
	tt := []struct {
		name   string
		input  map[string]interface{}
		expect string
	}{
		{
			name: "configmap",
			input: map[string]interface{}{
				"namespace": "operator",
				"config": prom_v1.SafeTLSConfig{
					ServerName:         "server",
					InsecureSkipVerify: true,
					CA: prom_v1.SecretOrConfigMap{
						ConfigMap: &v1.ConfigMapKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
							Key:                  "key",
						},
					},
					Cert: prom_v1.SecretOrConfigMap{
						ConfigMap: &v1.ConfigMapKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
							Key:                  "key",
						},
					},
					KeySecret: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
						Key:                  "key",
					},
				},
			},
			expect: util.Untab(`
				ca_file: /var/lib/grafana-agent/secrets/_configMaps_operator_obj_key
				cert_file: /var/lib/grafana-agent/secrets/_configMaps_operator_obj_key
				key_file: /var/lib/grafana-agent/secrets/_secrets_operator_obj_key
				server_name: server
				insecure_skip_verify: true
			`),
		},
		{
			name: "secrets",
			input: map[string]interface{}{
				"namespace": "operator",
				"config": prom_v1.SafeTLSConfig{
					ServerName:         "server",
					InsecureSkipVerify: true,
					CA: prom_v1.SecretOrConfigMap{
						Secret: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
							Key:                  "key",
						},
					},
					Cert: prom_v1.SecretOrConfigMap{
						Secret: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
							Key:                  "key",
						},
					},
					KeySecret: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
						Key:                  "key",
					},
				},
			},
			expect: util.Untab(`
				ca_file: /var/lib/grafana-agent/secrets/_secrets_operator_obj_key
				cert_file: /var/lib/grafana-agent/secrets/_secrets_operator_obj_key
				key_file: /var/lib/grafana-agent/secrets/_secrets_operator_obj_key
				server_name: server
				insecure_skip_verify: true
			`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			vm, err := createVM(testStore())
			require.NoError(t, err)

			args := []string{"namespace", "config"}
			for _, arg := range args {
				bb, err := jsonnetMarshal(tc.input[arg])
				require.NoError(t, err)
				vm.TLACode(arg, string(bb))
			}

			actual, err := runSnippet(vm, "./component/metrics/safe_tls_config.libsonnet", args...)
			require.NoError(t, err)
			require.YAMLEq(t, tc.expect, actual)
		})
	}
}

func TestServiceMonitor(t *testing.T) {
	trueVal := true
	tt := []struct {
		name   string
		input  map[string]interface{}
		expect string
	}{
		{
			name: "default",
			input: map[string]interface{}{
				"agentNamespace": "operator",
				"monitor": prom_v1.ServiceMonitor{
					ObjectMeta: meta_v1.ObjectMeta{
						Namespace: "operator",
						Name:      "servicemonitor",
					},
				},
				"endpoint": prom_v1.Endpoint{
					Port:          "metrics",
					FilterRunning: &trueVal,
				},
				"index":                    0,
				"apiServer":                prom_v1.APIServerConfig{},
				"overrideHonorLabels":      false,
				"overrideHonorTimestamps":  false,
				"ignoreNamespaceSelectors": false,
				"enforcedNamespaceLabel":   "",
				"enforcedSampleLimit":      nil,
				"enforcedTargetLimit":      nil,
				"shards":                   1,
			},
			expect: util.Untab(`
				job_name: serviceMonitor/operator/servicemonitor/0
				honor_labels: false
				kubernetes_sd_configs:
				- role: endpoints
				  namespaces:
						names: [operator]
				relabel_configs:
				- source_labels:
					- job
					target_label: __tmp_prometheus_job_name
				- action: keep
					regex: metrics
					source_labels:
					- __meta_kubernetes_endpoint_port_name
				- regex: Node;(.*)
					replacement: $1
					separator: ;
					source_labels:
					- __meta_kubernetes_endpoint_address_target_kind
					- __meta_kubernetes_endpoint_address_target_name
					target_label: node
				- regex: Pod;(.*)
					replacement: $1
					separator: ;
					source_labels:
					- __meta_kubernetes_endpoint_address_target_kind
					- __meta_kubernetes_endpoint_address_target_name
					target_label: pod
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
				- source_labels: [__meta_kubernetes_pod_phase]
					regex: (Failed|Succeeded)
					action: drop
				- replacement: $1
					source_labels:
					- __meta_kubernetes_service_name
					target_label: job
				- replacement: metrics
					target_label: endpoint
				- action: hashmod
					modulus: 1
					source_labels:
					- __address__
					target_label: __tmp_hash
				- action: keep
					regex: $(SHARD)
					source_labels:
					- __tmp_hash
			`),
		},
		{
			name: "no_filter_running",
			input: map[string]interface{}{
				"agentNamespace": "operator",
				"monitor": prom_v1.ServiceMonitor{
					ObjectMeta: meta_v1.ObjectMeta{
						Namespace: "operator",
						Name:      "servicemonitor",
					},
				},
				"endpoint": prom_v1.Endpoint{
					Port: "metrics",
				},
				"index":                    0,
				"apiServer":                prom_v1.APIServerConfig{},
				"overrideHonorLabels":      false,
				"overrideHonorTimestamps":  false,
				"ignoreNamespaceSelectors": false,
				"enforcedNamespaceLabel":   "",
				"enforcedSampleLimit":      nil,
				"enforcedTargetLimit":      nil,
				"shards":                   1,
			},
			expect: util.Untab(`
				job_name: serviceMonitor/operator/servicemonitor/0
				honor_labels: false
				kubernetes_sd_configs:
				- role: endpoints
				  namespaces:
						names: [operator]
				relabel_configs:
				- source_labels:
					- job
					target_label: __tmp_prometheus_job_name
				- action: keep
					regex: metrics
					source_labels:
					- __meta_kubernetes_endpoint_port_name
				- regex: Node;(.*)
					replacement: $1
					separator: ;
					source_labels:
					- __meta_kubernetes_endpoint_address_target_kind
					- __meta_kubernetes_endpoint_address_target_name
					target_label: node
				- regex: Pod;(.*)
					replacement: $1
					separator: ;
					source_labels:
					- __meta_kubernetes_endpoint_address_target_kind
					- __meta_kubernetes_endpoint_address_target_name
					target_label: pod
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
				- replacement: $1
					source_labels:
					- __meta_kubernetes_service_name
					target_label: job
				- replacement: metrics
					target_label: endpoint
				- action: hashmod
					modulus: 1
					source_labels:
					- __address__
					target_label: __tmp_hash
				- action: keep
					regex: $(SHARD)
					source_labels:
					- __tmp_hash
			`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			vm, err := createVM(testStore())
			require.NoError(t, err)

			args := []string{
				"agentNamespace", "monitor", "endpoint", "index", "apiServer", "overrideHonorLabels",
				"overrideHonorTimestamps", "ignoreNamespaceSelectors", "enforcedNamespaceLabel",
				"enforcedSampleLimit", "enforcedTargetLimit", "shards",
			}
			for _, arg := range args {
				bb, err := jsonnetMarshal(tc.input[arg])
				require.NoError(t, err)
				vm.TLACode(arg, string(bb))
			}

			actual, err := runSnippet(vm, "./component/metrics/service_monitor.libsonnet", args...)
			require.NoError(t, err)
			if !assert.YAMLEq(t, tc.expect, actual) {
				fmt.Fprintln(os.Stderr, actual)
			}
		})
	}
}

func TestTLSConfig(t *testing.T) {
	tt := []struct {
		name   string
		input  map[string]interface{}
		expect string
	}{
		{
			name: "passthrough",
			input: map[string]interface{}{
				"namespace": "operator",
				"config": prom_v1.TLSConfig{
					SafeTLSConfig: prom_v1.SafeTLSConfig{
						ServerName:         "server",
						InsecureSkipVerify: true,
						CA: prom_v1.SecretOrConfigMap{
							Secret: &v1.SecretKeySelector{
								LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
								Key:                  "key",
							},
						},
						Cert: prom_v1.SecretOrConfigMap{
							Secret: &v1.SecretKeySelector{
								LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
								Key:                  "key",
							},
						},
						KeySecret: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
							Key:                  "key",
						},
					},
				},
			},
			expect: util.Untab(`
				ca_file: /var/lib/grafana-agent/secrets/_secrets_operator_obj_key
				cert_file: /var/lib/grafana-agent/secrets/_secrets_operator_obj_key
				key_file: /var/lib/grafana-agent/secrets/_secrets_operator_obj_key
				server_name: server
				insecure_skip_verify: true
			`),
		},
		{
			name: "overrides",
			input: map[string]interface{}{
				"namespace": "operator",
				"config": prom_v1.TLSConfig{
					SafeTLSConfig: prom_v1.SafeTLSConfig{
						ServerName:         "server",
						InsecureSkipVerify: true,
						CA: prom_v1.SecretOrConfigMap{
							Secret: &v1.SecretKeySelector{
								LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
								Key:                  "key",
							},
						},
						Cert: prom_v1.SecretOrConfigMap{
							Secret: &v1.SecretKeySelector{
								LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
								Key:                  "key",
							},
						},
						KeySecret: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
							Key:                  "key",
						},
					},
					CAFile:   "ca",
					CertFile: "cert",
					KeyFile:  "key",
				},
			},
			expect: util.Untab(`
				ca_file: ca
				cert_file: cert
				key_file: key
				server_name: server
				insecure_skip_verify: true
			`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			vm, err := createVM(testStore())
			require.NoError(t, err)

			args := []string{"namespace", "config"}
			for _, arg := range args {
				bb, err := jsonnetMarshal(tc.input[arg])
				require.NoError(t, err)
				vm.TLACode(arg, string(bb))
			}

			actual, err := runSnippet(vm, "./component/metrics/tls_config.libsonnet", args...)
			require.NoError(t, err)
			require.YAMLEq(t, tc.expect, actual)
		})
	}
}

func runSnippet(vm *jsonnet.VM, filename string, args ...string) (string, error) {
	boundArgs := make([]string, len(args))
	for i := range args {
		boundArgs[i] = fmt.Sprintf("%[1]s=%[1]s", args[i])
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

func testStore() assets.SecretStore {
	store := make(assets.SecretStore)

	store[assets.KeyForConfigMap("operator", &v1.ConfigMapKeySelector{
		LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
		Key:                  "key",
	})] = "secretcm"

	store[assets.KeyForSecret("operator", &v1.SecretKeySelector{
		LocalObjectReference: v1.LocalObjectReference{Name: "obj"},
		Key:                  "key",
	})] = "secretkey"

	return store
}

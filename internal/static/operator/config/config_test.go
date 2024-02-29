package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	k8s_yaml "sigs.k8s.io/yaml"

	"github.com/grafana/agent/internal/static/operator/assets"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/agent/internal/util/subset"

	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
)

func TestBuildConfigMetrics(t *testing.T) {
	var store = make(assets.SecretStore)

	store[assets.Key("/secrets/default/example-secret/key")] = "somesecret"
	store[assets.Key("/configMaps/default/example-cm/key")] = "somecm"
	store[assets.Key("/secrets/default/client-id/client_id")] = "my-client-id"
	store[assets.Key("/secrets/default/client-secret/client_secret")] = "somesecret-client-secret"

	tt := []struct {
		input  string
		expect string
	}{
		{
			input: util.Untab(`
				metadata:
					name: example
					namespace: default
				spec:
					logLevel: debug
					metrics:
						scrapeInterval: 15s
						scrapeTimeout: 10s
						externalLabels:
							cluster: prod
							foo: bar
						remoteWrite:
						- name: rw-1
							url: http://localhost:9090/api/v1/write
			`),
			expect: util.Untab(`
				server:
					log_level: debug

				metrics:
					wal_directory: /var/lib/grafana-agent/data
					global:
						scrape_interval: 15s
						scrape_timeout: 10s
						external_labels:
							cluster: prod
							foo: bar
							__replica__: replica-$(STATEFULSET_ORDINAL_NUMBER)
						remote_write:
						- name: rw-1
							url: http://localhost:9090/api/v1/write
			`),
		},
		{
			input: util.Untab(`
					metadata:
						name: example
						namespace: default
					spec:
						logLevel: debug
						metrics:
							scrapeInterval: 15s
							scrapeTimeout: 10s
							externalLabels:
								cluster: prod
								foo: bar
							remoteWrite:
							- url: http://localhost:9090/api/v1/write
								basicAuth:
									username:
										name: example-secret
										key: key
									password:
										name: example-secret
										key: pword
								tlsConfig:
									ca:
										configMap:
											name:	example-cm
											key: key
									cert:
										secret:
											name: example-secret
											key: key
									keySecret:
										name: example-secret
										key: key
				`),
			expect: util.Untab(`
					server:
						log_level: debug

					metrics:
						wal_directory: /var/lib/grafana-agent/data
						global:
							scrape_interval: 15s
							scrape_timeout: 10s
							external_labels:
								cluster: prod
								foo: bar
								__replica__: replica-$(STATEFULSET_ORDINAL_NUMBER)
							remote_write:
							- url: http://localhost:9090/api/v1/write
								basic_auth:
									username: somesecret
									password_file: /var/lib/grafana-agent/secrets/_secrets_default_example_secret_pword
								tls_config:
									ca_file: /var/lib/grafana-agent/secrets/_configMaps_default_example_cm_key
									cert_file: /var/lib/grafana-agent/secrets/_secrets_default_example_secret_key
									key_file: /var/lib/grafana-agent/secrets/_secrets_default_example_secret_key
				`),
		},
		{
			input: util.Untab(`
metadata:
  name: example
  namespace: default
spec:
  logLevel: debug
  metrics:
    scrapeInterval: 15s
    scrapeTimeout: 10s
    externalLabels:
      cluster: prod
      foo: bar
    remoteWrite:
    - url: http://localhost:9090/api/v1/write
      oauth2:
        clientId:
          secret:
            key: client_id
            name: client-id
        clientSecret:
          key: client_secret
          name: my-client-secret
        tokenUrl: https://auth.example.com/realms/master/protocol/openid-connect/token
    - url: http://localhost:9090/api/v1/write
      oauth2:
        clientId:
          secret:
            key: client_id
            name: client-id
        clientSecret:
          key: client_secret
          name: my-client-secret
        # test optional parameters endpointParams and scopes
        endpointParams:
          params-key0: params-value
          params-key1: params-value
        scopes:
          - value0
          - value1
        tokenUrl: https://auth.example.com/realms/master/protocol/openid-connect/token
				`),
			expect: util.Untab(`
server:
  log_level: debug

metrics:
  wal_directory: /var/lib/grafana-agent/data
  global:
    scrape_interval: 15s
    scrape_timeout: 10s
    external_labels:
      cluster: prod
      foo: bar
      __replica__: replica-$(STATEFULSET_ORDINAL_NUMBER)
    remote_write:
      - url: http://localhost:9090/api/v1/write
        oauth2:
          client_id: my-client-id
          client_secret_file: /var/lib/grafana-agent/secrets/_secrets_default_my_client_secret_client_secret
          token_url: https://auth.example.com/realms/master/protocol/openid-connect/token
      - url: http://localhost:9090/api/v1/write
        oauth2:
          client_id: my-client-id
          client_secret_file: /var/lib/grafana-agent/secrets/_secrets_default_my_client_secret_client_secret
          endpoint_params:
            params-key0: params-value
            params-key1: params-value
          scopes:
            - value0
            - value1
          token_url: https://auth.example.com/realms/master/protocol/openid-connect/token
				`),
		},
	}

	for i, tc := range tt {
		t.Run(fmt.Sprintf("index_%d", i), func(t *testing.T) {
			var spec gragent.GrafanaAgent
			err := k8s_yaml.Unmarshal([]byte(tc.input), &spec)
			require.NoError(t, err)

			d := gragent.Deployment{Agent: &spec, Secrets: store}
			result, err := BuildConfig(&d, MetricsType)
			require.NoError(t, err)

			if !assert.YAMLEq(t, tc.expect, result) {
				fmt.Println(result)
			}
		})
	}
}

func TestAdditionalScrapeConfigsMetrics(t *testing.T) {
	var store = make(assets.SecretStore)

	additionalSelector := &v1.SecretKeySelector{
		LocalObjectReference: v1.LocalObjectReference{Name: "configs"},
		Key:                  "configs",
	}

	input := gragent.Deployment{
		Agent: &gragent.GrafanaAgent{
			ObjectMeta: meta_v1.ObjectMeta{
				Namespace: "operator",
				Name:      "agent",
			},
			Spec: gragent.GrafanaAgentSpec{
				Image:              ptr.To("grafana/agent:latest"),
				ServiceAccountName: "agent",
				Metrics: gragent.MetricsSubsystemSpec{
					InstanceSelector: &meta_v1.LabelSelector{
						MatchLabels: map[string]string{"agent": "agent"},
					},
				},
			},
		},
		Metrics: []gragent.MetricsDeployment{{
			Instance: &gragent.MetricsInstance{
				ObjectMeta: meta_v1.ObjectMeta{
					Namespace: "operator",
					Name:      "primary",
				},
				Spec: gragent.MetricsInstanceSpec{
					RemoteWrite: []gragent.RemoteWriteSpec{{
						URL: "http://cortex:80/api/prom/push",
					}},
					AdditionalScrapeConfigs: additionalSelector,
				},
			},
		}},

		Secrets: store,
	}

	store[assets.KeyForSecret("operator", additionalSelector)] = util.Untab(`
	- job_name: job
		kubernetes_sd_configs:
		- role: node
	- job_name: ec2
		ec2_sd_configs:
		- region: eu-west-1
		  port: 9100
	`)

	expect := util.Untab(`
server: {}

metrics:
  wal_directory: /var/lib/grafana-agent/data
  global:
    external_labels:
      __replica__: replica-$(STATEFULSET_ORDINAL_NUMBER)
      cluster: operator/agent
  configs:
  - name: operator/primary
    remote_write:
    - url: http://cortex:80/api/prom/push
    scrape_configs:
    - job_name: job
      kubernetes_sd_configs:
      - role: node
    - job_name: ec2
      ec2_sd_configs:
      - region: eu-west-1
        port: 9100
	`)

	result, err := BuildConfig(&input, MetricsType)
	require.NoError(t, err)

	if !assert.YAMLEq(t, expect, result) {
		fmt.Println(result)
	}
}

func TestBuildConfigLogs(t *testing.T) {
	var store = make(assets.SecretStore)

	store[assets.Key("/secrets/default/example-secret/key")] = "somesecret"
	store[assets.Key("/configMaps/default/example-cm/key")] = "somecm"

	tt := []struct {
		input  string
		expect string
	}{
		{
			input: util.Untab(`
				metadata:
					name: example
					namespace: default
				spec:
					logLevel: debug
			`),
			expect: util.Untab(`
				server:
					log_level: debug
				logs:
					positions_directory: /var/lib/grafana-agent/data
			`),
		},
	}

	for i, tc := range tt {
		t.Run(fmt.Sprintf("index_%d", i), func(t *testing.T) {
			var spec gragent.GrafanaAgent
			err := k8s_yaml.Unmarshal([]byte(tc.input), &spec)
			require.NoError(t, err)

			d := gragent.Deployment{Agent: &spec, Secrets: store}
			result, err := BuildConfig(&d, LogsType)
			require.NoError(t, err)

			if !assert.YAMLEq(t, tc.expect, result) {
				fmt.Println(result)
			}
		})
	}
}

func TestBuildConfigIntegrations(t *testing.T) {
	in := util.Untab(`
	Agent:
		kind: GrafanaAgent
		metadata:
			name: test-agent
			namespace: monitoring
	Integrations:
	- Instance:
			kind: MetricsIntegration
			metadata:
				name: mysql-a
				namespace: databases
			spec:
				name: mysqld_exporter
				config: 
					data_source_names: root@(server-a:3306)/
	- Instance:
			kind: MetricsIntegration
			metadata:
				name: node
				namespace: kube-system
			spec:
				name: node_exporter
				type:
					allNodes: true
					unique: true
				config: 
					rootfs_path: /host/root
					sysfs_path: /host/sys
					procfs_path: /host/proc
	- Instance:
			metadata:
				name: mysql-b
				namespace: databases
			spec:
				name: mysqld_exporter
				config: 
					data_source_names: root@(server-b:3306)/
	- Instance:
			kind: MetricsIntegration
			metadata:
				name: redis-a
				namespace: databases
			spec:
				name: redis_exporter
				config: 
					redis_addr: redis-a:6379
  `)

	var h gragent.Deployment
	err := k8s_yaml.UnmarshalStrict([]byte(in), &h)
	require.NoError(t, err)

	expect := util.Untab(`
	server: {}
	logs:
		positions_directory: /var/lib/grafana-agent/data
	metrics:
		global:
			external_labels:
				cluster: monitoring/test-agent
		wal_directory: /var/lib/grafana-agent/data
	integrations:
		metrics:
			autoscrape:
				enable: false
		mysqld_exporter_configs:
			- data_source_names: root@(server-a:3306)/
			- data_source_names: root@(server-b:3306)/
		node_exporter_configs:
			- rootfs_path: /host/root 
				sysfs_path: /host/sys
				procfs_path: /host/proc
		redis_exporter_configs:
			- redis_addr: redis-a:6379
  `)

	result, err := BuildConfig(&h, IntegrationsType)
	require.NoError(t, err)

	require.NoError(t, subset.YAMLAssert([]byte(expect), []byte(result)), "incomplete yaml\n%s", result)
}

// TestBuildConfigIntegrations_Instances ensures that metrics and logs
// instances are injected into the resulting config so integrations can use
// them for sending telemetry data.
func TestBuildConfigIntegrations_Instances(t *testing.T) {
	in := util.Untab(`
	Agent:
		kind: GrafanaAgent
		metadata:
			name: test-agent
			namespace: monitoring
	Metrics:
	- Instance:
			kind: MetricsInstance
			metadata:
				name: operator-metrics
				namespace: primary
			spec:
				remoteWrite:
				- url: http://cortex:80/api/prom/push
	Logs:
	- Instance:
			kind: LogsInstance
			metadata:
				name: operator-logs
				namespace: primary
			spec:
				clients:
				- url: http://loki:80/loki/api/v1/push
  `)

	var h gragent.Deployment
	err := k8s_yaml.UnmarshalStrict([]byte(in), &h)
	require.NoError(t, err)

	expect := util.Untab(`
	server: {}
	metrics:
		global:
			external_labels:
				cluster: monitoring/test-agent
		wal_directory: /var/lib/grafana-agent/data
		configs:
		- name: primary/operator-metrics
			remote_write:
			- url: http://cortex:80/api/prom/push
	logs:
		positions_directory: /var/lib/grafana-agent/data
		configs:
		- name: primary/operator-logs
			clients:
			- url: http://loki:80/loki/api/v1/push
	integrations:
		metrics:
			autoscrape:
				enable: false
  `)

	result, err := BuildConfig(&h, IntegrationsType)
	require.NoError(t, err)

	require.NoError(t, subset.YAMLAssert([]byte(expect), []byte(result)), "incomplete yaml\n%s", result)
}

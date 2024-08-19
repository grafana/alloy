package traces

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/grafana/alloy/internal/static/traces/pushreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	"gopkg.in/yaml.v2"
)

func tmpFile(t *testing.T, content string) (*os.File, func()) {
	f, err := os.CreateTemp("", "")
	require.NoError(t, err)

	_, err = f.Write([]byte(content))
	require.NoError(t, err)

	err = f.Close()
	require.NoError(t, err)

	return f, func() {
		os.Remove(f.Name())
	}
}

func TestOTelConfig(t *testing.T) {
	// create a password file to test the password file logic
	password := "password_in_file"
	passwordFile, teardown := tmpFile(t, password)
	defer teardown()

	// Extra linefeed in password_file. Spaces, tabs line feeds should be
	// stripped when reading it
	passwordFileExtraNewline, teardown := tmpFile(t, password+"\n")
	defer teardown()

	// tests!
	tt := []struct {
		name           string
		cfg            string
		expectedError  bool
		expectedConfig string
	}{
		{
			name:          "disabled",
			cfg:           "",
			expectedError: true,
		},
		{
			name: "no receivers",
			cfg: `
receivers:
`,
			expectedError: true,
		},
		{
			name: "no rw endpoint",
			cfg: `
receivers:
  jaeger:
`,
			expectedError: true,
		},
		{
			name: "empty receiver config",
			cfg: `
receivers:
  jaeger:
remote_write:
  - endpoint: example.com:12345
`,
			expectedError: true,
		},
		{
			name: "basic config",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
processors: {}
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: []
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "processor config",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
attributes:
  actions:
  - key: montgomery
    value: forever
    action: update
batch:
  timeout: 5s
  send_batch_size: 100
remote_write:
  - endpoint: example.com:12345
    retry_on_failure:
      initial_interval: 10s
    sending_queue:
      num_consumers: 15
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      initial_interval: 10s
      max_elapsed_time: 60s
    sending_queue:
      num_consumers: 15
processors:
  attributes:
    actions:
    - key: montgomery
      value: forever
      action: update
  batch:
    timeout: 5s
    send_batch_size: 100
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: ["attributes", "batch"]
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "password in file",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - insecure: true
    endpoint: example.com:12345
    basic_auth:
      username: test
      password_file: ` + passwordFile.Name(),
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    tls:
      insecure: true
    headers:
      authorization: Basic dGVzdDpwYXNzd29yZF9pbl9maWxl
    retry_on_failure:
      max_elapsed_time: 60s
processors: {}
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: []
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "password in file with extra newline",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - insecure: true
    endpoint: example.com:12345
    format: otlp
    basic_auth:
      username: test
      password_file: ` + passwordFileExtraNewline.Name(),
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    tls:
      insecure: true
    headers:
      authorization: Basic dGVzdDpwYXNzd29yZF9pbl9maWxl
    retry_on_failure:
      max_elapsed_time: 60s
processors: {}
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: []
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "insecure skip verify",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - insecure_skip_verify: true
    endpoint: example.com:12345`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    tls:
      insecure_skip_verify: true
    retry_on_failure:
      max_elapsed_time: 60s
processors: {}
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: []
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "no compression",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - insecure_skip_verify: true
    endpoint: example.com:12345
    compression: none`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    tls:
      insecure_skip_verify: true
    retry_on_failure:
      max_elapsed_time: 60s
    compression: none
processors: {}
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: []
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "jaeger remote sampling multiple configs",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
jaeger_remote_sampling:
  - source:
      remote:
        endpoint: jaeger-collector:14250
        tls:
          insecure: true
  - source:
      reload_interval: 1s
      file: /etc/otelcol/sampling_strategies.json
remote_write:
  - endpoint: example.com:12345
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
processors: {}
extensions:
  jaegerremotesampling/0:
    source:
      remote:
        endpoint: jaeger-collector:14250
        tls:
          insecure: true
  jaegerremotesampling/1:
    source:
      reload_interval: 1s
      file: /etc/otelcol/sampling_strategies.json
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: []
      receivers: ["push_receiver", "jaeger"]
  extensions: ["jaegerremotesampling/0", "jaegerremotesampling/1"]
`,
		},
		{
			name: "push_config and remote_write",
			cfg: `
receivers:
  jaeger:
push_config:
  endpoint: example:12345
remote_write:
  - endpoint: anotherexample.com:12345
`,
			expectedError: true,
		},
		{
			name: "push_config.batch and batch",
			cfg: `
receivers:
  jaeger:
push_config:
  endpoint: example:12345
  batch:
    timeout: 5s
    send_batch_size: 100
batch:
  timeout: 5s
  send_batch_size: 100
remote_write:
  - endpoint: anotherexample.com:12345
`,
			expectedError: true,
		},
		{
			name: "one backend with remote_write",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
    headers:
      x-some-header: Some value!
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    headers:
      x-some-header: Some value!
    retry_on_failure:
      max_elapsed_time: 60s
processors: {}
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: []
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "two backends in a remote_write block",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
    basic_auth:
      username: test
      password: blerg
  - endpoint: anotherexample.com:12345
    compression: none
    insecure: false
    insecure_skip_verify: true
    basic_auth:
      username: test
      password_file: ` + passwordFile.Name() + `
    retry_on_failure:
      initial_interval: 10s
    sending_queue:
      num_consumers: 15
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    headers:
      authorization: Basic dGVzdDpibGVyZw==
    retry_on_failure:
      max_elapsed_time: 60s
  otlp/1:
    endpoint: anotherexample.com:12345
    tls:
      insecure: false
      insecure_skip_verify: true
    headers:
      authorization: Basic dGVzdDpwYXNzd29yZF9pbl9maWxl
    retry_on_failure:
      initial_interval: 10s
      max_elapsed_time: 60s
    sending_queue:
      num_consumers: 15
    compression: none
processors: {}
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/1", "otlp/0"]
      processors: []
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "batch block",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
batch:
  timeout: 5s
  send_batch_size: 100
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
processors:
  batch:
    timeout: 5s
    send_batch_size: 100
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: ["batch"]
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "span metrics remote write exporter",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
spanmetrics:
  latency_histogram_buckets: [2ms, 6ms, 10ms, 100ms, 250ms]
  dimensions:
    - name: http.method
      default: GET
    - name: http.status_code
  metrics_instance: traces
  dimensions_cache_size: 10000
  aggregation_temporality: AGGREGATION_TEMPORALITY_DELTA
  metrics_flush_interval: 20s
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  noop:
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
  remote_write:
    namespace: traces_spanmetrics
    metrics_instance: traces
processors:
  spanmetrics:
    metrics_exporter: remote_write
    latency_histogram_buckets: [2ms, 6ms, 10ms, 100ms, 250ms]
    dimensions:
      - name: http.method
        default: GET
      - name: http.status_code
    dimensions_cache_size: 10000
    aggregation_temporality: AGGREGATION_TEMPORALITY_DELTA
    metrics_flush_interval: 20s
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: ["spanmetrics"]
      receivers: ["push_receiver", "jaeger"]
    metrics/spanmetrics:
      exporters: ["remote_write"]
      receivers: ["noop"]
`,
		},
		{
			name: "span metrics prometheus exporter",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
spanmetrics:
  handler_endpoint: "0.0.0.0:8889"
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  noop:
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
  prometheus:
    endpoint: "0.0.0.0:8889"
    namespace: traces_spanmetrics
processors:
  spanmetrics:
    metrics_exporter: prometheus
    latency_histogram_buckets: []
    dimensions: []
    aggregation_temporality: AGGREGATION_TEMPORALITY_CUMULATIVE
    metrics_flush_interval: 15s
    dimensions_cache_size: 1000

extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: ["spanmetrics"]
      receivers: ["push_receiver", "jaeger"]
    metrics/spanmetrics:
      exporters: ["prometheus"]
      receivers: ["noop"]
`,
		},
		{
			name: "span metrics prometheus and remote write exporters fail",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
spanmetrics:
  handler_endpoint: "0.0.0.0:8889"
  metrics_instance: traces
`,
			expectedError: true,
		},
		{
			name: "tail sampling config",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
tail_sampling:
  decision_wait: 11s
  num_traces: 98765
  expected_new_traces_per_sec: 76
  policies:
    - type: always_sample
    - type: latency
      latency:
        threshold_ms: 100
    - type: numeric_attribute
      numeric_attribute:
        key: key1
        min_value: 50
        max_value: 100
    - type: probabilistic
      probabilistic:
        sampling_percentage: 10
    - type: status_code
      status_code:
        status_codes:
          - ERROR
          - UNSET
    - type: string_attribute
      string_attribute:
        key: key
        values:
          - value1
          - value2
    - type: rate_limiting
      rate_limiting:
        spans_per_second: 35
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
processors:
  tail_sampling:
    decision_wait: 11s
    num_traces: 98765
    expected_new_traces_per_sec: 76
    policies:
      - name: always_sample/0
        type: always_sample
      - name: latency/1
        type: latency
        latency:
          threshold_ms: 100
      - name: numeric_attribute/2
        type: numeric_attribute
        numeric_attribute:
          key: key1
          min_value: 50
          max_value: 100
      - name: probabilistic/3
        type: probabilistic
        probabilistic:
          sampling_percentage: 10
      - name: status_code/4
        type: status_code
        status_code:
          status_codes:
            - ERROR
            - UNSET
      - name: string_attribute/5
        type: string_attribute
        string_attribute:
          key: key
          values:
            - value1
            - value2
      - name: rate_limiting/6
        type: rate_limiting
        rate_limiting:
          spans_per_second: 35
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: ["tail_sampling"]
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "tail sampling config with DNS load balancing",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
tail_sampling:
  policies:
    - type: always_sample
    - type: string_attribute
      string_attribute:
        key: key
        values:
          - value1
          - value2
load_balancing:
  receiver_port: 8181
  routing_key: service
  exporter:
    insecure: true
  resolver:
    dns:
      hostname: agent
      port: "8282"
      interval: 12m
      timeout: 76s
`,
			expectedConfig: `
receivers:
  jaeger:
    protocols:
      grpc:
  push_receiver: {}
  otlp/lb:
    protocols:
      grpc:
        endpoint: "0.0.0.0:8181"
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
  loadbalancing:
    routing_key: service
    protocol:
      otlp:
        tls:
          insecure: true
        endpoint: noop:8888
        retry_on_failure:
          max_elapsed_time: 60s
        compression: none
    resolver:
      dns:
        hostname: agent
        port: "8282"
        interval: 12m
        timeout: 76s
processors:
  tail_sampling:
    decision_wait: 5s
    policies:
      - name: always_sample/0
        type: always_sample
      - name: string_attribute/1
        type: string_attribute
        string_attribute:
          key: key
          values:
            - value1
            - value2
extensions: {}
service:
  pipelines:
    traces/0:
      exporters: ["loadbalancing"]
      processors: []
      receivers: ["jaeger", "push_receiver"]
    traces/1:
      exporters: ["otlp/0"]
      processors: ["tail_sampling"]
      receivers: ["otlp/lb"]
`,
		},
		{
			name: "tail sampling config with Kubernetes load balancing",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
tail_sampling:
  policies:
    - type: always_sample
    - type: string_attribute
      string_attribute:
        key: key
        values:
          - value1
          - value2
load_balancing:
  receiver_port: 8181
  routing_key: service
  exporter:
    insecure: true
  resolver:
    kubernetes:
      service: lb-svc.lb-ns
      ports:
      - 55690
      - 55691
`,
			expectedConfig: `
receivers:
  jaeger:
    protocols:
      grpc:
  push_receiver: {}
  otlp/lb:
    protocols:
      grpc:
        endpoint: "0.0.0.0:8181"
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
  loadbalancing:
    routing_key: service
    protocol:
      otlp:
        tls:
          insecure: true
        endpoint: noop:8888
        retry_on_failure:
          max_elapsed_time: 60s
        compression: none
    resolver:
      k8s:
        service: lb-svc.lb-ns
        ports:
        - 55690
        - 55691
processors:
  tail_sampling:
    decision_wait: 5s
    policies:
      - name: always_sample/0
        type: always_sample
      - name: string_attribute/1
        type: string_attribute
        string_attribute:
          key: key
          values:
            - value1
            - value2
extensions: {}
service:
  pipelines:
    traces/0:
      exporters: ["loadbalancing"]
      processors: []
      receivers: ["jaeger", "push_receiver"]
    traces/1:
      exporters: ["otlp/0"]
      processors: ["tail_sampling"]
      receivers: ["otlp/lb"]
`,
		},
		{
			name: "automatic logging : default",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
automatic_logging:
  spans: true
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
processors:
  automatic_logging:
    automatic_logging:
      spans: true
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: ["automatic_logging"]
      receivers: ["push_receiver", "jaeger"]
      `,
		},
		{
			name: "tls config",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - insecure: false
    tls_config:
      ca_file: server.crt
      cert_file: client.crt
      key_file: client.key
    endpoint: example.com:12345
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    tls:
      insecure: false
      ca_file: server.crt
      cert_file: client.crt
      key_file: client.key
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
processors: {}
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: []
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "otlp http & grpc exporters",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
    protocol: http
  - endpoint: example.com:12345
    protocol: grpc
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
exporters:
  otlphttp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
  otlp/1:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
processors: {}
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlphttp/0", "otlp/1"]
      processors: []
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "prom SD config",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
    protocol: grpc
scrape_configs:
  - im_a_scrape_config
prom_sd_operation_type: update
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
processors:
  prom_sd_processor:
    scrape_configs:
      - im_a_scrape_config
    operation_type: update
    pod_associations: []
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: ["prom_sd_processor"]
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "service graphs",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
service_graphs:
  enabled: true
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
processors:
  service_graphs:
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: ["service_graphs"]
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "one exporter with oauth2 and basic auth",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
    basic_auth:
      username: test
      password: blerg
    oauth2:
      client_id: somecclient
      client_secret: someclientsecret
`,
			expectedError: true,
		},
		{
			name: "simple oauth2 config",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
    protocol: http
    oauth2:
      client_id: someclientid
      client_secret: someclientsecret
      token_url: https://example.com/oauth2/default/v1/token
      scopes: ["api.metrics"]
      timeout: 2s
`,
			// The tls.ciphersuites would appear that it should output nothing or nil but during the conversion otelcolConfigFromStringMap the otelcol.Unmarshal converts the nil array to a blank one []. This is specifically used
			// in confmap.go that transforms nil arrays into [] on the zeroSliceHookFunc.
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
extensions:
  oauth2client/otlphttp0:
    client_id: someclientid
    client_secret: someclientsecret
    token_url: https://example.com/oauth2/default/v1/token
    scopes: ["api.metrics"]
    timeout: 2s
    tls:
      cipher_suites: []
exporters:
  otlphttp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
    auth:
      authenticator: oauth2client/otlphttp0
processors: {}
service:
  extensions: ["oauth2client/otlphttp0"]
  pipelines:
    traces:
      exporters: ["otlphttp/0"]
      processors: []
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "oauth2 TLS with certs and keys from files",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
    protocol: http
    oauth2:
      client_id: someclientid
      client_secret: someclientsecret
      endpoint_params:
        audience: [someaudience]
      token_url: https://example.com/oauth2/default/v1/token
      scopes: ["api.metrics"]
      timeout: 2s
      tls:
        insecure: true
        insecure_skip_verify: true
        ca_file: /var/lib/mycert.pem
        cert_file: certfile
        key_file: keyfile
        min_version: "1.3"
        reload_interval: 1h
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
extensions:
  oauth2client/otlphttp0:
    client_id: someclientid
    client_secret: someclientsecret
    endpoint_params:
      audience: someaudience
    token_url: https://example.com/oauth2/default/v1/token
    scopes: ["api.metrics"]
    timeout: 2s
    tls:
      insecure: true
      insecure_skip_verify: true
      ca_file: /var/lib/mycert.pem
      cert_file: certfile
      key_file: keyfile
      min_version: "1.3"
      reload_interval: 1h
      cipher_suites: []
exporters:
  otlphttp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
    auth:
      authenticator: oauth2client/otlphttp0
processors: {}
service:
  extensions: ["oauth2client/otlphttp0"]
  pipelines:
    traces:
      exporters: ["otlphttp/0"]
      processors: []
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "oauth2 TLS with certs and keys from strings",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
    protocol: http
    oauth2:
      client_id: someclientid
      client_secret: someclientsecret
      endpoint_params:
        audience: [someaudience]
      token_url: https://example.com/oauth2/default/v1/token
      scopes: ["api.metrics"]
      timeout: 2s
      tls:
        insecure: true
        insecure_skip_verify: true
        ca_pem: test_secret_ca_pem_string
        cert_pem: test_secret_cert_pem_string
        key_pem: test_secret_key_pem_string
        max_version: "1.2"
        reload_interval: 1h
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
extensions:
  oauth2client/otlphttp0:
    client_id: someclientid
    client_secret: someclientsecret
    endpoint_params:
      audience: someaudience
    token_url: https://example.com/oauth2/default/v1/token
    scopes: ["api.metrics"]
    timeout: 2s
    tls:
      insecure: true
      insecure_skip_verify: true
      ca_pem: test_secret_ca_pem_string
      cert_pem: test_secret_cert_pem_string
      key_pem: test_secret_key_pem_string
      max_version: "1.2"
      reload_interval: 1h
      cipher_suites: []
exporters:
  otlphttp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
    auth:
      authenticator: oauth2client/otlphttp0
processors: {}
service:
  extensions: ["oauth2client/otlphttp0"]
  pipelines:
    traces:
      exporters: ["otlphttp/0"]
      processors: []
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "2 exporters different auth",
			cfg: `
receivers:
 jaeger:
   protocols:
     grpc:
remote_write:
 - endpoint: example.com:12345
   protocol: http
   oauth2:
     client_id: someclientid
     client_secret: someclientsecret
     token_url: https://example.com/oauth2/default/v1/token
     scopes: ["api.metrics"]
     timeout: 2s
 - endpoint: example.com:12345
   protocol: grpc
   oauth2:
     client_id: anotherclientid
     client_secret: anotherclientsecret
     token_url: https://example.com/oauth2/default/v1/token
     scopes: ["api.metrics"]
     timeout: 2s
`,
			expectedConfig: `
receivers:
 push_receiver: {}
 jaeger:
   protocols:
     grpc:
extensions:
 oauth2client/otlphttp0:
   client_id: someclientid
   client_secret: someclientsecret
   token_url: https://example.com/oauth2/default/v1/token
   scopes: ["api.metrics"]
   timeout: 2s
   tls:
     cipher_suites: []
 oauth2client/otlp1:
   client_id: anotherclientid
   client_secret: anotherclientsecret
   token_url: https://example.com/oauth2/default/v1/token
   scopes: ["api.metrics"]
   timeout: 2s
   tls:
     cipher_suites: []
exporters:
  otlphttp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
    auth:
      authenticator: oauth2client/otlphttp0
  otlp/1:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
    auth:
      authenticator: oauth2client/otlp1
processors: {}
service:
  extensions: ["oauth2client/otlphttp0", "oauth2client/otlp1"]
  pipelines:
    traces:
      exporters: ["otlphttp/0", "otlp/1"]
      processors: []
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "exporter with insecure oauth",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: http://example.com:12345
    insecure: true
    protocol: http
    oauth2:
      client_id: someclientid
      client_secret: someclientsecret
      token_url: https://example.com/oauth2/default/v1/token
      scopes: ["api.metrics"]
      timeout: 2s
      tls:
        insecure: true
`,
			expectedConfig: `
receivers:
  push_receiver: {}
  jaeger:
    protocols:
      grpc:
extensions:
  oauth2client/otlphttp0:
    client_id: someclientid
    client_secret: someclientsecret
    token_url: https://example.com/oauth2/default/v1/token
    scopes: ["api.metrics"]
    timeout: 2s
    tls:
      insecure: true
      cipher_suites: []
exporters:
  otlphttp/0:
    endpoint: http://example.com:12345
    tls:
      insecure: true
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
    auth:
      authenticator: oauth2client/otlphttp0
processors: {}
service:
  extensions: ["oauth2client/otlphttp0"]
  pipelines:
    traces:
      exporters: ["otlphttp/0"]
      processors: []
      receivers: ["push_receiver", "jaeger"]
`,
		},
		{
			name: "OTLP receivers get include_metadata set to true by default",
			cfg: `
receivers:
  otlp/0:
    protocols:
      grpc:
      http:
        endpoint: localhost:4318
  otlp/1:
    protocols:
      grpc:
        include_metadata: false
      http:
        include_metadata: false
        endpoint: localhost:4318
remote_write:
  - endpoint: example.com:12345
`,
			expectedConfig: `
receivers:
  push_receiver:
  otlp/0:
    protocols:
      grpc:
        include_metadata: true
      http:
        include_metadata: true
        endpoint: localhost:4318
  otlp/1:
    protocols:
      grpc:
        include_metadata: false
      http:
        include_metadata: false
        endpoint: localhost:4318
exporters:
  otlp/0:
    endpoint: example.com:12345
    compression: gzip
    retry_on_failure:
      max_elapsed_time: 60s
processors: {}
extensions: {}
service:
  pipelines:
    traces:
      exporters: ["otlp/0"]
      processors: []
      receivers: ["push_receiver", "otlp/0", "otlp/1"]
`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var cfg InstanceConfig
			err := yaml.Unmarshal([]byte(tc.cfg), &cfg)
			require.NoError(t, err)
			// check error
			actualConfig, err := cfg.OtelConfig()
			if tc.expectedError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// convert actual config to otel config
			otelMapStructure := map[string]interface{}{}
			err = yaml.Unmarshal([]byte(tc.expectedConfig), otelMapStructure)
			require.NoError(t, err)

			factories, err := tracingFactories()
			require.NoError(t, err)

			expectedConfig, err := otelcolConfigFromStringMap(otelMapStructure, &factories)
			require.NoError(t, err)

			// Exporters/Receivers/Processors in the config's service.Pipelines, as well as
			// service.Extensions have to be in the same order for them to be asserted as equal.
			sortService(actualConfig)
			sortService(expectedConfig)

			assert.Equal(t, *expectedConfig, *actualConfig)
		})
	}
}

func TestProcessorOrder(t *testing.T) {
	// tests!
	tt := []struct {
		name               string
		cfg                string
		expectedProcessors map[component.ID][]component.ID
	}{
		{
			name: "no processors",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
    headers:
      x-some-header: Some value!
`,
			expectedProcessors: map[component.ID][]component.ID{
				component.NewID(component.MustNewType("traces")): nil,
			},
		},
		{
			name: "all processors w/o load balancing",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
    headers:
      x-some-header: Some value!
attributes:
  actions:
  - key: montgomery
    value: forever
    action: update
spanmetrics:
  latency_histogram_buckets: [2ms, 6ms, 10ms, 100ms, 250ms]
  dimensions:
    - name: http.method
      default: GET
    - name: http.status_code
  metrics_instance: traces
automatic_logging:
  spans: true
batch:
  timeout: 5s
  send_batch_size: 100
tail_sampling:
  policies:
    - type: always_sample
    - type: string_attribute
      string_attribute:
        key: key
        values:
          - value1
          - value2
service_graphs:
  enabled: true
`,
			expectedProcessors: map[component.ID][]component.ID{
				component.NewID(component.MustNewType("traces")): {
					component.NewID(component.MustNewType("attributes")),
					component.NewID(component.MustNewType("spanmetrics")),
					component.NewID(component.MustNewType("service_graphs")),
					component.NewID(component.MustNewType("tail_sampling")),
					component.NewID(component.MustNewType("automatic_logging")),
					component.NewID(component.MustNewType("batch")),
				},
				component.NewIDWithName(component.MustNewType(spanMetricsPipelineType), spanMetricsPipelineName): nil,
			},
		},
		{
			name: "all processors with load balancing",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
    headers:
      x-some-header: Some value!
attributes:
  actions:
  - key: montgomery
    value: forever
    action: update
spanmetrics:
  latency_histogram_buckets: [2ms, 6ms, 10ms, 100ms, 250ms]
  dimensions:
    - name: http.method
      default: GET
    - name: http.status_code
  metrics_instance: traces
automatic_logging:
  spans: true
batch:
  timeout: 5s
  send_batch_size: 100
tail_sampling:
  policies:
    - type: always_sample
    - type: string_attribute
      string_attribute:
        key: key
        values:
          - value1
          - value2
load_balancing:
  exporter:
    tls:
      insecure: true
  resolver:
    dns:
      hostname: agent
      port: "4318"
service_graphs:
  enabled: true
`,
			expectedProcessors: map[component.ID][]component.ID{
				component.NewIDWithName(component.MustNewType("traces"), "0"): {
					component.NewID(component.MustNewType("attributes")),
				},
				component.NewIDWithName(component.MustNewType("traces"), "1"): {
					component.NewID(component.MustNewType("spanmetrics")),
					component.NewID(component.MustNewType("service_graphs")),
					component.NewID(component.MustNewType("tail_sampling")),
					component.NewID(component.MustNewType("automatic_logging")),
					component.NewID(component.MustNewType("batch")),
				},
				component.NewIDWithName(component.MustNewType(spanMetricsPipelineType), spanMetricsPipelineName): nil,
			},
		},
		{
			name: "load balancing without tail sampling",
			cfg: `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345
    headers:
      x-some-header: Some value!
attributes:
  actions:
  - key: montgomery
    value: forever
    action: update
spanmetrics:
  latency_histogram_buckets: [2ms, 6ms, 10ms, 100ms, 250ms]
  dimensions:
    - name: http.method
      default: GET
    - name: http.status_code
  metrics_instance: traces
automatic_logging:
  spans: true
batch:
  timeout: 5s
  send_batch_size: 100
load_balancing:
  exporter:
    tls:
      insecure: true
  resolver:
    dns:
      hostname: agent
      port: "4318"
`,
			expectedProcessors: map[component.ID][]component.ID{
				component.NewIDWithName(component.MustNewType("traces"), "0"): {
					component.NewID(component.MustNewType("attributes")),
				},
				component.NewIDWithName(component.MustNewType("traces"), "1"): {
					component.NewID(component.MustNewType("spanmetrics")),
					component.NewID(component.MustNewType("automatic_logging")),
					component.NewID(component.MustNewType("batch")),
				},
				component.NewIDWithName(component.MustNewType(spanMetricsPipelineType), spanMetricsPipelineName): nil,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var cfg InstanceConfig
			err := yaml.Unmarshal([]byte(tc.cfg), &cfg)
			require.NoError(t, err)

			// check error
			actualConfig, err := cfg.OtelConfig()
			require.NoError(t, err)

			require.Equal(t, len(tc.expectedProcessors), len(actualConfig.Service.Pipelines))
			for componentID := range tc.expectedProcessors {
				if len(tc.expectedProcessors[componentID]) > 0 {
					assert.NotNil(t, tc.expectedProcessors)
					assert.NotNil(t, actualConfig.Service.Pipelines[componentID])

					assert.Equal(t, tc.expectedProcessors[componentID], actualConfig.Service.Pipelines[componentID].Processors)
				}
			}
		})
	}
}

func TestOrderProcessors(t *testing.T) {
	tests := []struct {
		processors     []string
		splitPipelines bool
		expected       [][]string
	}{
		{
			expected: [][]string{
				nil,
			},
		},
		{
			processors: []string{
				"tail_sampling",
			},
			expected: [][]string{
				{"tail_sampling"},
			},
		},
		{
			processors: []string{
				"batch",
				"tail_sampling",
				"automatic_logging",
			},
			expected: [][]string{
				{
					"tail_sampling",
					"automatic_logging",
					"batch",
				},
			},
		},
		{
			processors: []string{
				"spanmetrics",
				"batch",
				"tail_sampling",
				"attributes",
				"automatic_logging",
			},
			expected: [][]string{
				{
					"attributes",
					"spanmetrics",
					"tail_sampling",
					"automatic_logging",
					"batch",
				},
			},
		},
		{
			splitPipelines: true,
			expected: [][]string{
				nil,
				nil,
			},
		},
		{
			processors: []string{
				"spanmetrics",
				"batch",
				"tail_sampling",
				"attributes",
				"automatic_logging",
			},
			splitPipelines: true,
			expected: [][]string{
				{
					"attributes",
				},
				{
					"spanmetrics",
					"tail_sampling",
					"automatic_logging",
					"batch",
				},
			},
		},
		{
			processors: []string{
				"batch",
				"tail_sampling",
				"automatic_logging",
			},
			splitPipelines: true,
			expected: [][]string{
				{},
				{
					"tail_sampling",
					"automatic_logging",
					"batch",
				},
			},
		},
		{
			processors: []string{
				"spanmetrics",
				"attributes",
			},
			splitPipelines: true,
			expected: [][]string{
				{
					"attributes",
				},
				{
					"spanmetrics",
				},
			},
		},
	}

	for _, tc := range tests {
		actual := orderProcessors(tc.processors, tc.splitPipelines)
		assert.Equal(t, tc.expected, actual)
	}
}

func TestScrubbedReceivers(t *testing.T) {
	test := `
receivers:
  jaeger:
    protocols:
      grpc:`
	var cfg InstanceConfig
	err := yaml.Unmarshal([]byte(test), &cfg)
	assert.Nil(t, err)
	data, err := yaml.Marshal(cfg)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(string(data), "<secret>"))
}

func TestCreatingPushReceiver(t *testing.T) {
	test := `
receivers:
  jaeger:
    protocols:
      grpc:
remote_write:
  - endpoint: example.com:12345`
	cfg := InstanceConfig{}
	err := yaml.Unmarshal([]byte(test), &cfg)
	assert.Nil(t, err)
	otel, err := cfg.OtelConfig()
	assert.Nil(t, err)
	assert.Contains(t, otel.Service.Pipelines[component.NewID(component.MustNewType("traces"))].Receivers, component.NewID(component.MustNewType(pushreceiver.TypeStr)))
}

func TestUnmarshalYAMLEmptyOTLP(t *testing.T) {
	test := `
receivers:
  otlp:`
	cfg := InstanceConfig{}
	err := yaml.Unmarshal([]byte(test), &cfg)
	assert.NotNil(t, err)
	require.Contains(t, err.Error(), "failed to parse OTLP receiver config: otlp")
}

func TestUnmarshalYAMLEmptyOTLPProtocols(t *testing.T) {
	test := `
receivers:
  otlp:
    protocols:`
	cfg := InstanceConfig{}
	err := yaml.Unmarshal([]byte(test), &cfg)
	assert.NotNil(t, err)
	require.Contains(t, err.Error(), "otlp receiver requires a \"protocols\" field which must be a YAML map: otlp")
}

// sortService is a helper function to lexicographically sort all
// the possibly unsorted elements of a given cfg.Service
func sortService(cfg *otelcol.Config) {
	sort.Slice(cfg.Service.Extensions, func(i, j int) bool { return cfg.Service.Extensions[i].String() > cfg.Service.Extensions[j].String() })

	for _, pipeline := range cfg.Service.Pipelines {
		sort.Slice(pipeline.Exporters, func(i, j int) bool { return pipeline.Exporters[i].String() > pipeline.Exporters[j].String() })
		sort.Slice(pipeline.Receivers, func(i, j int) bool { return pipeline.Receivers[i].String() > pipeline.Receivers[j].String() })
		sort.Slice(pipeline.Processors, func(i, j int) bool { return pipeline.Processors[i].String() > pipeline.Processors[j].String() })
	}
}

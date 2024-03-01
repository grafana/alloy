package instance

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestUnmarshalConfig_Valid(t *testing.T) {
	validConfig := DefaultConfig
	validConfigContent, err := yaml.Marshal(validConfig)
	require.NoError(t, err)

	_, err = UnmarshalConfig(bytes.NewReader(validConfigContent))
	require.NoError(t, err)
}

func TestUnmarshalConfig_Invalid(t *testing.T) {
	invalidConfigContent := `whyWouldAnyoneThinkThisisAValidConfig: 12345`

	_, err := UnmarshalConfig(strings.NewReader(invalidConfigContent))
	require.Error(t, err)
}

// TestMarshal_UnmarshalConfig_RetainSecrets ensures that secrets can be
// retained.
func TestMarshal_UnmarshalConfig_RetainSecrets(t *testing.T) {
	cfg := `name: test
scrape_configs:
- job_name: local_scrape
  follow_redirects: true
  enable_http2: true
  honor_timestamps: true
  metrics_path: /metrics
  scheme: http
  track_timestamps_staleness: true
  static_configs:
  - targets:
    - 127.0.0.1:12345
    labels:
      cluster: localhost
  basic_auth:
    username: admin
    password: foobar
remote_write:
- url: http://admin:verysecret@localhost:9009/api/prom/push
  remote_timeout: 30s
  name: test-d0f32c
  send_exemplars: true
  basic_auth:
    username: admin
    password: verysecret
  queue_config:
    capacity: 500
    max_shards: 1000
    min_shards: 1
    max_samples_per_send: 100
    batch_send_deadline: 5s
    min_backoff: 30ms
    max_backoff: 100ms
    retry_on_http_429: true
  follow_redirects: true
  enable_http2: true
  metadata_config:
    max_samples_per_send: 500
    send: true
    send_interval: 1m
wal_truncate_frequency: 1m0s
min_wal_time: 5m0s
max_wal_time: 4h0m0s
remote_flush_deadline: 1m0s
`

	c, err := UnmarshalConfig(strings.NewReader(cfg))
	require.NoError(t, err)

	out, err := MarshalConfig(c, false)
	require.NoError(t, err)
	require.YAMLEq(t, cfg, string(out))
}

// TestMarshal_UnmarshalConfig_ScrubSecrets ensures that secrets can be
// scrubbed.
func TestMarshal_UnmarshalConfig_ScrubSecrets(t *testing.T) {
	cfg := `name: test
scrape_configs:
- job_name: local_scrape
  follow_redirects: true
  enable_http2: true
  honor_timestamps: true
  metrics_path: /metrics
  scheme: http
  track_timestamps_staleness: true
  static_configs:
  - targets:
    - 127.0.0.1:12345
    labels:
      cluster: localhost
  basic_auth:
    username: admin
    password: SCRUBME
remote_write:
- url: http://username:SCRUBURL@localhost:9009/api/prom/push
  remote_timeout: 30s
  name: test-d0f32c
  send_exemplars: true
  basic_auth:
    username: admin
    password: SCRUBME
  queue_config:
    capacity: 500
    max_shards: 1000
    min_shards: 1
    max_samples_per_send: 100
    batch_send_deadline: 5s
    min_backoff: 30ms
    max_backoff: 100ms
    retry_on_http_429: true
  follow_redirects: true
  enable_http2: true
  metadata_config:
    max_samples_per_send: 500
    send: true
    send_interval: 1m
wal_truncate_frequency: 1m0s
min_wal_time: 5m0s
max_wal_time: 4h0m0s
remote_flush_deadline: 1m0s
`

	scrub := func(in string) string {
		in = strings.ReplaceAll(in, "SCRUBME", "<secret>")
		in = strings.ReplaceAll(in, "SCRUBURL", "xxxxx")
		return in
	}

	t.Run("direct marshal", func(t *testing.T) {
		var c Config
		err := yaml.Unmarshal([]byte(cfg), &c)
		require.NoError(t, err)

		out, err := yaml.Marshal(c)
		require.NoError(t, err)
		require.YAMLEq(t, scrub(cfg), string(out))
	})

	t.Run("direct marshal pointer", func(t *testing.T) {
		var c Config
		err := yaml.Unmarshal([]byte(cfg), &c)
		require.NoError(t, err)

		out, err := yaml.Marshal(&c)
		require.NoError(t, err)
		require.YAMLEq(t, scrub(cfg), string(out))
	})

	t.Run("custom marshal methods", func(t *testing.T) {
		c, err := UnmarshalConfig(strings.NewReader(cfg))
		require.NoError(t, err)

		out, err := MarshalConfig(c, true)
		require.NoError(t, err)
		require.YAMLEq(t, scrub(cfg), string(out))
	})
}

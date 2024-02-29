package redis

import (
	"testing"
	"time"

	"github.com/grafana/agent/internal/static/integrations/redis_exporter"
	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

func TestRiverUnmarshal(t *testing.T) {
	riverConfig := `
		redis_addr                   = "localhost:6379"
		redis_user                   = "redis_user"
		redis_password_file          = "/tmp/pass"
		namespace                    = "namespace"
		config_command               = "TEST_CONFIG"
		check_keys                   = ["key1*", "cache_*"]
		check_key_groups             = ["other_key%d+"]
		check_key_groups_batch_size  = 5000
		max_distinct_key_groups      = 50
		check_single_keys            = ["particular_key"]
		check_streams                = ["stream1*"]
		check_single_streams         = ["particular_stream"]
		export_key_values            = false
		count_keys                   = ["count_key1", "count_key2"]
		script_path                  = "/tmp/metrics-script.lua,/tmp/cooler-metrics-script.lua"
		connection_timeout           = "7s"
		tls_client_key_file          = "/tmp/client-key.pem"
		tls_client_cert_file         = "/tmp/client-cert.pem"
		tls_ca_cert_file             = "/tmp/ca-cert.pem"
		set_client_name              = false
		is_tile38                    = true
		export_client_list           = false
		export_client_port           = true
		redis_metrics_only           = false
		ping_on_connect              = true
		incl_system_metrics          = true
		skip_tls_verification        = false
		is_cluster                   = true
	`
	var args Arguments
	err := river.Unmarshal([]byte(riverConfig), &args)

	require.NoError(t, err)
	expected := Arguments{
		RedisAddr:         "localhost:6379",
		RedisUser:         "redis_user",
		RedisPasswordFile: "/tmp/pass",
		Namespace:         "namespace",
		ConfigCommand:     "TEST_CONFIG",

		CheckKeys:               []string{"key1*", "cache_*"},
		CheckKeyGroups:          []string{"other_key%d+"},
		CheckSingleKeys:         []string{"particular_key"},
		CheckKeyGroupsBatchSize: int64(5000),
		MaxDistinctKeyGroups:    int64(50),

		CheckStreams:       []string{"stream1*"},
		CheckSingleStreams: []string{"particular_stream"},
		ExportKeyValues:    false,
		CountKeys:          []string{"count_key1", "count_key2"},

		ScriptPath:        "/tmp/metrics-script.lua,/tmp/cooler-metrics-script.lua",
		ConnectionTimeout: 7 * time.Second,

		TLSClientKeyFile:  "/tmp/client-key.pem",
		TLSClientCertFile: "/tmp/client-cert.pem",
		TLSCaCertFile:     "/tmp/ca-cert.pem",

		SetClientName:       false,
		IsTile38:            true,
		ExportClientList:    false,
		ExportClientPort:    true,
		RedisMetricsOnly:    false,
		PingOnConnect:       true,
		InclSystemMetrics:   true,
		SkipTLSVerification: false,
		IsCluster:           true,
	}
	require.Equal(t, expected, args)
}

func TestUnmarshalInvalid(t *testing.T) {
	validRiverConfig := `
	redis_addr  = "localhost:1234"
	script_path = "/tmp/metrics.lua"`

	var args Arguments
	err := river.Unmarshal([]byte(validRiverConfig), &args)
	require.NoError(t, err)

	invalidRiverConfig := `
	redis_addr   = "localhost:1234
	script_path  = "/tmp/metrics.lua"
	script_paths = ["/tmp/more-metrics.lua", "/tmp/even-more-metrics.lua"]`

	var invalidArgs Arguments
	err = river.Unmarshal([]byte(invalidRiverConfig), &invalidArgs)
	require.Error(t, err)
}

func TestRiverConvert(t *testing.T) {
	orig := Arguments{
		RedisAddr:         "localhost:6379",
		RedisUser:         "redis_user",
		RedisPasswordFile: "/tmp/pass",
		Namespace:         "namespace",
		ConfigCommand:     "TEST_CONFIG",

		CheckKeys:               []string{"key1*", "cache_*"},
		CheckKeyGroups:          []string{"other_key%d+"},
		CheckSingleKeys:         []string{"particular_key"},
		ExportKeyValues:         false,
		CountKeys:               []string{"count_key1", "count_key2"},
		CheckKeyGroupsBatchSize: 5000,
		MaxDistinctKeyGroups:    50,

		CheckStreams:       []string{"stream1*", "stream2*"},
		CheckSingleStreams: []string{"particular_stream"},

		ScriptPath:        "/tmp/metrics-script.lua,/tmp/cooler-metrics-script.lua",
		ConnectionTimeout: 7 * time.Second,

		TLSClientKeyFile:  "/tmp/client-key.pem",
		TLSClientCertFile: "/tmp/client-cert.pem",
		TLSCaCertFile:     "/tmp/ca-cert.pem",

		SetClientName:       false,
		IsTile38:            true,
		ExportClientList:    false,
		ExportClientPort:    true,
		RedisMetricsOnly:    false,
		PingOnConnect:       true,
		InclSystemMetrics:   true,
		SkipTLSVerification: false,
	}
	converted := orig.Convert()
	expected := redis_exporter.Config{
		RedisAddr:         "localhost:6379",
		RedisUser:         "redis_user",
		RedisPasswordFile: "/tmp/pass",
		Namespace:         "namespace",
		ConfigCommand:     "TEST_CONFIG",

		CheckKeys:               "key1*,cache_*",
		CheckKeyGroups:          "other_key%d+",
		CheckSingleKeys:         "particular_key",
		ExportKeyValues:         false,
		CountKeys:               "count_key1,count_key2",
		CheckKeyGroupsBatchSize: 5000,
		MaxDistinctKeyGroups:    50,

		CheckStreams:       "stream1*,stream2*",
		CheckSingleStreams: "particular_stream",

		ScriptPath:        "/tmp/metrics-script.lua,/tmp/cooler-metrics-script.lua",
		ConnectionTimeout: 7 * time.Second,

		TLSClientKeyFile:  "/tmp/client-key.pem",
		TLSClientCertFile: "/tmp/client-cert.pem",
		TLSCaCertFile:     "/tmp/ca-cert.pem",

		SetClientName:       false,
		IsTile38:            true,
		ExportClientList:    false,
		ExportClientPort:    true,
		RedisMetricsOnly:    false,
		PingOnConnect:       true,
		InclSystemMetrics:   true,
		SkipTLSVerification: false,
	}

	require.Equal(t, expected, *converted)
}

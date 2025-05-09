package tests

import (
	"testing"
	"time"

	config_util "github.com/prometheus/common/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/agent"
	"github.com/grafana/alloy/internal/static/integrations/apache_http"
	"github.com/grafana/alloy/internal/static/integrations/azure_exporter"
	"github.com/grafana/alloy/internal/static/integrations/blackbox_exporter"
	"github.com/grafana/alloy/internal/static/integrations/cadvisor"
	"github.com/grafana/alloy/internal/static/integrations/catchpoint_exporter"
	"github.com/grafana/alloy/internal/static/integrations/cloudwatch_exporter"
	"github.com/grafana/alloy/internal/static/integrations/consul_exporter"
	"github.com/grafana/alloy/internal/static/integrations/dnsmasq_exporter"
	"github.com/grafana/alloy/internal/static/integrations/elasticsearch_exporter"
	"github.com/grafana/alloy/internal/static/integrations/gcp_exporter"
	"github.com/grafana/alloy/internal/static/integrations/github_exporter"
	"github.com/grafana/alloy/internal/static/integrations/kafka_exporter"
	"github.com/grafana/alloy/internal/static/integrations/memcached_exporter"
	"github.com/grafana/alloy/internal/static/integrations/mongodb_exporter"
	"github.com/grafana/alloy/internal/static/integrations/mssql"
	"github.com/grafana/alloy/internal/static/integrations/mysqld_exporter"
	"github.com/grafana/alloy/internal/static/integrations/node_exporter"
	"github.com/grafana/alloy/internal/static/integrations/oracledb_exporter"
	"github.com/grafana/alloy/internal/static/integrations/postgres_exporter"
	"github.com/grafana/alloy/internal/static/integrations/process_exporter"
	"github.com/grafana/alloy/internal/static/integrations/redis_exporter"
	"github.com/grafana/alloy/internal/static/integrations/snmp_exporter"
	"github.com/grafana/alloy/internal/static/integrations/snowflake_exporter"
	"github.com/grafana/alloy/internal/static/integrations/squid_exporter"
	"github.com/grafana/alloy/internal/static/integrations/statsd_exporter"
	"github.com/grafana/alloy/internal/static/integrations/vmware_exporter"
	"github.com/grafana/alloy/internal/static/integrations/windows_exporter"
)

type testConfig struct {
	name                  string
	config                integrations.Config
	agentKey              string
	expected              string
	expectedErrorContains string
}

func TestInstanceKey(t *testing.T) {
	tests := []testConfig{
		{
			name:     "agent",
			config:   &agent.Config{},
			agentKey: "test-agent",
			expected: "test-agent",
		},
		{
			name: "apache_http",
			config: &apache_http.Config{
				ApacheAddr: "http://host01:8080/server-status?auto",
			},
			agentKey: "test-agent",
			expected: "host01:8080",
		},
		{
			name: "azure_exporter",
			config: &azure_exporter.Config{
				Subscriptions: []string{"sub1"},
				ResourceType:  "Microsoft.Storage/storageAccounts",
				Metrics:       []string{"Availability"},
			},
			expected: "af485df9ca7bb55236b1bf40f2566dcc",
		},
		{
			name: "blackbox_exporter",
			config: &blackbox_exporter.Config{
				BlackboxTargets: []blackbox_exporter.BlackboxTarget{
					{
						Name:   "test-target",
						Target: "https://example.com",
						Module: "http_2xx",
					},
					{
						Name:   "test-target2",
						Target: "https://example.com/2",
						Module: "http_2xx",
					},
				},
			},
			agentKey: "test-agent",
			expected: "test-agent", // TODO: fix as this may lead to issues with clustering
		},
		{
			name: "cadvisor",
			config: &cadvisor.Config{
				StoreContainerLabels:   true,
				Containerd:             "bar",
				ContainerdNamespace:    "foo",
				DockerOnly:             true,
				DisableRootCgroupStats: true,
			},
			agentKey: "test-agent",
			expected: "test-agent",
		},
		{
			name: "catchpoint_exporter",
			config: &catchpoint_exporter.Config{
				Port: "9090",
			},
			agentKey: "test-agent",
			expected: "9090",
		},
		{
			name: "cloudwatch_exporter",
			config: &cloudwatch_exporter.Config{
				STSRegion: "us-east-1",
				Discovery: cloudwatch_exporter.DiscoveryConfig{
					Jobs: []*cloudwatch_exporter.DiscoveryJob{
						{
							Type: "AWS/EC2",
						},
					},
				},
			},
			expected: "bf5b9f2a97b9a0c0a713b0dfe566f981",
		},
		{
			name: "consul_exporter",
			config: &consul_exporter.Config{
				Server: "http://host01:8500",
			},
			agentKey: "test-agent",
			expected: "host01:8500",
		},
		{
			name: "dnsmasq_exporter",
			config: &dnsmasq_exporter.Config{
				DnsmasqAddress: "host01:53",
			},
			agentKey: "test-agent",
			expected: "host01:53",
		},
		{
			name: "elasticsearch_exporter",
			config: &elasticsearch_exporter.Config{
				Address: "http://host01:9200",
			},
			expected: "host01:9200",
		},
		{
			name: "gcp_exporter",
			config: &gcp_exporter.Config{
				ProjectIDs:     []string{"project1"},
				MetricPrefixes: []string{"compute.googleapis.com"},
			},
			expected: "d624903751412e27de94ecfce264e25e",
		},
		{
			name: "github_exporter",
			config: &github_exporter.Config{
				APIURL:        "https://api.github.com",
				Repositories:  []string{"owner/repo1", "owner/repo2"},
				Organizations: []string{"org1", "org2"},
				Users:         []string{"user1", "user2"},
				APIToken:      "dummy-github-token",
			},
			agentKey: "test-agent",
			expected: "api.github.com", // TODO: fix as this may lead to conflicts
		},
		{
			name: "kafka_exporter",
			config: &kafka_exporter.Config{
				KafkaURIs:        []string{"kafka2:9092"},
				UseSASL:          true,
				UseSASLHandshake: true,
				SASLUsername:     "kafka-user",
				SASLPassword:     "kafka-password",
				KafkaVersion:     "2.8.0",
			},
			agentKey: "test-agent",
			expected: "kafka2:9092",
		},
		{
			name: "kafka_exporter error",
			config: &kafka_exporter.Config{
				KafkaURIs:        []string{"kafka2:9092", "kafka3:9092"},
				UseSASL:          true,
				UseSASLHandshake: true,
				SASLUsername:     "kafka-user",
				SASLPassword:     "kafka-password",
				KafkaVersion:     "2.8.0",
			},
			agentKey:              "test-agent",
			expectedErrorContains: "cannot be determined from 2 kafka servers",
		},
		{
			name: "kafka_exporter manually set",
			config: &kafka_exporter.Config{
				Instance:         "my-kafka-instance",
				KafkaURIs:        []string{"kafka2:9092", "kafka3:9092"},
				UseSASL:          true,
				UseSASLHandshake: true,
				SASLUsername:     "kafka-user",
				SASLPassword:     "kafka-password",
				KafkaVersion:     "2.8.0",
			},
			agentKey: "test-agent",
			expected: "my-kafka-instance",
		},
		{
			name: "memcached_exporter",
			config: &memcached_exporter.Config{
				MemcachedAddress: "host01:11211",
				Timeout:          5 * time.Second,
			},
			agentKey: "test-agent",
			expected: "host01:11211",
		},
		{
			name: "mongodb_exporter",
			config: &mongodb_exporter.Config{
				URI:             "mongodb://user:pass@host01:27017",
				DirectConnect:   true,
				DiscoveringMode: true,
			},
			agentKey: "test-agent",
			expected: "host01:27017",
		},
		{
			name: "mssql",
			config: &mssql.Config{
				ConnectionString:   "sqlserver://user:pass@host01:1433?database=master",
				MaxIdleConnections: 3,
				MaxOpenConnections: 3,
				Timeout:            10 * time.Second,
			},
			agentKey: "test-agent",
			expected: "host01:1433",
		},
		{
			name: "mysqld_exporter",
			config: &mysqld_exporter.Config{
				DataSourceName: "user:pass@tcp(host01:3306)/dbname?timeout=5s",
			},
			agentKey: "test-agent",
			expected: "tcp(host01:3306)/dbname",
		},
		{
			name: "node_exporter",
			config: &node_exporter.Config{
				ProcFSPath: "/proc",
				SysFSPath:  "/sys",
			},
			agentKey: "test-agent",
			expected: "test-agent",
		},
		{
			name: "oracledb_exporter",
			config: &oracledb_exporter.Config{
				ConnectionString: "oracle://user:pass@host01:1521/service",
			},
			agentKey: "test-agent",
			expected: "host01:1521",
		},
		{
			name: "postgres_exporter",
			config: &postgres_exporter.Config{
				DataSourceNames:  []config_util.Secret{"postgres://user:pass@host01:5432/dbname?sslmode=disable"},
				ExcludeDatabases: []string{"template0", "template1"},
			},
			agentKey: "test-agent",
			expected: "postgresql://host01:5432/dbname",
		},
		{
			name: "process_exporter",
			config: &process_exporter.Config{
				ProcFSPath: "/proc",
				Children:   true,
				Threads:    true,
				Recheck:    false,
			},
			agentKey: "test-agent",
			expected: "test-agent",
		},
		{
			name: "redis_exporter",
			config: &redis_exporter.Config{
				RedisAddr:     "host01:6379",
				RedisUser:     "user",
				RedisPassword: "pass",
				Namespace:     "redis",
			},
			agentKey: "test-agent",
			expected: "host01:6379",
		},
		{
			name:     "snmp_exporter",
			config:   &snmp_exporter.Config{},
			agentKey: "test-agent",
			expected: "test-agent", // TODO: fix as this may lead to issues with clustering
		},
		{
			name: "snowflake_exporter",
			config: &snowflake_exporter.Config{
				AccountName: "test-account",
				Username:    "test-user",
				Password:    "test-password",
				Role:        "ACCOUNTADMIN",
			},
			expected: "test-account",
		},
		{
			name: "squid_exporter",
			config: &squid_exporter.Config{
				Address: "host01:3128",
			},
			agentKey: "test-agent",
			expected: "host01:3128",
		},
		{
			name: "statsd_exporter",
			config: &statsd_exporter.Config{
				ListenUDP:     "localhost:9125",
				ListenTCP:     "localhost:9125",
				MappingConfig: "mapping.yml",
				ReadBuffer:    8192,
				CacheSize:     1000,
			},
			agentKey: "test-agent",
			expected: "test-agent", // TODO: fix as this may lead to issues with clustering
		},
		{
			name: "vmware_exporter",
			config: &vmware_exporter.Config{
				VSphereURL:  "https://vsphere.example.com:443",
				VSphereUser: "test-user",
				VSpherePass: "test-password",
			},
			expected: "vsphere.example.com:443",
		},
		{
			name: "windows_exporter",
			config: &windows_exporter.Config{
				EnabledCollectors: "cpu,cs,logical_disk,net,os,service,system",
			},
			agentKey: "test-agent",
			expected: "test-agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := tt.config.InstanceKey(tt.agentKey)
			if tt.expectedErrorContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorContains)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, key)
		})
	}
}

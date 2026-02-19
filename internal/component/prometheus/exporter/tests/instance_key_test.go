package exporter_test

import (
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	_ "github.com/grafana/alloy/internal/component/all" // import all components for the check if all exporters covered
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/apache"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/azure"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/blackbox"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/cadvisor"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/catchpoint"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/cloudwatch"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/consul"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/databricks"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/dnsmasq"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/elasticsearch"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/gcp"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/github"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/kafka"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/memcached"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/mongodb"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/mssql"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/mysql"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/oracledb"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/postgres"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/process"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/redis"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/self"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/snmp"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/snowflake"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/squid"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/static"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/statsd"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/unix"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/windows"
	httpservice "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/syntax/alloytypes"
)

type testConfig struct {
	testName              string
	componentName         string
	args                  component.Arguments
	expectedInstanceLabel string
	expectedErrorContains string
	temporaryHostname     string
}

func TestInstanceKey(t *testing.T) {
	tests := []testConfig{
		{
			testName:              "agent / self",
			componentName:         "prometheus.exporter.self",
			args:                  self.Arguments{},
			temporaryHostname:     "localhost.dummy",
			expectedInstanceLabel: "localhost.dummy",
		},
		{
			testName:      "apache",
			componentName: "prometheus.exporter.apache",
			args: apache.Arguments{
				ApacheAddr: "http://host01:8080/server-status?auto",
			},
			expectedInstanceLabel: "host01:8080",
		},
		{
			testName:      "azure",
			componentName: "prometheus.exporter.azure",
			args: azure.Arguments{
				Subscriptions: []string{"test-subscription"},
				ResourceType:  "Microsoft.Storage/storageAccounts",
				Metrics:       []string{"Availability"},
			},
			expectedInstanceLabel: "aa354aa3a12cfb94dc18a0c65eeb5384",
		},
		{
			testName:      "blackbox",
			componentName: "prometheus.exporter.blackbox",
			args: blackbox.Arguments{
				Targets: []blackbox.BlackboxTarget{
					{
						Name:   "test-target",
						Target: "http://localhost:9115",
						Module: "http_2xx",
					},
				},
			},
			temporaryHostname: "test-agent",
			// Blackbox exporter can target many hosts, so we don't have anything reliable to use.
			expectedInstanceLabel: "prometheus.exporter.blackbox.test_comp_id",
		},
		{
			testName:      "cadvisor",
			componentName: "prometheus.exporter.cadvisor",
			args: cadvisor.Arguments{
				StoreContainerLabels: true,
				ContainerdNamespace:  "foo",
			},
			temporaryHostname:     "test-agent",
			expectedInstanceLabel: "test-agent",
		},
		{
			testName:      "catchpoint",
			componentName: "prometheus.exporter.catchpoint",
			args: catchpoint.Arguments{
				Port: "9090",
			},
			// Port is better than hostname, but not ideal. Catchpoint is a webhook called externally, so there is no
			// clearly better option here.
			expectedInstanceLabel: "9090",
		},
		{
			testName:      "cloudwatch",
			componentName: "prometheus.exporter.cloudwatch",
			args: cloudwatch.Arguments{
				STSRegion:    "us-west-2",
				FIPSDisabled: true,
				Discovery: []cloudwatch.DiscoveryJob{
					{
						Type: "AWS/EC2",
						Auth: cloudwatch.RegionAndRoles{
							Regions: []string{"us-west-2"},
							Roles: []cloudwatch.Role{
								{
									RoleArn: "arn:aws:iam::123456789012:role/monitoring-role",
								},
							},
						},
						CustomTags: map[string]string{"Environment": "production"},
						Metrics: []cloudwatch.Metric{
							{
								Name:       "CPUUtilization",
								Statistics: []string{"Average"},
								Period:     time.Minute,
							},
						},
					},
				},
			},
			expectedInstanceLabel: "90f2b847c02d17d6c5b83f993d1235e0",
		},
		{
			testName:      "consul",
			componentName: "prometheus.exporter.consul",
			args: consul.Arguments{
				Server: "http://host01:8500",
			},
			expectedInstanceLabel: "host01:8500",
		},
		{
			testName:      "databricks",
			componentName: "prometheus.exporter.databricks",
			args: databricks.Arguments{
				ServerHostname:    "dbc-abc123.cloud.databricks.com",
				WarehouseHTTPPath: "/sql/1.0/warehouses/abc123",
				ClientID:          "test-client-id",
				ClientSecret:      "test-client-secret",
			},
			expectedInstanceLabel: "dbc-abc123.cloud.databricks.com",
		},
		{
			testName:      "dnsmasq",
			componentName: "prometheus.exporter.dnsmasq",
			args: dnsmasq.Arguments{
				Address: "host01:53",
			},
			expectedInstanceLabel: "host01:53",
		},
		{
			testName:      "elasticsearch",
			componentName: "prometheus.exporter.elasticsearch",
			args: elasticsearch.Arguments{
				Address: "http://host01:9200",
			},
			expectedInstanceLabel: "host01:9200",
		},
		// TODO: currently gcp tries to connect to remote servers on construction and fails if it cannot login.
		//       This makes this test hard to implement and may not be desired behaviour anyway.
		// {
		// 	testName:      "gcp",
		// 	componentName: "prometheus.exporter.gcp",
		// 	args: gcp.Arguments{
		// 		ProjectIDs:     []string{"project1"},
		// 		MetricPrefixes: []string{"compute.googleapis.com"},
		// 	},
		// 	expectedInstanceLabel: "d624903751412e27de94ecfce264e25e",
		// },
		{
			testName:              "gcp",
			componentName:         "prometheus.exporter.gcp",
			args:                  gcp.Arguments{},
			expectedErrorContains: "no project_ids defined",
		},
		{
			testName:      "github",
			componentName: "prometheus.exporter.github",
			args: github.Arguments{
				APIURL:        "https://api.github.com:8080",
				Repositories:  []string{"owner/repo1", "owner/repo2"},
				Organizations: []string{"org1", "org2"},
				Users:         []string{"user1", "user2"},
			},
			// This is better than hostname, but it may not be enough - we may need the repositories and orgs?
			expectedInstanceLabel: "api.github.com:8080",
		},
		// TODO: kafka exporters won't build successfully if it cannot connect right away to kafka. This is not
		//       desired, we should keep retrying connection.
		// {
		// 	testName:      "kafka",
		// 	componentName: "prometheus.exporter.kafka",
		// 	args: kafka.Arguments{
		// 		KafkaURIs:               []string{"localhost:9092"},
		// 		KafkaVersion:            "2.8.0",
		// 		InsecureSkipVerify:      true,
		// 	},
		// 	expectedInstanceLabel: "prometheus.exporter.kafka.test",
		// },
		{
			testName:      "kafka error",
			componentName: "prometheus.exporter.kafka",
			args: kafka.Arguments{
				KafkaURIs:          []string{"kafka2:9092", "kafka3:9092"},
				KafkaVersion:       "2.8.0",
				InsecureSkipVerify: true,
			},
			expectedErrorContains: "cannot be determined from 2 kafka servers",
		},
		// TODO: kafka exporters won't build successfully if it cannot connect right away to kafka. This is not
		//       desired, we should keep retrying connection.
		// {
		// 	testName:      "kafka manually set",
		// 	componentName: "prometheus.exporter.kafka",
		// 	args: kafka.Arguments{
		// 		KafkaURIs:               []string{"localhost:9092"},
		// 		KafkaVersion:            "2.8.0",
		// 		InsecureSkipVerify:      true,
		// 	},
		// 	expectedInstanceLabel: "prometheus.exporter.kafka.test",
		// },
		{
			testName:      "memcached",
			componentName: "prometheus.exporter.memcached",
			args: memcached.Arguments{
				Address: "host01:11211",
			},
			expectedInstanceLabel: "host01:11211",
		},
		{
			testName:      "mongodb",
			componentName: "prometheus.exporter.mongodb",
			args: mongodb.Arguments{
				URI:             "mongodb://user:pass@host01:27017",
				DirectConnect:   true,
				DiscoveringMode: true,
			},
			expectedInstanceLabel: "host01:27017",
		},
		{
			testName:      "mssql",
			componentName: "prometheus.exporter.mssql",
			args: mssql.Arguments{
				ConnectionString:   "sqlserver://user:pass@host01:1433?database=master",
				MaxOpenConnections: 1,
				MaxIdleConnections: 1,
				Timeout:            10 * time.Second,
			},
			expectedInstanceLabel: "host01:1433",
		},
		{
			testName:      "mysql",
			componentName: "prometheus.exporter.mysql",
			args: mysql.Arguments{
				DataSourceName: "user:pass@tcp(host01:3306)/dbname?timeout=5s",
			},
			expectedInstanceLabel: "tcp(host01:3306)/dbname",
		},
		{
			testName:      "oracledb",
			componentName: "prometheus.exporter.oracledb",
			args: oracledb.Arguments{
				ConnectionString: "oracle://user:pass@host01:1521/service",
			},
			expectedInstanceLabel: "host01:1521",
		},
		{
			testName:      "postgres",
			componentName: "prometheus.exporter.postgres",
			args: postgres.Arguments{
				DataSourceNames: []alloytypes.Secret{"postgres://user:pass@host01:5432/dbname?sslmode=disable"},
			},
			expectedInstanceLabel: "postgresql://host01:5432/dbname",
		},
		{
			testName:      "postgres multiple",
			componentName: "prometheus.exporter.postgres",
			args: postgres.Arguments{
				DataSourceNames: []alloytypes.Secret{
					alloytypes.Secret("postgresql://host01:5432/dbname"),
					alloytypes.Secret("postgresql://host02:5432/dbname"),
				},
			},
			expectedInstanceLabel: "prometheus.exporter.postgres.test_comp_id",
		},
		{
			testName:      "process",
			componentName: "prometheus.exporter.process",
			args: process.Arguments{
				ProcFSPath: "/proc",
				Children:   true,
			},
			temporaryHostname:     "test-agent",
			expectedInstanceLabel: "test-agent",
		},
		{
			testName:      "redis",
			componentName: "prometheus.exporter.redis",
			args: redis.Arguments{
				RedisAddr:     "host01:6379",
				RedisUser:     "user",
				RedisPassword: "pass",
				Namespace:     "redis",
			},
			expectedInstanceLabel: "host01:6379",
		},
		{
			testName:      "snmp",
			componentName: "prometheus.exporter.snmp",
			args: snmp.Arguments{
				ConfigMergeStrategy: "replace",
				Targets: []snmp.SNMPTarget{
					{
						Name:   "test-target",
						Target: "localhost:161",
						Module: "if_mib",
					},
				},
			},
			temporaryHostname: "test-agent",
			// SNMP can be used for many targets, there is no better target name we can be certain of
			expectedInstanceLabel: "prometheus.exporter.snmp.test_comp_id",
		},
		{
			testName:      "snowflake",
			componentName: "prometheus.exporter.snowflake",
			args: snowflake.Arguments{
				AccountName: "test-account",
				Username:    "test-user",
				Password:    "test-password",
				Role:        "ACCOUNTADMIN",
				Warehouse:   "carphone-warehouse",
			},
			// TODO: is this enough?
			expectedInstanceLabel: "test-account",
		},
		{
			testName:      "squid",
			componentName: "prometheus.exporter.squid",
			args: squid.Arguments{
				SquidAddr: "host01:3128",
			},
			expectedInstanceLabel: "host01:3128",
		},
		{
			testName:      "statsd",
			componentName: "prometheus.exporter.statsd",
			args: statsd.Arguments{
				ListenUDP:  "localhost:9125",
				ListenTCP:  "localhost:9125",
				ReadBuffer: 8192,
				CacheSize:  1000,
				CacheType:  "lru",
			},
			temporaryHostname: "test-agent",
			// StatsD exporter can receive data from network, so the best default we have is
			expectedInstanceLabel: "prometheus.exporter.statsd.test_comp_id",
		},
		{
			testName:      "unix",
			componentName: "prometheus.exporter.unix",
			args: unix.Arguments{
				ProcFSPath: "/proc",
				SysFSPath:  "/sys",
			},
			temporaryHostname:     "test-agent",
			expectedInstanceLabel: "test-agent",
		},
		{
			testName:      "windows",
			componentName: "prometheus.exporter.windows",
			args: windows.Arguments{
				EnabledCollectors: []string{},
				LogicalDisk: windows.LogicalDiskConfig{
					EnabledList: []string{"metrics"},
				},
				Net: windows.NetConfig{
					EnabledList: []string{"metrics"},
				},
			},
			temporaryHostname:     "test-agent",
			expectedInstanceLabel: "test-agent",
		},
		{
			testName:              "static",
			componentName:         "prometheus.exporter.static",
			args:                  static.Arguments{},
			temporaryHostname:     "test-agent",
			expectedInstanceLabel: "prometheus.exporter.static.test_comp_id",
		},
	}

	componentsCovered := map[string]any{}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			var capturedExports exporter.Exports
			opts := component.Options{
				ID: tt.componentName + ".test_comp_id",
				GetServiceData: func(name string) (any, error) {
					switch name {
					case httpservice.ServiceName:
						return httpservice.Data{
							HTTPListenAddr:   "localhost:12345",
							MemoryListenAddr: "alloy.internal:1245",
							BaseHTTPPath:     "/",
							DialFunc:         (&net.Dialer{}).DialContext,
						}, nil
					default:
						return nil, fmt.Errorf("service %q does not exist", name)
					}
				},
				OnStateChange: func(e component.Exports) {
					if exports, ok := e.(exporter.Exports); ok {
						capturedExports = exports
					} else {
						t.Fatalf("failed to convert component.Exports to exporter.Exports")
					}
				},
				Logger: log.NewLogfmtLogger(os.Stdout),
			}
			reg, ok := component.Get(tt.componentName)
			require.True(t, ok, "expected component to exist in registry")

			componentsCovered[tt.componentName] = struct{}{}

			if tt.temporaryHostname != "" {
				t.Setenv("HOSTNAME", tt.temporaryHostname)
			}
			c, err := reg.Build(opts, tt.args)
			if tt.expectedErrorContains != "" {
				require.Error(t, err, "expected component to be created with error")
				assert.Contains(t, err.Error(), tt.expectedErrorContains, "expected error to contain %q", tt.expectedErrorContains)
				return
			}
			assert.NoError(t, err, "expected component to be created without error")
			require.NotNil(t, c, "expected component to be created")

			require.NotNil(t, capturedExports, "expected exports to be captured")
			assert.Len(t, capturedExports.Targets, 1, "expected 1 target")
			actualInstance, ok := capturedExports.Targets[0].Get("instance")
			require.True(t, ok, "expected instance label to be present")
			assert.Equal(t, tt.expectedInstanceLabel, actualInstance, "expected instance label to be %q, got %q", tt.expectedInstanceLabel, actualInstance)
		})
	}
	t.Run("verify all exporters are covered", func(t *testing.T) {
		allExporters := map[string]any{}
		for _, n := range component.AllNames() {
			if strings.HasPrefix(n, "prometheus.exporter.") {
				allExporters[n] = struct{}{}
			}
		}
		assert.Equal(t, componentsCovered, allExporters, "expected all exporters to be covered")
	})
}

package kafka_exporter

import (
	"testing"

	"github.com/IBM/sarama"
	"github.com/go-kit/log"
	kafka_exporter "github.com/grafana/kafka_exporter/exporter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/static/config"
)

func TestConfig_SecretKafkaPassword(t *testing.T) {
	stringCfg := `
prometheus:
  wal_directory: /tmp/agent
integrations:
  kafka_exporter:
    enabled: true
    sasl_password: secret_password
`
	config.CheckSecret(t, stringCfg, "secret_password")
}

// TestOffsetShowAllFalse_EmptyMemberAssignment reproduces
// https://github.com/grafana/alloy/issues/7416: with offset_show_all = false,
// a group member with an empty assignment (nil, nil from sarama) caused a nil
// pointer panic in the exporter.
func TestOffsetShowAllFalse_EmptyMemberAssignment(t *testing.T) {
	broker := sarama.NewMockBroker(t, 1)
	defer broker.Close()

	broker.SetHandlerByMap(map[string]sarama.MockResponse{
		"MetadataRequest": sarama.NewMockMetadataResponse(t).
			SetBroker(broker.Addr(), broker.BrokerID()),
		"ListGroupsRequest": sarama.NewMockListGroupsResponse(t).
			AddGroup("rebalancing-group", "consumer"),
		"DescribeGroupsRequest": sarama.NewMockDescribeGroupsResponse(t).
			AddGroupDescription("rebalancing-group", &sarama.GroupDescription{
				GroupId:      "rebalancing-group",
				State:        "PreparingRebalance",
				ProtocolType: "consumer",
				Members: map[string]*sarama.GroupMemberDescription{
					"member-1": {
						MemberId: "member-1",
						ClientId: "client-1",
						// Empty assignment: GetMemberAssignment returns (nil, nil).
						MemberAssignment: nil,
					},
				},
			}),
		"OffsetFetchRequest": sarama.NewMockOffsetFetchResponse(t),
	})

	exp, err := kafka_exporter.New(log.NewNopLogger(), kafka_exporter.Options{
		Uri:                     []string{broker.Addr()},
		KafkaVersion:            sarama.V2_0_0_0.String(),
		MetadataRefreshInterval: "1m",
		OffsetShowAll:           false,
		AllowConcurrent:         true,
		TopicWorkers:            1,
		MaxOffsets:              1000,
	}, ".*", "^$", ".*", "^$")
	require.NoError(t, err)
	defer exp.Close()

	reg := prometheus.NewRegistry()
	require.NoError(t, reg.Register(exp))

	// Should not panic.
	_, err = reg.Gather()
	require.NoError(t, err)
}

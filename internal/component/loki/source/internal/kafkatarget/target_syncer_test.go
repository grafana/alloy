package kafkatarget

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/go-kit/log"
	"github.com/grafana/dskit/flagext"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki/client/fake"
)

func Test_TopicDiscovery(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	group := &testConsumerGroupHandler{}
	TopicPollInterval = time.Microsecond
	var closed bool
	client := &mockKafkaClient{
		topics: []string{"topic1"},
	}
	ts := &TargetSyncer{
		ctx:          ctx,
		cancel:       cancel,
		logger:       log.NewNopLogger(),
		topicManager: mustNewTopicsManager(client, []string{"topic1", "topic2"}),
		close: func() error {
			closed = true
			return nil
		},
		consumer: consumer{
			ctx:           t.Context(),
			cancel:        func() {},
			ConsumerGroup: group,
			logger:        log.NewNopLogger(),
			discoverer: DiscovererFn(func(s sarama.ConsumerGroupSession, c sarama.ConsumerGroupClaim) (Target, error) {
				return nil, nil
			}),
		},
		cfg: Config{
			RelabelConfigs: []*relabel.Config{},
			KafkaConfig: TargetConfig{
				UseIncomingTimestamp: true,
				Topics:               []string{"topic1", "topic2"},
			},
		},
	}

	ts.loop()

	require.Eventually(t, func() bool {
		if !group.consuming.Load() {
			return false
		}
		return reflect.DeepEqual([]string{"topic1"}, group.GetTopics())
	}, 1*time.Minute, 100*time.Millisecond, "expected topics: %v, got: %v", []string{"topic1"}, group.GetTopics())

	client.UpdateTopics([]string{"topic1", "topic2"})

	require.Eventually(t, func() bool {
		if !group.consuming.Load() {
			return false
		}
		return reflect.DeepEqual([]string{"topic1", "topic2"}, group.GetTopics())
	}, 5*time.Second, 100*time.Millisecond, "expected topics: %v, got: %v", []string{"topic1", "topic2"}, group.GetTopics())

	require.NoError(t, ts.Stop())
	require.True(t, closed)
}

func Test_NewTarget(t *testing.T) {
	ts := &TargetSyncer{
		logger: log.NewNopLogger(),
		client: fake.NewClient(func() {}),
		cfg: Config{
			RelabelConfigs: []*relabel.Config{
				{
					SourceLabels:         model.LabelNames{"__meta_kafka_topic"},
					TargetLabel:          "topic",
					Replacement:          "$1",
					Action:               relabel.Replace,
					Regex:                relabel.MustNewRegexp("(.*)"),
					NameValidationScheme: model.LegacyValidation,
				},
			},
			KafkaConfig: TargetConfig{
				UseIncomingTimestamp: true,
				GroupID:              "group_1",
				Topics:               []string{"topic1", "topic2"},
				Labels:               model.LabelSet{"static": "static1"},
			},
		},
	}
	tg, err := ts.NewTarget(&testSession{}, newTestClaim("foo", 10, 1))

	require.NoError(t, err)
	require.Equal(t, ConsumerDetails{
		MemberID:      "foo",
		GenerationID:  10,
		Topic:         "foo",
		Partition:     10,
		InitialOffset: 1,
	}, tg.Details())
	require.Equal(t, model.LabelSet{"static": "static1", "topic": "foo"}, tg.Labels())
	require.Equal(t, model.LabelSet{"__meta_kafka_member_id": "foo", "__meta_kafka_partition": "10", "__meta_kafka_topic": "foo", "__meta_kafka_group_id": "group_1"}, tg.DiscoveredLabels())
}

func Test_NewDroppedTarget(t *testing.T) {
	ts := &TargetSyncer{
		logger: log.NewNopLogger(),
		cfg: Config{
			KafkaConfig: TargetConfig{
				UseIncomingTimestamp: true,
				GroupID:              "group1",
				Topics:               []string{"topic1", "topic2"},
			},
		},
	}
	tg, err := ts.NewTarget(&testSession{}, newTestClaim("foo", 10, 1))

	require.NoError(t, err)
	require.Equal(t, "dropping target, no labels", tg.Details())
	require.Equal(t, model.LabelSet(nil), tg.Labels())
	require.Equal(t, model.LabelSet{"__meta_kafka_member_id": "foo", "__meta_kafka_partition": "10", "__meta_kafka_topic": "foo", "__meta_kafka_group_id": "group1"}, tg.DiscoveredLabels())
}

func Test_validateConfig(t *testing.T) {
	tests := []struct {
		cfg      *Config
		wantErr  bool
		expected *Config
	}{
		{
			&Config{
				KafkaConfig: TargetConfig{
					GroupID: "foo",
					Topics:  []string{"bar"},
				},
			},
			true,
			nil,
		},
		{
			&Config{
				KafkaConfig: TargetConfig{
					Brokers: []string{"foo"},
					GroupID: "bar",
				},
			},
			true,
			nil,
		},
		{
			&Config{
				KafkaConfig: TargetConfig{
					Brokers: []string{"foo"},
				},
			},
			true,
			nil,
		},
		{
			&Config{
				KafkaConfig: TargetConfig{
					Brokers: []string{"foo"},
					Topics:  []string{"bar"},
				},
			},
			false,
			&Config{
				KafkaConfig: TargetConfig{
					Brokers: []string{"foo"},
					Topics:  []string{"bar"},
					GroupID: "promtail",
					Version: "2.1.1",
				},
			},
		},
	}

	for i, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				require.Equal(t, tt.expected, tt.cfg)
			}
		})
	}
}

func Test_withAuthentication(t *testing.T) {
	var (
		tlsConf = config.TLSConfig{
			CAFile:             "testdata/example.com.ca.pem",
			CertFile:           "testdata/example.com.pem",
			KeyFile:            "testdata/example.com-key.pem",
			ServerName:         "example.com",
			InsecureSkipVerify: true,
		}
		expectedTLSConf, _ = config.NewTLSConfig(&config.TLSConfig{
			CAFile:             "testdata/example.com.ca.pem",
			CertFile:           "testdata/example.com.pem",
			KeyFile:            "testdata/example.com-key.pem",
			ServerName:         "example.com",
			InsecureSkipVerify: true,
		})
		cfg = sarama.NewConfig()
	)

	// no authentication
	noAuthCfg, err := withAuthentication(*cfg, Authentication{
		Type: AuthenticationTypeNone,
	})
	assert.Nil(t, err)
	assert.Equal(t, false, noAuthCfg.Net.TLS.Enable)
	assert.Equal(t, false, noAuthCfg.Net.SASL.Enable)
	assert.NoError(t, noAuthCfg.Validate())

	// specify unsupported auth type
	illegalAuthTypeCfg, err := withAuthentication(*cfg, Authentication{
		Type: "illegal",
	})
	assert.NotNil(t, err)
	assert.Nil(t, illegalAuthTypeCfg)

	// mTLS authentication
	mTLSCfg, err := withAuthentication(*cfg, Authentication{
		Type:      AuthenticationTypeSSL,
		TLSConfig: tlsConf,
	})
	assert.Nil(t, err)
	assert.Equal(t, true, mTLSCfg.Net.TLS.Enable)
	assert.NotNil(t, mTLSCfg.Net.TLS.Config)
	assert.Equal(t, "example.com", mTLSCfg.Net.TLS.Config.ServerName)
	assert.Equal(t, true, mTLSCfg.Net.TLS.Config.InsecureSkipVerify)
	assert.Equal(t, expectedTLSConf.Certificates, mTLSCfg.Net.TLS.Config.Certificates)
	assert.NotNil(t, mTLSCfg.Net.TLS.Config.RootCAs)
	assert.NoError(t, mTLSCfg.Validate())

	// mTLS authentication expect ignore sasl
	mTLSCfg, err = withAuthentication(*cfg, Authentication{
		Type:      AuthenticationTypeSSL,
		TLSConfig: tlsConf,
		SASLConfig: SASLConfig{
			Mechanism: sarama.SASLTypeSCRAMSHA256,
			User:      "user",
			Password:  flagext.SecretWithValue("pass"),
			UseTLS:    false,
		},
	})
	assert.Nil(t, err)
	assert.Equal(t, false, mTLSCfg.Net.SASL.Enable)

	// SASL/PLAIN
	saslCfg, err := withAuthentication(*cfg, Authentication{
		Type: AuthenticationTypeSASL,
		SASLConfig: SASLConfig{
			Mechanism: sarama.SASLTypePlaintext,
			User:      "user",
			Password:  flagext.SecretWithValue("pass"),
		},
	})
	assert.Nil(t, err)
	assert.Equal(t, false, saslCfg.Net.TLS.Enable)
	assert.Equal(t, true, saslCfg.Net.SASL.Enable)
	assert.Equal(t, "user", saslCfg.Net.SASL.User)
	assert.Equal(t, "pass", saslCfg.Net.SASL.Password)
	assert.Equal(t, sarama.SASLTypePlaintext, string(saslCfg.Net.SASL.Mechanism))
	assert.NoError(t, saslCfg.Validate())

	// SASL/SCRAM
	saslCfg, err = withAuthentication(*cfg, Authentication{
		Type: AuthenticationTypeSASL,
		SASLConfig: SASLConfig{
			Mechanism: sarama.SASLTypeSCRAMSHA512,
			User:      "user",
			Password:  flagext.SecretWithValue("pass"),
		},
	})
	assert.Nil(t, err)
	assert.Equal(t, false, saslCfg.Net.TLS.Enable)
	assert.Equal(t, true, saslCfg.Net.SASL.Enable)
	assert.Equal(t, "user", saslCfg.Net.SASL.User)
	assert.Equal(t, "pass", saslCfg.Net.SASL.Password)
	assert.Equal(t, sarama.SASLTypeSCRAMSHA512, string(saslCfg.Net.SASL.Mechanism))
	assert.NoError(t, saslCfg.Validate())

	// SASL unsupported mechanism
	_, err = withAuthentication(*cfg, Authentication{
		Type: AuthenticationTypeSASL,
		SASLConfig: SASLConfig{
			Mechanism: sarama.SASLTypeGSSAPI,
			User:      "user",
			Password:  flagext.SecretWithValue("pass"),
		},
	})
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "error unsupported sasl mechanism: GSSAPI")

	// SASL over TLS
	saslCfg, err = withAuthentication(*cfg, Authentication{
		Type: AuthenticationTypeSASL,
		SASLConfig: SASLConfig{
			Mechanism: sarama.SASLTypeSCRAMSHA512,
			User:      "user",
			Password:  flagext.SecretWithValue("pass"),
			UseTLS:    true,
			TLSConfig: tlsConf,
		},
	})
	assert.Nil(t, err)
	assert.Equal(t, true, saslCfg.Net.TLS.Enable)
	assert.Equal(t, true, saslCfg.Net.SASL.Enable)
	assert.NotNil(t, saslCfg.Net.TLS.Config)
	assert.Equal(t, "example.com", saslCfg.Net.TLS.Config.ServerName)
	assert.Equal(t, true, saslCfg.Net.TLS.Config.InsecureSkipVerify)
	assert.Equal(t, expectedTLSConf.Certificates, saslCfg.Net.TLS.Config.Certificates)
	assert.NotNil(t, saslCfg.Net.TLS.Config.RootCAs)
	assert.NoError(t, saslCfg.Validate())
}

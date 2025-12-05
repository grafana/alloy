package kafka

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/static/integrations/kafka_exporter"
	"github.com/grafana/alloy/syntax"
)

func TestAlloyUnmarshal(t *testing.T) {
	alloyConfig := `
		instance = "example"
		kafka_uris = ["localhost:9092","localhost:19092"]
		use_sasl_handshake = false
		kafka_version = "2.0.0"
		metadata_refresh_interval = "1m"
		allow_concurrency = true
		max_offsets = 1000
		topics_filter_regex = ".*"
		groups_filter_regex = ".*"
	`
	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)

	require.NoError(t, err)
	expected := Arguments{
		Instance:                "example",
		KafkaURIs:               []string{"localhost:9092", "localhost:19092"},
		UseSASLHandshake:        false,
		KafkaVersion:            "2.0.0",
		MetadataRefreshInterval: "1m",
		AllowConcurrent:         true,
		MaxOffsets:              1000,
		PruneIntervalSeconds:    30,
		OffsetShowAll:           true,
		TopicWorkers:            100,
		TopicsFilter:            ".*",
		GroupFilter:             ".*",
		TopicsExclude:           "^$",
		GroupExclude:            "^$",
	}
	require.Equal(t, expected, args)
}

func TestUnmarshalInvalid(t *testing.T) {
	validAlloyConfig := `
		instance = "example"
		kafka_uris = ["localhost:9092","localhost:19092"]
		use_sasl_handshake = false
		kafka_version = "2.0.0"
		metadata_refresh_interval = "1m"
		allow_concurrency = true
		max_offsets = 1000
		topics_filter_regex = ".*"
		groups_filter_regex = ".*"
`

	var args Arguments
	err := syntax.Unmarshal([]byte(validAlloyConfig), &args)
	require.NoError(t, err)

	invalidAlloyConfig := `
		instance = "example"
		kafka_uris = "localhost:9092"
	`
	var invalidArgs Arguments
	err = syntax.Unmarshal([]byte(invalidAlloyConfig), &invalidArgs)
	require.Error(t, err)
}

func TestAlloyConvert(t *testing.T) {
	orig := Arguments{
		Instance:                "example",
		KafkaURIs:               []string{"localhost:9092", "localhost:19092"},
		UseSASLHandshake:        false,
		KafkaVersion:            "2.0.0",
		MetadataRefreshInterval: "1m",
		AllowConcurrent:         true,
		MaxOffsets:              1000,
		OffsetShowAll:           true,
		TopicWorkers:            100,
		PruneIntervalSeconds:    30,
		TopicsFilter:            ".*",
		GroupFilter:             ".*",
		TopicsExclude:           "^$",
		GroupExclude:            "^$",
	}
	converted := orig.Convert()
	expected := kafka_exporter.Config{
		Instance:                "example",
		KafkaURIs:               []string{"localhost:9092", "localhost:19092"},
		KafkaVersion:            "2.0.0",
		MetadataRefreshInterval: "1m",
		AllowConcurrent:         true,
		MaxOffsets:              1000,
		OffsetShowAll:           true,
		TopicWorkers:            100,
		PruneIntervalSeconds:    30,
		TopicsFilter:            ".*",
		GroupFilter:             ".*",
		TopicsExclude:           "^$",
		GroupExclude:            "^$",
	}

	require.Equal(t, expected, *converted)
}

func TestCustomizeTarget(t *testing.T) {
	args := Arguments{
		Instance:  "example",
		KafkaURIs: []string{"localhost:9200", "localhost:19200"},
	}

	baseTarget := discovery.Target{}
	newTargets := customizeTarget(baseTarget, args)
	require.Equal(t, 1, len(newTargets))
	val, ok := newTargets[0].Get("instance")
	require.True(t, ok)
	require.Equal(t, "example", val)
}

func TestSASLPassword(t *testing.T) { // #6044
	var exampleAlloyConfig = `
		kafka_uris    = ["broker1"]
		use_sasl      = true
		sasl_password = "foobar"
	`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

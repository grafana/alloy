package kafka

import (
	"testing"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/static/integrations/kafka_exporter"
	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

func TestRiverUnmarshal(t *testing.T) {
	riverConfig := `
		instance = "example"
		kafka_uris = ["localhost:9092","localhost:19092"]
		use_sasl_handshake = false
		kafka_version = "2.0.0"
		metadata_refresh_interval = "1m"
		allow_concurrency = true
		max_offsets = 1000
		prune_interval_seconds = 30
		topics_filter_regex = ".*"
		groups_filter_regex = ".*"
	`
	var args Arguments
	err := river.Unmarshal([]byte(riverConfig), &args)

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
		TopicsFilter:            ".*",
		GroupFilter:             ".*",
	}
	require.Equal(t, expected, args)
}

func TestUnmarshalInvalid(t *testing.T) {
	validRiverConfig := `
		instance = "example"
		kafka_uris = ["localhost:9092","localhost:19092"]
		use_sasl_handshake = false
		kafka_version = "2.0.0"
		metadata_refresh_interval = "1m"
		allow_concurrency = true
		max_offsets = 1000
		prune_interval_seconds = 30
		topics_filter_regex = ".*"
		groups_filter_regex = ".*"
`

	var args Arguments
	err := river.Unmarshal([]byte(validRiverConfig), &args)
	require.NoError(t, err)

	invalidRiverConfig := `
		instance = "example"
		kafka_uris = "localhost:9092"
	`
	var invalidArgs Arguments
	err = river.Unmarshal([]byte(invalidRiverConfig), &invalidArgs)
	require.Error(t, err)
}

func TestRiverConvert(t *testing.T) {
	orig := Arguments{
		Instance:                "example",
		KafkaURIs:               []string{"localhost:9092", "localhost:19092"},
		UseSASLHandshake:        false,
		KafkaVersion:            "2.0.0",
		MetadataRefreshInterval: "1m",
		AllowConcurrent:         true,
		MaxOffsets:              1000,
		PruneIntervalSeconds:    30,
		TopicsFilter:            ".*",
		GroupFilter:             ".*",
	}
	converted := orig.Convert()
	expected := kafka_exporter.Config{
		Instance:                "example",
		KafkaURIs:               []string{"localhost:9092", "localhost:19092"},
		KafkaVersion:            "2.0.0",
		MetadataRefreshInterval: "1m",
		AllowConcurrent:         true,
		MaxOffsets:              1000,
		PruneIntervalSeconds:    30,
		TopicsFilter:            ".*",
		GroupFilter:             ".*",
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
	require.Equal(t, "example", newTargets[0]["instance"])
}

func TestSASLPassword(t *testing.T) { // #6044
	var exampleRiverConfig = `
		kafka_uris    = ["broker1"]
		use_sasl      = true 
		sasl_password = "foobar"
	`

	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)
}

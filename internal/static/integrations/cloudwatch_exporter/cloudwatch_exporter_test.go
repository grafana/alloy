package cloudwatch_exporter

import (
	"bytes"
	"testing"

	"github.com/go-kit/log"
	yaceModel "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestCloudwatchExporterIntegrationProperSetup(t *testing.T) {
	givenName := "test_exporter"
	var logbuff bytes.Buffer
	givenLogger := log.NewJSONLogger(&logbuff)
	givenConfig := yaceModel.JobsConfig{
		StsRegion:           "us-east-1",
		DiscoveryJobs:       []yaceModel.DiscoveryJob{},
		StaticJobs:          []yaceModel.StaticJob{},
		CustomNamespaceJobs: []yaceModel.CustomNamespaceJob{},
	}
	givenFipsEnabled := false
	givenLabelsSnakeCase := true
	givenDebug := false
	givenuseAWSSDKVersionV2 := true

	e, err := NewCloudwatchExporter(givenName, givenLogger, givenConfig, givenFipsEnabled, givenLabelsSnakeCase, givenDebug, givenuseAWSSDKVersionV2)
	require.NoError(t, err, "failed to construct cloudwatch exporter")

	logbuff.Reset()
	require.Equal(t, givenName, e.name, "exporter name should be set correctly")
	require.Equal(t, givenLabelsSnakeCase, e.labelsSnakeCase, "labelsSnakeCase should be set correctly")
	require.NotNil(t, e.logger, "logger should be initialized")
	require.NotNil(t, e.cachingClientFactory, "cachingClientFactory should be initialized")

	e.logger.Debug("debug")
	if bytes.Contains(logbuff.Bytes(), []byte("debug")) != givenDebug {
		t.Error("logger does not respect debug flag")
	}
}

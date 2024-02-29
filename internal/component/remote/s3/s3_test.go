//go:build linux

package s3

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/agent/internal/component"
	"github.com/stretchr/testify/require"
)

func TestCorrectBucket(t *testing.T) {
	o := component.Options{
		ID:            "t1",
		OnStateChange: func(_ component.Exports) {},
		Registerer:    prometheus.NewRegistry(),
	}
	s3File, err := New(o,
		Arguments{
			Path:          "s3://bucket/file",
			PollFrequency: 30 * time.Second,
			IsSecret:      false,
		})
	require.NoError(t, err)
	require.NotNil(t, s3File)
}

func TestPathBucketAndFileParsing(t *testing.T) {
	// Path without parent directory
	pathWithoutParentDir := "s3://bucket/file"
	bucket, file := getPathBucketAndFile(pathWithoutParentDir)
	require.Equal(t, "bucket", bucket)
	require.Equal(t, "file", file)

	// Path with parent directory
	pathWithParentDir := "s3://bucket/parent/file"
	bucket, file = getPathBucketAndFile(pathWithParentDir)
	require.Equal(t, "bucket", bucket)
	require.Equal(t, "parent/file", file)
}

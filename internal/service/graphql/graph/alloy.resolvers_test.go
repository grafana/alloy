package graph

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service/graphql/graph/model"
)

func TestAlloyLogs(t *testing.T) {
	buffer := logging.NewBuffer(1024)
	_, err := buffer.Write([]byte("level=info msg=first\nlevel=warn msg=second\nlevel=error msg=third\n"))
	require.NoError(t, err)

	resolver := &alloyResolver{Resolver: &Resolver{LogBuffer: buffer}}
	result, err := resolver.Logs(context.Background(), &model.Alloy{}, 1, []model.LogLevel{
		model.LogLevelWarn,
		model.LogLevelError,
	})

	require.NoError(t, err)
	require.Equal(t, []string{"level=warn msg=second"}, result.Lines)
}

func TestAlloyLogsRejectInvalidFirst(t *testing.T) {
	resolver := &alloyResolver{Resolver: &Resolver{}}

	_, err := resolver.Logs(context.Background(), &model.Alloy{}, -1, nil)
	require.ErrorContains(t, err, "first must not be negative")

	_, err = resolver.Logs(context.Background(), &model.Alloy{}, maxLogsPageSize+1, nil)
	require.ErrorContains(t, err, "first must not exceed")
}

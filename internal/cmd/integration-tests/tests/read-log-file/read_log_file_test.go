//go:build !windows

package main

import (
	"testing"

	"github.com/grafana/agent/internal/cmd/integration-tests/common"
	"github.com/stretchr/testify/assert"
)

const query = "http://localhost:3100/loki/api/v1/query?query={test_name=%22read_log_file%22}"

func TestReadLogFile(t *testing.T) {
	var logResponse common.LogResponse
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		err := common.FetchDataFromURL(query, &logResponse)
		assert.NoError(c, err)
		if assert.NotEmpty(c, logResponse.Data.Result) {
			assert.Equal(c, logResponse.Data.Result[0].Stream["filename"], "logs.txt")
			logs := make([]string, len(logResponse.Data.Result[0].Values))
			for i, valuePair := range logResponse.Data.Result[0].Values {
				logs[i] = valuePair[1]
			}
			assert.Contains(c, logs, "[2023-10-02 14:25:43] INFO: Starting the web application...")
		}
	}, common.DefaultTimeout, common.DefaultRetryInterval, "Data did not satisfy the conditions within the time limit")
}

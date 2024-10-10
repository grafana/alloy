//go:build !windows

package main

import (
	"testing"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
	"github.com/stretchr/testify/assert"
)

const query = "http://localhost:3100/loki/api/v1/query?query={test_name=%22read_log_file%22}"

func TestReadLogFile(t *testing.T) {
	var logResponse common.LogResponse
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		err := common.FetchDataFromURL(query, &logResponse)
		assert.NoError(c, err)
		if assert.NotEmpty(c, logResponse.Data.Result) {
			logs := make([]string, 0)
			for _, result := range logResponse.Data.Result {
				assert.Equal(c, result.Stream["filename"], "/etc/alloy/logs.txt")
				for _, valuePair := range result.Values {
					logs = append(logs, valuePair[1])
				}
			}
			assert.Contains(c, logs, "[2023-10-02 14:25:43] INFO: Starting the web application...")
		}
	}, common.DefaultTimeout, common.DefaultRetryInterval, "Data did not satisfy the conditions within the time limit")
}

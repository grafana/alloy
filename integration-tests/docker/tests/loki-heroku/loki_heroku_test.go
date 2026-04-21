//go:build alloyintegrationtests

package main

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
	"time"

	herokuencoding "github.com/heroku/x/logplex/encoding"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestLokiHeroku(t *testing.T) {
	err := pushHerokuDrain(3, 10, "app-1", "app-2")
	require.NoError(t, err)

	common.AssertLogsPresent(
		t,
		60,
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "app-1",
			},
			StructuredMetadata: map[string]string{
				"host": "host",
			},
			EntryCount: 30,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "app-2",
			},
			StructuredMetadata: map[string]string{
				"host": "host",
			},
			EntryCount: 30,
		},
	)

	common.AssertLabelsNotIndexed(t, "app", "host")
}

const url = "http://127.0.0.1:1516/heroku/api/v1/drain"

func pushHerokuDrain(requests int, messagesPerRequest int, apps ...string) error {
	now := time.Now().UTC()

	for i := range requests {
		var (
			body     bytes.Buffer
			msgCount int
		)

		for _, app := range apps {
			for y := range messagesPerRequest {
				frame, err := herokuencoding.Encode(herokuencoding.Message{
					Timestamp:   now,
					Hostname:    "host",
					Application: app,
					Process:     "web.1",
					ID:          "-",
					Message:     fmt.Sprintf("request %d log line %d from %s\n", i, y, app),
					Version:     1,
					Priority:    190,
				})
				if err != nil {
					return err
				}
				if _, err := body.Write(frame); err != nil {
					return err
				}
				now = now.Add(time.Second)
				msgCount++
			}
		}

		req, err := http.NewRequest(http.MethodPost, url, &body)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/heroku_drain-1")
		req.Header.Set("Logplex-Drain-Token", "d.integration-test")
		req.Header.Set("Logplex-Frame-Id", fmt.Sprintf("integration-test-%d", i))
		req.Header.Set("Logplex-Msg-Count", fmt.Sprintf("%d", msgCount))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("unexpected status code %d", resp.StatusCode)
		}
	}

	return nil
}

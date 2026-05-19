//go:build alloyintegrationtests

package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/grafana/go-gelf/v2/gelf"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestLokiGELF(t *testing.T) {
	require.NoError(t, write(30, "host-1", "host-2"))

	common.AssertLogsPresent(
		t,
		60,
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "host-1",
			},
			StructuredMetadata: map[string]string{
				"gelf_version": "1.1",
			},
			EntryCount: 30,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "host-2",
			},
			StructuredMetadata: map[string]string{
				"gelf_version": "1.1",
			},
			EntryCount: 30,
		},
	)

	common.AssertLabelsNotIndexed(t, "host")
}

const addr = "127.0.0.1:12201"

func write(n int, hosts ...string) error {
	writer, err := gelf.NewUDPWriter(addr)
	if err != nil {
		return err
	}
	defer writer.Close()

	writer.CompressionType = gelf.CompressNone

	now := time.Now()
	for _, h := range hosts {
		for i := range n {
			err := writer.WriteMessage(&gelf.Message{
				Version:  "1.1",
				Host:     h,
				Short:    fmt.Sprintf("log line %d from %s", i, h),
				TimeUnix: float64(now.UnixNano()) / float64(time.Second),
				Level:    gelf.LOG_INFO,
				Facility: "integration-test",
			})
			if err != nil {
				return err
			}
			now = now.Add(time.Second)
		}
	}

	return nil
}

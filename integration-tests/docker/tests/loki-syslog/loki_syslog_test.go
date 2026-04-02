//go:build alloyintegrationtests

package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

const (
	tcpRFC5424Addr = "127.0.0.1:51893"
	tcpRFC3164Addr = "127.0.0.1:51894"
	udpRFC5424Addr = "127.0.0.1:51898"
	udpRFC3164Addr = "127.0.0.1:51899"
)

func TestLokiSyslog(t *testing.T) {
	var (
		wg   sync.WaitGroup
		errs [4]error
	)

	wg.Go(func() { errs[0] = sendSyslog("tcp", tcpRFC5424Addr, formatRFC5424Message) })
	wg.Go(func() { errs[1] = sendSyslog("udp", udpRFC5424Addr, formatRFC5424Message) })
	wg.Go(func() { errs[2] = sendSyslog("tcp", tcpRFC3164Addr, formatRFC3164Message) })
	wg.Go(func() { errs[3] = sendSyslog("udp", udpRFC3164Addr, formatRFC3164Message) })

	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}

	common.AssertLogsPresent(
		t,
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format":   "rfc5424",
				"severity": "debug",
			},
			EntryCount: 2,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format":   "rfc5424",
				"severity": "informational",
			},
			EntryCount: 2,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format":   "rfc5424",
				"severity": "notice",
			},
			EntryCount: 2,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format":   "rfc5424",
				"severity": "warning",
			},
			EntryCount: 2,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format":   "rfc5424",
				"severity": "error",
			},
			EntryCount: 2,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format":   "rfc3164",
				"severity": "debug",
			},
			EntryCount: 2,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format":   "rfc3164",
				"severity": "informational",
			},
			EntryCount: 2,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format":   "rfc3164",
				"severity": "notice",
			},
			EntryCount: 2,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format":   "rfc3164",
				"severity": "warning",
			},
			EntryCount: 2,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format":   "rfc3164",
				"severity": "error",
			},
			EntryCount: 2,
		},
	)
}

func sendSyslog(network, addr string, producer func(string) []string) error {
	conn, err := net.DialTimeout(network, addr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	messages := producer(network)
	if network == "tcp" {
		return sendTCPMessages(conn, messages)
	}

	return sendUDPMessages(conn, messages)
}

func sendTCPMessages(conn net.Conn, messages []string) error {
	_, err := conn.Write([]byte(strings.Join(messages, "\n") + "\n"))
	return err
}

func sendUDPMessages(conn net.Conn, messages []string) error {
	for _, msg := range messages {
		_, err := conn.Write([]byte(msg))
		if err != nil {
			return err
		}
	}

	return nil
}

const local0PRIBase = 16 * 8

func formatRFC5424Message(network string) []string {
	return produceMessages(func(severityCode int) string {
		return fmt.Sprintf(
			`<%d>1 %s alloy-test app - - [example@32473 protocol="%s"] rfc5424 message`,
			local0PRIBase+severityCode,
			time.Now().UTC().Format(time.RFC3339),
			network,
		)
	})
}

func formatRFC3164Message(_ string) []string {
	return produceMessages(func(severityCode int) string {
		return fmt.Sprintf(
			`<%d>%s alloy-test rfc3164 message`,
			local0PRIBase+severityCode,
			time.Now().Format("Jan _2 15:04:05"),
		)
	})
}

func produceMessages(format func(severityCode int) string) []string {
	return []string{
		format(7),
		format(6),
		format(5),
		format(4),
		format(3),
	}
}

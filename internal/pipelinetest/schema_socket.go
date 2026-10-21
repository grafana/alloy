package pipelinetest

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

const (
	// socketInputTimeout bounds each write, so a stalled listener fails the
	// test instead of hanging it.
	socketInputTimeout = 5 * time.Second

	// socketInputWaitTimeout is how long to retry connecting to a TCP
	// listener. Components can start their listeners shortly after the config
	// finishes loading.
	socketInputWaitTimeout = 10 * time.Second
	socketInputWaitBackoff = 50 * time.Millisecond
)

// TCPInputSchema describes bytes written to a component's own TCP listener,
// such as a syslog receiver. Each entry opens one connection, writes Data as
// is, and closes the connection. Data is raw, so the test controls the
// framing, for example newline-separated or octet-counted syslog.
//
// The address must be the one the component listens on in the config.
type TCPInputSchema struct {
	Address string `yaml:"address"`
	Data    string `yaml:"data"`
}

// UDPInputSchema describes datagrams sent to a component's own UDP listener.
// Each entry of Datagrams is sent as one packet.
//
// Datagrams are sent as soon as the config has loaded, with no wait for the
// listener: UDP has no handshake that shows whether anything is listening, and
// a packet sent too early is silently lost. This works for components that
// bind their listeners while they are built, as loki.source.syslog and
// loki.source.gelf do.
type UDPInputSchema struct {
	Address   string   `yaml:"address"`
	Datagrams []string `yaml:"datagrams"`
}

func validateSocketInputs(tcp []TCPInputSchema, udp []UDPInputSchema) error {
	for i, input := range tcp {
		if _, _, err := net.SplitHostPort(input.Address); err != nil {
			return fmt.Errorf("tcp input %d: address must be host:port: %w", i, err)
		}
		if input.Data == "" {
			return fmt.Errorf("tcp input %d: data is required", i)
		}
	}
	for i, input := range udp {
		if _, _, err := net.SplitHostPort(input.Address); err != nil {
			return fmt.Errorf("udp input %d: address must be host:port: %w", i, err)
		}
		if len(input.Datagrams) == 0 {
			return fmt.Errorf("udp input %d: at least one datagram is required", i)
		}
	}
	return nil
}

func produceTCPInputs(ctx context.Context, inputs []TCPInputSchema) error {
	for i, input := range inputs {
		if err := sendTCPInput(ctx, input); err != nil {
			return fmt.Errorf("tcp input %d: %w", i, err)
		}
	}
	return nil
}

func sendTCPInput(ctx context.Context, input TCPInputSchema) error {
	var (
		dialer   net.Dialer
		deadline = time.Now().Add(socketInputWaitTimeout)
	)
	for {
		conn, err := dialer.DialContext(ctx, "tcp", input.Address)
		if err != nil {
			// Retry while nothing listens yet. Data is only written once a
			// connection exists, so a retry never sends it twice.
			if time.Now().Before(deadline) && ctx.Err() == nil {
				if err := sleepCtx(ctx, socketInputWaitBackoff); err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("dial %s: %w", input.Address, err)
		}

		_ = conn.SetWriteDeadline(time.Now().Add(socketInputTimeout))
		_, writeErr := conn.Write([]byte(input.Data))
		closeErr := conn.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return fmt.Errorf("write to %s: %w", input.Address, err)
		}
		return nil
	}
}

func produceUDPInputs(ctx context.Context, inputs []UDPInputSchema) error {
	for i, input := range inputs {
		if err := sendUDPInput(ctx, input); err != nil {
			return fmt.Errorf("udp input %d: %w", i, err)
		}
	}
	return nil
}

func sendUDPInput(ctx context.Context, input UDPInputSchema) error {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "udp", input.Address)
	if err != nil {
		return fmt.Errorf("dial %s: %w", input.Address, err)
	}
	defer conn.Close()

	for _, datagram := range input.Datagrams {
		_ = conn.SetWriteDeadline(time.Now().Add(socketInputTimeout))
		if _, err := conn.Write([]byte(datagram)); err != nil {
			return fmt.Errorf("write to %s: %w", input.Address, err)
		}
	}
	return nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

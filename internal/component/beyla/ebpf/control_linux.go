//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"reflect"
	"slices"
	"strconv"
	"time"

	"golang.org/x/sys/unix"

	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/subprocess"
	"github.com/grafana/alloy/internal/component/otelcol"
)

const (
	// configControlEnv tells Beyla which inherited file descriptor is its configuration control
	// socket. It must match ControlFDEnv in Beyla's cmd/beyla/internal/configrefresh package.
	configControlEnv = "BEYLA_CONFIG_CONTROL_FD"

	// configControlChildFD is the control socket in the subprocess.
	configControlChildFD = subprocess.FirstExtraFD

	// configRefreshTimeout bounds the wait for Beyla, which answers once its process matcher
	// has applied the new discovery criteria, or rejected them.
	configRefreshTimeout = 30 * time.Second
)

// configControl is Alloy's end of the socket over which it asks the running Beyla to apply a
// new configuration without restarting. Beyla answers each request in order.
type configControl struct {
	conn    net.Conn
	encoder *json.Encoder
	decoder *json.Decoder
}

type configRefreshRequest struct {
	Config string `json:"config"`
}

// configRefreshResponse has an empty Error when Beyla applied the configuration. Otherwise,
// Beyla keeps running its previous configuration.
type configRefreshResponse struct {
	Error string `json:"error"`
}

// newConfigControl returns Alloy's end of a new control socket and what to pass to Beyla.
// Only Alloy and that subprocess hold the socket. No other process can connect to it.
func newConfigControl() (*configControl, *os.File, error) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("create config control socket: %w", err)
	}

	parent := os.NewFile(uintptr(fds[0]), "beyla-config-control")
	defer parent.Close()

	conn, err := net.FileConn(parent)
	if err != nil {
		unix.Close(fds[1])
		return nil, nil, fmt.Errorf("open config control socket: %w", err)
	}

	control := &configControl{conn: conn, encoder: json.NewEncoder(conn), decoder: json.NewDecoder(conn)}
	return control, os.NewFile(uintptr(fds[1]), "beyla-config-control"), nil
}

func configControlEnvVar() string {
	return configControlEnv + "=" + strconv.Itoa(configControlChildFD)
}

// refresh asks Beyla to apply a new config and returns why it didn't (if it didn't). Beyla answers only once the
// change is in effect, so a nil error means that it runs the new configuration.
func (cc *configControl) refresh(ctx context.Context, config []byte) error {
	ctx, cancel := context.WithTimeout(ctx, configRefreshTimeout)
	defer cancel()

	deadline, _ := ctx.Deadline()
	if err := cc.conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set config refresh deadline: %w", err)
	}
	stop := context.AfterFunc(ctx, func() { _ = cc.conn.SetDeadline(time.Now()) })
	defer stop()

	if err := cc.encoder.Encode(configRefreshRequest{Config: string(config)}); err != nil {
		return fmt.Errorf("send config refresh: %w", err)
	}

	var response configRefreshResponse
	if err := cc.decoder.Decode(&response); err != nil {
		return fmt.Errorf("read config refresh result: %w", err)
	}

	if response.Error != "" {
		return fmt.Errorf("beyla rejected the config refresh: %s", response.Error)
	}

	return nil
}

func (cc *configControl) Close() error {
	return cc.conn.Close()
}

func sameExporters(a, b *otelcol.ConsumerArguments) bool {
	if a == nil || b == nil {
		return a == b
	}

	return sameConsumers(a.Metrics, b.Metrics) && sameConsumers(a.Logs, b.Logs) && sameConsumers(a.Traces, b.Traces)
}

func sameConsumers(a, b []otelcol.Consumer) bool {
	return slices.EqualFunc(a, b, func(x, y otelcol.Consumer) bool {
		return reflect.ValueOf(x).Comparable() && x == y
	})
}

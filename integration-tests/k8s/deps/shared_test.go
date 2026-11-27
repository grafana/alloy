package deps

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWaitForPortForwardReady(t *testing.T) {
	tests := []struct {
		name    string
		listen  bool
		exitErr error
		wantErr string
	}{
		{
			name:   "port accepts connections",
			listen: true,
		},
		{
			name:    "process exited before listening",
			exitErr: errors.New("exit status 1"),
			wantErr: "port-forward exited early: exit status 1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			port, err := pickFreeLocalPort()
			require.NoError(t, err)

			if tc.listen {
				l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
				require.NoError(t, err)
				t.Cleanup(func() { _ = l.Close() })
			}

			exited := make(chan error, 1)
			if tc.exitErr != nil {
				exited <- tc.exitErr
			}

			err = waitForPortForwardReady(port, exited)
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

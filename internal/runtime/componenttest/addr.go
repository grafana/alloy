package componenttest

import (
	"fmt"
	"testing"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
)

func GetFreeAddr(t *testing.T) string {
	t.Helper()

	portNumber, err := freeport.GetFreePort()
	require.NoError(t, err)

	return fmt.Sprintf("127.0.0.1:%d", portNumber)
}

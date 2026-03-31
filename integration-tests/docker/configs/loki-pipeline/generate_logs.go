package lokipipeline

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func GenerateLogs(t *testing.T, w io.Writer) {
	t.Helper()

	// Write two cri formatted logs for the same stream. One with P flag and another one with F flag.
	buf := bytes.Buffer{}
	buf.WriteString(time.Now().Format(time.RFC3339Nano))
	buf.WriteString(" stdout P partial chunk\n")

	buf.WriteString(time.Now().Format(time.RFC3339Nano))
	buf.WriteString(" stdout F final chunk\n")

	require.NoError(t, writeAll(w, buf.Bytes()))

}

func writeAll(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}
	return nil
}

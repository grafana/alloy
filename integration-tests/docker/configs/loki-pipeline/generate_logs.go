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

	// Write two cri log lines for the same stream that should be parsed by stage.cri.
	// One with P flag and another one with F flag.
	buf := bytes.Buffer{}
	buf.WriteString(time.Now().Format(time.RFC3339Nano))
	buf.WriteString(" stdout P partial chunk\n")

	buf.WriteString(time.Now().Format(time.RFC3339Nano))
	buf.WriteString(" stdout F final chunk\n")

	// Write one log line that should be parsed by stage.docker.
	buf.WriteString(`{"log":"docker json line\n","stream":"stderr","time":"`)
	buf.WriteString(time.Now().Format(time.RFC3339Nano))
	buf.WriteString(`","format":"json"}` + "\n")

	// Write one plain JSON log line that should be parsed by stage.json.
	buf.WriteString(`{"format":"json","msg":"plain json line"}` + "\n")

	// Write one plain logfmt log line that should be parsed by stage.logfmt.
	buf.WriteString("format=logfmt msg=\"plain logfmt line\"\n")

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

//go:build windows

package positions

import (
	"bytes"

	"github.com/natefinch/atomic"
)

func writeFileAtomic(path string, buf []byte) error {
	return atomic.WriteFile(path, bytes.NewReader(buf))
}

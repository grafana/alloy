//go:build linux && (amd64 || arm64)

package asprof

import (
	"fmt"
	"os"
)

func readlinkFD(f *os.File) (string, error) {
	return os.Readlink(fmt.Sprintf("/proc/self/fd/%d", f.Fd()))
}

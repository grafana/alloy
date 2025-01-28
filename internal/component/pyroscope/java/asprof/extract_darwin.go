//go:build darwin

package asprof

import (
	"os"
)

func readlinkFD(f *os.File) (string, error) {
	return f.Name(), nil
}

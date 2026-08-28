//go:build (linux && arm64) || (linux && amd64)

package subprocess

import (
	"errors"

	"golang.org/x/sys/unix"
)

// WriteAll writes all of data to fd, retrying on EINTR.
func WriteAll(fd int, data []byte) error {
	for len(data) > 0 {
		n, err := unix.Write(fd, data)

		if errors.Is(err, unix.EINTR) {
			continue
		}

		if err != nil {
			return err
		}

		data = data[n:]
	}

	return nil
}

func createExecMemfd(name string) (int, error) {
	// MFD_EXEC is required on kernels >= 6.3 when vm.memfd_noexec > 0
	fd, err := unix.MemfdCreate(name, unix.MFD_CLOEXEC|unix.MFD_EXEC)

	// older kernels don't recognise the flag and return EINVAL, so we fall back
	if err != nil && errors.Is(err, unix.EINVAL) {
		fd, err = unix.MemfdCreate(name, unix.MFD_CLOEXEC)
	}

	return fd, err
}

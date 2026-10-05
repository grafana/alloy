package tail

import (
	"bytes"
	"io"
)

// lastNewline returns the offset of the start of the last line in the reader.
func lastNewline(reader io.ReadSeeker, nl []byte) (int64, error) {
	n, err := reader.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	if n == 0 {
		return 0, nil
	}

	const chunkSize = 1024
	buf := make([]byte, chunkSize)

	var pos = n - chunkSize
	if pos < 0 {
		pos = 0
	}

	for {
		_, err = reader.Seek(pos, io.SeekStart)
		if err != nil {
			return 0, err
		}

		bytesRead, err := reader.Read(buf)
		if err != nil {
			return 0, err
		}

		i := bytes.LastIndex(buf[:bytesRead], nl)
		if i != -1 {
			return pos + int64(i) + int64(len(nl)), nil
		}

		if pos == 0 {
			return 0, nil
		}

		pos -= chunkSize
		if pos < 0 {
			pos = 0
		}
	}
}

package tail

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"unsafe"

	"golang.org/x/text/encoding"
)

func newScanner(f *os.File, offset int64, enc encoding.Encoding) (*reader, error) {
	if offset == 0 {
		offset = skipBOM(f)
	}

	var (
		decoder = enc.NewDecoder()
		encoder = enc.NewEncoder()
	)

	reader := &reader{
		scanner: bufio.NewScanner(f),
		decoder: decoder,
		splitFn: newSplitFn(encoder),
		pos:     offset,
	}

	reader.scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = reader.splitFn(data, atEOF)
		reader.pos += int64(advance)
		return advance, token, err
	})

	return reader, nil
}

type reader struct {
	pos     int64
	scanner *bufio.Scanner
	splitFn bufio.SplitFunc
	decoder *encoding.Decoder
}

func (r *reader) next() (string, error) {
	var err error
	ok := r.scanner.Scan()

	if !ok {
		err = r.scanner.Err()
		if err != nil {
			return "", err
		}
		return "", io.EOF
	}

	bytes, decodeErr := r.decoder.Bytes(r.scanner.Bytes())
	if decodeErr != nil {
		return "", decodeErr
	}
	str := unsafe.String(unsafe.SliceData(bytes), len(bytes))
	return str, err
}

func (r *reader) position() int64 {
	return r.pos
}

func (r *reader) reset(f *os.File, offset int64) {
	if offset == 0 {
		offset = skipBOM(f)
	}
	r.pos = offset
	r.scanner = bufio.NewScanner(f)
	r.scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = r.splitFn(data, atEOF)
		r.pos += int64(advance)
		return advance, token, err
	})
}

func newSplitFn(e *encoding.Encoder) bufio.SplitFunc {
	nl, _ := encodedNewline(e)
	cr, _ := encodedCarriageReturn(e)
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		i := bytes.Index(data, nl)
		if i == 0 {
			return len(nl), []byte{}, nil
		}

		if i >= 0 {
			// We have a full line so we should strip out cr.
			return i + len(nl), bytes.TrimSuffix(data[:i], cr), nil
		}

		// We have a partial line so we need to wait for more data.
		return 0, nil, nil
	}
}

func encodedNewline(e *encoding.Encoder) ([]byte, error) {
	out := make([]byte, 10)
	nDst, _, err := e.Transform(out, []byte{'\n'}, true)
	return out[:nDst], err
}

func encodedCarriageReturn(e *encoding.Encoder) ([]byte, error) {
	out := make([]byte, 10)
	nDst, _, err := e.Transform(out, []byte{'\r'}, true)
	return out[:nDst], err
}

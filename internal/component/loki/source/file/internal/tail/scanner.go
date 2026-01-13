package tail

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"unsafe"

	"golang.org/x/text/encoding"
)

func newScanner(f *os.File, offset int64, enc encoding.Encoding) (*scanner, error) {
	var (
		decoder = enc.NewDecoder()
		encoder = enc.NewEncoder()
	)

	scanner := &scanner{
		s:       bufio.NewScanner(f),
		decoder: decoder,
		splitFn: newSplitFn(encoder),
		pos:     offset,
	}

	scanner.s.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = scanner.splitFn(data, atEOF)
		scanner.pos += int64(advance)
		return advance, token, err
	})

	return scanner, nil
}

type scanner struct {
	pos     int64
	s       *bufio.Scanner
	splitFn bufio.SplitFunc
	decoder *encoding.Decoder
}

func (r *scanner) next() (string, error) {
	var err error
	ok := r.s.Scan()

	if !ok {
		err = r.s.Err()
		if err != nil {
			return "", err
		}
		return "", io.EOF
	}

	bytes, decodeErr := r.decoder.Bytes(r.s.Bytes())
	if decodeErr != nil {
		return "", decodeErr
	}
	str := unsafe.String(unsafe.SliceData(bytes), len(bytes))
	return str, err
}

func (r *scanner) position() (int64, error) {
	return r.pos, nil
}

func (r *scanner) reset(f *os.File, offset int64) {
	r.pos = offset
	r.s = bufio.NewScanner(f)
	r.s.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
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
			// We have a full newline-terminated line.
			return i + len(nl), bytes.TrimSuffix(data[:i], cr), nil
		}

		// We have a partial line so we need to wait for more data
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

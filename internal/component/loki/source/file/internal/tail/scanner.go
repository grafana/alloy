package tail

import (
	"bufio"
	"io"
	"os"
	"strings"

	"golang.org/x/text/encoding"
)

func newScanner(f *os.File, decoder *encoding.Decoder) *scanner {
	return &scanner{
		b:       bufio.NewReader(decoder.Reader(f)),
		f:       f,
		decoder: decoder,
	}
}

type scanner struct {
	b       *bufio.Reader
	f       *os.File
	decoder *encoding.Decoder
}

func (r *scanner) next() (string, error) {
	line, err := r.b.ReadString('\n')
	if err != nil {
		return line, err
	}
	return strings.TrimRight(line, "\r\n"), err
}

func (r *scanner) offset() (int64, error) {
	offset, err := r.f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	return offset - int64(r.b.Buffered()), nil
}

func (r *scanner) reset(f *os.File) {
	r.f = f
	r.b = bufio.NewReader(r.decoder.Reader(f))
}

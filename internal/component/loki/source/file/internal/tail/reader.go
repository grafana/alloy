package tail

import (
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"compress/zlib"
	"io"
	"os"
	"unsafe"

	"golang.org/x/text/encoding"
)

const defaultBufSize = 4096

// newReader creates a new reader that is used to read from file.
// It is important that the provided file is positioned at the start of the file.
func newReader(f *os.File, offset int64, enc encoding.Encoding, compression string) (*reader, error) {
	rr, err := newReaderAt(f, compression, 0)
	if err != nil {
		return nil, err
	}

	br := bufio.NewReader(rr)

	var bom BOM
	offset, bom = detectBOM(br, offset)
	enc = resolveEncodingFromBOM(bom, enc)

	var (
		decoder = enc.NewDecoder()
		encoder = enc.NewEncoder()
	)

	nl, err := encodedNewline(encoder)
	if err != nil {
		return nil, err
	}

	cr, err := encodedCarriageReturn(encoder)
	if err != nil {
		return nil, err
	}

	if offset != 0 {
		rr, err = newReaderAt(f, compression, offset)
		if err != nil {
			return nil, err
		}
		br.Reset(rr)
	}

	return &reader{
		pos:     offset,
		br:      br,
		decoder: decoder,
		nl:      nl,
		lastNl:  nl[len(nl)-1],
		cr:      cr,
		pending: make([]byte, 0, defaultBufSize),
	}, nil
}

type reader struct {
	pos     int64
	br      *bufio.Reader
	pending []byte

	compression string
	decoder     *encoding.Decoder

	nl     []byte
	lastNl byte
	cr     []byte
}

// next reads and returns the next complete line from the file.
// It will return EOF if there is no more data to read.
func (r *reader) next() (string, error) {
	// First we check if we already have a full line buffered.
	if line, ok := r.consumeLine(); ok {
		return r.decode(line)
	}

	for {
		// Read more data up until the last byte of nl.
		chunk, err := r.br.ReadBytes(r.lastNl)
		if len(chunk) > 0 {
			r.pending = append(r.pending, chunk...)

			if line, ok := r.consumeLine(); ok {
				return r.decode(line)
			}
		}

		// If we did not get an error and did not find a full line we
		// need to read more data.
		if err == nil {
			continue
		}

		return "", err
	}
}

// flush returns any remaining buffered data as a line, even if it doesn't end with a newline.
// This should be used when reaching EOF to handle the final partial line in the file.
// Returns io.EOF if there is no pending data.
func (r *reader) flush() (string, error) {
	if len(r.pending) == 0 {
		return "", io.EOF
	}

	line := r.pending[:]
	r.pos += int64(len(line))
	r.pending = make([]byte, 0, defaultBufSize)
	return r.decode(bytes.TrimSuffix(line, r.nl))
}

func (r *reader) decode(line []byte) (string, error) {
	// Decode the line we have consumed.
	converted, err := r.decoder.Bytes(bytes.TrimSuffix(line, r.cr))
	if err != nil {
		return "", err
	}

	// It is safe to convert this into a string here because converter will always copy
	// the bytes given to it, even Nop decoder will do that.
	return unsafe.String(unsafe.SliceData(converted), len(converted)), nil
}

// consumeLine checks pending for the delimiter; if found, it splits
// pending into line and remainder.
func (r *reader) consumeLine() ([]byte, bool) {
	// Check if pending contains a full line.
	i := bytes.Index(r.pending, r.nl)
	if i < 0 {
		return nil, false
	}

	// Extract everything up until newline.
	line := r.pending[:i]
	// Keep everything except the line we extracted and newline.
	rem := r.pending[i+len(r.nl):]
	r.pending = append(make([]byte, 0, defaultBufSize), rem...)

	// Advance the position on bytes we have consumed as a full line.
	r.pos += int64(len(line) + len(r.nl))
	return line, true
}

// position returns the byte offset for completed lines,
// not necessarily all bytes consumed from the file.
func (r *reader) position() int64 {
	return r.pos
}

// reset prepares the reader for a new file handle, assuming the same encoding.
// It is important that the provided file is positioned at the start of the file.
func (r *reader) reset(f *os.File, offset int64) error {
	rr, err := newReaderAt(f, r.compression, 0)
	if err != nil {
		return err
	}
	r.br.Reset(rr)

	offset, _ = detectBOM(r.br, offset)
	if offset != 0 {
		rr, err = newReaderAt(f, r.compression, offset)
		if err != nil {
			return nil
		}
		r.br.Reset(rr)
	}

	r.pos = offset
	r.pending = make([]byte, 0, defaultBufSize)
	return nil
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

func newReaderAt(f *os.File, compression string, offset int64) (io.Reader, error) {
	// NOTE: If compression is used we always need to read from the beginning.
	if compression != "" && offset != 0 {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
	}

	var (
		reader io.Reader
		err    error
	)

	switch compression {
	case "gz":
		reader, err = gzip.NewReader(f)
	case "z":
		reader, err = zlib.NewReader(f)
	case "bz2":
		reader = bzip2.NewReader(f)
	default:
		if offset != 0 {
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				return nil, err
			}
		}

		reader = f
	}

	if err != nil {
		return nil, err
	}

	// NOTE: If compression is used there is no easy way to seek to correct offset in the file
	// because the offset we store is for uncompressed data. Instead we can discard until the correct
	// offset.
	if compression != "" && offset != 0 {
		if _, err := io.CopyN(io.Discard, reader, offset); err != nil {
			return nil, err
		}
	}

	return reader, nil
}

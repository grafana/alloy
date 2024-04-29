package units

type scanner struct {
	text   string
	offset int
}

func newScanner(in string) *scanner {
	return &scanner{text: in, offset: 0}
}

// Next returns true if there are more bytes to scan. It does not advance the scanner.
func (s *scanner) Next() bool {
	return s.offset < len(s.text)
}

// Scan returns the next byte and advances the scanner.
func (s *scanner) Scan() byte {
	ch := s.text[s.offset]
	s.offset++
	return ch
}

// String returns the substring up to the current offset.
func (s *scanner) String() string {
	return s.text[:s.offset]
}

// Peek returns the byte at the current offset without advancing the scanner.
func (s *scanner) Peek() byte {
	return s.text[s.offset]
}

// Rem returns the number of bytes remaining in the scanner.
func (s *scanner) Rem() int {
	return len(s.text) - s.offset
}

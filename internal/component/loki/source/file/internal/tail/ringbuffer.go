package tail

// ringBuffer is a growable ring buffer for bytes.
type ringBuffer struct {
	buf  []byte
	head int
	tail int
	// linearBuf is used when buffer wraps.
	linearBuf []byte
}

// NewRingBuffer creates a new ring buffer with the given initial capacity.
func NewRingBuffer(initialCap int) *ringBuffer {
	return &ringBuffer{
		buf: make([]byte, initialCap),
	}
}

// Append appends data the buffer. It will grow the buffer if it does not have enough space.
func (rb *ringBuffer) Append(data []byte) {
	n := len(data)
	if n == 0 {
		return
	}
	need := rb.Len() + n
	if need >= len(rb.buf) {
		rb.grow(need)
	}

	spaceAfterTail := len(rb.buf) - rb.tail
	if n <= spaceAfterTail {
		// Fast path: data fits in the remaining space after tail
		rb.tail += copy(rb.buf[rb.tail:], data)
	} else {
		// Slow path: data wraps around the end of the buffer.
		rb.tail += copy(rb.buf[rb.tail:], data[:spaceAfterTail])
		rb.tail = copy(rb.buf, data[spaceAfterTail:])
	}
}

// grow allocates a new buffer of at least need bytes, copies the current
// content and resets head/tail.
func (rb *ringBuffer) grow(need int) {
	newCap := max(len(rb.buf)*2, need)
	newBuf := make([]byte, newCap)
	l := rb.Len()
	if l == 0 {
		rb.buf = newBuf
		rb.head = 0
		rb.tail = 0
		rb.linearBuf = nil
		return
	}

	if rb.tail >= rb.head {
		// Fast path: buffer is not wrapped so we can copy as is
		rb.tail = copy(newBuf, rb.buf[rb.head:rb.tail])
	} else {
		// Slow path: buffered is wrapped so we need to create a linear view.
		linear := rb.linearizeInto(newBuf[:0])
		rb.tail = len(linear)
	}
	rb.buf = newBuf
	rb.head = 0
	rb.linearBuf = nil
}

// Bytes returns a view of all bytes in the buffer.
// The slice is valid until the next call to Commit or Append.
func (rb *ringBuffer) Bytes() []byte {
	if rb.Len() == 0 {
		return nil
	}

	// Fast path: we can just return a view into buf.
	if rb.tail >= rb.head {
		return rb.buf[rb.head:rb.tail]
	}

	// Slow path: buffered is wrapped so we need to create a linear view.
	l := rb.Len()
	if cap(rb.linearBuf) < l {
		rb.linearBuf = make([]byte, l, l*2)
	}
	return rb.linearizeInto(rb.linearBuf[:0])
}

// linearizeInto creates a linear view of a wrapped buffer.
// Caller must ensure the buffer is wrapped before calling this.
func (rb *ringBuffer) linearizeInto(dst []byte) []byte {
	dst = append(dst, rb.buf[rb.head:]...)
	return append(dst, rb.buf[:rb.tail]...)
}

// Commit discards the n bytes from the buffer.
func (rb *ringBuffer) Commit(n int) {
	if n <= 0 {
		return
	}
	l := rb.Len()
	if n > l {
		n = l
	}
	rb.head += n
	if rb.head > len(rb.buf) {
		rb.head -= len(rb.buf)
	}
	rb.linearBuf = nil
}

// Len returns the number of bytes in the buffer.
func (rb *ringBuffer) Len() int {
	if rb.tail >= rb.head {
		return rb.tail - rb.head
	}
	return len(rb.buf) - rb.head + rb.tail
}

// Reset discards all buffered data and clears the buffer for reuse.
func (rb *ringBuffer) Reset() {
	rb.head = 0
	rb.tail = 0
	rb.linearBuf = nil
}

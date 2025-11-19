package pfunsafe

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSliceFromPointer(t *testing.T) {
	s := 0xcafebabe
	p := &s
	actual := FromPointer(p)
	assert.Equal(t, []byte{0xbe, 0xba, 0xfe, 0xca, 0x0, 0x0, 0x0, 0x0}, actual)
	assert.Panics(t, func() {
		p = nil
		FromPointer(p)
	})
}

func TestSliceFromSlice(t *testing.T) {
	s := []uint64{0xcafebabe, 0xdeadbeef}
	actual := FromSlice(s)
	expected := []byte{
		0xbe, 0xba, 0xfe, 0xca, 0x0, 0x0, 0x0, 0x0,
		0xef, 0xbe, 0xad, 0xde, 0x0, 0x0, 0x0, 0x0,
	}
	assert.Equal(t, expected, actual)
	assert.NotPanics(t, func() {
		s = nil
		actual = FromSlice(s)
		expected = nil
		assert.Equal(t, expected, actual)
		s = []uint64{}
		actual = FromSlice(s)
		assert.Equal(t, expected, actual)
	})
}

func TestByteSlice2String(t *testing.T) {
	var b [4]byte
	s := ToString(b[:1]) // create s with length 1 and a 0 byte inside
	assert.Equal(t, "\x00", s)

	b[0] = 'a'
	assert.Equal(t, "a", s)
}

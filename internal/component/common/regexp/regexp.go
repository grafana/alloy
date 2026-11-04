// Package regexp provides Alloy wrapper types for regular expressions that
// implement encoding.TextMarshaler and encoding.TextUnmarshaler for use in
// Alloy component configs.
package regexp

import (
	"encoding"
	"regexp"
)

// Compile parses a regular expression and returns a Regexp.
func Compile(s string) (*Regexp, error) {
	re, err := regexp.Compile(s)
	if err != nil {
		return nil, err
	}
	return &Regexp{re}, err
}

// MustCompile is like Compile but panics on error.
func MustCompile(s string) *Regexp {
	re, err := Compile(s)
	if err != nil {
		panic(err)
	}
	return re
}

var (
	_ encoding.TextMarshaler   = Regexp{}
	_ encoding.TextUnmarshaler = (*Regexp)(nil)
)

// Regexp wraps regexp.Regexp and implements encoding.TextMarshaler and
// encoding.TextUnmarshaler so it can be used in Alloy attributes. The pattern may be empty.
type Regexp struct {
	*regexp.Regexp
}

// MarshalText implements encoding.TextMarshaler for Regexp.
func (r Regexp) MarshalText() (text []byte, err error) {
	if r.Regexp != nil && r.Regexp.String() != "" {
		return []byte(r.String()), nil
	}
	return nil, nil
}

// UnmarshalText implements encoding.TextUnmarshaler for Regexp.
func (r *Regexp) UnmarshalText(text []byte) error {
	re, err := Compile(string(text))
	if err != nil {
		return err
	}
	*r = *re
	return nil
}

func (r *Regexp) IsEmpty() bool {
	return r == nil || r.Regexp == nil || r.Regexp.String() == ""
}

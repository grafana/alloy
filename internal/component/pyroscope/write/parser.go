// Package write
// This label parser is copy-pasted from grafana/pyroscope/pkg/og/storage/segment/key.go.
// TODO: Replace this copy with the upstream parser once it's moved to pyroscope/api.
package write

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const (
	ReservedTagKeyName = "__name__"
)

type ParserState int

const (
	nameParserState ParserState = iota
	tagKeyParserState
	tagValueParserState
	doneParserState
)

var reservedTagKeys = []string{
	ReservedTagKeyName,
}

type Key struct {
	labels map[string]string
}

type parser struct {
	parserState ParserState
	key         *bytes.Buffer
	value       *bytes.Buffer
}

var parserPool = sync.Pool{
	New: func() any {
		return &parser{
			parserState: nameParserState,
			key:         new(bytes.Buffer),
			value:       new(bytes.Buffer),
		}
	},
}

func ParseKey(name string) (*Key, error) {
	k := &Key{labels: make(map[string]string)}
	p := parserPool.Get().(*parser)
	defer parserPool.Put(p)
	p.reset()
	var err error
	for _, r := range name + "{" {
		switch p.parserState {
		case nameParserState:
			err = p.nameParserCase(r, k)
		case tagKeyParserState:
			p.tagKeyParserCase(r)
		case tagValueParserState:
			err = p.tagValueParserCase(r, k)
		}
		if err != nil {
			return nil, err
		}
	}
	return k, nil
}

func (p *parser) reset() {
	p.parserState = nameParserState
	p.key.Reset()
	p.value.Reset()
}

func (p *parser) nameParserCase(r int32, k *Key) error {
	switch r {
	case '{':
		p.parserState = tagKeyParserState
		appName := strings.TrimSpace(p.value.String())
		if err := validateAppName(appName); err != nil {
			return err
		}
		k.labels["__name__"] = appName
	default:
		p.value.WriteRune(r)
	}
	return nil
}

func (p *parser) tagKeyParserCase(r rune) {
	switch r {
	case '}':
		p.parserState = doneParserState
	case '=':
		p.parserState = tagValueParserState
		p.value.Reset()
	default:
		p.key.WriteRune(r)
	}
}

func (p *parser) tagValueParserCase(r rune, k *Key) error {
	switch r {
	case ',', '}':
		p.parserState = tagKeyParserState
		key := strings.TrimSpace(p.key.String())
		if !isTagKeyReserved(key) {
			if err := validateTagKey(key); err != nil {
				return err
			}
		}
		k.labels[key] = strings.TrimSpace(p.value.String())
		p.key.Reset()
	default:
		p.value.WriteRune(r)
	}
	return nil
}

// Normalized is a helper for formatting the key back to string
func (k *Key) Normalized() string {
	var sb strings.Builder

	sortedMap := NewSortedMap()
	for k, v := range k.labels {
		if k == "__name__" {
			sb.WriteString(v)
		} else {
			sortedMap.Put(k, v)
		}
	}

	sb.WriteString("{")
	for i, k := range sortedMap.Keys() {
		v := sortedMap.Get(k).(string)
		if i != 0 {
			sb.WriteString(",")
		}
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(v)
	}
	sb.WriteString("}")

	return sb.String()
}

// SortedMap provides a deterministic way to iterate over map entries
type SortedMap struct {
	data map[string]interface{}
	keys []string
}

func NewSortedMap() *SortedMap {
	return &SortedMap{
		data: make(map[string]interface{}),
		keys: make([]string, 0),
	}
}

func (s *SortedMap) Put(k string, v interface{}) {
	s.data[k] = v
	i := sort.Search(len(s.keys), func(i int) bool { return s.keys[i] >= k })
	s.keys = append(s.keys, "")
	copy(s.keys[i+1:], s.keys[i:])
	s.keys[i] = k
}

func (s *SortedMap) Get(k string) (v interface{}) {
	return s.data[k]
}

func (s *SortedMap) Keys() []string {
	return s.keys
}

func validateAppName(n string) error {
	if len(n) == 0 {
		return errors.New("application name is required")
	}
	for _, r := range n {
		if !isAppNameRuneAllowed(r) {
			return newInvalidAppNameRuneError(n, r)
		}
	}
	return nil
}

func isAppNameRuneAllowed(r rune) bool {
	return r == '-' || r == '.' || r == '/' || isTagKeyRuneAllowed(r)
}

func isTagKeyReserved(k string) bool {
	for _, s := range reservedTagKeys {
		if s == k {
			return true
		}
	}
	return false
}

func isTagKeyRuneAllowed(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.'
}

func validateTagKey(k string) error {
	if len(k) == 0 {
		return errors.New("tag key is required")
	}
	for _, r := range k {
		if !isTagKeyRuneAllowed(r) {
			return newInvalidTagKeyRuneError(k, r)
		}
	}
	if isTagKeyReserved(k) {
		return newErr(errors.New("tag key is reserved"), k)
	}
	return nil
}

type Error struct {
	Inner error
	Expr  string
}

func newInvalidAppNameRuneError(k string, r rune) *Error {
	return newInvalidRuneError(errors.New("invalid application name"), k, r)
}

func newErr(err error, expr string) *Error { return &Error{Inner: err, Expr: expr} }

func (e *Error) Error() string { return e.Inner.Error() + ": " + e.Expr }

func newInvalidTagKeyRuneError(k string, r rune) *Error {
	return newInvalidRuneError(errors.New("invalid tag key"), k, r)
}

func newInvalidRuneError(err error, k string, r rune) *Error {
	return newErr(err, fmt.Sprintf("%s: character is not allowed: %q", k, r))
}

package main

import (
	"debug/dwarf"
	"errors"
	"fmt"
)

func ReadEntry(reader *dwarf.Reader, name string, expectedTag dwarf.Tag) (*dwarf.Entry, error) {
	reader.Seek(0)
	for {
		e, err := reader.Next()
		if e == nil {
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		if !e.Children {
			continue
		}
		for {
			e, err := reader.Next()
			if e == nil {
				return nil, err
			}
			if err != nil {
				return nil, err
			}
			if e.Tag == 0 {
				break
			}
			for _, f := range e.Field {
				if e.Tag != expectedTag {
					continue
				}
				if f.Attr == dwarf.AttrName && f.Val == name {
					return e, nil
				}
			}
			reader.SkipChildren()
		}
	}
}

func ReadChild(reader *dwarf.Reader, name string) (*dwarf.Entry, error) {
	for {
		e, err := reader.Next()
		if err != nil {
			return nil, err
		}
		if e == nil || e.Tag == 0 {
			return nil, fmt.Errorf("field %s not found", name)
		}
		for _, f := range e.Field {
			if f.Attr == dwarf.AttrName && f.Val == name {
				return e, nil
			}
		}
		reader.SkipChildren()
	}
}

func ReadField(e *dwarf.Entry, key dwarf.Attr) any {
	for _, f := range e.Field {
		if f.Attr == key {
			return f.Val
		}
	}
	return nil
}

func readType(r *dwarf.Reader, e *dwarf.Entry,
	seen map[dwarf.Offset]struct{}) (*dwarf.Entry, error) {
	offset := e.Offset
	if _, found := seen[offset]; found {
		return nil, fmt.Errorf("infinite loop detected at %d", offset)
	}
	seen[offset] = struct{}{}
	t, ok := ReadField(e, dwarf.AttrType).(dwarf.Offset)
	if !ok {
		return nil, errors.New("type not found")
	}
	r.Seek(t)
	candidate, err := r.Next()
	if err != nil {
		return nil, err
	}
	if candidate.Tag == dwarf.TagTypedef {
		return readType(r, candidate, seen)
	}
	return candidate, nil
}

func ReadType(r *dwarf.Reader, e *dwarf.Entry) (*dwarf.Entry, error) {
	return readType(r, e, make(map[dwarf.Offset]struct{}))
}

func ReadChildTypeAndOffset(r *dwarf.Reader, name string) (*dwarf.Entry, int64, error) {
	child, err := ReadChild(r, name)
	if err != nil {
		return nil, 0, err
	}

	offset, ok := ReadField(child, dwarf.AttrDataMemberLoc).(int64)
	if !ok {
		return nil, 0, fmt.Errorf("offset not found for field %s", name)
	}

	typ, err := ReadType(r, child)
	if err != nil {
		return nil, 0, err
	}

	return typ, offset, nil
}

// Package structwalk allows you to "walk" the hierarchy of a struct. It is
// very similar to github.com/mitchellh/reflectwalk but allows you to change
// the visitor mid-walk.
package structwalk

import (
	"reflect"

	"github.com/mitchellh/reflectwalk"
)

// Walk traverses the hierarchy of o in depth-first order. It starts by calling
// v.Visit(o). If the visitor w returned by v.Visit(o) is not nil, Walk is
// invoked recursively with visitor w for each of the structs inside of o,
// followed by a call to w.Visit(nil).
//
// o must be non-nil.
func Walk(v Visitor, o interface{}) {
	sw := structWalker{v: v}
	_ = reflectwalk.Walk(o, &sw)
}

// Visitor will have its Visit method invoked for each struct value encountered
// by Walk. If w returned from Visit is non-nil, Walk will then visit each child
// of value with w. The final call after visiting all children will be to
// w.Visit(nil).
type Visitor interface {
	Visit(value interface{}) (w Visitor)
}

type structWalker struct {
	cur interface{}
	v   Visitor
}

// Struct invoke the Visitor for v and its children.
func (sw *structWalker) Struct(v reflect.Value) error {
	// structWalker will walk absolutely all fields, even unexported fields or
	// types. We can only interface exported fields, so we need to abort early
	// for anything that's not supported.
	if !v.CanInterface() {
		return nil
	}

	// Get the interface to the value. reflectwalk will fully derefernce all
	// structs, so if it's possible for us to get address it into a pointer,
	// we will use that for visiting.
	var (
		rawValue = v.Interface()
		ptrValue = rawValue
	)
	if v.Kind() != reflect.Ptr && v.CanAddr() {
		ptrValue = v.Addr().Interface()
	}

	// Struct will recursively call reflectwalk.Walk with a new walker, which
	// means that sw.Struct will be called twice for the same value. We want
	// to ignore calls to Struct with the same value so we don't recurse
	// infinitely.
	if sw.cur != nil && reflect.DeepEqual(rawValue, sw.cur) {
		return nil
	}

	// Visit our struct and create a new walker with the returned Visitor.
	w := sw.v.Visit(ptrValue)
	if w == nil {
		return reflectwalk.SkipEntry
	}
	_ = reflectwalk.Walk(rawValue, &structWalker{cur: rawValue, v: w})
	w.Visit(nil)

	return reflectwalk.SkipEntry
}

func (sw *structWalker) StructField(reflect.StructField, reflect.Value) error {
	return nil
}

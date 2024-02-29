package http

import (
	"errors"
	"fmt"
	"strings"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/service"
)

// splitURLPath splits a path from a URL into two parts: a component ID and the
// remaining string.
//
// For example, given a path of /prometheus.exporter.unix/metrics, the result
// will be prometheus.exporter.unix and /metrics.
//
// The "remain" portion is optional; it's valid to give a path just containing
// a component name.
func splitURLPath(host service.Host, path string) (id component.ID, remain string, err error) {
	if len(path) == 0 {
		return component.ID{}, "", fmt.Errorf("invalid path")
	}

	// Trim leading and tailing slashes so it's not treated as part of a
	// component name.
	var trimmedLeadingSlash, trimmedTailingSlash bool
	if path[0] == '/' {
		path = path[1:]
		trimmedLeadingSlash = true
	}
	if path[len(path)-1] == '/' {
		path = path[:len(path)-1]
		trimmedTailingSlash = true
	}

	it := newReversePathIterator(path)
	for it.Next() {
		idText, path := it.Value()
		componentID := component.ParseID(idText)

		_, err := host.GetComponent(componentID, component.InfoOptions{})
		if errors.Is(err, component.ErrComponentNotFound) {
			continue
		} else if err != nil {
			return component.ID{}, "", err
		}

		return componentID, preparePath(path, trimmedLeadingSlash, trimmedTailingSlash), nil
	}

	return component.ID{}, "", fmt.Errorf("invalid path")
}

func preparePath(path string, addLeadingSlash, addTrailingSlash bool) string {
	if addLeadingSlash && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if addTrailingSlash && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

type reversePathIterator struct {
	path string

	slashIndex  int
	searchIndex int
}

// newReversePathIterator creates a new reversePathIterator for the given path,
// where each iteration will split on another / character.
//
// The returned reversePathIterator is uninitialized; call Next to prepare it.
func newReversePathIterator(path string) *reversePathIterator {
	return &reversePathIterator{
		path: path,

		slashIndex:  -1,
		searchIndex: -1,
	}
}

// Next advances the iterator and prepares the next element.
func (it *reversePathIterator) Next() bool {
	// Special case: first iteration.
	if it.searchIndex == -1 {
		it.slashIndex = len(it.path)
		it.searchIndex = len(it.path)
		return true
	}

	it.slashIndex = strings.LastIndexByte(it.path[:it.searchIndex], '/')
	if it.slashIndex != -1 {
		it.searchIndex = it.slashIndex
	} else {
		it.searchIndex = 0
	}
	return it.slashIndex != -1
}

// Value returns the current iterator value. The before string is the string
// before the / character being split on, and the after string is the string
// after the / character being split on.
//
// The first iteration will use the entire input string as the "before".
// The final interation will split on the first / character found in the
// input path.
func (it *reversePathIterator) Value() (before string, after string) {
	return it.path[:it.slashIndex], it.path[it.slashIndex:]
}

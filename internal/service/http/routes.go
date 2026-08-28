package http

import (
	"net/http"
	"sort"
	"strings"
)

type serviceRoute struct {
	Base    string
	Handler http.Handler
}

// serviceRoutes is a sortable collection of serviceRoute.
type serviceRoutes []serviceRoute

var _ sort.Interface = (serviceRoutes)(nil)

func (sr serviceRoutes) Len() int { return len(sr) }

func (sr serviceRoutes) Less(i, j int) bool {
	// Prefer longer paths.
	return strings.Count(sr[i].Base, "/") > strings.Count(sr[j].Base, "/")
}

func (sr serviceRoutes) Swap(i, j int) {
	sr[i], sr[j] = sr[j], sr[i]
}

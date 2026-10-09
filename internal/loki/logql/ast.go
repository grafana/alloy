package logql

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
)

// Filter is a line filter sent to a querier to filter out log line.
type Filter func(string) bool

// Expr is a LogQL expression.
type Expr interface {
	Filter() (Filter, error)
	Matchers() []*labels.Matcher
}

type matchersExpr struct {
	matchers []*labels.Matcher
}

func (e *matchersExpr) Matchers() []*labels.Matcher {
	return e.matchers
}

func (e *matchersExpr) Filter() (Filter, error) {
	return nil, nil
}

type filterExpr struct {
	left  Expr
	ty    labels.MatchType
	match string
}

func (e *filterExpr) Matchers() []*labels.Matcher {
	return e.left.Matchers()
}

// NewFilterExpr wraps an existing Expr with a next filter expression.
func NewFilterExpr(left Expr, ty labels.MatchType, match string) Expr {
	return &filterExpr{
		left:  left,
		ty:    ty,
		match: match,
	}
}

func (e *filterExpr) Filter() (Filter, error) {
	var f func(string) bool
	switch e.ty {
	case labels.MatchRegexp:
		re, err := regexp.Compile(e.match)
		if err != nil {
			return nil, err
		}
		f = re.MatchString

	case labels.MatchNotRegexp:
		re, err := regexp.Compile(e.match)
		if err != nil {
			return nil, err
		}
		f = func(line string) bool {
			return !re.MatchString(line)
		}

	case labels.MatchEqual:
		f = func(line string) bool {
			return strings.Contains(line, e.match)
		}

	case labels.MatchNotEqual:
		f = func(line string) bool {
			return !strings.Contains(line, e.match)
		}

	default:
		return nil, fmt.Errorf("unknow matcher: %v", e.match)
	}
	next, ok := e.left.(*filterExpr)
	if ok {
		nextFilter, err := next.Filter()
		if err != nil {
			return nil, err
		}
		return func(line string) bool {
			return nextFilter(line) && f(line)
		}, nil
	}
	return f, nil
}

func mustNewMatcher(t labels.MatchType, n, v string) *labels.Matcher {
	m, err := labels.NewMatcher(t, n, v)
	if err != nil {
		panic(err)
	}
	return m
}

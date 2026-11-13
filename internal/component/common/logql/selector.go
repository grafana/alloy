// Package logql provides an Alloy wrapper type for LogQL log selectors that
// implements encoding.TextMarshaler and encoding.TextUnmarshaler for use in
// Alloy component configs.
package logql

import (
	"encoding"
	"errors"
	"fmt"
	"slices"
	"unsafe"

	"github.com/grafana/loki/v3/pkg/logql/log"
	"github.com/grafana/loki/v3/pkg/logql/syntax"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

// Selector is a LogQL selector made of labels matchers and line
// filters, parsed and validated at decode time. It is safe for concurrent use.
type Selector struct {
	raw      string
	matchers []*labels.Matcher
	filter   log.Filterer
}

var (
	_ encoding.TextMarshaler   = Selector{}
	_ encoding.TextUnmarshaler = (*Selector)(nil)
)

// UnmarshalText implements encoding.TextUnmarshaler.
func (s *Selector) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return errors.New("selector cannot be empty")
	}

	// validate is false to keep accepting selectors where every matcher can match
	// an empty value, e.g. {app!="foo"}.
	expr, err := syntax.ParseLogSelector(string(text), false)
	if err != nil {
		return err
	}

	matchers := expr.Matchers()

	if len(matchers) == 0 {
		return errors.New("selector must contain at least one matcher")
	}

	filter, err := lineFilter(expr)
	if err != nil {
		return err
	}

	*s = Selector{raw: string(text), matchers: matchers, filter: filter}
	return nil
}

// lineFilter builds the filter for the line filters of expr. It returns nil if
// there are none and an error if expr has anything else after the matchers.
func lineFilter(expr syntax.LogSelectorExpr) (log.Filterer, error) {
	p, ok := expr.(*syntax.PipelineExpr)
	if !ok {
		return nil, nil
	}

	var filters []log.Filterer
	for _, stage := range p.MultiStages {
		lf, ok := stage.(*syntax.LineFilterExpr)
		if !ok {
			return nil, fmt.Errorf("unsupported stage in selector: %s", stage.String())
		}

		if err := validateLineFilter(lf); err != nil {
			return nil, err
		}

		f, err := lf.Filter()
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}

	return log.NewAndFilters(filters), nil
}

// allowedLineMatchTypes are the line filter types a selector may use.
// The full list is at https://github.com/grafana/loki/blob/5cf8c23a258d/pkg/logql/log/filter.go#L21-L29
var allowedLineMatchTypes = []log.LineMatchType{
	log.LineMatchEqual,
	log.LineMatchNotEqual,
	log.LineMatchRegexp,
	log.LineMatchNotRegexp,
}

// validateLineFilter only allows contains and regex line filters, including
// those chained to lf with other filters or "or".
func validateLineFilter(lf *syntax.LineFilterExpr) error {
	var err error
	lf.Walk(func(e syntax.Expr) bool {
		n, ok := e.(*syntax.LineFilterExpr)
		if !ok {
			err = fmt.Errorf("unsupported expression in selector: %s", e.String())
			return false
		}

		// Op is set for filters such as ip().
		if n.Op != "" {
			err = fmt.Errorf("unsupported line filter in selector: %s", n.String())
			return false
		}

		if !slices.Contains(allowedLineMatchTypes, n.Ty) {
			err = fmt.Errorf("unsupported line filter in selector: %s", n.String())
			return false
		}
		return true
	})
	return err
}

// MarshalText implements encoding.TextMarshaler.
func (s Selector) MarshalText() (text []byte, err error) {
	return []byte(s.raw), nil
}

// Matches reports whether a line from a stream with the given labels is
// selected.
func (s Selector) Matches(lbs model.LabelSet, line string) bool {
	for _, m := range s.matchers {
		if !m.Matches(string(lbs[model.LabelName(m.Name)])) {
			return false
		}
	}

	if s.filter == nil {
		return true
	}

	// The filters only read the line and do not retain or mutate it. Loki does
	// the same: https://github.com/grafana/loki/blob/5cf8c23a258d/pkg/logql/log/pipeline.go#L215-L220
	// #nosec G103
	// nosemgrep: use-of-unsafe-block
	b := unsafe.Slice(unsafe.StringData(line), len(line))
	return s.filter.Filter(b)
}

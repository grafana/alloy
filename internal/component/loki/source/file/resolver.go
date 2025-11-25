package file

import (
	"iter"
	"path/filepath"

	"github.com/bmatcuk/doublestar"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/prometheus/common/model"
)

type resolvedTarget struct {
	Path   string
	Labels model.LabelSet
}

// resolver converts discovery targets into concrete file paths to be tailed.
//
// Implementations may expand patterns (e.g., globs) and may yield multiple
// concrete files for a single input target. Results are returned via an
// iterator that yields (ResolvedTarget, error) pairs to support best-effort
// processing: an implementation can continue yielding other results even if a
// particular target fails to resolve.
type resolver interface {
	Resolve(targets []discovery.Target) iter.Seq2[resolvedTarget, error]
}

var _ resolver = (*staticResolver)(nil)

func newStaticResolver() *staticResolver {
	return &staticResolver{}
}

// staticResolver treats each discovery target as a concrete file reference.
// It does not perform any pattern expansion; the value of __path__ is returned
// as-is, paired with the target's non-reserved labels. Use when FileMatch is
// disabled and targets already point to specific files.
type staticResolver struct{}

func (s *staticResolver) Resolve(targets []discovery.Target) iter.Seq2[resolvedTarget, error] {
	return func(yield func(resolvedTarget, error) bool) {
		for _, target := range targets {
			path, _ := target.Get(labelPath)
			labels := target.NonReservedLabelSet()
			if !yield(resolvedTarget{Path: path, Labels: labels}, nil) {
				return
			}
		}
	}
}

var _ resolver = (*globResolver)(nil)

func newGlobResolver() *globResolver {
	return &globResolver{}
}

// globResolver expands discovery targets using doublestar globbing. It reads
// __path__ as a glob pattern and yields one ResolvedTarget per matched file.
// If __path_exclude__ is present, matches that satisfy the exclude pattern are
// filtered out. Returned paths are normalized to absolute form.
type globResolver struct{}

func (s *globResolver) Resolve(targets []discovery.Target) iter.Seq2[resolvedTarget, error] {
	return func(yield func(resolvedTarget, error) bool) {
		for _, target := range targets {
			targetPath, _ := target.Get(labelPath)
			labels := target.NonReservedLabelSet()

			matches, err := doublestar.Glob(targetPath)
			if err != nil {
				if !yield(resolvedTarget{}, err) {
					return
				}
				continue
			}

			exclude, _ := target.Get(labelPathExclude)

			for _, m := range matches {
				if exclude != "" {
					if match, _ := doublestar.PathMatch(exclude, m); match {
						continue
					}
				}

				path, err := filepath.Abs(m)
				if err != nil {
					if !yield(resolvedTarget{}, err) {
						return
					}
					continue
				}

				if !yield(resolvedTarget{Path: path, Labels: labels}, nil) {
					return
				}
			}
		}
	}
}

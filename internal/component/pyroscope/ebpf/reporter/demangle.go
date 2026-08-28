//go:build unix

package reporter

import (
	"github.com/ianlancetaylor/demangle"
	"go.opentelemetry.io/ebpf-profiler/libpf"
)

var demangleUnspecified []demangle.Option = nil
var demangleNoneSpecified = make([]demangle.Option, 0)
var demangleSimplified = []demangle.Option{demangle.NoParams, demangle.NoEnclosingParams, demangle.NoTemplateParams}
var demangleTemplates = []demangle.Option{demangle.NoParams, demangle.NoEnclosingParams}
var demangleFull = []demangle.Option{demangle.NoClones}

func convertDemangleOptions(o string) []demangle.Option {
	switch o {
	case "none":
		return demangleNoneSpecified
	case "simplified":
		return demangleSimplified
	case "templates":
		return demangleTemplates
	case "full":
		return demangleFull
	default:
		return demangleUnspecified
	}
}

func (p *PPROFReporter) demangle(name libpf.String) libpf.String {
	if name == libpf.NullString {
		return name
	}
	if p.cfg.Demangle == "none" {
		return name
	}
	options := convertDemangleOptions(p.cfg.Demangle)
	return libpf.Intern(demangle.Filter(name.String(), options...))
}

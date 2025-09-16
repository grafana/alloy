package reporter

import "github.com/ianlancetaylor/demangle"

var demangleUnspecified []demangle.Option = nil
var demangleNoneSpecified []demangle.Option = make([]demangle.Option, 0)
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

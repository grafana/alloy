package stages

import (
	"github.com/grafana/regexp"
)

type DecolorizeConfig struct{}

type decolorizeStage struct{}

func newDecolorizeStage(_ DecolorizeConfig) (Stage, error) {
	return &decolorizeStage{}, nil
}

// regexp to select ANSI characters courtesy of https://github.com/acarl005/stripansi
const ansiPattern = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

var ansiRegex = regexp.MustCompile(ansiPattern)

// Run implements Stage
func (m *decolorizeStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range in {
			decolorizedLine := ansiRegex.ReplaceAll([]byte(e.Line), []byte{})
			e.Entry.Line = string(decolorizedLine)
			out <- e
		}
	}()
	return out
}

// Name implements Stage
func (m *decolorizeStage) Name() string {
	return StageTypeDecolorize
}

// Cleanup implements Stage.
func (*decolorizeStage) Cleanup() {
	// no-op
}

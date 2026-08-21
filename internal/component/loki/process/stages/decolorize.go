package stages

import (
	"context"

	"github.com/grafana/regexp"
)

type DecolorizeConfig struct{}

var (
	_ Stage = (*decolorizeStage)(nil)
	_ stage = (*decolorizeStage)(nil)
)

func newDecolorizeStage(_ DecolorizeConfig, next NextFn) *decolorizeStage {
	return &decolorizeStage{next: next}
}

type decolorizeStage struct {
	next NextFn
}

// regexp to select ANSI characters courtesy of https://github.com/acarl005/stripansi
const ansiPattern = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

var ansiRegex = regexp.MustCompile(ansiPattern)

// Run implements Stage
func (m *decolorizeStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		decolorizedLine := ansiRegex.ReplaceAll([]byte(e.Line), []byte{})
		e.Entry.Line = string(decolorizedLine)
		return e
	})
}

// process implements stage and is only used by our new pipeline.
func (m *decolorizeStage) process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		decolorizedLine := ansiRegex.ReplaceAll([]byte(entries[i].Line), []byte{})
		entries[i].Line = string(decolorizedLine)
	}
	return m.next(ctx, entries)
}

func (*decolorizeStage) Cleanup() {}

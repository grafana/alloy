package testappender

import (
	"context"
	"fmt"

	"github.com/prometheus/prometheus/storage"
)

// Versions lists the Prometheus appender versions, so that tests can run
// each case through all of them.
//
// TODO: Remove V1 when the V1 appenders are removed.
var Versions = []string{"V1", "V2"}

// Appends are the same data appended through the V1 appender and through the
// V2 appender, for tests that check that both versions give the same result.
type Appends struct {
	V1 func(storage.Appender) error
	V2 func(storage.AppenderV2) error
}

// AppendAndCommit makes the appends of version, one of Versions, through an
// appender of appendable, and commits them. The appendable must implement
// storage.Appendable as well, for V1.
func (a Appends) AppendAndCommit(ctx context.Context, version string, appendable storage.AppendableV2) error {
	switch version {
	case "V1":
		app := appendable.(storage.Appendable).Appender(ctx)
		if err := a.V1(app); err != nil {
			_ = app.Rollback()
			return err
		}
		return app.Commit()
	case "V2":
		app := appendable.AppenderV2(ctx)
		if err := a.V2(app); err != nil {
			_ = app.Rollback()
			return err
		}
		return app.Commit()
	default:
		panic(fmt.Sprintf("unknown appender version %q", version))
	}
}

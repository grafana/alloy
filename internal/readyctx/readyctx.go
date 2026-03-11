package readyctx

import "context"

type ctxKey struct{}

func WithOnReady(ctx context.Context, fn func()) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, fn)
}

func OnReadyFromContext(ctx context.Context) (fn func(), ok bool) {
	v := ctx.Value(ctxKey{})
	if v == nil {
		return nil, false
	}
	fn, ok = v.(func())
	return fn, ok
}

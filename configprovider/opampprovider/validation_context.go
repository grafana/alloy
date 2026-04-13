package opampprovider // import "github.com/grafana/alloy/configprovider/opampprovider"

import "context"

type nestedValidationKey struct{}

// ContextWithoutRemoteWatch returns ctx that disables fsnotify in Retrieve while nested config validation runs.
func ContextWithoutRemoteWatch(ctx context.Context) context.Context {
	return context.WithValue(ctx, nestedValidationKey{}, true)
}

func remoteWatchDisabled(ctx context.Context) bool {
	v, _ := ctx.Value(nestedValidationKey{}).(bool)
	return v
}

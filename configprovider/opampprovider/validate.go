package opampprovider // import "github.com/grafana/alloy/configprovider/opampprovider"

import "context"

// ValidateRemoteConfig validates merged remote OpAMP YAML before reload; distributions set it from the collector binary.
var ValidateRemoteConfig func(ctx context.Context, remoteDir string) error

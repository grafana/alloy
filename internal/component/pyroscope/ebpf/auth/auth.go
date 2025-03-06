package auth

import (
	"context"
	"encoding/base64"
	"os"
	"strings"

	"google.golang.org/grpc"
)

func NewBasicAuth(user, passwordFile string) (grpc.CallOption, error) {
	basicAuth := func(username, password string) string {
		auth := username + ":" + password
		return base64.StdEncoding.EncodeToString([]byte(auth))
	}
	password, err := os.ReadFile(passwordFile)
	if passwordFile != "" && err != nil {
		return nil, err
	}

	return grpc.PerRPCCredentials(&BasicAuthRPCCreds{
		header: "Basic " + basicAuth(user, strings.TrimSpace(string(password))),
	}), nil
}

type BasicAuthRPCCreds struct {
	header string
}

func (b *BasicAuthRPCCreds) GetRequestMetadata(
	context.Context,
	...string,
) (map[string]string, error) {
	return map[string]string{
		"Authorization": b.header,
	}, nil
}

func (b *BasicAuthRPCCreds) RequireTransportSecurity() bool {
	return false
}

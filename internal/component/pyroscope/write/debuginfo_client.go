package write

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"

	commonconfig "github.com/prometheus/common/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func newDebugInfoGRPCClient(u *url.URL, e *EndpointOptions) (*grpc.ClientConn, error) {
	var creds credentials.TransportCredentials
	var auth *basicAuthCredential
	switch u.Scheme {
	case "http":
		creds = insecure.NewCredentials()
	case "https":
		if promTLSConfig := e.HTTPClientConfig.TLSConfig.Convert(); promTLSConfig != nil {
			tlsConf, err := commonconfig.NewTLSConfig(promTLSConfig)
			if err != nil {
				return nil, err
			}
			creds = credentials.NewTLS(tlsConf)
		} else {
			creds = credentials.NewTLS(&tls.Config{})
		}
		var err error
		if auth, err = newGrpcBasicAuthCredentials(e); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}
	if auth != nil {
		opts = append(opts, grpc.WithPerRPCCredentials(auth))
	}
	target := u.Host
	if u.Port() == "" {
		defaultPort := "80"
		if u.Scheme == "https" {
			defaultPort = "443"
		}
		target = fmt.Sprintf("%s:%s", u.Hostname(), defaultPort)
	}
	cc, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, err
	}

	return cc, nil
}

func newGrpcBasicAuthCredentials(e *EndpointOptions) (*basicAuthCredential, error) {
	auth := e.HTTPClientConfig.BasicAuth
	if auth == nil || auth.Username == "" {
		return nil, nil
	}
	if auth.Password != "" {
		return &basicAuthCredential{
			username: auth.Username,
			password: string(auth.Password),
		}, nil
	}
	if auth.PasswordFile != "" {
		passwordBytes, err := os.ReadFile(auth.PasswordFile)
		if err != nil {
			return nil, err
		}
		return &basicAuthCredential{
			username: auth.Username,
			password: strings.TrimSpace(string(passwordBytes)),
		}, nil
	}
	return nil, nil
}

type basicAuthCredential struct {
	username string
	password string
}

func (b *basicAuthCredential) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	auth := b.username + ":" + b.password
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	return map[string]string{
		"authorization": "Basic " + encodedAuth,
	}, nil
}

func (b *basicAuthCredential) RequireTransportSecurity() bool {
	return true
}

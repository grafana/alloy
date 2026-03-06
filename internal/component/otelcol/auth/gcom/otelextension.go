package gcom

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"
)

const (
	defaultHeader     = "Authorization"
	defaultScheme     = "Bearer"
	defaultSigningAlg = "ES384"
)

type Config struct {
	Tenant  string
	RoleARN string
}

// NewFactory creates a factory for the static bearer token Authenticator extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		component.MustNewType("gcomawsauth"),
		createDefaultConfig,
		createExtension,
		component.StabilityLevelBeta,
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createExtension(ctx context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	return newGcomAwsAuth(ctx, cfg.(*Config), set.Logger)
}

// GcomAwsAuth is an implementation of extensionauth interfaces. It embeds a dynamic authorization JWT token in every rpc call.
type gcomAwsAuth struct {
	tenant string

	logger *zap.Logger

	aws *sts.Client
}

var (
	_ extension.Extension      = (*gcomAwsAuth)(nil)
	_ extensionauth.HTTPClient = (*gcomAwsAuth)(nil)
	_ extensionauth.GRPCClient = (*gcomAwsAuth)(nil)
)

func newGcomAwsAuth(ctx context.Context, cfg *Config, logger *zap.Logger) (*gcomAwsAuth, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	client := sts.NewFromConfig(awsCfg)
	if cfg.RoleARN != "" {
		stsSvc := sts.NewFromConfig(awsCfg)
		creds := stscreds.NewAssumeRoleProvider(stsSvc, cfg.RoleARN)

		awsCfg.Credentials = aws.NewCredentialsCache(creds)
		client = sts.NewFromConfig(awsCfg)
	}

	return &gcomAwsAuth{
		tenant: cfg.Tenant,

		logger: logger,
		aws:    client,
	}, nil
}

func (b *gcomAwsAuth) Start(ctx context.Context, _ component.Host) error {
	return nil
}

func (b *gcomAwsAuth) Shutdown(_ context.Context) error {
	return nil
}

// authorizationValue is actually retrieving a JWT token from AWS
func (b *gcomAwsAuth) authorizationValue(ctx context.Context) (string, error) {
	tmpAlg := defaultSigningAlg
	input := sts.GetWebIdentityTokenInput{
		Audience:         []string{b.tenant},
		SigningAlgorithm: &tmpAlg,
	}

	// TODO: cache token
	output, err := b.aws.GetWebIdentityToken(ctx, &input)
	if err != nil {
		return "", fmt.Errorf("failed to get JWT from AWS STS: %w", err)
	}

	token := *output.WebIdentityToken

	return defaultScheme + " " + token, nil
}

// RoundTripper is not implemented by BearerTokenAuth
func (b *gcomAwsAuth) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return &gcomAwsAuthRoundTripper{
		baseTransport: base,
		auth:          b,
	}, nil
}

// BearerAuthRoundTripper intercepts and adds Bearer token Authorization headers to each http request.
type gcomAwsAuthRoundTripper struct {
	baseTransport http.RoundTripper
	auth          *gcomAwsAuth
}

// RoundTrip modifies the original request and adds Bearer JWT token Authorization headers. Incoming requests support multiple tokens, but outgoing requests only use one.
func (interceptor *gcomAwsAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	if req2.Header == nil {
		req2.Header = make(http.Header)
	}
	authVal, err := interceptor.auth.authorizationValue(req2.Context())
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve authorization value: %w", err)
	}
	req2.Header.Set(defaultHeader, authVal)
	return interceptor.baseTransport.RoundTrip(req2)
}

// PerRPCCredentials returns PerRPCAuth an implementation of credentials.PerRPCCredentials that
func (b *gcomAwsAuth) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return &perRPCAuth{
		auth: b,
	}, nil
}

var _ credentials.PerRPCCredentials = (*perRPCAuth)(nil)

// PerRPCAuth is a gRPC credentials.PerRPCCredentials implementation that returns an 'authorization' header.
type perRPCAuth struct {
	auth *gcomAwsAuth
}

// GetRequestMetadata returns the request metadata to be used with the RPC.
func (c *perRPCAuth) GetRequestMetadata(ctx context.Context, s ...string) (map[string]string, error) {
	authVal, err := c.auth.authorizationValue(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve authorization value: %w", err)
	}
	return map[string]string{strings.ToLower(defaultHeader): authVal}, nil
}

// RequireTransportSecurity always returns true for this implementation. Passing bearer tokens in plain-text connections is a bad idea.
func (*perRPCAuth) RequireTransportSecurity() bool {
	return true
}

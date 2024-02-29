package kafkatarget

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/IBM/sarama"
)

func NewOAuthProvider(opts OAuthConfig) (sarama.AccessTokenProvider, error) {
	switch opts.TokenProvider {
	case TokenProviderTypeAzure:
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, err
		}
		return &TokenProviderAzure{tokenProvider: cred, scopes: opts.Scopes}, nil
	default:
		return nil, fmt.Errorf("token provider '%s' is not supported", opts.TokenProvider)
	}
}

type azureTokenProvider interface {
	GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error)
}

// TokenProviderAzure implements sarama.AccessTokenProvider
type TokenProviderAzure struct {
	tokenProvider azureTokenProvider
	scopes        []string
}

// Token returns a new *sarama.AccessToken or an error
func (t *TokenProviderAzure) Token() (*sarama.AccessToken, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	token, err := t.tokenProvider.GetToken(ctx, policy.TokenRequestOptions{Scopes: t.scopes})
	if err != nil {
		return nil, fmt.Errorf("failed to acquire token: %w", err)
	}
	return &sarama.AccessToken{Token: token.Token}, nil
}

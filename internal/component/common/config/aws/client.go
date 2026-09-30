// Package aws holds AWS client configuration that remote.aws.* components share.
package aws

import (
	"context"
	"errors"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/grafana/alloy/syntax/alloytypes"
)

// DefaultSessionName is the STS session name when assume_role does not set one.
const DefaultSessionName = "alloy"

// Client configures how a component connects to AWS.
type Client struct {
	Region     string            `alloy:"region,attr,optional"`
	Endpoint   string            `alloy:"endpoint,attr,optional"`
	AccessKey  string            `alloy:"key,attr,optional"`
	Secret     alloytypes.Secret `alloy:"secret,attr,optional"`
	AssumeRole *AssumeRole       `alloy:"assume_role,block,optional"`
}

// AssumeRole configures an STS AssumeRole call on top of the base credentials.
type AssumeRole struct {
	RoleARN     string            `alloy:"role_arn,attr"`
	SessionName string            `alloy:"session_name,attr,optional"`
	ExternalID  alloytypes.Secret `alloy:"external_id,attr,optional"`
}

// Validate implements syntax.Validator.
func (c Client) Validate() error {
	if (c.AccessKey == "") != (c.Secret == "") {
		return errors.New("client: key and secret must both be set or both be empty")
	}
	if c.AssumeRole != nil && c.AssumeRole.RoleARN == "" {
		return errors.New("client: assume_role.role_arn must not be empty")
	}
	return nil
}

// LoadConfig builds an AWS config. Settings that are not set come from the SDK default chain.
func (c Client) LoadConfig(ctx context.Context) (awssdk.Config, error) {
	var opts []func(*config.LoadOptions) error
	if c.Region != "" {
		opts = append(opts, config.WithRegion(c.Region))
	}
	if c.AccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(c.AccessKey, string(c.Secret), ""),
		))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return awssdk.Config{}, fmt.Errorf("loading AWS config: %w", err)
	}

	if ar := c.AssumeRole; ar != nil {
		sessionName := ar.SessionName
		if sessionName == "" {
			sessionName = DefaultSessionName
		}
		provider := stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), ar.RoleARN, func(o *stscreds.AssumeRoleOptions) {
			o.RoleSessionName = sessionName
			if ar.ExternalID != "" {
				o.ExternalID = awssdk.String(string(ar.ExternalID))
			}
		})
		cfg.Credentials = awssdk.NewCredentialsCache(provider)
	}

	return cfg, nil
}

// BaseEndpoint returns the endpoint override for a service client, or nil if there is none.
// The endpoint applies per service, so LoadConfig does not set it.
func (c Client) BaseEndpoint() *string {
	if c.Endpoint == "" {
		return nil
	}
	return awssdk.String(c.Endpoint)
}

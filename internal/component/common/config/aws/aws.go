package aws

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	aws_config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/grafana/alloy/syntax/alloytypes"
)

// Client implements specific AWS configuration options
type Client struct {
	AccessKey     string            `alloy:"key,attr,optional"`
	Secret        alloytypes.Secret `alloy:"secret,attr,optional"`
	Endpoint      string            `alloy:"endpoint,attr,optional"`
	DisableSSL    bool              `alloy:"disable_ssl,attr,optional"`
	Region        string            `alloy:"region,attr,optional"`
	SigningRegion string            `alloy:"signing_region,attr,optional"`
}

func GenerateAWSConfig(o Client) (*aws.Config, error) {
	configOptions := make([]func(*aws_config.LoadOptions) error, 0)
	// Override the endpoint.
	if o.Endpoint != "" {
		endFunc := aws.EndpointResolverWithOptionsFunc(func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
			// The S3 compatible system used for testing with does not require signing region, so it's fine to be blank
			// but when using a proxy to real S3 it needs to be injected.
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           o.Endpoint,
				SigningRegion: o.SigningRegion,
			}, nil
		})
		endResolver := aws_config.WithEndpointResolverWithOptions(endFunc)
		configOptions = append(configOptions, endResolver)
	}

	// This incredibly nested option turns off SSL.
	if o.DisableSSL {
		httpOverride := aws_config.WithHTTPClient(
			&http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: o.DisableSSL,
					},
				},
			},
		)
		configOptions = append(configOptions, httpOverride)
	}

	if o.Region != "" {
		configOptions = append(configOptions, aws_config.WithRegion(o.Region))
	}

	// Check to see if we need to override the credentials, else it will use the default ones.
	// https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html
	if o.AccessKey != "" {
		if o.Secret == "" {
			return nil, fmt.Errorf("if accesskey or secret are specified then the other must also be specified")
		}
		credFunc := aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     o.AccessKey,
				SecretAccessKey: string(o.Secret),
			}, nil
		})
		credProvider := aws_config.WithCredentialsProvider(credFunc)
		configOptions = append(configOptions, credProvider)
	}

	cfg, err := aws_config.LoadDefaultConfig(context.TODO(), configOptions...)
	if err != nil {
		return nil, err
	}

	// Set region.
	if o.Region != "" {
		cfg.Region = o.Region
	}

	return &cfg, nil
}

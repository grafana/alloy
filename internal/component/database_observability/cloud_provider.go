package database_observability

import (
	"github.com/aws/aws-sdk-go-v2/aws/arn"
)

type CloudProvider struct {
	AWS   *AWSCloudProviderInfo
	Azure *AzureCloudProviderInfo
}

type AWSCloudProviderInfo struct {
	ARN arn.ARN
}

type AzureCloudProviderInfo struct {
	SubscriptionID string
	ResourceGroup  string
	ServerName     string
}

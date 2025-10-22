package database_observability

import "github.com/aws/aws-sdk-go-v2/aws/arn"

type CloudProvider struct {
	AWS *AWSCloudProviderInfo
}

type AWSCloudProviderInfo struct {
	ARN arn.ARN
}

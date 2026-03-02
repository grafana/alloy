package database_observability

import (
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
)

var (
	RdsRegex             = regexp.MustCompile(`(?P<identifier>[^\.]+)\.([^\.]+)\.(?P<region>[^\.]+)\.rds\.amazonaws\.com`)
	AzureMySQLRegex      = regexp.MustCompile(`(?P<identifier>[^\.]+)\.(?:privatelink\.)?mysql\.database\.azure\.com`)
	AzurePostgreSQLRegex = regexp.MustCompile(`(?P<identifier>[^\.]+)\.(?:privatelink\.)?postgres\.database\.azure\.com`)
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

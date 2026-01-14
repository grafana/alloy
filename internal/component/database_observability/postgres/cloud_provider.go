package postgres

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/collector"
)

var (
	rdsRegex   = regexp.MustCompile(`(?P<identifier>[^\.]+)\.([^\.]+)\.(?P<region>[^\.]+)\.rds\.amazonaws\.com`)
	azureRegex = regexp.MustCompile(`(?P<identifier>[^\.]+)\.postgres\.database\.azure\.com`)
)

func populateCloudProviderFromConfig(config *CloudProvider) (*database_observability.CloudProvider, error) {
	var cloudProvider database_observability.CloudProvider
	if config.AWS != nil {
		arn, err := arn.Parse(config.AWS.ARN)
		if err != nil {
			return nil, fmt.Errorf("failed to parse AWS cloud provider ARN: %w", err)
		}
		cloudProvider.AWS = &database_observability.AWSCloudProviderInfo{
			ARN: arn,
		}
	}
	if config.Azure != nil {
		cloudProvider.Azure = &database_observability.AzureCloudProviderInfo{
			SubscriptionID: config.Azure.SubscriptionID,
			ResourceGroup:  config.Azure.ResourceGroup,
			ServerName:     config.Azure.ServerName,
		}
	}
	return &cloudProvider, nil
}

func populateCloudProviderFromDSN(dsn string) (*database_observability.CloudProvider, error) {
	var cloudProvider database_observability.CloudProvider

	parts, err := collector.ParseURL(dsn)
	if err != nil {
		return nil, err
	}

	if host, ok := parts["host"]; ok {
		if strings.HasSuffix(host, "rds.amazonaws.com") {
			matches := rdsRegex.FindStringSubmatch(host)
			cloudProvider.AWS = &database_observability.AWSCloudProviderInfo{
				ARN: arn.ARN{
					Resource:  fmt.Sprintf("db:%s", matches[1]),
					Region:    matches[3],
					AccountID: "unknown",
				},
			}
		} else if strings.HasSuffix(host, "postgres.database.azure.com") {
			matches := azureRegex.FindStringSubmatch(host)
			if len(matches) > 1 {
				cloudProvider.Azure = &database_observability.AzureCloudProviderInfo{
					ServerName: matches[1],
				}
			}
		}
	}

	return &cloudProvider, nil
}

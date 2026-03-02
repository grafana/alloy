package postgres

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/collector"
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
			matches := database_observability.RdsRegex.FindStringSubmatch(host)
			cloudProvider.AWS = &database_observability.AWSCloudProviderInfo{
				ARN: arn.ARN{
					Resource:  fmt.Sprintf("db:%s", matches[1]),
					Region:    matches[3],
					AccountID: "unknown",
				},
			}
		} else if strings.HasSuffix(host, "postgres.database.azure.com") {
			matches := database_observability.AzurePostgreSQLRegex.FindStringSubmatch(host)
			if len(matches) > 1 {
				cloudProvider.Azure = &database_observability.AzureCloudProviderInfo{
					ServerName: matches[1],
				}
			}
		}
	}

	return &cloudProvider, nil
}

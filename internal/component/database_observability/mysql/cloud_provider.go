package mysql

import (
	"fmt"
	"net"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/go-sql-driver/mysql"
	"github.com/grafana/alloy/internal/component/database_observability"
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

	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	host, _, err := net.SplitHostPort(cfg.Addr)
	if err == nil && host != "" {
		if strings.HasSuffix(host, "rds.amazonaws.com") {
			if matches := database_observability.RdsRegex.FindStringSubmatch(host); len(matches) >= 4 {
				cloudProvider.AWS = &database_observability.AWSCloudProviderInfo{
					ARN: arn.ARN{
						Resource:  fmt.Sprintf("db:%s", matches[1]),
						Region:    matches[3],
						AccountID: "unknown",
					},
				}
			}
		} else if strings.HasSuffix(host, "mysql.database.azure.com") {
			if matches := database_observability.AzureMySQLRegex.FindStringSubmatch(host); len(matches) >= 2 {
				cloudProvider.Azure = &database_observability.AzureCloudProviderInfo{
					ServerName: matches[1],
				}
			}
		}
	}

	return &cloudProvider, nil
}

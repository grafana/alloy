package database_observability

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/go-sql-driver/mysql"
)

var (
	rdsRegex   = regexp.MustCompile(`(?P<identifier>[^\.]+)\.([^\.]+)\.(?P<region>[^\.]+)\.rds\.amazonaws\.com`)
	azureRegex = regexp.MustCompile(`(?P<identifier>[^\.]+)\.mysql\.database\.azure\.com`)
)

type CloudProvider struct {
	AWS   *AWSCloudProviderInfo
	Azure *AzureCloudProviderInfo
}

type AWSCloudProviderInfo struct {
	ARN arn.ARN
}

type AzureCloudProviderInfo struct {
	Resource string
}

func PopulateCloudProvider(cloudProvider *CloudProvider, dsn string) (*CloudProvider, error) {
	if cloudProvider != nil {
		return cloudProvider, nil
	}

	cloudProvider = &CloudProvider{}

	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	host, _, err := net.SplitHostPort(cfg.Addr)
	if err == nil && host != "" {
		if strings.HasSuffix(host, "rds.amazonaws.com") {
			matches := rdsRegex.FindStringSubmatch(host)
			cloudProvider.AWS = &AWSCloudProviderInfo{
				ARN: arn.ARN{
					Resource:  fmt.Sprintf("db:%s", matches[1]),
					Region:    matches[3],
					AccountID: "unknown",
				},
			}
		} else if strings.HasSuffix(host, "mysql.database.azure.com") {
			matches := azureRegex.FindStringSubmatch(host)
			cloudProvider.Azure = &AzureCloudProviderInfo{
				Resource: matches[1],
			}
		}
	}

	return cloudProvider, nil
}

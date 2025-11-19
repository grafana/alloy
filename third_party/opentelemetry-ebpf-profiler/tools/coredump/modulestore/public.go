package modulestore // import "go.opentelemetry.io/ebpf-profiler/tools/coredump/modulestore"

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// moduleStoreRegion defines the S3 bucket OCI region.
const moduleStoreRegion = "us-sanjose-1"

// moduleStoreObjectNamespace defines the S3 bucket OCI object name space.
const moduleStoreObjectNamespace = "axtwf1hkrwcy"

// modulePublicReadUrl defines the S3 bucket OCI public read only base path.
//
//nolint:lll
const modulePublicReadURL = "sm-wftyyzHJkBghWeexmK1o5ArimNwZC-5eBej5Lx4e46sLVHtO_y7Zf7FZgoIu_/n/axtwf1hkrwcy"

// moduleStoreS3Bucket defines the S3 bucket used for the module store.
const moduleStoreS3Bucket = "ebpf-profiling-coredumps"

func InitModuleStore(localCachePath string) (*Store, error) {
	publicReadURL := fmt.Sprintf("https://%s.objectstorage.%s.oci.customer-oci.com/p/%s/b/%s/o/",
		moduleStoreObjectNamespace, moduleStoreRegion, modulePublicReadURL, moduleStoreS3Bucket)

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		baseEndpoint := fmt.Sprintf("https://%s.compat.objectstorage.%s.oraclecloud.com/",
			moduleStoreObjectNamespace, moduleStoreRegion)
		o.Region = moduleStoreRegion
		o.BaseEndpoint = aws.String(baseEndpoint)
		o.UsePathStyle = true
	})
	return New(s3Client, publicReadURL, moduleStoreS3Bucket, localCachePath)
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package modulestore // import "go.opentelemetry.io/ebpf-profiler/tools/coredump/modulestore"

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// getS3ObjectList gets all matching objects in an S3 bucket.
func getS3ObjectList(client *s3.Client, bucket, prefix string,
	itemLimit int) ([]s3types.Object, error) {
	var objects []s3types.Object
	var contToken *string
	var batchSize int32 = s3ResultsPerPage

	for {
		resp, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket:            &bucket,
			Prefix:            &prefix,
			MaxKeys:           &batchSize,
			ContinuationToken: contToken,
		})

		if err != nil {
			return nil, fmt.Errorf("s3 request failed: %w", err)
		}

		objects = append(objects, resp.Contents...)

		if int32(len(resp.Contents)) != batchSize {
			break
		}
		if len(resp.Contents) > itemLimit {
			return nil, errors.New("too many matching items in bucket")
		}

		contToken = resp.ContinuationToken
	}

	return objects, nil
}

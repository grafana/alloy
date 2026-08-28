package test

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"github.com/google/pprof/profile"
	querierv1 "github.com/grafana/pyroscope/api/gen/proto/go/querier/v1"
	"github.com/grafana/pyroscope/api/gen/proto/go/querier/v1/querierv1connect"
)

func Query(url string, q *querierv1.SelectMergeProfileRequest) (*profile.Profile, error) {
	client := querierv1connect.NewQuerierServiceClient(http.DefaultClient, url)
	res, err := client.SelectMergeProfile(context.Background(), connect.NewRequest(q))
	if err != nil {
		return nil, err
	}
	bs, err := res.Msg.MarshalVT()
	if err != nil {
		return nil, err
	}
	return profile.ParseData(bs)
}

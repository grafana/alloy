package cloudflare

// This code is copied from Promtail (a1c1152b79547a133cc7be520a0b2e6db8b84868).
// The cloudflaretarget package is used to configure and run a target that can
// read from the Cloudflare Logpull API and forward entries to other loki
// components.

import (
	"context"
	"time"

	"github.com/grafana/cloudflare-go"
)

// Client is a wrapper around the Cloudflare API that allow for testing and being zone/fields aware.
type Client interface {
	LogpullReceived(ctx context.Context, start, end time.Time) (cloudflare.LogpullReceivedIterator, error)
}

type wrappedClient struct {
	client *cloudflare.API
	zoneID string
	fields []string
}

func (w *wrappedClient) LogpullReceived(ctx context.Context, start, end time.Time) (cloudflare.LogpullReceivedIterator, error) {
	return w.client.LogpullReceived(ctx, w.zoneID, start, end, cloudflare.LogpullReceivedOption{
		Fields: w.fields,
	})
}

var getClient = func(apiKey, zoneID string, fields []string) (Client, error) {
	c, err := cloudflare.NewWithAPIToken(apiKey)
	if err != nil {
		return nil, err
	}
	return &wrappedClient{
		client: c,
		zoneID: zoneID,
		fields: fields,
	}, nil
}

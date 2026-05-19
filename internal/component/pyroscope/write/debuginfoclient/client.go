package debuginfoclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1/debuginfov1alpha1connect"
)

type Client struct {
	debuginfov1alpha1connect.DebuginfoServiceClient
	HTTPClient    *http.Client
	BaseURL       string
	UploadTimeout time.Duration
}

func (c *Client) Upload(ctx context.Context, buildID string, body io.Reader) error {
	ctx, cancel := context.WithTimeout(ctx, c.UploadTimeout)
	defer cancel()
	t1 := time.Now()
	uploadURL := strings.TrimRight(c.BaseURL, "/") + "/debuginfo.v1alpha1.DebuginfoService/Upload/" + url.PathEscape(buildID)
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, body)
	if err != nil {
		return err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload: HTTP %d (duration %s)", resp.StatusCode, time.Since(t1))
	}
	return nil
}

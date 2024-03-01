package usagestats

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/grafana/agent/internal/agentseed"
	"github.com/grafana/agent/internal/useragent"
	"github.com/prometheus/common/version"
)

var (
	httpClient    = http.Client{Timeout: 5 * time.Second}
	usageStatsURL = "https://stats.grafana.org/agent-usage-report"
)

// Report is the payload to be sent to stats.grafana.org
type Report struct {
	UsageStatsID string                 `json:"usageStatsId"`
	CreatedAt    time.Time              `json:"createdAt"`
	Interval     time.Time              `json:"interval"`
	Version      string                 `json:"version"`
	Metrics      map[string]interface{} `json:"metrics"`
	Os           string                 `json:"os"`
	Arch         string                 `json:"arch"`
	DeployMode   string                 `json:"deployMode"`
}

func sendReport(ctx context.Context, seed *agentseed.AgentSeed, interval time.Time, metrics map[string]interface{}) error {
	report := Report{
		UsageStatsID: seed.UID,
		CreatedAt:    seed.CreatedAt,
		Version:      version.Version,
		Os:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		Interval:     interval,
		Metrics:      metrics,
		DeployMode:   useragent.GetDeployMode(),
	}
	out, err := json.MarshalIndent(report, "", " ")
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, usageStatsURL, bytes.NewBuffer(out))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("failed to send usage stats: %s  body: %s", resp.Status, string(data))
	}
	return nil
}

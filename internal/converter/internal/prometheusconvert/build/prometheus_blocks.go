package build

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/syntax/token/builder"
)

// PrometheusBlocks is a type for categorizing Alloy blocks before appending
// them to an Alloy file. This gives control over the order they are written
// versus appending them in the order the Blocks are created.
type PrometheusBlocks struct {
	DiscoveryBlocks             []prometheusBlock
	DiscoveryRelabelBlocks      []prometheusBlock
	PrometheusScrapeBlocks      []prometheusBlock
	PrometheusRelabelBlocks     []prometheusBlock
	PrometheusRemoteWriteBlocks []prometheusBlock
}

func NewPrometheusBlocks() *PrometheusBlocks {
	return &PrometheusBlocks{
		DiscoveryBlocks:             []prometheusBlock{},
		DiscoveryRelabelBlocks:      []prometheusBlock{},
		PrometheusScrapeBlocks:      []prometheusBlock{},
		PrometheusRelabelBlocks:     []prometheusBlock{},
		PrometheusRemoteWriteBlocks: []prometheusBlock{},
	}
}

// AppendToBody attaches prometheus blocks in a specific order.
//
// Order of blocks:
// 1. Discovery component(s)
// 2. Discovery relabel component(s) (if any)
// 3. Prometheus scrape component(s)
// 4. Prometheus relabel component(s) (if any)
// 5. Prometheus remote_write
func (pb *PrometheusBlocks) AppendToBody(body *builder.Body) {
	for _, promBlock := range pb.DiscoveryBlocks {
		body.AppendBlock(promBlock.block)
	}

	for _, promBlock := range pb.DiscoveryRelabelBlocks {
		body.AppendBlock(promBlock.block)
	}

	for _, promBlock := range pb.PrometheusScrapeBlocks {
		body.AppendBlock(promBlock.block)
	}

	for _, promBlock := range pb.PrometheusRelabelBlocks {
		body.AppendBlock(promBlock.block)
	}

	for _, promBlock := range pb.PrometheusRemoteWriteBlocks {
		body.AppendBlock(promBlock.block)
	}
}

func (pb *PrometheusBlocks) GetScrapeInfo() diag.Diagnostics {
	var diags diag.Diagnostics

	for _, promScrapeBlock := range pb.PrometheusScrapeBlocks {
		detail := promScrapeBlock.detail

		for _, promDiscoveryBlock := range pb.DiscoveryBlocks {
			if strings.HasPrefix(promDiscoveryBlock.label, promScrapeBlock.label) {
				detail = fmt.Sprintln(detail) + fmt.Sprintf("	A %s.%s component", strings.Join(promDiscoveryBlock.name, "."), promDiscoveryBlock.label)
			}
		}

		for _, promDiscoveryRelabelBlock := range pb.DiscoveryRelabelBlocks {
			if strings.HasPrefix(promDiscoveryRelabelBlock.label, promScrapeBlock.label) {
				detail = fmt.Sprintln(detail) + fmt.Sprintf("	A %s.%s component", strings.Join(promDiscoveryRelabelBlock.name, "."), promDiscoveryRelabelBlock.label)
			}
		}

		for _, promRelabelBlock := range pb.PrometheusRelabelBlocks {
			if strings.HasPrefix(promRelabelBlock.label, promScrapeBlock.label) {
				detail = fmt.Sprintln(detail) + fmt.Sprintf("	A %s.%s component", strings.Join(promRelabelBlock.name, "."), promRelabelBlock.label)
			}
		}

		diags.AddWithDetail(diag.SeverityLevelInfo, promScrapeBlock.summary, detail)
	}

	for _, promRemoteWriteBlock := range pb.PrometheusRemoteWriteBlocks {
		diags.AddWithDetail(diag.SeverityLevelInfo, promRemoteWriteBlock.summary, promRemoteWriteBlock.detail)
	}

	return diags
}

type prometheusBlock struct {
	block   *builder.Block
	name    []string
	label   string
	summary string
	detail  string
}

func NewPrometheusBlock(block *builder.Block, name []string, label string, summary string, detail string) prometheusBlock {
	return prometheusBlock{
		block:   block,
		name:    name,
		label:   label,
		summary: summary,
		detail:  detail,
	}
}

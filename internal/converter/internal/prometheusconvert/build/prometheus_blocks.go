package build

import (
	"fmt"
	"strings"

	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/river/token/builder"
)

// PrometheusBlocks is a type for categorizing River Blocks before appending
// them to a River File. This gives control over the order they are written
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

// AppendToFile attaches prometheus blocks in a specific order.
//
// Order of blocks:
// 1. Discovery component(s)
// 2. Discovery relabel component(s) (if any)
// 3. Prometheus scrape component(s)
// 4. Prometheus relabel component(s) (if any)
// 5. Prometheus remote_write
func (pb *PrometheusBlocks) AppendToFile(f *builder.File) {
	for _, promBlock := range pb.DiscoveryBlocks {
		f.Body().AppendBlock(promBlock.block)
	}

	for _, promBlock := range pb.DiscoveryRelabelBlocks {
		f.Body().AppendBlock(promBlock.block)
	}

	for _, promBlock := range pb.PrometheusScrapeBlocks {
		f.Body().AppendBlock(promBlock.block)
	}

	for _, promBlock := range pb.PrometheusRelabelBlocks {
		f.Body().AppendBlock(promBlock.block)
	}

	for _, promBlock := range pb.PrometheusRemoteWriteBlocks {
		f.Body().AppendBlock(promBlock.block)
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

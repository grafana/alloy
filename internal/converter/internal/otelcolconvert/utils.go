package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/grafana/river/token/builder"
	"go.opentelemetry.io/collector/component"
)

func stringifyInstanceID(id component.InstanceID) string {
	return fmt.Sprintf("%s/%s", stringifyKind(id.Kind), id.ID)
}

func stringifyKind(k component.Kind) string {
	switch k {
	case component.KindReceiver:
		return "receiver"
	case component.KindProcessor:
		return "processor"
	case component.KindExporter:
		return "exporter"
	case component.KindExtension:
		return "extension"
	case component.KindConnector:
		return "connector"
	default:
		return fmt.Sprintf("Kind(%d)", k)
	}
}

func stringifyBlock(block *builder.Block) string {
	return fmt.Sprintf("%s.%s", strings.Join(block.Name, "."), block.Label)
}

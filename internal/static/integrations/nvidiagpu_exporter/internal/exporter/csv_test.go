package exporter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/static/integrations/nvidiagpu_exporter/internal/exporter"
)

const testCsv = `
name, power.draw [W]
NVIDIA GeForce RTX 2080 SUPER, 30.14 W
Some Dummy GPU, 12.34 W
`

func TestParseCsvIntoTable(t *testing.T) {
	t.Parallel()

	parsed, err := exporter.ParseCSVIntoTable(testCsv, []exporter.QField{"name", "power.draw"})

	require.NoError(t, err)
	assert.Len(t, parsed.Rows, 2)
	assert.Equal(t, []exporter.RField{"name", "power.draw [W]"}, parsed.RFields)

	cell00 := exporter.Cell{
		QField:   "name",
		RField:   "name",
		RawValue: "NVIDIA GeForce RTX 2080 SUPER",
	}
	cell01 := exporter.Cell{QField: "power.draw", RField: "power.draw [W]", RawValue: "30.14 W"}
	cell10 := exporter.Cell{QField: "name", RField: "name", RawValue: "Some Dummy GPU"}
	cell11 := exporter.Cell{QField: "power.draw", RField: "power.draw [W]", RawValue: "12.34 W"}

	row0 := exporter.Row{
		QFieldToCells: map[exporter.QField]exporter.Cell{"name": cell00, "power.draw": cell01},
		Cells:         []exporter.Cell{cell00, cell01},
	}

	row1 := exporter.Row{
		QFieldToCells: map[exporter.QField]exporter.Cell{"name": cell10, "power.draw": cell11},
		Cells:         []exporter.Cell{cell10, cell11},
	}

	expected := exporter.Table{
		Rows:    []exporter.Row{row0, row1},
		RFields: []exporter.RField{"name", "power.draw [W]"},
		QFieldToCells: map[exporter.QField][]exporter.Cell{
			"name":       {cell00, cell10},
			"power.draw": {cell01, cell11},
		},
	}

	assert.Equal(t, expected, parsed)
}

package exporter

import (
	"fmt"
	"strings"
)

type Table struct {
	Rows          []Row
	RFields       []RField
	QFieldToCells map[QField][]Cell
}

type Row struct {
	QFieldToCells map[QField]Cell
	Cells         []Cell
}

type Cell struct {
	QField   QField
	RField   RField
	RawValue string
}

func ParseCSVIntoTable(queryResult string, qFields []QField) (Table, error) {
	lines := strings.Split(strings.TrimSpace(queryResult), "\n")
	titlesLine := lines[0]
	valuesLines := lines[1:]
	rFields := toRFieldSlice(parseCSVLine(titlesLine))

	numCols := len(qFields)
	numRows := len(valuesLines)

	rows := make([]Row, numRows)

	qFieldToCells := make(map[QField][]Cell)
	for _, q := range qFields {
		qFieldToCells[q] = make([]Cell, numRows)
	}

	for rowIndex, valuesLine := range valuesLines {
		qFieldToCell := make(map[QField]Cell, numCols)
		cells := make([]Cell, numCols)
		rawValues := parseCSVLine(valuesLine)

		if len(qFields) != len(rFields) {
			return Table{}, fmt.Errorf(
				"field count mismatch: query fields: %d, returned fields: %d",
				len(qFields),
				len(rFields),
			)
		}

		for colIndex, rawValue := range rawValues {
			currentQField := qFields[colIndex]
			currentRField := rFields[colIndex]
			tableCell := Cell{
				QField:   currentQField,
				RField:   currentRField,
				RawValue: rawValue,
			}
			qFieldToCell[currentQField] = tableCell
			cells[colIndex] = tableCell
			qFieldToCells[currentQField][rowIndex] = tableCell
		}

		tableRow := Row{
			QFieldToCells: qFieldToCell,
			Cells:         cells,
		}

		rows[rowIndex] = tableRow
	}

	return Table{
		Rows:          rows,
		RFields:       rFields,
		QFieldToCells: qFieldToCells,
	}, nil
}

func parseCSVLine(line string) []string {
	values := strings.Split(line, ",")
	result := make([]string, len(values))

	for i, field := range values {
		result[i] = strings.TrimSpace(field)
	}

	return result
}

package collector

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"
)

func TestExplainPlanRedactor(t *testing.T) {
	tests := []struct {
		txtarfile string
		file      string
		original  [][]byte
		redacted  [][]byte
	}{
		{
			txtarfile: "./testdata/explain_plan/join_and_order.txtar",
			file:      "join_and_order.json",
			original: [][]byte{
				[]byte("(`employees`.`de`.`to_date` = DATE'9999-01-01')"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `de` . `to_date` = date ? )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/subquery_with_aggregate.txtar",
			file:      "subquery_with_aggregate.json",
			original: [][]byte{
				[]byte("((`employees`.`s`.`to_date` = DATE'9999-01-01') and (`employees`.`s`.`salary` > (/* select#2 */ select (avg(`employees`.`salaries`.`salary`) * 1.5) from `employees`.`salaries`)))"),
			},
			redacted: [][]byte{
				[]byte("( ( `employees` . `s` . `to_date` = date ? ) and ( `employees` . `s` . `salary` > ( select ( avg ( `employees` . `salaries` . `salary` ) * ? ) from `employees` . `salaries` ) ) )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/group_by_with_having.txtar",
			file:      "group_by_with_having.json",
			original: [][]byte{
				[]byte("(`employees`.`de`.`to_date` = DATE'9999-01-01')"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `de` . `to_date` = date ? )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/multiple_joins_with_date_functions.txtar",
			file:      "multiple_joins_with_date_functions.json",
			original: [][]byte{
				[]byte("(`employees`.`de`.`to_date` = DATE'9999-01-01')"),
				[]byte("(year(`employees`.`e`.`hire_date`) = 1985)"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `de` . `to_date` = date ? )"),
				[]byte("( year ( `employees` . `e` . `hire_date` ) = ? )"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			archive, err := txtar.ParseFile(test.txtarfile)
			require.NoError(t, err)
			require.Equal(t, 1, len(archive.Files))
			jsonFile := archive.Files[0]
			require.Equal(t, test.file, jsonFile.Name)
			jsonData := jsonFile.Data
			for _, original := range test.original {
				require.Contains(t, string(jsonData), string(original))
				// Comparing the byte arrays directly fails, even though the contents are identical?
				// require.Contains(t, data, original)
			}

			redactedExplainPlanJSON, redactedAttachedConditionsCount, err := redactAttachedConditions(jsonData)
			require.NoError(t, err, "Failed to redact file: %s", test.file)

			for _, tRedacted := range test.redacted {
				require.Contains(t, string(redactedExplainPlanJSON), string(tRedacted))
			}

			require.Equal(t, len(test.original), redactedAttachedConditionsCount)
		})
	}
}

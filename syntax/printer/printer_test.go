package printer_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/printer"
	"github.com/stretchr/testify/require"
)

func TestPrinter(t *testing.T) {
	filepath.WalkDir("testdata", func(path string, d fs.DirEntry, _ error) error {
		if d.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, ".in") {
			inputFile := path
			expectFile := strings.TrimSuffix(path, ".in") + ".expect"
			expectErrorFile := strings.TrimSuffix(path, ".in") + ".error"

			caseName := filepath.Base(path)
			caseName = strings.TrimSuffix(caseName, ".in")

			t.Run(caseName, func(t *testing.T) {
				testPrinter(t, inputFile, expectFile, expectErrorFile)
			})
		}

		return nil
	})
}

func TestSecretRedaction(t *testing.T) {
	input := `password = "my_password"
string = "normal string"`

	f, err := parser.ParseFile("", []byte(input))
	require.NoError(t, err)

	pw := f.Body[0].(*ast.AttributeStmt)
	require.Equal(t, "\"my_password\"", pw.Value.(*ast.LiteralExpr).Value)

	unredactedOutput := `password = "my_password"
string   = "normal string"`
	var buf bytes.Buffer
	require.NoError(t, printer.Fprint(&buf, f))
	require.Equal(t, unredactedOutput, buf.String())

	redactedOutput := `password = "(secret)"
string   = "normal string"`

	pw.Value.SetSecret(true)
	c := printer.Config{
		RedactSecrets: true,
	}
	buf.Reset()
	require.NoError(t, c.Fprint(&buf, f))
	require.Equal(t, redactedOutput, buf.String())
}

func testPrinter(t *testing.T, inputFile string, expectFile string, expectErrorFile string) {
	inputBB, err := os.ReadFile(inputFile)
	require.NoError(t, err)

	f, err := parser.ParseFile(t.Name()+".rvr", inputBB)
	if expectedError := getExpectedErrorMessage(t, expectErrorFile); expectedError != "" {
		require.Error(t, err)
		require.Contains(t, err.Error(), expectedError)
		return
	}

	expectBB, err := os.ReadFile(expectFile)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, printer.Fprint(&buf, f))

	trimmed := strings.TrimRightFunc(string(expectBB), unicode.IsSpace)
	require.Equal(t, trimmed, buf.String(), "%s", buf.String())
}

// getExpectedErrorMessage will retrieve an optional expected error message for the test.
func getExpectedErrorMessage(t *testing.T, errorFile string) string {
	if _, err := os.Stat(errorFile); err == nil {
		errorBytes, err := os.ReadFile(errorFile)
		require.NoError(t, err)
		errorsString := string(normalizeLineEndings(errorBytes))
		return errorsString
	}

	return ""
}

// normalizeLineEndings will replace '\r\n' with '\n'.
func normalizeLineEndings(data []byte) []byte {
	normalized := bytes.ReplaceAll(data, []byte{'\r', '\n'}, []byte{'\n'})
	return normalized
}

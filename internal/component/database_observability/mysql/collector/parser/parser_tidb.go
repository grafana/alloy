package parser

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"golang.org/x/exp/maps"

	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/mysql"
	_ "github.com/pingcap/tidb/pkg/parser/test_driver"
)

type TiDBSqlParser struct{}

func NewTiDBSqlParser() *TiDBSqlParser {
	return &TiDBSqlParser{}
}

func (p *TiDBSqlParser) Parse(sql string) (StatementAstNode, error) {
	// mysql will redact auth details with <secret> but the tidb parser
	// will fail to parse it so we replace it with '<secret>'
	sql = strings.ReplaceAll(sql, "IDENTIFIED BY <secret>", "IDENTIFIED BY '<secret>'")

	// tidb parser doesn't support text line IN (...), so we replace it with (?)
	sql = strings.ReplaceAll(sql, "( ... )", "(?)")
	sql = strings.ReplaceAll(sql, "(...)", "(?)")

	// similar cleanup for functions with redacted values
	sql = strings.ReplaceAll(sql, ", ... )", ", ?)")
	sql = strings.ReplaceAll(sql, ", ...)", ", ?)")

	tParser := parser.New()
	stmtNodes, _, err := tParser.ParseSQL(sql)
	if err != nil {
		tParser.SetSQLMode(mysql.ModeIgnoreSpace)
		stmtNodes, _, err = tParser.ParseSQL(sql)
		if err != nil {
			return nil, errors.Unwrap(err)
		}
	}

	if len(stmtNodes) == 0 {
		return nil, fmt.Errorf("no statements parsed")
	}

	return stmtNodes[0], nil
}

func (p *TiDBSqlParser) Redact(sql string) (string, error) {
	res := parser.Normalize(sql, "ON")
	if res == "" {
		return "", fmt.Errorf("error normalizing SQL")
	}
	return res, nil
}

func (p *TiDBSqlParser) ExtractTableNames(stmt StatementAstNode) []string {
	v := &tableNameVisitor{
		tables: map[string]struct{}{},
	}
	stmt.Accept(v)
	keys := maps.Keys(v.tables)
	slices.Sort(keys)
	return keys
}

type tableNameVisitor struct {
	tables map[string]struct{}
}

func (v *tableNameVisitor) Enter(n ast.Node) (ast.Node, bool) {
	if tableRef, ok := n.(*ast.TableName); ok {
		tableName := parseTableName(tableRef)
		v.tables[tableName] = struct{}{}
	}
	return n, false
}

func (v *tableNameVisitor) Leave(n ast.Node) (ast.Node, bool) {
	return n, true
}

func parseTableName(t *ast.TableName) string {
	schema := t.Schema.String()
	tableName := t.Name.String()
	if schema != "" {
		return schema + "." + tableName
	}
	return tableName
}

func (p *TiDBSqlParser) CleanTruncatedText(sql string) (string, error) {
	if !strings.HasSuffix(sql, "...") {
		return sql, nil
	}

	// best-effort attempt to detect truncated trailing comment
	idx := strings.LastIndex(sql, "/*")
	if idx < 0 {
		return "", fmt.Errorf("sql text is truncated")
	}

	trailingText := sql[idx:]
	if strings.LastIndex(trailingText, "*/") >= 0 {
		return "", fmt.Errorf("sql text is truncated after a comment")
	}

	return strings.TrimSpace(sql[:idx]), nil
}

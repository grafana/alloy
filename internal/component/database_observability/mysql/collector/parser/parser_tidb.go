package parser

import (
	"fmt"

	"golang.org/x/exp/maps"

	"github.com/go-kit/log"
	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	_ "github.com/pingcap/tidb/pkg/parser/test_driver"
)

type TiDBSqlParser struct{}

func NewTiDBSqlParser() *TiDBSqlParser {
	return &TiDBSqlParser{}
}

func (p *TiDBSqlParser) Parse(sql string) (any, error) {
	stmtNodes, _, err := parser.New().ParseSQL(sql)
	if err != nil {
		return nil, err
	}

	if len(stmtNodes) == 0 {
		return nil, fmt.Errorf("no statements parsed")
	}

	return &stmtNodes[0], nil
}

func (p *TiDBSqlParser) Redact(sql string) (string, error) {
	res := parser.Normalize(sql, "ON")
	if res == "" {
		return "", fmt.Errorf("error normalizing SQL")
	}
	return res, nil
}

func (p *TiDBSqlParser) StmtType(stmt any) string {
	s := stmt.(*ast.StmtNode)
	switch ast.GetStmtLabel(*s) {
	case "Select":
		return "select"
	case "Insert":
		return "insert"
	case "Update":
		return "update"
	case "Delete":
		return "delete"
	default:
		return ""
	}
}

func (p *TiDBSqlParser) ExtractTableNames(_ log.Logger, _ string, stmt any) []string {
	v := &tableNameVisitor{
		tables: map[string]struct{}{},
	}
	(*stmt.(*ast.StmtNode)).Accept(v)
	return maps.Keys(v.tables)
}

func (p *TiDBSqlParser) ParseTableName(t any) string {
	return parseTableName(t.(*ast.TableName))
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

package parser

import (
	"github.com/pingcap/tidb/pkg/parser/ast"
)

type Parser interface {
	Parse(sql string) (StatementAstNode, error)
	Redact(sql string) (string, error)
	ExtractTableNames(stmt StatementAstNode) []string
	CleanTruncatedText(sql string) (string, error)
}

type StatementAstNode ast.StmtNode

var _ Parser = (*TiDBSqlParser)(nil)

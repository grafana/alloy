package parser

import "github.com/go-kit/log"

type Parser interface {
	Parse(sql string) (any, error)
	Redact(sql string) (string, error)
	StmtType(stmt any) StatementType
	ParseTableName(t any) string
	ExtractTableNames(logger log.Logger, digest string, stmt any) []string
	CleanTruncatedText(sql string) (string, error)
}

type StatementType string

var (
	StatementTypeSelect StatementType = "select"
	StatementTypeInsert StatementType = "insert"
	StatementTypeUpdate StatementType = "update"
	StatementTypeDelete StatementType = "delete"

	_ Parser = (*XwbSqlParser)(nil)
	_ Parser = (*TiDBSqlParser)(nil)
)

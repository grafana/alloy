package parser

import (
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/xwb1989/sqlparser"
)

type XwbSqlParser struct{}

func NewXwbSqlParser() *XwbSqlParser {
	return &XwbSqlParser{}
}

func (p *XwbSqlParser) ParseSql(sql string) (any, error) {
	return sqlparser.Parse(sql)
}

func (p *XwbSqlParser) RedactSQL(sql string) (string, error) {
	return sqlparser.RedactSQLQuery(sql)
}

func (p *XwbSqlParser) StmtType(stmt any) string {
	switch stmt.(type) {
	case *sqlparser.Select:
		return "select"
	case *sqlparser.Insert:
		return "insert"
	case *sqlparser.Update:
		return "update"
	case *sqlparser.Delete:
		return "delete"
	case *sqlparser.Union:
		return "select" // label union as a select
	default:
		return ""
	}
}

func (p *XwbSqlParser) ParseTableName(table any) string {
	t := table.(sqlparser.TableName)
	qualifier := t.Qualifier.String()
	tableName := t.Name.String()
	if qualifier != "" {
		return qualifier + "." + tableName
	}
	return tableName
}

func (p *XwbSqlParser) ExtractTableNames(logger log.Logger, digest string, stmt any) []string {
	var parsedTables []string

	switch stmt := stmt.(type) {
	case *sqlparser.Select:
		for _, selExpr := range stmt.SelectExprs {
			if expr, ok := selExpr.(*sqlparser.AliasedExpr); ok {
				switch exp := expr.Expr.(type) {
				case *sqlparser.Subquery:
					parsedTables = append(parsedTables, p.ExtractTableNames(logger, digest, exp.Select)...)
				default:
					// ignore anything else
				}
			}
		}
		parsedTables = append(parsedTables, p.parseTableExprs(logger, digest, stmt.From)...)
	case *sqlparser.Update:
		parsedTables = p.parseTableExprs(logger, digest, stmt.TableExprs)
	case *sqlparser.Delete:
		parsedTables = p.parseTableExprs(logger, digest, stmt.TableExprs)
	case *sqlparser.Insert:
		parsedTables = []string{p.ParseTableName(stmt.Table)}
		switch insRowsStmt := stmt.Rows.(type) {
		case sqlparser.Values:
			// ignore raw values
		case *sqlparser.Select:
			parsedTables = append(parsedTables, p.ExtractTableNames(logger, digest, insRowsStmt)...)
		case *sqlparser.Union:
			for _, side := range []sqlparser.SelectStatement{insRowsStmt.Left, insRowsStmt.Right} {
				parsedTables = append(parsedTables, p.ExtractTableNames(logger, digest, side)...)
			}
		case *sqlparser.ParenSelect:
			parsedTables = append(parsedTables, p.ExtractTableNames(logger, digest, insRowsStmt.Select)...)
		default:
			level.Error(logger).Log("msg", "unknown insert type", "digest", digest)
		}
	case *sqlparser.Union:
		for _, side := range []sqlparser.SelectStatement{stmt.Left, stmt.Right} {
			parsedTables = append(parsedTables, p.ExtractTableNames(logger, digest, side)...)
		}
	case *sqlparser.Show:
		if stmt.HasOnTable() {
			parsedTables = append(parsedTables, p.ParseTableName(stmt.OnTable))
		}
	case *sqlparser.DDL:
		parsedTables = append(parsedTables, p.ParseTableName(stmt.Table))
	case *sqlparser.Begin, *sqlparser.Commit, *sqlparser.Rollback, *sqlparser.Set, *sqlparser.DBDDL:
		// ignore
	default:
		level.Error(logger).Log("msg", "unknown statement type", "digest", digest)
	}

	return parsedTables
}

func (p *XwbSqlParser) parseTableExprs(logger log.Logger, digest string, tables sqlparser.TableExprs) []string {
	parsedTables := []string{}
	for i := 0; i < len(tables); i++ {
		t := tables[i]
		switch tableExpr := t.(type) {
		case *sqlparser.AliasedTableExpr:
			switch expr := tableExpr.Expr.(type) {
			case sqlparser.TableName:
				parsedTables = append(parsedTables, p.ParseTableName(expr))
			case *sqlparser.Subquery:
				switch subqueryExpr := expr.Select.(type) {
				case *sqlparser.Select:
					parsedTables = append(parsedTables, p.parseTableExprs(logger, digest, subqueryExpr.From)...)
				case *sqlparser.Union:
					for _, side := range []sqlparser.SelectStatement{subqueryExpr.Left, subqueryExpr.Right} {
						parsedTables = append(parsedTables, p.ExtractTableNames(logger, digest, side)...)
					}
				case *sqlparser.ParenSelect:
					parsedTables = append(parsedTables, p.ExtractTableNames(logger, digest, subqueryExpr.Select)...)
				default:
					level.Error(logger).Log("msg", "unknown subquery type", "digest", digest)
				}
			default:
				level.Error(logger).Log("msg", "unknown nested table expression", "digest", digest, "table", tableExpr)
			}
		case *sqlparser.JoinTableExpr:
			// continue parsing both sides of join
			tables = append(tables, tableExpr.LeftExpr, tableExpr.RightExpr)
		case *sqlparser.ParenTableExpr:
			tables = append(tables, tableExpr.Exprs...)
		default:
			level.Error(logger).Log("msg", "unknown table type", "digest", digest, "table", t)
		}
	}
	return parsedTables
}

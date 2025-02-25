package collector

import (
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/service/logging/level"
	"github.com/xwb1989/sqlparser"
)

func ParseSql(sql string) (sqlparser.Statement, error) {
	return sqlparser.Parse(sql)
}

func RedactSQL(sql string) (string, error) {
	return sqlparser.RedactSQLQuery(sql)
}

func StmtType(stmt sqlparser.Statement) string {
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

func ParseTableName(t sqlparser.TableName) string {
	qualifier := t.Qualifier.String()
	tableName := t.Name.String()
	if qualifier != "" {
		return qualifier + "." + tableName
	}
	return tableName
}

func ExtractTableNames(logger log.Logger, digest string, stmt sqlparser.Statement) []string {
	var parsedTables []string

	switch stmt := stmt.(type) {
	case *sqlparser.Select:
		for _, selExpr := range stmt.SelectExprs {
			if expr, ok := selExpr.(*sqlparser.AliasedExpr); ok {
				switch exp := expr.Expr.(type) {
				case *sqlparser.Subquery:
					parsedTables = append(parsedTables, ExtractTableNames(logger, digest, exp.Select)...)
				default:
					// ignore anything else
				}
			}
		}
		parsedTables = append(parsedTables, parseTableExprs(logger, digest, stmt.From)...)
	case *sqlparser.Update:
		parsedTables = parseTableExprs(logger, digest, stmt.TableExprs)
	case *sqlparser.Delete:
		parsedTables = parseTableExprs(logger, digest, stmt.TableExprs)
	case *sqlparser.Insert:
		parsedTables = []string{ParseTableName(stmt.Table)}
		switch insRowsStmt := stmt.Rows.(type) {
		case sqlparser.Values:
			// ignore raw values
		case *sqlparser.Select:
			parsedTables = append(parsedTables, ExtractTableNames(logger, digest, insRowsStmt)...)
		case *sqlparser.Union:
			for _, side := range []sqlparser.SelectStatement{insRowsStmt.Left, insRowsStmt.Right} {
				parsedTables = append(parsedTables, ExtractTableNames(logger, digest, side)...)
			}
		case *sqlparser.ParenSelect:
			parsedTables = append(parsedTables, ExtractTableNames(logger, digest, insRowsStmt.Select)...)
		default:
			level.Error(logger).Log("msg", "unknown insert type", "digest", digest)
		}
	case *sqlparser.Union:
		for _, side := range []sqlparser.SelectStatement{stmt.Left, stmt.Right} {
			parsedTables = append(parsedTables, ExtractTableNames(logger, digest, side)...)
		}
	case *sqlparser.Show:
		if stmt.HasOnTable() {
			parsedTables = append(parsedTables, ParseTableName(stmt.OnTable))
		}
	case *sqlparser.DDL:
		parsedTables = append(parsedTables, ParseTableName(stmt.Table))
	case *sqlparser.Begin, *sqlparser.Commit, *sqlparser.Rollback, *sqlparser.Set, *sqlparser.DBDDL:
		// ignore
	default:
		level.Error(logger).Log("msg", "unknown statement type", "digest", digest)
	}

	return parsedTables
}

func parseTableExprs(logger log.Logger, digest string, tables sqlparser.TableExprs) []string {
	parsedTables := []string{}
	for i := 0; i < len(tables); i++ {
		t := tables[i]
		switch tableExpr := t.(type) {
		case *sqlparser.AliasedTableExpr:
			switch expr := tableExpr.Expr.(type) {
			case sqlparser.TableName:
				parsedTables = append(parsedTables, ParseTableName(expr))
			case *sqlparser.Subquery:
				switch subqueryExpr := expr.Select.(type) {
				case *sqlparser.Select:
					parsedTables = append(parsedTables, parseTableExprs(logger, digest, subqueryExpr.From)...)
				case *sqlparser.Union:
					for _, side := range []sqlparser.SelectStatement{subqueryExpr.Left, subqueryExpr.Right} {
						parsedTables = append(parsedTables, ExtractTableNames(logger, digest, side)...)
					}
				case *sqlparser.ParenSelect:
					parsedTables = append(parsedTables, ExtractTableNames(logger, digest, subqueryExpr.Select)...)
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

package infra

import (
	"github.com/xwb1989/sqlparser"
)

type SqlOp string

const (
	Insert  SqlOp = "insert"
	Delete  SqlOp = "delete"
	Update  SqlOp = "update"
	Select  SqlOp = "select"
	DDL     SqlOp = "ddl"
	Unknown SqlOp = "unknown"
)

type sqlMonitor struct {
	FixTbName func(name string) string
}

var SqlMonitor = &sqlMonitor{}

var defaultFixName = func(name string) string {
	return name
}

func (s *sqlMonitor) SetFixName(f func(name string) string) {
	s.FixTbName = f
}

func (s *sqlMonitor) parseTable(sql string) ([]string, SqlOp, error) {
	stmt, err := sqlparser.Parse(sql)
	if err != nil {
		return nil, "", err
	}
	tables, op := getTable(stmt)
	res := make([]string, 0, len(tables))
	for _, tbname := range tables {
		res = append(res, s.FixTbName(tbname))
	}
	return res, op, nil
}

func getTable(stmt sqlparser.Statement) ([]string, SqlOp) {
	var (
		tables []string
	)
	switch stmt := stmt.(type) {
	case *sqlparser.Select, *sqlparser.Insert, *sqlparser.Update, *sqlparser.Delete:
		sqlparser.Walk(func(node sqlparser.SQLNode) (kontinue bool, err error) {
			exprs, ok := node.(sqlparser.TableExprs)
			if ok {
				tables = append(tables, getTables(exprs)...)
			}
			if len(tables) >= 2 {
				return false, nil
			}
			return true, nil
		}, stmt)
		return tables, getOp(stmt)
	case *sqlparser.DDL:
		return []string{stmt.NewName.Name.String()}, DDL
	}
	return nil, Unknown
}

func getOp(stmt sqlparser.Statement) SqlOp {
	switch stmt.(type) {
	case *sqlparser.Select:
		return Select
	case *sqlparser.Insert:
		return Insert
	case *sqlparser.Update:
		return Update
	case *sqlparser.Delete:
		return Delete
	}
	return ""
}

func getTables(node sqlparser.TableExprs) []string {

	var tables []string
	for _, tableExpr := range node {
		switch tableExpr := tableExpr.(type) {
		case *sqlparser.AliasedTableExpr:
			tableName := tableExpr.Expr.(sqlparser.TableName).Name.String()
			tables = append(tables, tableName)
		case *sqlparser.JoinTableExpr:
			tables = append(tables, getTables([]sqlparser.TableExpr{tableExpr.LeftExpr})...)
			tables = append(tables, getTables([]sqlparser.TableExpr{tableExpr.RightExpr})...)
		}
	}
	return tables
}

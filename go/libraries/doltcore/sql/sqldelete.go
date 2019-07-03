package sql

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/liquidata-inc/ld/dolt/go/libraries/doltcore/doltdb"
	"github.com/liquidata-inc/ld/dolt/go/libraries/doltcore/schema"
	"github.com/liquidata-inc/ld/dolt/go/libraries/doltcore/table/typed/noms"
	"github.com/liquidata-inc/ld/dolt/go/libraries/doltcore/table/untyped/resultset"
	"github.com/liquidata-inc/ld/dolt/go/store/types"
	"vitess.io/vitess/go/vt/sqlparser"
)

type DeleteResult struct {
	Root           *doltdb.RootValue
	NumRowsDeleted int
}

func ExecuteDelete(ctx context.Context, db *doltdb.DoltDB, root *doltdb.RootValue, s *sqlparser.Delete, query string) (*DeleteResult, error) {
	tableExprs := s.TableExprs
	if len(tableExprs) != 1 {
		return errDelete("Exactly one table to delete from must be specified")
	}

	var tableName string
	tableExpr := tableExprs[0]
	switch t := tableExpr.(type) {
	case *sqlparser.AliasedTableExpr:
		switch e := t.Expr.(type) {
		case sqlparser.TableName:
			tableName = e.Name.String()
		case *sqlparser.Subquery:
			return errDelete("Subqueries are not supported: %v.", query)
		default:
			return errDelete("Unrecognized expression: %v", nodeToString(e))
		}
	case *sqlparser.ParenTableExpr:
		return errDelete("Only simple table expression are supported")
	case *sqlparser.JoinTableExpr:
		return errDelete("Joins are not supported")
	default:
		return errDelete("Unsupported update statement %v", query)
	}

	if !root.HasTable(ctx, tableName) {
		return errDelete("Unknown table '%s'", tableName)
	}
	table, _ := root.GetTable(ctx, tableName)
	tableSch := table.GetSchema(ctx)
	rss := resultset.Identity(tableName, tableSch)

	// TODO: support aliases
	filter, err := createFilterForWhere(s.Where, map[string]schema.Schema{tableName: tableSch}, NewAliases())
	if err != nil {
		return errDelete(err.Error())
	}
	if err = filter.Init(rss); err != nil {
		return errDelete(err.Error())
	}

	// Perform the delete
	var result DeleteResult
	rowData := table.GetRowData(ctx)
	me := rowData.Edit()
	rowReader := noms.NewNomsMapReader(ctx, rowData, tableSch)

	for {
		r, err := rowReader.ReadRow(ctx)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if !filter.filter(r) {
			continue
		}

		result.NumRowsDeleted++
		me.Remove(r.NomsMapKey(types.Format_7_18, tableSch))
	}

	table = table.UpdateRows(ctx, me.Map(ctx))

	result.Root = root.PutTable(ctx, db, tableName, table)
	return &result, nil
}

func errDelete(errorFmt string, args ...interface{}) (*DeleteResult, error) {
	return nil, errors.New(fmt.Sprintf(errorFmt, args...))
}

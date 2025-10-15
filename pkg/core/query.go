package core

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/blastrain/vitess-sqlparser/sqlparser"
	shell "github.com/ipfs/go-ipfs-api"
	pb "github.com/nnlgsakib/wwfsdb/pkg/core/proto"
	"github.com/nnlgsakib/wwfsdb/pkg/sql"
	utils "github.com/nnlgsakib/wwfsdb/pkg/util"
	"google.golang.org/protobuf/types/known/anypb"
)

// CombinedRow represents a row resulting from a join, mapping table names to their respective row data.
type CombinedRow map[string]*pb.Row

// Query retrieves rows from a table
func Query(ipfsAPI, dbName, queryString string) ([]map[string]interface{}, error) {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return nil, err
	}

	stmt, err := sqlparser.Parse(queryString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	selectStmt, ok := stmt.(*sqlparser.Select)
	if !ok {
		return nil, fmt.Errorf("query is not a SELECT statement")
	}

	return QueryDB(sh, db, selectStmt)
}

// Helper function to process a table expression and add its schema to the map
func processTableExprForSchemas(sh *shell.Shell, db *pb.Database, tableExpr sqlparser.TableExpr, schemas map[string]*pb.Schema, aliases map[string]string) error {
	switch expr := tableExpr.(type) {
	case *sqlparser.AliasedTableExpr:
		// Handle subqueries in FROM clause (derived tables)
		if subquery, ok := expr.Expr.(*sqlparser.Subquery); ok {
			if expr.As.IsEmpty() {
				return fmt.Errorf("subquery in FROM must have an alias")
			}
			aliasName := expr.As.String()

			// Execute subquery to infer schema
			subqueryRows, err := QueryDB(sh, db, subquery.Select.(*sqlparser.Select))
			if err != nil {
				return fmt.Errorf("error executing subquery for alias %s: %w", aliasName, err)
			}

			if len(subqueryRows) == 0 {
				schemas[aliasName] = &pb.Schema{}
				return nil
			}

			// Infer schema from the first row of the subquery result
			firstRow := subqueryRows[0]
			derivedSchema := &pb.Schema{Columns: []*pb.Column{}}

			// Sort keys for consistent schema order
			colNames := make([]string, 0, len(firstRow))
			for colName := range firstRow {
				colNames = append(colNames, colName)
			}
			sort.Strings(colNames)

			for _, colName := range colNames {
				colValue := firstRow[colName]
				colType := "TEXT" // Default
				if colValue != nil {
					switch colValue.(type) {
					case float64, float32:
						colType = "FLOAT"
					case int, int32, int64:
						colType = "INT"
					case bool:
						colType = "BOOL"
					case time.Time:
						colType = "TIMESTAMP"
					}
				}
				derivedSchema.Columns = append(derivedSchema.Columns, &pb.Column{
					Name: colName,
					Type: colType,
				})
			}
			schemas[aliasName] = derivedSchema
			return nil
		}

		// Regular table
		tableName, err := sql.ExtractTableName(expr)
		if err != nil {
			return fmt.Errorf("failed to extract table name: %w", err)
		}

		tableCID, ok := db.Tables[tableName]
		if !ok {
			return fmt.Errorf("table %s not found", tableName)
		}
		table, err := LoadTable(sh, tableCID)
		if err != nil {
			return err
		}
		schema, err := LoadSchema(sh, table.SchemaCid)
		if err != nil {
			return err
		}

		// Store schema under original table name
		schemas[tableName] = schema

		// If an alias is specified, map the alias to the schema
		if !expr.As.IsEmpty() {
			aliasName := expr.As.String()
			schemas[aliasName] = schema
			aliases[aliasName] = tableName
		}
	case *sqlparser.JoinTableExpr:
		// Process both sides of the JOIN recursively
		if err := processTableExprForSchemas(sh, db, expr.LeftExpr, schemas, aliases); err != nil {
			return err
		}
		if err := processTableExprForSchemas(sh, db, expr.RightExpr, schemas, aliases); err != nil {
			return err
		}
	}
	return nil
}

// QueryDB retrieves rows from an in-memory database object
func QueryDB(sh *shell.Shell, db *pb.Database, s *sqlparser.Select) ([]map[string]interface{}, error) {
	// Pre-calculate all table schemas involved in the query, including aliases
	schemas := make(map[string]*pb.Schema)
	aliases := make(map[string]string)

	for _, tableExpr := range s.From {
		if err := processTableExprForSchemas(sh, db, tableExpr, schemas, aliases); err != nil {
			return nil, err
		}
	}

	// Execute the FROM clause
	combinedRows, err := executeFromClause(sh, db, s.From, schemas)
	if err != nil {
		return nil, fmt.Errorf("error executing FROM/JOIN clause: %w", err)
	}

	// Extract table names from the FROM clause
	fromTableNames := getTableNamesFromClause(s.From)

	// Filter rows using the WHERE clause
	var filteredRows []CombinedRow
	if s.Where == nil {
		filteredRows = combinedRows
	} else {
		for _, row := range combinedRows {
			include, err := evaluateExpression(sh, db, row, s.Where.Expr, schemas, fromTableNames)
			if err != nil {
				return nil, err
			}
			if include {
				filteredRows = append(filteredRows, row)
			}
		}
	}

	// Handle GROUP BY and aggregates
	if s.GroupBy != nil || isAggregateQuery(s) {
		results, err := executeAggregateQuery(sh, db, filteredRows, s, schemas)
		if err != nil {
			return nil, err
		}
		if s.OrderBy != nil {
			applyOrderByToMap(results, s.OrderBy, schemas)
		}
		return applyLimitAndOffsetToMap(sh, db, results, s.Limit), nil
	}

	// Handle ORDER BY for non-aggregate queries
	if s.OrderBy != nil {
		applyOrderBy(sh, db, filteredRows, s.OrderBy, schemas)
	}

	// Handle OFFSET and LIMIT
	finalRows := applyLimitAndOffset(sh, db, filteredRows, s.Limit)

	// Project the final columns
	results, err := projectColumns(sh, db, s.From, finalRows, s.SelectExprs, schemas)
	if err != nil {
		return nil, fmt.Errorf("error projecting columns: %w", err)
	}

	return results, nil
}

func isAggregateQuery(s *sqlparser.Select) bool {
	for _, col := range s.SelectExprs {
		if aliasedExpr, ok := col.(*sqlparser.AliasedExpr); ok {
			if _, ok := aliasedExpr.Expr.(*sqlparser.FuncExpr); ok {
				return true
			}
		}
	}
	return false
}

func applyOrderBy(sh *shell.Shell, db *pb.Database, rows []CombinedRow, orderBy sqlparser.OrderBy, schemas map[string]*pb.Schema) {
	sort.SliceStable(rows, func(i, j int) bool {
		for _, order := range orderBy {
			valI, _ := evaluateExpressionValue(sh, db, rows[i], order.Expr, schemas, getTableNamesFromClause(nil))
			valJ, _ := evaluateExpressionValue(sh, db, rows[j], order.Expr, schemas, getTableNamesFromClause(nil))

			if valI == nil && valJ == nil {
				continue
			}
			if valI == nil {
				return order.Direction != sqlparser.DescScr
			}
			if valJ == nil {
				return order.Direction == sqlparser.DescScr
			}

			numI, isNumI := getNumericValue(valI)
			numJ, isNumJ := getNumericValue(valJ)
			if isNumI && isNumJ {
				if numI != numJ {
					if order.Direction == sqlparser.DescScr {
						return numI > numJ
					}
					return numI < numJ
				}
				continue
			}

			strI, isStrI := valI.(string)
			strJ, isStrJ := valJ.(string)
			if isStrI && isStrJ {
				if strI != strJ {
					if order.Direction == sqlparser.DescScr {
						return strI > strJ
					}
					return strI < strJ
				}
				continue
			}
		}
		return false
	})
}

func getIntValue(v interface{}) (int, bool) {
	switch val := v.(type) {
	case float64:
		return int(val), true
	case int64:
		return int(val), true
	case int:
		return val, true
	}
	return 0, false
}

func applyLimitAndOffset(sh *shell.Shell, db *pb.Database, rows []CombinedRow, limit *sqlparser.Limit) []CombinedRow {
	if limit == nil {
		return rows
	}

	offset := 0
	if limit.Offset != nil {
		offsetVal, err := evaluateExpressionValue(sh, db, nil, limit.Offset, nil, nil)
		if err == nil {
			if o, ok := getIntValue(offsetVal); ok {
				offset = o
			}
		}
	}

	if offset >= len(rows) {
		return []CombinedRow{}
	}
	slicedRows := rows[offset:]

	rowCount := len(slicedRows)
	if limit.Rowcount != nil {
		limitVal, err := evaluateExpressionValue(sh, db, nil, limit.Rowcount, nil, nil)
		if err == nil {
			if l, ok := getIntValue(limitVal); ok {
				rowCount = l
			}
		}
	}

	if rowCount > len(slicedRows) {
		rowCount = len(slicedRows)
	}
	return slicedRows[:rowCount]
}

func applyOrderByToMap(results []map[string]interface{}, orderBy sqlparser.OrderBy, schemas map[string]*pb.Schema) {
	sort.SliceStable(results, func(i, j int) bool {
		for _, order := range orderBy {
			colName, ok := order.Expr.(*sqlparser.ColName)
			if !ok {
				continue
			}

			valI, okI := results[i][colName.Name.String()]
			valJ, okJ := results[j][colName.Name.String()]

			if !okI || !okJ || valI == nil || valJ == nil {
				if valI == nil && valJ != nil {
					return order.Direction != sqlparser.DescScr
				}
				if valJ == nil && valI != nil {
					return order.Direction == sqlparser.DescScr
				}
				continue
			}

			numI, isNumI := getNumericValue(valI)
			numJ, isNumJ := getNumericValue(valJ)
			if isNumI && isNumJ {
				if numI != numJ {
					if order.Direction == sqlparser.DescScr {
						return numI > numJ
					}
					return numI < numJ
				}
				continue
			}

			strI, isStrI := valI.(string)
			strJ, isStrJ := valJ.(string)
			if isStrI && isStrJ {
				if strI != strJ {
					if order.Direction == sqlparser.DescScr {
						return strI > strJ
					}
					return strI < strJ
				}
				continue
			}
		}
		return false
	})
}

func applyLimitAndOffsetToMap(sh *shell.Shell, db *pb.Database, rows []map[string]interface{}, limit *sqlparser.Limit) []map[string]interface{} {
	if limit == nil {
		return rows
	}

	offset := 0
	if limit.Offset != nil {
		offsetVal, err := evaluateExpressionValue(sh, db, nil, limit.Offset, nil, nil)
		if err == nil {
			if o, ok := getIntValue(offsetVal); ok {
				offset = o
			}
		}
	}

	if offset >= len(rows) {
		return []map[string]interface{}{}
	}
	slicedRows := rows[offset:]

	rowCount := len(slicedRows)
	if limit.Rowcount != nil {
		limitVal, err := evaluateExpressionValue(sh, db, nil, limit.Rowcount, nil, nil)
		if err == nil {
			if l, ok := getIntValue(limitVal); ok {
				rowCount = l
			}
		}
	}

	if rowCount > len(slicedRows) {
		rowCount = len(slicedRows)
	}
	return slicedRows[:rowCount]
}

func executeAggregateQuery(sh *shell.Shell, db *pb.Database, rows []CombinedRow, s *sqlparser.Select, schemas map[string]*pb.Schema) ([]map[string]interface{}, error) {
	// Handle non-grouped aggregate query
	if s.GroupBy == nil {
		resultRow := make(map[string]interface{})
		for _, colExpr := range s.SelectExprs {
			aliasedExpr, ok := colExpr.(*sqlparser.AliasedExpr)
			if !ok {
				return nil, fmt.Errorf("unsupported select expression type in aggregate query: %T", colExpr)
			}
			aggExpr, ok := aliasedExpr.Expr.(*sqlparser.FuncExpr)
			if !ok {
				// For non-aggregate expressions in derived tables (e.g., total_spent)
				resultColName := aliasedExpr.As.String()
				if resultColName == "" {
					if colName, ok := aliasedExpr.Expr.(*sqlparser.ColName); ok {
						resultColName = colName.Name.String()
					} else {
						resultColName = sqlparser.String(aliasedExpr.Expr)
					}
				}

				// Check if the column is from a derived table
				if colName, ok := aliasedExpr.Expr.(*sqlparser.ColName); ok {
					qualifier := colName.Qualifier.Name.String()
					for tableName, tableData := range rows[0] {
						if tableData == nil {
							continue
						}
						if qualifier != "" && tableName != qualifier {
							continue
						}
						if valAny, ok := tableData.Values[colName.Name.String()]; ok {
							val, err := utils.FromAny(valAny)
							if err != nil {
								return nil, fmt.Errorf("error converting value for column %s in table %s: %w", colName.Name.String(), tableName, err)
							}
							resultRow[resultColName] = val
							break
						}
					}
					if _, exists := resultRow[resultColName]; !exists {
						return nil, fmt.Errorf("column %s not found in derived table", colName.Name.String())
					}
					continue
				}
				return nil, fmt.Errorf("cannot mix aggregate and non-aggregate columns without GROUP BY")
			}

			var argName string
			if len(aggExpr.Exprs) > 0 {
				if _, ok := aggExpr.Exprs[0].(*sqlparser.StarExpr); ok {
					argName = "*"
				} else if aliased, ok := aggExpr.Exprs[0].(*sqlparser.AliasedExpr); ok {
					if colName, ok := aliased.Expr.(*sqlparser.ColName); ok {
						argName = colName.Name.String()
					}
				}
			}
			resultColName := aliasedExpr.As.String()
			if resultColName == "" {
				resultColName = fmt.Sprintf("%s(%s)", strings.ToUpper(aggExpr.Name.String()), argName)
			}

			switch strings.ToUpper(aggExpr.Name.String()) {
			case "COUNT":
				resultRow[resultColName] = len(rows)
			case "SUM", "AVG", "MIN", "MAX":
				if argName == "*" {
					return nil, fmt.Errorf("%s requires a column argument", strings.ToUpper(aggExpr.Name.String()))
				}
				var total float64
				var min float64
				var max float64
				count := 0
				for i, row := range rows {
					// Check row data for derived table columns first
					var val interface{}
					var err error
					found := false
					for tableName, tableData := range row {
						if tableData == nil {
							continue
						}
						if valAny, ok := tableData.Values[argName]; ok {
							val, err = utils.FromAny(valAny)
							if err != nil {
								return nil, fmt.Errorf("error converting value for column %s in table %s: %w", argName, tableName, err)
							}
							found = true
							break
						}
					}
					if !found {
						// Fallback to schema-based lookup
						val, err = evaluateExpressionValue(sh, db, row, &sqlparser.ColName{Name: sqlparser.NewColIdent(argName)}, schemas, getTableNamesFromClause(s.From))
						if err != nil {
							return nil, fmt.Errorf("error evaluating column %s: %w", argName, err)
						}
					}
					num, isNum := getNumericValue(val)
					if !isNum {
						continue
					}
					if i == 0 || count == 0 {
						min = num
						max = num
					}
					total += num
					if num < min {
						min = num
					}
					if num > max {
						max = num
					}
					count++
				}

				switch strings.ToUpper(aggExpr.Name.String()) {
				case "SUM":
					resultRow[resultColName] = total
				case "AVG":
					if count == 0 {
						resultRow[resultColName] = nil
					} else {
						resultRow[resultColName] = total / float64(count)
					}
				case "MIN":
					if count == 0 {
						resultRow[resultColName] = nil
					} else {
						resultRow[resultColName] = min
					}
				case "MAX":
					if count == 0 {
						resultRow[resultColName] = nil
					} else {
						resultRow[resultColName] = max
					}
				}
			default:
				return nil, fmt.Errorf("unsupported aggregate function: %s", aggExpr.Name.String())
			}
		}

		return []map[string]interface{}{resultRow}, nil
	}

	// GROUP BY Implementation
	type Group struct {
		KeyValues map[string]interface{}
		Rows      []CombinedRow
	}
	groups := make(map[string]*Group)

	for _, row := range rows {
		keyParts := make([]string, len(s.GroupBy))
		keyValues := make(map[string]interface{})
		isNilGroup := false

		for i, expr := range s.GroupBy {
			val, err := evaluateExpressionValue(sh, db, row, expr, schemas, getTableNamesFromClause(s.From))
			if err != nil {
				return nil, err
			}
			if val == nil {
				isNilGroup = true
				break
			}
			keyParts[i] = fmt.Sprintf("%v", val)
			if ident, ok := expr.(*sqlparser.ColName); ok {
				keyValues[ident.Name.String()] = val
			}
		}
		if isNilGroup {
			continue
		}

		groupKey := strings.Join(keyParts, "||")

		if _, ok := groups[groupKey]; !ok {
			groups[groupKey] = &Group{
				KeyValues: keyValues,
				Rows:      []CombinedRow{},
			}
		}
		groups[groupKey].Rows = append(groups[groupKey].Rows, row)
	}

	var finalResults []map[string]interface{}
	for _, group := range groups {
		resultRow := make(map[string]interface{})
		for key, val := range group.KeyValues {
			resultRow[key] = val
		}

		for _, colExpr := range s.SelectExprs {
			aliasedExpr, ok := colExpr.(*sqlparser.AliasedExpr)
			if !ok {
				return nil, fmt.Errorf("unsupported select expression type: %T", colExpr)
			}
			resultColName := aliasedExpr.As.String()
			if resultColName == "" {
				if colName, ok := aliasedExpr.Expr.(*sqlparser.ColName); ok {
					resultColName = colName.Name.String()
				} else if funcExpr, ok := aliasedExpr.Expr.(*sqlparser.FuncExpr); ok {
					if len(funcExpr.Exprs) > 0 {
						if _, ok := funcExpr.Exprs[0].(*sqlparser.StarExpr); ok {
							resultColName = fmt.Sprintf("%s(*)", strings.ToUpper(funcExpr.Name.String()))
						} else if aliased, ok := funcExpr.Exprs[0].(*sqlparser.AliasedExpr); ok {
							if colName, ok := aliased.Expr.(*sqlparser.ColName); ok {
								resultColName = fmt.Sprintf("%s(%s)", strings.ToUpper(funcExpr.Name.String()), colName.Name.String())
							}
						}
					}
				}
				if resultColName == "" {
					resultColName = sqlparser.String(aliasedExpr.Expr)
				}
			}

			if aggExpr, ok := aliasedExpr.Expr.(*sqlparser.FuncExpr); ok {
				var argName string
				if len(aggExpr.Exprs) > 0 {
					if _, ok := aggExpr.Exprs[0].(*sqlparser.StarExpr); ok {
						argName = "*"
					} else if aliased, ok := aggExpr.Exprs[0].(*sqlparser.AliasedExpr); ok {
						if colName, ok := aliased.Expr.(*sqlparser.ColName); ok {
							argName = colName.Name.String()
						}
					}
				}

				switch strings.ToUpper(aggExpr.Name.String()) {
				case "COUNT":
					resultRow[resultColName] = len(group.Rows)
				case "SUM", "AVG", "MIN", "MAX":
					if argName == "*" {
						return nil, fmt.Errorf("%s requires a column argument", strings.ToUpper(aggExpr.Name.String()))
					}
					var total float64
					var min float64
					var max float64
					count := 0
					for i, row := range group.Rows {
						// Check row data for derived table columns first
						var val interface{}
						var err error
						found := false
						for tableName, tableData := range row {
							if tableData == nil {
								continue
							}
							if valAny, ok := tableData.Values[argName]; ok {
								val, err = utils.FromAny(valAny)
								if err != nil {
									return nil, fmt.Errorf("error converting value for column %s in table %s: %w", argName, tableName, err)
								}
								found = true
								break
							}
						}
						if !found {
							// Fallback to schema-based lookup
							val, err = evaluateExpressionValue(sh, db, row, &sqlparser.ColName{Name: sqlparser.NewColIdent(argName)}, schemas, getTableNamesFromClause(s.From))
							if err != nil {
								return nil, fmt.Errorf("error evaluating column %s: %w", argName, err)
							}
						}
						num, isNum := getNumericValue(val)
						if !isNum {
							continue
						}
						if i == 0 || count == 0 {
							min = num
							max = num
						}
						total += num
						if num < min {
							min = num
						}
						if num > max {
							max = num
						}
						count++
					}

					switch strings.ToUpper(aggExpr.Name.String()) {
					case "SUM":
						resultRow[resultColName] = total
					case "AVG":
						if count == 0 {
							resultRow[resultColName] = nil
						} else {
							resultRow[resultColName] = total / float64(count)
						}
					case "MIN":
						if count == 0 {
							resultRow[resultColName] = nil
						} else {
							resultRow[resultColName] = min
						}
					case "MAX":
						if count == 0 {
							resultRow[resultColName] = nil
						} else {
							resultRow[resultColName] = max
						}
					}
				default:
					return nil, fmt.Errorf("unsupported aggregate function: %s", aggExpr.Name.String())
				}
			} else {
				// Non-aggregate column (should be part of GROUP BY or derived table)
				if len(group.Rows) == 0 {
					resultRow[resultColName] = nil
				} else {
					if colName, ok := aliasedExpr.Expr.(*sqlparser.ColName); ok {
						// Check derived table columns first
						qualifier := colName.Qualifier.Name.String()
						for tableName, tableData := range group.Rows[0] {
							if tableData == nil {
								continue
							}
							if qualifier != "" && tableName != qualifier {
								continue
							}
							if valAny, ok := tableData.Values[colName.Name.String()]; ok {
								val, err := utils.FromAny(valAny)
								if err != nil {
									return nil, fmt.Errorf("error converting value for column %s in table %s: %w", colName.Name.String(), tableName, err)
								}
								resultRow[resultColName] = val
								break
							}
						}
						if _, exists := resultRow[resultColName]; !exists {
							// Fallback to schema-based lookup
							val, err := evaluateExpressionValue(sh, db, group.Rows[0], aliasedExpr.Expr, schemas, getTableNamesFromClause(s.From))
							if err != nil {
								return nil, err
							}
							resultRow[resultColName] = val
						}
					} else {
						val, err := evaluateExpressionValue(sh, db, group.Rows[0], aliasedExpr.Expr, schemas, getTableNamesFromClause(s.From))
						if err != nil {
							return nil, err
						}
						resultRow[resultColName] = val
					}
				}
			}
		}
		finalResults = append(finalResults, resultRow)
	}

	return finalResults, nil
}

func executeFromClause(sh *shell.Shell, db *pb.Database, from sqlparser.TableExprs, schemas map[string]*pb.Schema) ([]CombinedRow, error) {
	if len(from) == 0 {
		return []CombinedRow{make(CombinedRow)}, nil
	}

	var finalRows []CombinedRow

	for _, tableExpr := range from {
		var currentRows []CombinedRow
		var err error

		switch expr := tableExpr.(type) {
		case *sqlparser.AliasedTableExpr:
			switch subExpr := expr.Expr.(type) {
			case sqlparser.TableName:
				originalTableName := subExpr.Name.String()
				tableKey := originalTableName
				if !expr.As.IsEmpty() {
					tableKey = expr.As.String()
				}
				currentRows, err = loadTableRows(sh, db, originalTableName, tableKey)
			case *sqlparser.Subquery:
				if expr.As.IsEmpty() {
					return nil, fmt.Errorf("subquery in FROM must have an alias")
				}
				aliasName := expr.As.String()
				subqueryResults, subErr := QueryDB(sh, db, subExpr.Select.(*sqlparser.Select))
				if subErr != nil {
					return nil, fmt.Errorf("error executing subquery for alias %s: %w", aliasName, subErr)
				}
				currentRows, err = processSubqueryResults(subqueryResults, aliasName)
			default:
				err = fmt.Errorf("unsupported expression in FROM clause: %T", expr.Expr)
			}
		case *sqlparser.JoinTableExpr:
			currentRows, err = executeJoin(sh, db, expr, schemas)
		default:
			err = fmt.Errorf("unsupported table expression type: %T", tableExpr)
		}

		if err != nil {
			return nil, err
		}

		finalRows = crossProduct(finalRows, currentRows)
	}

	return finalRows, nil
}

func crossProduct(left, right []CombinedRow) []CombinedRow {
	if len(left) == 0 {
		return right
	}
	if len(right) == 0 {
		return left
	}

	newRows := make([]CombinedRow, 0, len(left)*len(right))
	for _, lRow := range left {
		for _, rRow := range right {
			mergedRow := make(CombinedRow)
			for k, v := range lRow {
				mergedRow[k] = v
			}
			for k, v := range rRow {
				mergedRow[k] = v
			}
			newRows = append(newRows, mergedRow)
		}
	}
	return newRows
}

func processSubqueryResults(results []map[string]interface{}, aliasName string) ([]CombinedRow, error) {
	var combinedRows []CombinedRow
	for _, resultMap := range results {
		pbRow := &pb.Row{Values: make(map[string]*anypb.Any)}
		for colName, colValue := range resultMap {
			anyVal, err := utils.ToAny(colValue)
			if err != nil {
				anyVal, _ = utils.ToAny(fmt.Sprintf("%v", colValue))
			}
			pbRow.Values[colName] = anyVal
		}
		combinedRows = append(combinedRows, CombinedRow{aliasName: pbRow})
	}
	return combinedRows, nil
}

func loadTableRows(sh *shell.Shell, db *pb.Database, tableName, tableKey string) ([]CombinedRow, error) {
	tableCID, ok := db.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table %s not found in database", tableName)
	}
	table, err := LoadTable(sh, tableCID)
	if err != nil {
		return nil, err
	}

	var results []CombinedRow
	for _, pageCID := range table.PageCids {
		page, err := LoadPage(sh, pageCID)
		if err != nil {
			fmt.Printf("Warning: failed to load page %s: %v\n", pageCID, err)
			continue
		}
		for _, row := range page.Rows {
			results = append(results, CombinedRow{tableKey: row})
		}
	}
	return results, nil
}

func executeJoin(sh *shell.Shell, db *pb.Database, joinExpr *sqlparser.JoinTableExpr, schemas map[string]*pb.Schema) ([]CombinedRow, error) {
	leftRows, err := executeFromClause(sh, db, sqlparser.TableExprs{joinExpr.LeftExpr}, schemas)
	if err != nil {
		return nil, fmt.Errorf("error executing left side of join: %w", err)
	}

	rightRows, err := executeFromClause(sh, db, sqlparser.TableExprs{joinExpr.RightExpr}, schemas)
	if err != nil {
		return nil, fmt.Errorf("error executing right side of join: %w", err)
	}

	var joinedRows []CombinedRow

	joinCondition := joinExpr.On

	for _, lRow := range leftRows {
		for _, rRow := range rightRows {
			shouldJoin := true
			if joinCondition != nil {
				mergedRow := make(CombinedRow)
				for k, v := range lRow {
					mergedRow[k] = v
				}
				for k, v := range rRow {
					mergedRow[k] = v
				}

				// Use all table names from both sides of the join
				leftTables := getTableNamesFromClause(sqlparser.TableExprs{joinExpr.LeftExpr})
				rightTables := getTableNamesFromClause(sqlparser.TableExprs{joinExpr.RightExpr})
				joinTableNames := append(leftTables, rightTables...)

				result, err := evaluateExpression(sh, db, mergedRow, joinCondition, schemas, joinTableNames)
				if err != nil {
					return nil, fmt.Errorf("error evaluating join condition: %w", err)
				}
				shouldJoin = result
			}

			if shouldJoin {
				mergedRow := make(CombinedRow)
				for k, v := range lRow {
					mergedRow[k] = v
				}
				for k, v := range rRow {
					mergedRow[k] = v
				}
				joinedRows = append(joinedRows, mergedRow)
			}
		}
	}

	return joinedRows, nil
}

func projectColumns(sh *shell.Shell, db *pb.Database, from sqlparser.TableExprs, rows []CombinedRow, columns sqlparser.SelectExprs, schemas map[string]*pb.Schema) ([]map[string]interface{}, error) {
	results := make([]map[string]interface{}, 0, len(rows))
	fromTableNames := getTableNamesFromClause(from)

	for _, combinedRow := range rows {
		resultRow := make(map[string]interface{})
		for _, colExpr := range columns {
			switch c := colExpr.(type) {
			case *sqlparser.StarExpr:
				if c.TableName.Name.IsEmpty() {
					for tName, rowData := range combinedRow {
						if rowData == nil {
							for _, schemaCol := range schemas[tName].Columns {
								resultRow[tName+"."+schemaCol.Name] = nil
							}
							continue
						}
						for colName, valAny := range rowData.Values {
							val, _ := utils.FromAny(valAny)
							resultRow[tName+"."+colName] = val
						}
					}
				} else {
					tName := c.TableName.Name.String()
					rowData, ok := combinedRow[tName]
					if !ok {
						return nil, fmt.Errorf("table %s not found in query", tName)
					}
					if rowData == nil {
						for _, schemaCol := range schemas[tName].Columns {
							resultRow[tName+"."+schemaCol.Name] = nil
						}
						continue
					}
					for colName, valAny := range rowData.Values {
						val, _ := utils.FromAny(valAny)
						resultRow[tName+"."+colName] = val
					}
				}
			case *sqlparser.AliasedExpr:
				val, err := evaluateExpressionValue(sh, db, combinedRow, c.Expr, schemas, fromTableNames)
				if err != nil {
					return nil, fmt.Errorf("error evaluating expression for column %s: %w", c.As.String(), err)
				}
				finalColName := c.As.String()
				if finalColName == "" {
					switch expr := c.Expr.(type) {
					case *sqlparser.ColName:
						finalColName = expr.Name.String()
					case *sqlparser.FuncExpr:
						if len(expr.Exprs) > 0 {
							if _, ok := expr.Exprs[0].(*sqlparser.StarExpr); ok {
								finalColName = fmt.Sprintf("%s(*)", strings.ToUpper(expr.Name.String()))
							} else if aliased, ok := expr.Exprs[0].(*sqlparser.AliasedExpr); ok {
								if colName, ok := aliased.Expr.(*sqlparser.ColName); ok {
									finalColName = fmt.Sprintf("%s(%s)", strings.ToUpper(expr.Name.String()), colName.Name.String())
								}
							}
						}
					case *sqlparser.Subquery:
						finalColName = sqlparser.String(expr)
					}
					if finalColName == "" {
						finalColName = sqlparser.String(c.Expr)
					}
				}
				resultRow[finalColName] = val
			}
		}
		results = append(results, resultRow)
	}
	return results, nil
}

func getTableNamesFromClause(from sqlparser.TableExprs) []string {
	var tableNames []string
	for _, tableExpr := range from {
		switch expr := tableExpr.(type) {
		case *sqlparser.AliasedTableExpr:
			if tableName, err := sql.ExtractTableName(expr); err == nil {
				if !expr.As.IsEmpty() {
					tableNames = append(tableNames, expr.As.String())
				} else {
					tableNames = append(tableNames, tableName)
				}
			}
		case *sqlparser.JoinTableExpr:
			// Recursively get table names from left and right expressions
			leftTables := getTableNamesFromClause(sqlparser.TableExprs{expr.LeftExpr})
			rightTables := getTableNamesFromClause(sqlparser.TableExprs{expr.RightExpr})
			tableNames = append(tableNames, leftTables...)
			tableNames = append(tableNames, rightTables...)
		}
	}
	return tableNames
}

func evaluateExpression(sh *shell.Shell, db *pb.Database, row CombinedRow, expr sqlparser.Expr, schemas map[string]*pb.Schema, fromTableNames []string) (bool, error) {
	if expr == nil {
		return true, nil
	}

	switch e := expr.(type) {
	case *sqlparser.ExistsExpr:
		// Handle EXISTS clause
		subqueryResults, err := executeCorrelatedSubquery(sh, db, e.Subquery.Select.(*sqlparser.Select), row, schemas)
		if err != nil {
			return false, fmt.Errorf("error executing EXISTS subquery: %w", err)
		}
		// EXISTS returns true if subquery returns at least one row
		return len(subqueryResults) > 0, nil

	case *sqlparser.AndExpr:
		left, err := evaluateExpression(sh, db, row, e.Left, schemas, fromTableNames)
		if err != nil {
			return false, err
		}
		right, err := evaluateExpression(sh, db, row, e.Right, schemas, fromTableNames)
		if err != nil {
			return false, err
		}
		return left && right, nil

	case *sqlparser.OrExpr:
		left, err := evaluateExpression(sh, db, row, e.Left, schemas, fromTableNames)
		if err != nil {
			return false, err
		}
		right, err := evaluateExpression(sh, db, row, e.Right, schemas, fromTableNames)
		if err != nil {
			return false, err
		}
		return left || right, nil

	case *sqlparser.ComparisonExpr:
		left, err := evaluateExpressionValue(sh, db, row, e.Left, schemas, fromTableNames)
		if err != nil {
			return false, err
		}

		if e.Operator == sqlparser.InStr {
			if subquery, ok := e.Right.(*sqlparser.Subquery); ok {
				subqueryResults, err := executeCorrelatedSubquery(sh, db, subquery.Select.(*sqlparser.Select), row, schemas)
				if err != nil {
					return false, fmt.Errorf("error executing IN subquery: %w", err)
				}

				for _, subRow := range subqueryResults {
					if len(subRow) != 1 {
						return false, fmt.Errorf("subquery in IN clause must select only one column")
					}
					for _, val := range subRow {
						if compareValues(left, val) {
							return true, nil
						}
					}
				}
				return false, nil
			}

			// Handle IN with ValTuple (list of values)
			if valTuple, ok := e.Right.(sqlparser.ValTuple); ok {
				for _, tupleExpr := range valTuple {
					right, err := evaluateExpressionValue(sh, db, row, tupleExpr, schemas, fromTableNames)
					if err != nil {
						return false, err
					}
					if compareValues(left, right) {
						return true, nil
					}
				}
				return false, nil
			}
		}

		if e.Operator == sqlparser.NotInStr {
			if subquery, ok := e.Right.(*sqlparser.Subquery); ok {
				subqueryResults, err := executeCorrelatedSubquery(sh, db, subquery.Select.(*sqlparser.Select), row, schemas)
				if err != nil {
					return false, fmt.Errorf("error executing NOT IN subquery: %w", err)
				}

				for _, subRow := range subqueryResults {
					if len(subRow) != 1 {
						return false, fmt.Errorf("subquery in NOT IN clause must select only one column")
					}
					for _, val := range subRow {
						if compareValues(left, val) {
							return false, nil
						}
					}
				}
				return true, nil
			}

			// Handle NOT IN with ValTuple
			if valTuple, ok := e.Right.(sqlparser.ValTuple); ok {
				for _, tupleExpr := range valTuple {
					right, err := evaluateExpressionValue(sh, db, row, tupleExpr, schemas, fromTableNames)
					if err != nil {
						return false, err
					}
					if compareValues(left, right) {
						return false, nil
					}
				}
				return true, nil
			}
		}

		// Handle LIKE operator
		if e.Operator == sqlparser.LikeStr {
			right, err := evaluateExpressionValue(sh, db, row, e.Right, schemas, fromTableNames)
			if err != nil {
				return false, err
			}
			return evaluateLike(left, right)
		}

		// Handle NOT LIKE operator
		if e.Operator == sqlparser.NotLikeStr {
			right, err := evaluateExpressionValue(sh, db, row, e.Right, schemas, fromTableNames)
			if err != nil {
				return false, err
			}
			result, err := evaluateLike(left, right)
			if err != nil {
				return false, err
			}
			return !result, nil
		}

		right, err := evaluateExpressionValue(sh, db, row, e.Right, schemas, fromTableNames)
		if err != nil {
			return false, err
		}

		return compareValuesWithOperator(left, right, e.Operator)

	default:
		return false, fmt.Errorf("unsupported expression type: %T", e)
	}
}

// Helper function to compare two values for equality
func compareValues(left, right interface{}) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}

	// Try numeric comparison
	leftNum, leftIsNum := getNumericValue(left)
	rightNum, rightIsNum := getNumericValue(right)
	if leftIsNum && rightIsNum {
		return leftNum == rightNum
	}

	// Try string comparison
	leftStr, leftIsStr := toString(left)
	rightStr, rightIsStr := toString(right)
	if leftIsStr && rightIsStr {
		return leftStr == rightStr
	}

	// Direct comparison
	return left == right
}

// Helper function to compare values with an operator
func compareValuesWithOperator(left, right interface{}, operator string) (bool, error) {
	if left == nil || right == nil {
		switch operator {
		case sqlparser.EqualStr:
			return left == right, nil
		case sqlparser.NotEqualStr:
			return left != right, nil
		default:
			return false, nil
		}
	}

	// Handle time.Time comparisons
	leftTime, leftIsTime := left.(time.Time)
	rightTime, rightIsTime := right.(time.Time)

	if leftIsTime || rightIsTime {
		// Convert both to time.Time
		if !leftIsTime {
			if leftStr, ok := toString(left); ok {
				parsedTime, err := parseDateTime(leftStr)
				if err == nil {
					leftTime = parsedTime
					leftIsTime = true
				}
			}
		}
		if !rightIsTime {
			if rightStr, ok := toString(right); ok {
				parsedTime, err := parseDateTime(rightStr)
				if err == nil {
					rightTime = parsedTime
					rightIsTime = true
				}
			}
		}

		if leftIsTime && rightIsTime {
			switch operator {
			case sqlparser.EqualStr:
				return leftTime.Equal(rightTime), nil
			case sqlparser.NotEqualStr:
				return !leftTime.Equal(rightTime), nil
			case sqlparser.GreaterThanStr:
				return leftTime.After(rightTime), nil
			case sqlparser.LessThanStr:
				return leftTime.Before(rightTime), nil
			case sqlparser.GreaterEqualStr:
				return leftTime.Equal(rightTime) || leftTime.After(rightTime), nil
			case sqlparser.LessEqualStr:
				return leftTime.Equal(rightTime) || leftTime.Before(rightTime), nil
			}
		}
	}

	leftNum, leftIsNum := getNumericValue(left)
	rightNum, rightIsNum := getNumericValue(right)

	if leftIsNum && rightIsNum {
		switch operator {
		case sqlparser.EqualStr:
			return leftNum == rightNum, nil
		case sqlparser.NotEqualStr:
			return leftNum != rightNum, nil
		case sqlparser.GreaterThanStr:
			return leftNum > rightNum, nil
		case sqlparser.LessThanStr:
			return leftNum < rightNum, nil
		case sqlparser.GreaterEqualStr:
			return leftNum >= rightNum, nil
		case sqlparser.LessEqualStr:
			return leftNum <= rightNum, nil
		}
	}

	leftBool, leftIsBool := left.(bool)
	rightBool, rightIsBool := right.(bool)

	if leftIsBool && rightIsBool {
		switch operator {
		case sqlparser.EqualStr:
			return leftBool == rightBool, nil
		case sqlparser.NotEqualStr:
			return leftBool != rightBool, nil
		default:
			return false, fmt.Errorf("unsupported operator '%s' for boolean comparison", operator)
		}
	}

	leftStr, leftIsStr := toString(left)
	rightStr, rightIsStr := toString(right)

	if leftIsStr && rightIsStr {
		switch operator {
		case sqlparser.EqualStr:
			return leftStr == rightStr, nil
		case sqlparser.NotEqualStr:
			return leftStr != rightStr, nil
		case sqlparser.GreaterThanStr:
			return leftStr > rightStr, nil
		case sqlparser.LessThanStr:
			return leftStr < rightStr, nil
		case sqlparser.GreaterEqualStr:
			return leftStr >= rightStr, nil
		case sqlparser.LessEqualStr:
			return leftStr <= rightStr, nil
		default:
			return false, fmt.Errorf("unsupported comparison operator: %s", operator)
		}
	}

	return false, fmt.Errorf("cannot compare types %T and %T", left, right)
}

// Helper function to evaluate LIKE pattern matching
func evaluateLike(value, pattern interface{}) (bool, error) {
	valueStr, ok := toString(value)
	if !ok {
		return false, fmt.Errorf("LIKE left operand must be a string, got %T", value)
	}

	patternStr, ok := toString(pattern)
	if !ok {
		return false, fmt.Errorf("LIKE pattern must be a string, got %T", pattern)
	}

	// Convert SQL LIKE pattern to regex
	// % matches any sequence of characters
	// _ matches any single character
	regexPattern := "^"
	for i := 0; i < len(patternStr); i++ {
		ch := patternStr[i]
		switch ch {
		case '%':
			regexPattern += ".*"
		case '_':
			regexPattern += "."
		case '.', '*', '+', '?', '(', ')', '[', ']', '{', '}', '|', '^', '$', '\\':
			// Escape regex special characters
			regexPattern += "\\" + string(ch)
		default:
			regexPattern += string(ch)
		}
	}
	regexPattern += "$"

	// Use simple pattern matching instead of regex for basic cases
	return matchLikePattern(valueStr, patternStr), nil
}

// Simple LIKE pattern matching without regex
func matchLikePattern(value, pattern string) bool {
	return matchLikeHelper(value, pattern, 0, 0)
}

func matchLikeHelper(value, pattern string, vIdx, pIdx int) bool {
	for pIdx < len(pattern) {
		if pIdx < len(pattern) && pattern[pIdx] == '%' {
			// % matches zero or more characters
			pIdx++
			if pIdx == len(pattern) {
				return true // % at end matches everything
			}
			// Try matching the rest of the pattern at every position
			for i := vIdx; i <= len(value); i++ {
				if matchLikeHelper(value, pattern, i, pIdx) {
					return true
				}
			}
			return false
		} else if pIdx < len(pattern) && pattern[pIdx] == '_' {
			// _ matches exactly one character
			if vIdx >= len(value) {
				return false
			}
			vIdx++
			pIdx++
		} else {
			// Regular character must match
			if vIdx >= len(value) || value[vIdx] != pattern[pIdx] {
				return false
			}
			vIdx++
			pIdx++
		}
	}
	return vIdx == len(value)
}

func getNumericValue(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int64:
		return float64(val), true
	case int:
		return float64(val), true
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}

// Update evaluateExpressionValue to handle ValTuple
func evaluateExpressionValue(sh *shell.Shell, db *pb.Database, row CombinedRow, expr sqlparser.Expr, schemas map[string]*pb.Schema, fromTableNames []string) (interface{}, error) {
	switch e := expr.(type) {
	case sqlparser.ValTuple:
		// Return the tuple as a slice for processing
		values := make([]interface{}, len(e))
		for i, tupleExpr := range e {
			val, err := evaluateExpressionValue(sh, db, row, tupleExpr, schemas, fromTableNames)
			if err != nil {
				return nil, err
			}
			values[i] = val
		}
		return values, nil

	case *sqlparser.Subquery:
		// Execute a scalar subquery
		subqueryResults, err := executeCorrelatedSubquery(sh, db, e.Select.(*sqlparser.Select), row, schemas)
		if err != nil {
			return nil, fmt.Errorf("error executing scalar subquery: %w", err)
		}
		if len(subqueryResults) > 1 {
			return nil, fmt.Errorf("subquery returned more than one row")
		}
		if len(subqueryResults) == 0 {
			return nil, nil // Subquery returned no rows, treat as NULL
		}
		firstRow := subqueryResults[0]
		if len(firstRow) > 1 {
			return nil, fmt.Errorf("subquery returned more than one column")
		}
		for _, val := range firstRow {
			return val, nil
		}
		return nil, nil

	case *sqlparser.ColName:
		// Try to resolve as an identifier first
		val, err := evaluateIdentifier(row, e, schemas, fromTableNames)
		if err != nil && strings.Contains(err.Error(), "column not found") {
			// Check if it's an alias from a derived table
			for _, tableData := range row {
				if tableData == nil {
					continue
				}
				if valAny, ok := tableData.Values[e.Name.String()]; ok {
					return utils.FromAny(valAny)
				}
			}
		}
		return val, err

	case *sqlparser.SQLVal:
		switch e.Type {
		case sqlparser.StrVal:
			return string(e.Val), nil
		case sqlparser.IntVal:
			i, err := strconv.ParseInt(string(e.Val), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid integer value: %s", string(e.Val))
			}
			return i, nil
		case sqlparser.FloatVal:
			f, err := strconv.ParseFloat(string(e.Val), 64)
			if err != nil {
				return nil, fmt.Errorf("invalid float value: %s", string(e.Val))
			}
			return f, nil
		default:
			return nil, fmt.Errorf("unsupported SQLVal type: %v", e.Type)
		}

	case *sqlparser.UnaryExpr:
		right, err := evaluateExpressionValue(sh, db, row, e.Expr, schemas, fromTableNames)
		if err != nil {
			return nil, err
		}
		if e.Operator == "-" {
			if num, ok := getNumericValue(right); ok {
				return -num, nil
			}
			return nil, fmt.Errorf("unary minus operator can only be applied to numbers")
		}
		return nil, fmt.Errorf("unsupported prefix operator: %s", e.Operator)

	case *sqlparser.BinaryExpr:
		left, err := evaluateExpressionValue(sh, db, row, e.Left, schemas, fromTableNames)
		if err != nil {
			return nil, err
		}
		right, err := evaluateExpressionValue(sh, db, row, e.Right, schemas, fromTableNames)
		if err != nil {
			return nil, err
		}

		leftNum, leftIsNum := getNumericValue(left)
		rightNum, rightIsNum := getNumericValue(right)

		if !leftIsNum || !rightIsNum {
			return nil, fmt.Errorf("arithmetic operations can only be performed on numbers, got %T and %T", left, right)
		}

		switch e.Operator {
		case "+":
			return leftNum + rightNum, nil
		case "-":
			return leftNum - rightNum, nil
		case "*":
			return leftNum * rightNum, nil
		case "/":
			if rightNum == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			return leftNum / rightNum, nil
		default:
			return nil, fmt.Errorf("unsupported binary operator: %s", e.Operator)
		}

	case *sqlparser.FuncExpr:
		return nil, fmt.Errorf("aggregate functions like %s must be used in the SELECT clause with GROUP BY", strings.ToUpper(e.Name.String()))

	default:
		return nil, fmt.Errorf("unsupported expression value type: %T", e)
	}
}

func executeCorrelatedSubquery(sh *shell.Shell, db *pb.Database, selectStmt *sqlparser.Select, outerRow CombinedRow, outerSchemas map[string]*pb.Schema) ([]map[string]interface{}, error) {
	// Create schemas for the subquery's own tables
	subquerySchemas := make(map[string]*pb.Schema)
	aliases := make(map[string]string)
	for _, tableExpr := range selectStmt.From {
		if err := processTableExprForSchemas(sh, db, tableExpr, subquerySchemas, aliases); err != nil {
			return nil, err
		}
	}

	// Collect outer table references used in the subquery
	referencedOuterTables := make(map[string]bool)
	collectOuterReferences(selectStmt, referencedOuterTables)

	// Create a filtered schema map that only includes outer tables actually referenced
	filteredOuterSchemas := make(map[string]*pb.Schema)
	for tableName := range referencedOuterTables {
		if schema, ok := outerSchemas[tableName]; ok {
			filteredOuterSchemas[tableName] = schema
		}
	}

	// Merge subquery schemas with filtered outer schemas
	effectiveSchemas := make(map[string]*pb.Schema)
	for tableName, schema := range subquerySchemas {
		effectiveSchemas[tableName] = schema
	}
	for tableName, schema := range filteredOuterSchemas {
		effectiveSchemas[tableName] = schema
	}

	// Extract table names from the FROM clause
	fromTableNames := getTableNamesFromClause(selectStmt.From)

	// Execute the FROM clause
	combinedRows, err := executeFromClause(sh, db, selectStmt.From, subquerySchemas)
	if err != nil {
		return nil, fmt.Errorf("error executing FROM/JOIN clause in subquery: %w", err)
	}

	// Filter rows using the WHERE clause, incorporating outer row data
	var filteredRows []CombinedRow
	if selectStmt.Where == nil {
		filteredRows = combinedRows
	} else {
		for _, row := range combinedRows {
			mergedRow := make(CombinedRow)
			for k, v := range row {
				mergedRow[k] = v
			}
			for k, v := range outerRow {
				mergedRow[k] = v
			}

			include, err := evaluateExpression(sh, db, mergedRow, selectStmt.Where.Expr, effectiveSchemas, fromTableNames)
			if err != nil {
				return nil, err
			}
			if include {
				filteredRows = append(filteredRows, mergedRow)
			}
		}
	}

	// Handle GROUP BY and aggregates
	var results []map[string]interface{}
	if selectStmt.GroupBy != nil || isAggregateQuery(selectStmt) {
		results, err = executeAggregateQuery(sh, db, filteredRows, selectStmt, effectiveSchemas)
		if err != nil {
			return nil, fmt.Errorf("error executing aggregate query in subquery: %w", err)
		}
	} else {
		// Project columns for non-aggregate queries
		results, err = projectColumns(sh, db, selectStmt.From, filteredRows, selectStmt.SelectExprs, effectiveSchemas)
		if err != nil {
			return nil, fmt.Errorf("error projecting columns in subquery: %w", err)
		}
	}

	// Handle ORDER BY
	if selectStmt.OrderBy != nil {
		applyOrderByToMap(results, selectStmt.OrderBy, effectiveSchemas)
	}

	// Apply LIMIT and OFFSET
	if selectStmt.Limit != nil {
		results = applyLimitAndOffsetToMap(sh, db, results, selectStmt.Limit)
	}

	return results, nil
}

func collectOuterReferences(selectStmt *sqlparser.Select, referencedTables map[string]bool) {
	var walkExpr func(expr sqlparser.Expr)
	walkExpr = func(expr sqlparser.Expr) {
		if expr == nil {
			return
		}
		switch e := expr.(type) {
		case *sqlparser.ColName:
			if !e.Qualifier.IsEmpty() {
				referencedTables[e.Qualifier.Name.String()] = true
			}
		case *sqlparser.Subquery:
			collectOuterReferences(e.Select.(*sqlparser.Select), referencedTables)
		case *sqlparser.AndExpr:
			walkExpr(e.Left)
			walkExpr(e.Right)
		case *sqlparser.OrExpr:
			walkExpr(e.Left)
			walkExpr(e.Right)
		case *sqlparser.ComparisonExpr:
			walkExpr(e.Left)
			walkExpr(e.Right)
		case *sqlparser.BinaryExpr:
			walkExpr(e.Left)
			walkExpr(e.Right)
		case *sqlparser.UnaryExpr:
			walkExpr(e.Expr)
		case *sqlparser.FuncExpr:
			for _, expr := range e.Exprs {
				if aliased, ok := expr.(*sqlparser.AliasedExpr); ok {
					walkExpr(aliased.Expr)
				}
			}
		}
	}

	for _, expr := range selectStmt.SelectExprs {
		if aliasedExpr, ok := expr.(*sqlparser.AliasedExpr); ok {
			walkExpr(aliasedExpr.Expr)
		}
	}

	if selectStmt.Where != nil {
		walkExpr(selectStmt.Where.Expr)
	}

	var walkTableExpr func(tableExpr sqlparser.TableExpr)
	walkTableExpr = func(tableExpr sqlparser.TableExpr) {
		switch expr := tableExpr.(type) {
		case *sqlparser.AliasedTableExpr:
			if subquery, ok := expr.Expr.(*sqlparser.Subquery); ok {
				collectOuterReferences(subquery.Select.(*sqlparser.Select), referencedTables)
			}
		case *sqlparser.JoinTableExpr:
			if expr.On != nil {
				walkExpr(expr.On)
			}
			walkTableExpr(expr.LeftExpr)
			walkTableExpr(expr.RightExpr)
		}
	}
	for _, tableExpr := range selectStmt.From {
		walkTableExpr(tableExpr)
	}

	for _, expr := range selectStmt.GroupBy {
		walkExpr(expr)
	}
	for _, order := range selectStmt.OrderBy {
		walkExpr(order.Expr)
	}
}

func evaluateIdentifier(row CombinedRow, ident *sqlparser.ColName, schemas map[string]*pb.Schema, fromTableNames []string) (interface{}, error) {
	if !ident.Qualifier.IsEmpty() {
		originalTableName := ident.Qualifier.Name.String()
		tableSchema, ok := schemas[originalTableName]
		if !ok {
			return nil, fmt.Errorf("table %s not found in FROM clause", originalTableName)
		}

		foundInSchema := false
		for _, col := range tableSchema.Columns {
			if col.Name == ident.Name.String() {
				foundInSchema = true
				break
			}
		}
		if !foundInSchema {
			// Check if the column exists in the row data (for derived tables)
			if tableData, ok := row[originalTableName]; ok && tableData != nil {
				if valAny, ok := tableData.Values[ident.Name.String()]; ok {
					return utils.FromAny(valAny)
				}
			}
			return nil, fmt.Errorf("column %s not found in table %s", ident.Name.String(), originalTableName)
		}

		tableData, ok := row[originalTableName]
		if !ok {
			for rowTableName, rowData := range row {
				if rowSchema, exists := schemas[rowTableName]; exists && rowSchema == tableSchema {
					tableData = rowData
					ok = true
					break
				}
			}
		}

		if !ok {
			return nil, fmt.Errorf("table %s not found in FROM clause data", originalTableName)
		}
		if tableData == nil {
			return nil, nil
		}

		valAny, ok := tableData.Values[ident.Name.String()]
		if !ok {
			return nil, nil
		}
		return utils.FromAny(valAny)
	}

	var foundValue interface{}
	var foundInTable string
	var valueFound bool
	var possibleTables []string

	// ONLY check tables from the FROM clause to avoid ambiguity with outer query tables
	for _, tName := range fromTableNames {
		if tableSchema, ok := schemas[tName]; ok {
			for _, col := range tableSchema.Columns {
				if col.Name == ident.Name.String() {
					if foundInTable != "" {
						possibleTables = append(possibleTables, tName)
					} else {
						foundInTable = tName
						if tableData, ok := row[tName]; ok && tableData != nil {
							if valAny, dataOk := tableData.Values[ident.Name.String()]; dataOk {
								val, err := utils.FromAny(valAny)
								if err != nil {
									return nil, err
								}
								foundValue = val
								valueFound = true
							}
						}
					}
				}
			}
		}
	}

	// Check row data for derived table columns (only for tables in FROM clause)
	for _, tName := range fromTableNames {
		tableData, ok := row[tName]
		if !ok || tableData == nil {
			continue
		}
		if valAny, ok := tableData.Values[ident.Name.String()]; ok {
			if foundInTable != "" && foundInTable != tName {
				possibleTables = append(possibleTables, tName)
			} else if foundInTable == "" {
				foundInTable = tName
				val, err := utils.FromAny(valAny)
				if err != nil {
					return nil, err
				}
				foundValue = val
				valueFound = true
			}
		}
	}

	if len(possibleTables) > 0 {
		return nil, fmt.Errorf("ambiguous column name: %s exists in multiple tables: %s", ident.Name.String(), strings.Join(append([]string{foundInTable}, possibleTables...), ", "))
	}

	if foundInTable == "" {
		return nil, fmt.Errorf("column %s not found in any table in FROM clause", ident.Name.String())
	}

	if !valueFound {
		return nil, nil
	}

	return foundValue, nil
}

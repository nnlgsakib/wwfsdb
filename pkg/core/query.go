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

	// Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return nil, err
	}

	stmt, err := sqlparser.Parse(queryString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	switch s := stmt.(type) {
	case *sqlparser.Select:
		return QueryDB(sh, db, s)
	case *sqlparser.Union:
		return executeUnion(sh, db, s)
	default:
		return nil, fmt.Errorf("unsupported query type: %T", s)
	}
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
		results, err := executeAggregateQuery(sh, db, filteredRows, s, schemas, fromTableNames)
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
		applyOrderBy(sh, db, filteredRows, s.OrderBy, schemas, fromTableNames)
	}

	// Handle OFFSET and LIMIT
	finalRows := applyLimitAndOffset(sh, db, filteredRows, s.Limit)

	// Project the final columns
	results, err := projectColumns(sh, db, s.From, finalRows, s.SelectExprs, schemas, fromTableNames)
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
			if _, ok := aliasedExpr.Expr.(*sqlparser.Subquery); ok {
				return true
			}
		}
	}
	return false
}

func applyOrderBy(sh *shell.Shell, db *pb.Database, rows []CombinedRow, orderBy sqlparser.OrderBy, schemas map[string]*pb.Schema, fromTableNames []string) {
	sort.SliceStable(rows, func(i, j int) bool {
		for _, order := range orderBy {
			valI, _ := evaluateExpressionValue(sh, db, rows[i], order.Expr, schemas, fromTableNames)
			valJ, _ := evaluateExpressionValue(sh, db, rows[j], order.Expr, schemas, fromTableNames)

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

func executeAggregateQuery(sh *shell.Shell, db *pb.Database, rows []CombinedRow, s *sqlparser.Select, schemas map[string]*pb.Schema, fromTableNames []string) ([]map[string]interface{}, error) {
	// Handle non-grouped aggregate query
	if s.GroupBy == nil {
		resultRow := make(map[string]interface{})
		for _, colExpr := range s.SelectExprs {
			aliasedExpr, ok := colExpr.(*sqlparser.AliasedExpr)
			if !ok {
				return nil, fmt.Errorf("unsupported select expression type in aggregate query: %T", colExpr)
			}

			// Handle CASE expressions
			if caseExpr, ok := aliasedExpr.Expr.(*sqlparser.CaseExpr); ok {
				resultColName := aliasedExpr.As.String()
				if resultColName == "" {
					resultColName = sqlparser.String(aliasedExpr.Expr)
				}
				val, err := evaluateCaseExpression(sh, db, rows, caseExpr, schemas, fromTableNames, true)
				if err != nil {
					return nil, fmt.Errorf("error evaluating CASE expression: %w", err)
				}
				resultRow[resultColName] = val
				continue
			}

			// Handle arithmetic expressions in SELECT
			if binExpr, ok := aliasedExpr.Expr.(*sqlparser.BinaryExpr); ok {
				resultColName := aliasedExpr.As.String()
				if resultColName == "" {
					resultColName = sqlparser.String(aliasedExpr.Expr)
				}
				val, err := evaluateAggregateArithmetic(sh, db, rows, binExpr, schemas, fromTableNames)
				if err != nil {
					return nil, fmt.Errorf("error evaluating arithmetic expression: %w", err)
				}
				resultRow[resultColName] = val
				continue
			}

			aggExpr, ok := aliasedExpr.Expr.(*sqlparser.FuncExpr)
			if !ok {
				// Handle subqueries in SELECT clause
				if subquery, ok := aliasedExpr.Expr.(*sqlparser.Subquery); ok {
					resultColName := aliasedExpr.As.String()
					if resultColName == "" {
						resultColName = sqlparser.String(aliasedExpr.Expr)
					}
					subqueryResults, err := executeCorrelatedSubquery(sh, db, subquery.Select.(*sqlparser.Select), nil, schemas)
					if err != nil {
						return nil, fmt.Errorf("error executing subquery for column %s: %w", resultColName, err)
					}
					if len(subqueryResults) > 1 {
						return nil, fmt.Errorf("subquery for column %s returned more than one row", resultColName)
					}
					if len(subqueryResults) == 0 {
						resultRow[resultColName] = nil
						continue
					}
					if len(subqueryResults[0]) != 1 {
						return nil, fmt.Errorf("subquery for column %s must return exactly one column", resultColName)
					}
					for _, val := range subqueryResults[0] {
						resultRow[resultColName] = val
					}
					continue
				}
				// For non-aggregate expressions in derived tables
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
			isDistinct := aggExpr.Distinct

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
				distinctStr := ""
				if isDistinct {
					distinctStr = "DISTINCT "
				}
				resultColName = fmt.Sprintf("%s(%s%s)", strings.ToUpper(aggExpr.Name.String()), distinctStr, argName)
			}

			switch strings.ToUpper(aggExpr.Name.String()) {
			case "COUNT":
				if isDistinct && argName != "*" {
					distinctValues := make(map[string]bool)
					for _, row := range rows {
						var val interface{}
						var err error
						found := false
						for tableName, tableData := range row {
							if tableData == nil {
								continue
							}
							if valAny, ok := tableData.Values[argName]; ok {
								if valAny == nil {
									continue
								}
								val, err = utils.FromAny(valAny)
								if err != nil {
									return nil, fmt.Errorf("error converting value for column %s in table %s: %w", argName, tableName, err)
								}
								found = true
								break
							}
						}
						if !found {
							val, err = evaluateExpressionValue(sh, db, row, &sqlparser.ColName{Name: sqlparser.NewColIdent(argName)}, schemas, fromTableNames)
							if err != nil {
								continue
							}
						}
						if val != nil {
							distinctValues[fmt.Sprintf("%v", val)] = true
						}
					}
					resultRow[resultColName] = len(distinctValues)
				} else {
					resultRow[resultColName] = len(rows)
				}
			case "SUM", "AVG", "MIN", "MAX":
				if argName == "*" {
					return nil, fmt.Errorf("%s requires a column argument", strings.ToUpper(aggExpr.Name.String()))
				}
				var total float64
				var min float64
				var max float64
				count := 0
				distinctValues := make(map[string]float64)

				for i, row := range rows {
					var val interface{}
					var err error
					found := false
					for tableName, tableData := range row {
						if tableData == nil {
							continue
						}
						if valAny, ok := tableData.Values[argName]; ok {
							if valAny == nil {
								continue
							}
							val, err = utils.FromAny(valAny)
							if err != nil {
								return nil, fmt.Errorf("error converting value for column %s in table %s: %w", argName, tableName, err)
							}
							found = true
							break
						}
					}
					if !found {
						val, err = evaluateExpressionValue(sh, db, row, &sqlparser.ColName{Name: sqlparser.NewColIdent(argName)}, schemas, fromTableNames)
						if err != nil {
							continue
						}
					}
					if val == nil {
						continue
					}
					num, isNum := getNumericValue(val)
					if !isNum {
						continue
					}

					if isDistinct {
						valKey := fmt.Sprintf("%v", num)
						if _, exists := distinctValues[valKey]; exists {
							continue
						}
						distinctValues[valKey] = num
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
					if count == 0 {
						resultRow[resultColName] = nil
					} else {
						resultRow[resultColName] = total
					}
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

		// Apply HAVING clause if present
		if s.Having != nil {
			include, err := evaluateHavingClause(sh, db, resultRow, rows, s.Having.Expr, schemas, fromTableNames)
			if err != nil {
				return nil, fmt.Errorf("error evaluating HAVING clause: %w", err)
			}
			if !include {
				return []map[string]interface{}{}, nil
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
			val, err := evaluateExpressionValue(sh, db, row, expr, schemas, fromTableNames)
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
						distinctStr := ""
						if funcExpr.Distinct {
							distinctStr = "DISTINCT "
						}
						if _, ok := funcExpr.Exprs[0].(*sqlparser.StarExpr); ok {
							resultColName = fmt.Sprintf("%s(%s*)", strings.ToUpper(funcExpr.Name.String()), distinctStr)
						} else if aliased, ok := funcExpr.Exprs[0].(*sqlparser.AliasedExpr); ok {
							if colName, ok := aliased.Expr.(*sqlparser.ColName); ok {
								resultColName = fmt.Sprintf("%s(%s%s)", strings.ToUpper(funcExpr.Name.String()), distinctStr, colName.Name.String())
							}
						}
					}
				} else if _, ok := aliasedExpr.Expr.(*sqlparser.Subquery); ok {
					resultColName = sqlparser.String(aliasedExpr.Expr)
				}
				if resultColName == "" {
					resultColName = sqlparser.String(aliasedExpr.Expr)
				}
			}

			// Handle CASE expressions
			if caseExpr, ok := aliasedExpr.Expr.(*sqlparser.CaseExpr); ok {
				val, err := evaluateCaseExpression(sh, db, group.Rows, caseExpr, schemas, fromTableNames, true)
				if err != nil {
					return nil, fmt.Errorf("error evaluating CASE expression: %w", err)
				}
				resultRow[resultColName] = val
				continue
			}

			// Handle arithmetic expressions
			if binExpr, ok := aliasedExpr.Expr.(*sqlparser.BinaryExpr); ok {
				val, err := evaluateAggregateArithmetic(sh, db, group.Rows, binExpr, schemas, fromTableNames)
				if err != nil {
					return nil, fmt.Errorf("error evaluating arithmetic expression: %w", err)
				}
				resultRow[resultColName] = val
				continue
			}

			if aggExpr, ok := aliasedExpr.Expr.(*sqlparser.FuncExpr); ok {
				var argName string
				isDistinct := aggExpr.Distinct

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
					if isDistinct && argName != "*" {
						distinctValues := make(map[string]bool)
						for _, row := range group.Rows {
							var val interface{}
							var err error
							found := false
							for tableName, tableData := range row {
								if tableData == nil {
									continue
								}
								if valAny, ok := tableData.Values[argName]; ok {
									if valAny == nil {
										continue
									}
									val, err = utils.FromAny(valAny)
									if err != nil {
										return nil, fmt.Errorf("error converting value for column %s in table %s: %w", argName, tableName, err)
									}
									found = true
									break
								}
							}
							if !found {
								val, err = evaluateExpressionValue(sh, db, row, &sqlparser.ColName{Name: sqlparser.NewColIdent(argName)}, schemas, fromTableNames)
								if err != nil {
									continue
								}
							}
							if val != nil {
								distinctValues[fmt.Sprintf("%v", val)] = true
							}
						}
						resultRow[resultColName] = len(distinctValues)
					} else {
						resultRow[resultColName] = len(group.Rows)
					}
				case "SUM", "AVG", "MIN", "MAX":
					if argName == "*" {
						return nil, fmt.Errorf("%s requires a column argument", strings.ToUpper(aggExpr.Name.String()))
					}
					var total float64
					var min float64
					var max float64
					count := 0
					distinctValues := make(map[string]float64)

					for i, row := range group.Rows {
						var val interface{}
						var err error
						found := false
						for tableName, tableData := range row {
							if tableData == nil {
								continue
							}
							if valAny, ok := tableData.Values[argName]; ok {
								if valAny == nil {
									continue
								}
								val, err = utils.FromAny(valAny)
								if err != nil {
									return nil, fmt.Errorf("error converting value for column %s in table %s: %w", argName, tableName, err)
								}
								found = true
								break
							}
						}
						if !found {
							val, err = evaluateExpressionValue(sh, db, row, &sqlparser.ColName{Name: sqlparser.NewColIdent(argName)}, schemas, fromTableNames)
							if err != nil {
								continue
							}
						}
						if val == nil {
							continue
						}
						num, isNum := getNumericValue(val)
						if !isNum {
							continue
						}

						if isDistinct {
							valKey := fmt.Sprintf("%v", num)
							if _, exists := distinctValues[valKey]; exists {
								continue
							}
							distinctValues[valKey] = num
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
						if count == 0 {
							resultRow[resultColName] = nil
						} else {
							resultRow[resultColName] = total
						}
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
			} else if subquery, ok := aliasedExpr.Expr.(*sqlparser.Subquery); ok {
				subqueryResults, err := executeCorrelatedSubquery(sh, db, subquery.Select.(*sqlparser.Select), group.Rows[0], schemas)
				if err != nil {
					return nil, fmt.Errorf("error executing subquery for column %s: %w", resultColName, err)
				}
				if len(subqueryResults) > 1 {
					return nil, fmt.Errorf("subquery for column %s returned more than one row", resultColName)
				}
				if len(subqueryResults) == 0 {
					resultRow[resultColName] = nil
					continue
				}
				if len(subqueryResults[0]) != 1 {
					return nil, fmt.Errorf("subquery for column %s must return exactly one column", resultColName)
				}
				for _, val := range subqueryResults[0] {
					resultRow[resultColName] = val
				}
			} else {
				if len(group.Rows) == 0 {
					resultRow[resultColName] = nil
				} else {
					if colName, ok := aliasedExpr.Expr.(*sqlparser.ColName); ok {
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
							val, err := evaluateExpressionValue(sh, db, group.Rows[0], aliasedExpr.Expr, schemas, fromTableNames)
							if err != nil {
								return nil, err
							}
							resultRow[resultColName] = val
						}
					} else {
						val, err := evaluateExpressionValue(sh, db, group.Rows[0], aliasedExpr.Expr, schemas, fromTableNames)
						if err != nil {
							return nil, err
						}
						resultRow[resultColName] = val
					}
				}
			}
		}

		// Apply HAVING clause if present
		if s.Having != nil {
			include, err := evaluateHavingClause(sh, db, resultRow, group.Rows, s.Having.Expr, schemas, fromTableNames)
			if err != nil {
				return nil, fmt.Errorf("error evaluating HAVING clause: %w", err)
			}
			if !include {
				continue
			}
		}

		finalResults = append(finalResults, resultRow)
	}

	return finalResults, nil
}

// New function to evaluate HAVING clause using computed aggregate values
func evaluateHavingClause(sh *shell.Shell, db *pb.Database, resultRow map[string]interface{}, rows []CombinedRow, expr sqlparser.Expr, schemas map[string]*pb.Schema, fromTableNames []string) (bool, error) {
	if expr == nil {
		return true, nil
	}

	switch e := expr.(type) {
	case *sqlparser.AndExpr:
		left, err := evaluateHavingClause(sh, db, resultRow, rows, e.Left, schemas, fromTableNames)
		if err != nil {
			return false, err
		}
		if !left {
			return false, nil
		}
		return evaluateHavingClause(sh, db, resultRow, rows, e.Right, schemas, fromTableNames)

	case *sqlparser.OrExpr:
		left, err := evaluateHavingClause(sh, db, resultRow, rows, e.Left, schemas, fromTableNames)
		if err != nil {
			return false, err
		}
		if left {
			return true, nil
		}
		return evaluateHavingClause(sh, db, resultRow, rows, e.Right, schemas, fromTableNames)

	case *sqlparser.ComparisonExpr:
		left, err := evaluateHavingExpression(sh, db, resultRow, rows, e.Left, schemas, fromTableNames)
		if err != nil {
			return false, fmt.Errorf("error evaluating left operand: %w", err)
		}

		right, err := evaluateHavingExpression(sh, db, resultRow, rows, e.Right, schemas, fromTableNames)
		if err != nil {
			return false, fmt.Errorf("error evaluating right operand: %w", err)
		}

		return compareValuesWithOperator(left, right, e.Operator)
	}

	return false, fmt.Errorf("unsupported HAVING expression type: %T", expr)
}

// Helper function to evaluate expressions in HAVING clause
func evaluateHavingExpression(sh *shell.Shell, db *pb.Database, resultRow map[string]interface{}, rows []CombinedRow, expr sqlparser.Expr, schemas map[string]*pb.Schema, fromTableNames []string) (interface{}, error) {
	switch e := expr.(type) {
	case *sqlparser.FuncExpr:
		// Compute the aggregate on the fly for HAVING clause
		return evaluateAggregateInHaving(sh, db, rows, e, schemas, fromTableNames)

	case *sqlparser.Subquery:
		// Execute subquery
		subqueryResults, err := executeCorrelatedSubquery(sh, db, e.Select.(*sqlparser.Select), nil, schemas)
		if err != nil {
			return nil, fmt.Errorf("error executing subquery: %w", err)
		}
		if len(subqueryResults) > 1 {
			return nil, fmt.Errorf("subquery returned more than one row")
		}
		if len(subqueryResults) == 0 {
			return nil, nil
		}
		if len(subqueryResults[0]) != 1 {
			return nil, fmt.Errorf("subquery must return exactly one column")
		}
		for _, val := range subqueryResults[0] {
			return val, nil
		}
		return nil, nil

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

	case *sqlparser.ColName:
		// Try to get from resultRow first (for GROUP BY columns)
		if val, ok := resultRow[e.Name.String()]; ok {
			return val, nil
		}
		return nil, fmt.Errorf("column %s not found", e.Name.String())

	default:
		return nil, fmt.Errorf("unsupported HAVING expression: %T", e)
	}
}

// Helper to evaluate aggregate functions within HAVING clause
func evaluateAggregateInHaving(sh *shell.Shell, db *pb.Database, rows []CombinedRow, funcExpr *sqlparser.FuncExpr, schemas map[string]*pb.Schema, fromTableNames []string) (interface{}, error) {
	var argName string
	isDistinct := funcExpr.Distinct

	if len(funcExpr.Exprs) > 0 {
		if _, ok := funcExpr.Exprs[0].(*sqlparser.StarExpr); ok {
			argName = "*"
		} else if aliased, ok := funcExpr.Exprs[0].(*sqlparser.AliasedExpr); ok {
			if colName, ok := aliased.Expr.(*sqlparser.ColName); ok {
				argName = colName.Name.String()
			}
		}
	}

	switch strings.ToUpper(funcExpr.Name.String()) {
	case "COUNT":
		if isDistinct && argName != "*" {
			distinctValues := make(map[string]bool)
			for _, row := range rows {
				var val interface{}
				var err error
				found := false
				for _, tableData := range row {
					if tableData == nil {
						continue
					}
					if valAny, ok := tableData.Values[argName]; ok {
						if valAny == nil {
							continue
						}
						val, err = utils.FromAny(valAny)
						if err != nil {
							continue
						}
						found = true
						break
					}
				}
				if !found {
					val, err = evaluateExpressionValue(sh, db, row, &sqlparser.ColName{Name: sqlparser.NewColIdent(argName)}, schemas, fromTableNames)
					if err != nil {
						continue
					}
				}
				if val != nil {
					distinctValues[fmt.Sprintf("%v", val)] = true
				}
			}
			return len(distinctValues), nil
		}
		return len(rows), nil

	case "SUM", "AVG", "MIN", "MAX":
		if argName == "*" {
			return nil, fmt.Errorf("%s requires a column argument", strings.ToUpper(funcExpr.Name.String()))
		}
		var total float64
		var min float64
		var max float64
		count := 0
		distinctValues := make(map[string]float64)

		for i, row := range rows {
			var val interface{}
			var err error
			found := false
			for _, tableData := range row {
				if tableData == nil {
					continue
				}
				if valAny, ok := tableData.Values[argName]; ok {
					if valAny == nil {
						continue
					}
					val, err = utils.FromAny(valAny)
					if err != nil {
						continue
					}
					found = true
					break
				}
			}
			if !found {
				val, err = evaluateExpressionValue(sh, db, row, &sqlparser.ColName{Name: sqlparser.NewColIdent(argName)}, schemas, fromTableNames)
				if err != nil {
					continue
				}
			}
			if val == nil {
				continue
			}
			num, isNum := getNumericValue(val)
			if !isNum {
				continue
			}

			if isDistinct {
				valKey := fmt.Sprintf("%v", num)
				if _, exists := distinctValues[valKey]; exists {
					continue
				}
				distinctValues[valKey] = num
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

		switch strings.ToUpper(funcExpr.Name.String()) {
		case "SUM":
			if count == 0 {
				return nil, nil
			}
			return total, nil
		case "AVG":
			if count == 0 {
				return nil, nil
			}
			return total / float64(count), nil
		case "MIN":
			if count == 0 {
				return nil, nil
			}
			return min, nil
		case "MAX":
			if count == 0 {
				return nil, nil
			}
			return max, nil
		}
	}

	return nil, fmt.Errorf("unsupported aggregate function: %s", funcExpr.Name.String())
}

// Helper to evaluate CASE expressions with aggregates
func evaluateCaseExpression(sh *shell.Shell, db *pb.Database, rows []CombinedRow, caseExpr *sqlparser.CaseExpr, schemas map[string]*pb.Schema, fromTableNames []string, isAggregate bool) (interface{}, error) {
	if !isAggregate {
		return nil, fmt.Errorf("CASE expressions in non-aggregate context not yet supported")
	}

	// For aggregate CASE, we need to sum values based on conditions
	var total float64

	for _, row := range rows {
		var resultVal interface{}
		matched := false

		// Evaluate each WHEN clause
		for _, when := range caseExpr.Whens {
			condResult, err := evaluateExpression(sh, db, row, when.Cond, schemas, fromTableNames)
			if err != nil {
				continue
			}

			if condResult {
				// Evaluate the THEN expression
				thenVal, err := evaluateExpressionValue(sh, db, row, when.Val, schemas, fromTableNames)
				if err != nil {
					continue
				}
				resultVal = thenVal
				matched = true
				break
			}
		}

		// If no WHEN matched, use ELSE
		if !matched && caseExpr.Else != nil {
			elseVal, err := evaluateExpressionValue(sh, db, row, caseExpr.Else, schemas, fromTableNames)
			if err != nil {
				continue
			}
			resultVal = elseVal
		}

		// If no ELSE and no match, result is NULL (0 for sum)
		if resultVal != nil {
			num, isNum := getNumericValue(resultVal)
			if isNum {
				total += num
			}
		}
	}

	return total, nil
}

// Helper to evaluate arithmetic expressions with aggregates
func evaluateAggregateArithmetic(sh *shell.Shell, db *pb.Database, rows []CombinedRow, binExpr *sqlparser.BinaryExpr, schemas map[string]*pb.Schema, fromTableNames []string) (interface{}, error) {
	// Check if left side is an aggregate
	leftAgg, leftIsAgg := binExpr.Left.(*sqlparser.FuncExpr)
	rightAgg, rightIsAgg := binExpr.Right.(*sqlparser.FuncExpr)

	var leftVal, rightVal interface{}
	var err error

	if leftIsAgg {
		leftVal, err = evaluateAggregateInHaving(sh, db, rows, leftAgg, schemas, fromTableNames)
		if err != nil {
			return nil, err
		}
	} else if colName, ok := binExpr.Left.(*sqlparser.ColName); ok {
		// It's a column - get value from first row (for GROUP BY columns)
		if len(rows) > 0 {
			leftVal, err = evaluateExpressionValue(sh, db, rows[0], colName, schemas, fromTableNames)
			if err != nil {
				// If evaluation fails, try getting directly from row data
				for _, tableData := range rows[0] {
					if tableData == nil {
						continue
					}
					if valAny, ok := tableData.Values[colName.Name.String()]; ok {
						leftVal, err = utils.FromAny(valAny)
						if err == nil {
							break
						}
					}
				}
				if err != nil {
					return nil, err
				}
			}
		} else {
			return nil, fmt.Errorf("no rows available for column evaluation")
		}
	} else {
		if len(rows) > 0 {
			leftVal, err = evaluateExpressionValue(sh, db, rows[0], binExpr.Left, schemas, fromTableNames)
			if err != nil {
				return nil, err
			}
		}
	}

	if rightIsAgg {
		rightVal, err = evaluateAggregateInHaving(sh, db, rows, rightAgg, schemas, fromTableNames)
		if err != nil {
			return nil, err
		}
	} else if colName, ok := binExpr.Right.(*sqlparser.ColName); ok {
		// It's a column - get value from first row (for GROUP BY columns)
		if len(rows) > 0 {
			rightVal, err = evaluateExpressionValue(sh, db, rows[0], colName, schemas, fromTableNames)
			if err != nil {
				// If evaluation fails, try getting directly from row data
				for _, tableData := range rows[0] {
					if tableData == nil {
						continue
					}
					if valAny, ok := tableData.Values[colName.Name.String()]; ok {
						rightVal, err = utils.FromAny(valAny)
						if err == nil {
							break
						}
					}
				}
				if err != nil {
					return nil, err
				}
			}
		} else {
			return nil, fmt.Errorf("no rows available for column evaluation")
		}
	} else {
		if len(rows) > 0 {
			rightVal, err = evaluateExpressionValue(sh, db, rows[0], binExpr.Right, schemas, fromTableNames)
			if err != nil {
				return nil, err
			}
		}
	}

	// Handle nil values
	if leftVal == nil || rightVal == nil {
		return nil, nil
	}

	leftNum, leftIsNum := getNumericValue(leftVal)
	rightNum, rightIsNum := getNumericValue(rightVal)

	if !leftIsNum || !rightIsNum {
		return nil, fmt.Errorf("arithmetic operations require numeric values, got %T and %T", leftVal, rightVal)
	}

	switch binExpr.Operator {
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
		return nil, fmt.Errorf("unsupported operator: %s", binExpr.Operator)
	}
}

// Fixed executeUnion function
func executeUnion(sh *shell.Shell, db *pb.Database, union *sqlparser.Union) ([]map[string]interface{}, error) {
	// Execute the left SELECT statement
	var leftResults []map[string]interface{}
	var err error

	if leftSelect, ok := union.Left.(*sqlparser.Select); ok {
		leftResults, err = QueryDB(sh, db, leftSelect)
	} else if leftUnion, ok := union.Left.(*sqlparser.Union); ok {
		leftResults, err = executeUnion(sh, db, leftUnion)
	} else {
		return nil, fmt.Errorf("unsupported left operand in UNION: %T", union.Left)
	}

	if err != nil {
		return nil, fmt.Errorf("error executing left SELECT in UNION: %w", err)
	}

	// Execute the right SELECT statement
	var rightResults []map[string]interface{}

	if rightSelect, ok := union.Right.(*sqlparser.Select); ok {
		rightResults, err = QueryDB(sh, db, rightSelect)
	} else if rightUnion, ok := union.Right.(*sqlparser.Union); ok {
		rightResults, err = executeUnion(sh, db, rightUnion)
	} else {
		return nil, fmt.Errorf("unsupported right operand in UNION: %T", union.Right)
	}

	if err != nil {
		return nil, fmt.Errorf("error executing right SELECT in UNION: %w", err)
	}

	// Validate that both SELECT statements have the same number of columns
	if len(leftResults) > 0 && len(rightResults) > 0 {
		leftCols := getColumnNames(leftResults[0])
		rightCols := getColumnNames(rightResults[0])
		if len(leftCols) != len(rightCols) {
			return nil, fmt.Errorf("UNION queries must return the same number of columns: left has %d, right has %d", len(leftCols), len(rightCols))
		}
	}

	// Combine results
	combinedResults := make([]map[string]interface{}, 0, len(leftResults)+len(rightResults))

	// For UNION (not UNION ALL), we need to deduplicate
	if union.Type == "" || union.Type == sqlparser.UnionStr {
		resultMap := make(map[string]bool) // For deduplication

		for _, row := range leftResults {
			rowKey := rowToString(row)
			if !resultMap[rowKey] {
				combinedResults = append(combinedResults, row)
				resultMap[rowKey] = true
			}
		}

		for _, row := range rightResults {
			rowKey := rowToString(row)
			if !resultMap[rowKey] {
				combinedResults = append(combinedResults, row)
				resultMap[rowKey] = true
			}
		}
	} else {
		// UNION ALL - just append
		combinedResults = append(combinedResults, leftResults...)
		combinedResults = append(combinedResults, rightResults...)
	}

	// Apply ORDER BY if present
	if union.OrderBy != nil {
		applyOrderByToMap(combinedResults, union.OrderBy, nil)
	}

	// Apply LIMIT if present
	if union.Limit != nil {
		combinedResults = applyLimitAndOffsetToMap(sh, db, combinedResults, union.Limit)
	}

	return combinedResults, nil
}

// Helper function to get column names (already exists, but ensuring it's consistent)
func getColumnNames(row map[string]interface{}) []string {
	cols := make([]string, 0, len(row))
	for col := range row {
		cols = append(cols, col)
	}
	sort.Strings(cols) // Ensure consistent order
	return cols
}

// Helper function to create string representation (already exists, but ensuring it's consistent)
func rowToString(row map[string]interface{}) string {
	var parts []string
	cols := getColumnNames(row)
	for _, col := range cols {
		val := row[col]
		if val == nil {
			parts = append(parts, "NULL")
		} else {
			parts = append(parts, fmt.Sprintf("%v", val))
		}
	}
	return strings.Join(parts, "||")
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
				return nil, fmt.Errorf("error converting value for column %s: %w", colName, err)
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

// getCommonColumns identifies common column names between two tables
func getCommonColumns(leftSchema, rightSchema *pb.Schema) []string {
	var common []string
	leftCols := make(map[string]bool)
	for _, col := range leftSchema.Columns {
		leftCols[col.Name] = true
	}
	for _, col := range rightSchema.Columns {
		if leftCols[col.Name] {
			common = append(common, col.Name)
		}
	}
	return common
}

// createNaturalJoinCondition generates an ON condition for NATURAL JOIN
func createNaturalJoinCondition(leftTableName, rightTableName string, commonCols []string) sqlparser.Expr {
	if len(commonCols) == 0 {
		return nil
	}
	var expr sqlparser.Expr
	for i, col := range commonCols {
		comparison := &sqlparser.ComparisonExpr{
			Operator: sqlparser.EqualStr,
			Left: &sqlparser.ColName{
				Name:      sqlparser.NewColIdent(col),
				Qualifier: sqlparser.TableName{Name: sqlparser.NewTableIdent(leftTableName)},
			},
			Right: &sqlparser.ColName{
				Name:      sqlparser.NewColIdent(col),
				Qualifier: sqlparser.TableName{Name: sqlparser.NewTableIdent(rightTableName)},
			},
		}
		if i == 0 {
			expr = comparison
		} else {
			expr = &sqlparser.AndExpr{Left: expr, Right: comparison}
		}
	}
	return expr
}

// nullRowForTable creates a row with NULL values for a given table's schema
func nullRowForTable(tableName string, schema *pb.Schema) *pb.Row {
	row := &pb.Row{Values: make(map[string]*anypb.Any)}
	for _, col := range schema.Columns {
		anyVal, _ := utils.ToAny(nil) // Properly serialize NULL as an anypb.Any
		row.Values[col.Name] = anyVal
	}
	return row
}

func executeJoin(sh *shell.Shell, db *pb.Database, joinExpr *sqlparser.JoinTableExpr, schemas map[string]*pb.Schema) ([]CombinedRow, error) {
	// Extract table names for left and right sides
	leftTables := getTableNamesFromClause(sqlparser.TableExprs{joinExpr.LeftExpr})
	rightTables := getTableNamesFromClause(sqlparser.TableExprs{joinExpr.RightExpr})
	if len(leftTables) == 0 || len(rightTables) == 0 {
		return nil, fmt.Errorf("invalid table names in JOIN")
	}
	primaryLeftTable := leftTables[0]
	primaryRightTable := rightTables[0]

	// Load rows for both sides
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

	// Normalize join type - handle different string representations
	joinType := strings.ToLower(strings.TrimSpace(joinExpr.Join))

	// Handle NATURAL JOIN
	if (joinType == "join" || joinType == "") && joinCondition == nil {
		commonCols := getCommonColumns(schemas[primaryLeftTable], schemas[primaryRightTable])
		if len(commonCols) > 0 {
			joinCondition = createNaturalJoinCondition(primaryLeftTable, primaryRightTable, commonCols)
		}
		if joinCondition == nil {
			return crossProduct(leftRows, rightRows), nil
		}
	}

	// Track matched rows for outer joins
	matchedLeft := make(map[int]bool)
	matchedRight := make(map[int]bool)

	// Perform the join based on condition
	for i, lRow := range leftRows {
		for j, rRow := range rightRows {
			shouldJoin := true
			if joinCondition != nil {
				mergedRow := make(CombinedRow)
				for k, v := range lRow {
					mergedRow[k] = v
				}
				for k, v := range rRow {
					mergedRow[k] = v
				}

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
				matchedLeft[i] = true
				matchedRight[j] = true
			}
		}
	}

	// Handle LEFT OUTER JOIN or LEFT JOIN
	if joinType == "left outer join" || joinType == "left join" || joinType == "left" {
		for i, lRow := range leftRows {
			if !matchedLeft[i] {
				mergedRow := make(CombinedRow)
				for k, v := range lRow {
					mergedRow[k] = v
				}
				// Add NULL rows for right tables
				for _, tName := range rightTables {
					if schema, exists := schemas[tName]; exists {
						mergedRow[tName] = nullRowForTable(tName, schema)
					}
				}
				joinedRows = append(joinedRows, mergedRow)
			}
		}
	}

	// Handle RIGHT OUTER JOIN or RIGHT JOIN
	if joinType == "right outer join" || joinType == "right join" || joinType == "right" {
		for j, rRow := range rightRows {
			if !matchedRight[j] {
				mergedRow := make(CombinedRow)
				// Add NULL rows for left tables FIRST
				for _, tName := range leftTables {
					if schema, exists := schemas[tName]; exists {
						mergedRow[tName] = nullRowForTable(tName, schema)
					}
				}
				// Then add right row data
				for k, v := range rRow {
					mergedRow[k] = v
				}
				joinedRows = append(joinedRows, mergedRow)
			}
		}
	}

	// Handle FULL OUTER JOIN (simulate it since parser doesn't support it natively)
	if joinType == "full outer join" || joinType == "full" {
		// Add unmatched left rows
		for i, lRow := range leftRows {
			if !matchedLeft[i] {
				mergedRow := make(CombinedRow)
				for k, v := range lRow {
					mergedRow[k] = v
				}
				for _, tName := range rightTables {
					if schema, exists := schemas[tName]; exists {
						mergedRow[tName] = nullRowForTable(tName, schema)
					}
				}
				joinedRows = append(joinedRows, mergedRow)
			}
		}
		// Add unmatched right rows
		for j, rRow := range rightRows {
			if !matchedRight[j] {
				mergedRow := make(CombinedRow)
				for _, tName := range leftTables {
					if schema, exists := schemas[tName]; exists {
						mergedRow[tName] = nullRowForTable(tName, schema)
					}
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
func projectColumns(sh *shell.Shell, db *pb.Database, from sqlparser.TableExprs, rows []CombinedRow, columns sqlparser.SelectExprs, schemas map[string]*pb.Schema, fromTableNames []string) ([]map[string]interface{}, error) {
	results := make([]map[string]interface{}, 0, len(rows))
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
							val, err := utils.FromAny(valAny)
							if err != nil {
								return nil, fmt.Errorf("error converting value for column %s in table %s: %w", colName, tName, err)
							}
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
						val, err := utils.FromAny(valAny)
						if err != nil {
							return nil, fmt.Errorf("error converting value for column %s in table %s: %w", colName, tName, err)
						}
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
		subqueryResults, err := executeCorrelatedSubquery(sh, db, e.Subquery.Select.(*sqlparser.Select), row, schemas)
		if err != nil {
			return false, fmt.Errorf("error executing EXISTS subquery: %w", err)
		}
		return len(subqueryResults) > 0, nil

	case *sqlparser.AndExpr:
		left, err := evaluateExpression(sh, db, row, e.Left, schemas, fromTableNames)
		if err != nil {
			return false, err
		}
		if !left {
			return false, nil // Short-circuit
		}
		right, err := evaluateExpression(sh, db, row, e.Right, schemas, fromTableNames)
		if err != nil {
			return false, err
		}
		return right, nil

	case *sqlparser.OrExpr:
		left, err := evaluateExpression(sh, db, row, e.Left, schemas, fromTableNames)
		if err != nil {
			return false, err
		}
		if left {
			return true, nil // Short-circuit
		}
		right, err := evaluateExpression(sh, db, row, e.Right, schemas, fromTableNames)
		if err != nil {
			return false, err
		}
		return right, nil

	case *sqlparser.IsExpr:
		exprVal, err := evaluateExpressionValue(sh, db, row, e.Expr, schemas, fromTableNames)
		if err != nil {
			return false, fmt.Errorf("error evaluating IS expression: %w", err)
		}
		switch e.Operator {
		case sqlparser.IsNullStr:
			return exprVal == nil, nil
		case sqlparser.IsNotNullStr:
			return exprVal != nil, nil
		default:
			return false, fmt.Errorf("unsupported IS operator: %s", e.Operator)
		}

	case *sqlparser.ComparisonExpr:
		left, err := evaluateExpressionValue(sh, db, row, e.Left, schemas, fromTableNames)
		if err != nil {
			// If column doesn't exist (e.g., due to NULL row from RIGHT JOIN), treat as NULL
			if strings.Contains(err.Error(), "not found") {
				left = nil
			} else {
				return false, fmt.Errorf("error evaluating left operand: %w", err)
			}
		}

		if e.Operator == sqlparser.InStr {
			if subquery, ok := e.Right.(*sqlparser.Subquery); ok {
				subqueryResults, err := executeCorrelatedSubquery(sh, db, subquery.Select.(*sqlparser.Select), row, schemas)
				if err != nil {
					return false, fmt.Errorf("error executing IN subquery: %w", err)
				}

				// NULL IN (...) returns false
				if left == nil {
					return false, nil
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

			if valTuple, ok := e.Right.(sqlparser.ValTuple); ok {
				// NULL IN (...) returns false
				if left == nil {
					return false, nil
				}

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

				// NULL NOT IN (...) returns false
				if left == nil {
					return false, nil
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

			if valTuple, ok := e.Right.(sqlparser.ValTuple); ok {
				// NULL NOT IN (...) returns false
				if left == nil {
					return false, nil
				}

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

		if e.Operator == sqlparser.LikeStr {
			right, err := evaluateExpressionValue(sh, db, row, e.Right, schemas, fromTableNames)
			if err != nil {
				return false, err
			}
			// NULL LIKE ... returns false
			if left == nil || right == nil {
				return false, nil
			}
			return evaluateLike(left, right)
		}

		if e.Operator == sqlparser.NotLikeStr {
			right, err := evaluateExpressionValue(sh, db, row, e.Right, schemas, fromTableNames)
			if err != nil {
				return false, err
			}
			// NULL NOT LIKE ... returns false
			if left == nil || right == nil {
				return false, nil
			}
			result, err := evaluateLike(left, right)
			if err != nil {
				return false, err
			}
			return !result, nil
		}

		right, err := evaluateExpressionValue(sh, db, row, e.Right, schemas, fromTableNames)
		if err != nil {
			// If column doesn't exist (e.g., due to NULL row from RIGHT JOIN), treat as NULL
			if strings.Contains(err.Error(), "not found") {
				right = nil
			} else {
				return false, fmt.Errorf("error evaluating right operand: %w", err)
			}
		}

		return compareValuesWithOperator(left, right, e.Operator)
	}

	return false, fmt.Errorf("unsupported expression type: %T", expr)
}
func compareValues(left, right interface{}) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}

	leftNum, leftIsNum := getNumericValue(left)
	rightNum, rightIsNum := getNumericValue(right)
	if leftIsNum && rightIsNum {
		return leftNum == rightNum
	}

	leftStr, leftIsStr := toString(left)
	rightStr, rightIsStr := toString(right)
	if leftIsStr && rightIsStr {
		return leftStr == rightStr
	}

	return left == right
}

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

	leftTime, leftIsTime := left.(time.Time)
	rightTime, rightIsTime := right.(time.Time)

	if leftIsTime || rightIsTime {
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

func evaluateLike(value, pattern interface{}) (bool, error) {
	valueStr, ok := toString(value)
	if !ok {
		return false, fmt.Errorf("LIKE left operand must be a string, got %T", value)
	}

	patternStr, ok := toString(pattern)
	if !ok {
		return false, fmt.Errorf("LIKE pattern must be a string, got %T", pattern)
	}

	return matchLikePattern(valueStr, patternStr), nil
}

func matchLikePattern(value, pattern string) bool {
	return matchLikeHelper(value, pattern, 0, 0)
}

func matchLikeHelper(value, pattern string, vIdx, pIdx int) bool {
	for pIdx < len(pattern) {
		if pIdx < len(pattern) && pattern[pIdx] == '%' {
			pIdx++
			if pIdx == len(pattern) {
				return true
			}
			for i := vIdx; i <= len(value); i++ {
				if matchLikeHelper(value, pattern, i, pIdx) {
					return true
				}
			}
			return false
		} else if pIdx < len(pattern) && pattern[pIdx] == '_' {
			if vIdx >= len(value) {
				return false
			}
			vIdx++
			pIdx++
		} else {
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

func evaluateExpressionValue(sh *shell.Shell, db *pb.Database, row CombinedRow, expr sqlparser.Expr, schemas map[string]*pb.Schema, fromTableNames []string) (interface{}, error) {
	switch e := expr.(type) {
	case sqlparser.ValTuple:
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
		subqueryResults, err := executeCorrelatedSubquery(sh, db, e.Select.(*sqlparser.Select), row, schemas)
		if err != nil {
			return nil, fmt.Errorf("error executing scalar subquery: %w", err)
		}
		if len(subqueryResults) > 1 {
			return nil, fmt.Errorf("subquery returned more than one row")
		}
		if len(subqueryResults) == 0 {
			return nil, nil
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
		// Check for aggregate alias in the "result" table
		if resultData, ok := row["result"]; ok && resultData != nil {
			if valAny, ok := resultData.Values[e.Name.String()]; ok {
				val, err := utils.FromAny(valAny)
				if err != nil {
					return nil, fmt.Errorf("error converting value for column %s in result: %w", e.Name.String(), err)
				}
				return val, nil
			}
		}
		// Fallback to regular column evaluation
		val, err := evaluateIdentifier(row, e, schemas, fromTableNames)
		if err != nil {
			// Handle case where column is not found due to LEFT OUTER JOIN
			if strings.Contains(err.Error(), "column") && strings.Contains(err.Error(), "not found") {
				return nil, nil // Return nil for non-existent columns in LEFT OUTER JOIN
			}
			return nil, fmt.Errorf("error evaluating column %s: %w", e.Name.String(), err)
		}
		return val, nil

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
	subquerySchemas := make(map[string]*pb.Schema)
	aliases := make(map[string]string)
	for _, tableExpr := range selectStmt.From {
		if err := processTableExprForSchemas(sh, db, tableExpr, subquerySchemas, aliases); err != nil {
			return nil, err
		}
	}

	referencedOuterTables := make(map[string]bool)
	collectOuterReferences(selectStmt, referencedOuterTables)

	filteredOuterSchemas := make(map[string]*pb.Schema)
	for tableName := range referencedOuterTables {
		if schema, ok := outerSchemas[tableName]; ok {
			filteredOuterSchemas[tableName] = schema
		}
	}

	effectiveSchemas := make(map[string]*pb.Schema)
	for tableName, schema := range subquerySchemas {
		effectiveSchemas[tableName] = schema
	}
	for tableName, schema := range filteredOuterSchemas {
		effectiveSchemas[tableName] = schema
	}

	fromTableNames := getTableNamesFromClause(selectStmt.From)

	combinedRows, err := executeFromClause(sh, db, selectStmt.From, subquerySchemas)
	if err != nil {
		return nil, fmt.Errorf("error executing FROM/JOIN clause in subquery: %w", err)
	}

	var filteredRows []CombinedRow
	if selectStmt.Where == nil {
		filteredRows = combinedRows
	} else {
		for _, row := range combinedRows {
			mergedRow := make(CombinedRow)
			for k, v := range row {
				mergedRow[k] = v
			}
			if outerRow != nil {
				for k, v := range outerRow {
					mergedRow[k] = v
				}
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

	var results []map[string]interface{}
	if selectStmt.GroupBy != nil || isAggregateQuery(selectStmt) {
		results, err = executeAggregateQuery(sh, db, filteredRows, selectStmt, effectiveSchemas, fromTableNames)
		if err != nil {
			return nil, fmt.Errorf("error executing aggregate query in subquery: %w", err)
		}
	} else {
		results, err = projectColumns(sh, db, selectStmt.From, filteredRows, selectStmt.SelectExprs, effectiveSchemas, fromTableNames)
		if err != nil {
			return nil, fmt.Errorf("error projecting columns in subquery: %w", err)
		}
	}

	if selectStmt.OrderBy != nil {
		applyOrderByToMap(results, selectStmt.OrderBy, effectiveSchemas)
	}

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
		case *sqlparser.IsExpr:
			walkExpr(e.Expr)
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

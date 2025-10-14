package core

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/blastrain/vitess-sqlparser/sqlparser"
	shell "github.com/ipfs/go-ipfs-api"
	pb "github.com/nnlgsakib/wwfsdb/pkg/core/proto"
	"github.com/nnlgsakib/wwfsdb/pkg/sql"
	utils "github.com/nnlgsakib/wwfsdb/pkg/util"
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

		// If an alias is specified, also map the alias to the schema
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
	aliases := make(map[string]string) // alias -> original table name mapping

	for _, tableExpr := range s.From {
		switch expr := tableExpr.(type) {
		case *sqlparser.AliasedTableExpr:
			// Handle aliased table expressions: tableName alias
			tableName, err := sql.ExtractTableName(expr)
			if err != nil {
				return nil, fmt.Errorf("failed to extract table name: %w", err)
			}

			tableCID, ok := db.Tables[tableName]
			if !ok {
				return nil, fmt.Errorf("table %s not found", tableName)
			}
			table, err := LoadTable(sh, tableCID)
			if err != nil {
				return nil, err
			}
			schema, err := LoadSchema(sh, table.SchemaCid)
			if err != nil {
				return nil, err
			}

			// Store schema under original table name
			schemas[tableName] = schema

			// If an alias is specified, also map the alias to the schema
			if !expr.As.IsEmpty() {
				aliasName := expr.As.String()
				schemas[aliasName] = schema
				aliases[aliasName] = tableName // Keep track of which original table the alias refers to
			}
		case *sqlparser.JoinTableExpr:
			// For JOIN expressions, we need to recursively process both sides
			// This requires a more complex approach to extract all tables and their aliases
			// For now, let's handle the simple case by processing each side of the join
			if err := processTableExprForSchemas(sh, db, expr.LeftExpr, schemas, aliases); err != nil {
				return nil, err
			}
			if err := processTableExprForSchemas(sh, db, expr.RightExpr, schemas, aliases); err != nil {
				return nil, err
			}
		}
	}

	// 1. Execute the FROM clause (including any JOINs) to get a set of combined rows.
	combinedRows, err := executeFromClause(sh, db, s.From, schemas)
	if err != nil {
		return nil, fmt.Errorf("error executing FROM/JOIN clause: %w", err)
	}

	// 2. Filter the combined rows using the WHERE clause.
	var filteredRows []CombinedRow
	if s.Where == nil {
		filteredRows = combinedRows
	} else {
		for _, row := range combinedRows {
			include, err := evaluateExpression(row, s.Where.Expr, schemas)
			if err != nil {
				return nil, err
			}
			if include {
				filteredRows = append(filteredRows, row)
			}
		}
	}

	// 3. Handle GROUP BY and aggregates
	if s.GroupBy != nil || isAggregateQuery(s) {
		results, err := executeAggregateQuery(filteredRows, s, schemas)
		if err != nil {
			return nil, err
		}
		// Order and limit after aggregation
		if s.OrderBy != nil {
			applyOrderByToMap(results, s.OrderBy)
		}
		return applyLimitAndOffsetToMap(results, s.Limit), nil
	}

	// 4. Handle ORDER BY for non-aggregate queries
	if s.OrderBy != nil {
		applyOrderBy(filteredRows, s.OrderBy, schemas)
	}

	// 5. Handle OFFSET and LIMIT for non-aggregate queries
	finalRows := applyLimitAndOffset(filteredRows, s.Limit)

	// 6. Project the final columns for the result set.
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
				// This is a simplistic check. A more robust implementation would check
				// if the function is actually an aggregate function (e.g., COUNT, SUM, etc.)
				return true
			}
		}
	}
	return false
}

func applyOrderBy(rows []CombinedRow, orderBy sqlparser.OrderBy, schemas map[string]*pb.Schema) {
	sort.SliceStable(rows, func(i, j int) bool {
		for _, order := range orderBy {
			valI, _ := evaluateExpressionValue(rows[i], order.Expr, schemas)
			valJ, _ := evaluateExpressionValue(rows[j], order.Expr, schemas)

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
	}
	return 0, false
}

func applyLimitAndOffset(rows []CombinedRow, limit *sqlparser.Limit) []CombinedRow {
	if limit == nil {
		return rows
	}

	offset := 0
	if limit.Offset != nil {
		offsetVal, err := evaluateExpressionValue(nil, limit.Offset, nil)
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
		limitVal, err := evaluateExpressionValue(nil, limit.Rowcount, nil)
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

func applyLimitAndOffsetToMap(rows []map[string]interface{}, limit *sqlparser.Limit) []map[string]interface{} {
	if limit == nil {
		return rows
	}

	offset := 0
	if limit.Offset != nil {
		offsetVal, err := evaluateExpressionValue(nil, limit.Offset, nil)
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
		limitVal, err := evaluateExpressionValue(nil, limit.Rowcount, nil)
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

func applyOrderByToMap(results []map[string]interface{}, orderBy sqlparser.OrderBy) {
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

func executeAggregateQuery(rows []CombinedRow, s *sqlparser.Select, schemas map[string]*pb.Schema) ([]map[string]interface{}, error) {
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
				return nil, fmt.Errorf("cannot mix aggregate and non-aggregate columns without GROUP BY")
			}

			var argName string
			if len(aggExpr.Exprs) > 0 {
				if _, ok := aggExpr.Exprs[0].(*sqlparser.StarExpr); ok {
					argName = "*"
				} else if colName, ok := aggExpr.Exprs[0].(*sqlparser.AliasedExpr).Expr.(*sqlparser.ColName); ok {
					argName = colName.Name.String()
				}
			}
			resultColName := fmt.Sprintf("%s(%s)", strings.ToUpper(aggExpr.Name.String()), argName)

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
					val, err := evaluateIdentifier(row, &sqlparser.ColName{Name: sqlparser.NewColIdent(argName)}, schemas)
					if err != nil {
						return nil, err
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

	// --- GROUP BY Implementation ---
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
			val, err := evaluateExpressionValue(row, expr, schemas)
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
			if aliasedExpr, ok := colExpr.(*sqlparser.AliasedExpr); ok {
				if aggExpr, ok := aliasedExpr.Expr.(*sqlparser.FuncExpr); ok {
					var argName string
					if len(aggExpr.Exprs) > 0 {
						if _, ok := aggExpr.Exprs[0].(*sqlparser.StarExpr); ok {
							argName = "*"
						} else if colName, ok := aggExpr.Exprs[0].(*sqlparser.AliasedExpr).Expr.(*sqlparser.ColName); ok {
							argName = colName.Name.String()
						}
					}
					resultColName := fmt.Sprintf("%s(%s)", strings.ToUpper(aggExpr.Name.String()), argName)

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
							val, err := evaluateIdentifier(row, &sqlparser.ColName{Name: sqlparser.NewColIdent(argName)}, schemas)
							if err != nil {
								return nil, err
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
					}
				}
			}
		}
		finalResults = append(finalResults, resultRow)
	}

	return finalResults, nil
}

// executeFromClause is the core of the JOIN implementation. It recursively processes the FromClause.
func executeFromClause(sh *shell.Shell, db *pb.Database, from sqlparser.TableExprs, schemas map[string]*pb.Schema) ([]CombinedRow, error) {
	if len(from) == 0 {
		return nil, nil
	}

	// Process each table expression individually to handle JOIN expressions
	var allRows []CombinedRow

	for _, tableExpr := range from {
		// Check if this is a JOIN expression
		if joinExpr, ok := tableExpr.(*sqlparser.JoinTableExpr); ok {
			// Handle JOIN expressions
			joinedRows, err := executeJoin(sh, db, joinExpr, schemas)
			if err != nil {
				return nil, err
			}
			// If this is the first table or we're doing a multi-way join, combine appropriately
			if len(allRows) == 0 {
				allRows = joinedRows
			} else {
				// For subsequent JOINs, we'd need to implement more complex logic
				// For now, just assign the result
				allRows = joinedRows
			}
		} else {
			// Handle single table expressions (including aliased tables)
			originalTableName, err := sql.ExtractTableName(tableExpr)
			if err != nil {
				return nil, err
			}

			// Check if this is an aliased table expression to get the alias
			var tableKey string
			if aliasedExpr, ok := tableExpr.(*sqlparser.AliasedTableExpr); ok && !aliasedExpr.As.IsEmpty() {
				tableKey = aliasedExpr.As.String() // Use alias as the key
			} else {
				tableKey = originalTableName // Use original name if no alias
			}

			tableCID, ok := db.Tables[originalTableName]
			if !ok {
				return nil, fmt.Errorf("table %s not found in database", originalTableName)
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
			// Combine with existing rows (Cartesian product if multiple tables)
			if len(allRows) == 0 {
				allRows = results
			} else {
				var newAllRows []CombinedRow
				for _, existingRow := range allRows {
					for _, newRow := range results {
						// Merge the rows
						mergedRow := make(CombinedRow)
						for k, v := range existingRow {
							mergedRow[k] = v
						}
						for k, v := range newRow {
							mergedRow[k] = v
						}
						newAllRows = append(newAllRows, mergedRow)
					}
				}
				allRows = newAllRows
			}
		}
	}

	return allRows, nil
}

// executeJoin handles JOIN expressions
func executeJoin(sh *shell.Shell, db *pb.Database, joinExpr *sqlparser.JoinTableExpr, schemas map[string]*pb.Schema) ([]CombinedRow, error) {
	// Get rows from the left side of the join
	leftRows, err := executeFromClause(sh, db, sqlparser.TableExprs{joinExpr.LeftExpr}, schemas)
	if err != nil {
		return nil, fmt.Errorf("error executing left side of join: %w", err)
	}

	// Get rows from the right side of the join
	rightRows, err := executeFromClause(sh, db, sqlparser.TableExprs{joinExpr.RightExpr}, schemas)
	if err != nil {
		return nil, fmt.Errorf("error executing right side of join: %w", err)
	}

	var joinedRows []CombinedRow

	// Check if there's a join condition (ON clause)
	joinCondition := joinExpr.On

	for _, lRow := range leftRows {
		for _, rRow := range rightRows {
			// Check if the rows match the join condition
			shouldJoin := true
			if joinCondition != nil {
				// Create a combined row to evaluate the join condition
				combinedRow := make(CombinedRow)
				for k, v := range lRow {
					combinedRow[k] = v
				}
				for k, v := range rRow {
					combinedRow[k] = v
				}

				// Evaluate the join condition
				result, err := evaluateExpression(combinedRow, joinCondition, schemas)
				if err != nil {
					return nil, fmt.Errorf("error evaluating join condition: %w", err)
				}
				shouldJoin = result
			}

			if shouldJoin {
				// Merge the rows that satisfy the join condition
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

// projectColumns creates the final result set based on the selected columns.
func projectColumns(sh *shell.Shell, db *pb.Database, from sqlparser.TableExprs, rows []CombinedRow, columns sqlparser.SelectExprs, schemas map[string]*pb.Schema) ([]map[string]interface{}, error) {
	results := make([]map[string]interface{}, 0, len(rows))

	for _, combinedRow := range rows {
		resultRow := make(map[string]interface{})
		for _, colExpr := range columns {
			switch c := colExpr.(type) {
			case *sqlparser.StarExpr:
				if c.TableName.Name.IsEmpty() { // for `*`
					for tableName, rowData := range combinedRow {
						if rowData == nil {
							for _, schemaCol := range schemas[tableName].Columns {
								resultRow[tableName+"."+schemaCol.Name] = nil
							}
							continue
						}
						for colName, valAny := range rowData.Values {
							val, _ := utils.FromAny(valAny)
							resultRow[tableName+"."+colName] = val
						}
					}
				} else { // for `table.*`
					tableName := c.TableName.Name.String()
					rowData, ok := combinedRow[tableName]
					if !ok {
						return nil, fmt.Errorf("table %s not found in query", tableName)
					}
					if rowData == nil {
						for _, schemaCol := range schemas[tableName].Columns {
							resultRow[tableName+"."+schemaCol.Name] = nil
						}
						continue
					}
					for colName, valAny := range rowData.Values {
						val, _ := utils.FromAny(valAny)
						resultRow[tableName+"."+colName] = val
					}
				}
			case *sqlparser.AliasedExpr:
				val, err := evaluateExpressionValue(combinedRow, c.Expr, schemas)
				if err != nil {
					return nil, err
				}
				finalColName := c.As.String()
				if finalColName == "" {
					if colName, ok := c.Expr.(*sqlparser.ColName); ok {
						finalColName = colName.Name.String()
					}
				}
				resultRow[finalColName] = val
			}
		}
		results = append(results, resultRow)
	}
	return results, nil
}

// getTableNamesFromClause recursively finds all table names involved in a FromClause.
func getTableNamesFromClause(from sqlparser.TableExprs) []string {
	var tableNames []string
	for _, tableExpr := range from {
		tableName, err := sql.ExtractTableName(tableExpr)
		if err == nil {
			tableNames = append(tableNames, tableName)
		}
	}
	return tableNames
}

func evaluateExpression(row CombinedRow, expr sqlparser.Expr, schemas map[string]*pb.Schema) (bool, error) {
	if expr == nil {
		return true, nil
	}

	switch e := expr.(type) {
	case *sqlparser.AndExpr:
		left, err := evaluateExpression(row, e.Left, schemas)
		if err != nil {
			return false, err
		}
		right, err := evaluateExpression(row, e.Right, schemas)
		if err != nil {
			return false, err
		}
		return left && right, nil
	case *sqlparser.OrExpr:
		left, err := evaluateExpression(row, e.Left, schemas)
		if err != nil {
			return false, err
		}
		right, err := evaluateExpression(row, e.Right, schemas)
		if err != nil {
			return false, err
		}
		return left || right, nil
	case *sqlparser.ComparisonExpr:
		left, err := evaluateExpressionValue(row, e.Left, schemas)
		if err != nil {
			return false, err
		}
		right, err := evaluateExpressionValue(row, e.Right, schemas)
		if err != nil {
			return false, err
		}

		if left == nil || right == nil {
			switch e.Operator {
			case "=":
				return left == right, nil
			case "!=":
				return left != right, nil
			default:
				return false, nil
			}
		}

		leftNum, leftIsNum := getNumericValue(left)
		rightNum, rightIsNum := getNumericValue(right)

		if leftIsNum && rightIsNum {
			switch e.Operator {
			case "=":
				return leftNum == rightNum, nil
			case "!=":
				return leftNum != rightNum, nil
			case ">":
				return leftNum > rightNum, nil
			case "<":
				return leftNum < rightNum, nil
			case ">=":
				return leftNum >= rightNum, nil
			case "<=":
				return leftNum <= rightNum, nil
			}
		}

		leftBool, leftIsBool := left.(bool)
		rightBool, rightIsBool := right.(bool)

		if leftIsBool && rightIsBool {
			switch e.Operator {
			case "=":
				return leftBool == rightBool, nil
			case "!=":
				return leftBool != rightBool, nil
			default:
				return false, fmt.Errorf("unsupported operator '%s' for boolean comparison", e.Operator)
			}
		}

		leftStr, leftIsStr := left.(string)
		rightStr, rightIsStr := right.(string)

		if leftIsStr && rightIsStr {
			switch e.Operator {
			case "=":
				return leftStr == rightStr, nil
			case "!=":
				return leftStr != rightStr, nil
			case ">":
				return leftStr > rightStr, nil
			case "<":
				return leftStr < rightStr, nil
			case ">=":
				return leftStr >= rightStr, nil
			case "<=":
				return leftStr <= rightStr, nil
			default:
				return false, fmt.Errorf("unsupported comparison operator: %s", e.Operator)
			}
		}

		return false, fmt.Errorf("cannot compare types %T and %T", left, right)
	default:
		return false, fmt.Errorf("unsupported expression type: %T", e)
	}
}

func getNumericValue(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int64:
		return float64(val), true
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}

func evaluateExpressionValue(row CombinedRow, expr sqlparser.Expr, schemas map[string]*pb.Schema) (interface{}, error) {
	switch e := expr.(type) {
	case *sqlparser.ColName:
		return evaluateIdentifier(row, e, schemas)
	case *sqlparser.SQLVal:
		switch e.Type {
		case sqlparser.StrVal:
			return string(e.Val), nil
		case sqlparser.IntVal:
			return strconv.ParseInt(string(e.Val), 10, 64)
		case sqlparser.FloatVal:
			return strconv.ParseFloat(string(e.Val), 64)
		default:
			return nil, fmt.Errorf("unsupported SQLVal type: %v", e.Type)
		}
	case *sqlparser.UnaryExpr:
		right, err := evaluateExpressionValue(row, e.Expr, schemas)
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
		left, err := evaluateExpressionValue(row, e.Left, schemas)
		if err != nil {
			return nil, err
		}
		right, err := evaluateExpressionValue(row, e.Right, schemas)
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
	default:
		return nil, fmt.Errorf("unsupported expression value type: %T", e)
	}
}

// evaluateIdentifier resolves an identifier (e.g., `users.id` or `id`) against a combined row.
// It is now schema-aware and handles table aliases.
func evaluateIdentifier(row CombinedRow, ident *sqlparser.ColName, schemas map[string]*pb.Schema) (interface{}, error) {
	// Case 1: Identifier is fully qualified (e.g., users.id or a.id where 'a' is an alias)
	if !ident.Qualifier.IsEmpty() {
		originalTableName := ident.Qualifier.Name.String()

		// First, check if the qualifier directly matches a schema (could be original name or alias)
		tableSchema, ok := schemas[originalTableName]
		if !ok {
			return nil, fmt.Errorf("table %s not found in FROM clause", originalTableName)
		}

		// Check if column exists in schema
		foundInSchema := false
		for _, col := range tableSchema.Columns {
			if col.Name == ident.Name.String() {
				foundInSchema = true
				break
			}
		}
		if !foundInSchema {
			return nil, fmt.Errorf("column %s not found in table %s", ident.Name.String(), originalTableName)
		}

		// The row data is stored with the original table name (not the alias)
		// So if "a" is an alias for "accounts", we need to look up row["accounts"]
		tableData, ok := row[originalTableName]
		if !ok {
			// If we didn't find it with the qualifier name, the original table name might be different
			// In the executeFromClause, we store data using original table names
			// So if the user is querying with an alias, we have to map it back
			// This is complex - let's try to find the actual table that corresponds to this alias
			for rowTableName, rowData := range row {
				// Check if this row table name's schema matches the schema we found
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
		if tableData == nil { // This happens in a LEFT JOIN where there was no match
			return nil, nil
		}

		valAny, ok := tableData.Values[ident.Name.String()]
		if !ok {
			return nil, nil // Column is in schema but not in this specific row's data, return null
		}
		return utils.FromAny(valAny)
	}

	// Case 2: Identifier is unqualified (e.g., id). We need to find which table it belongs to.
	var foundValue interface{}
	var foundInTable string
	var valueFound bool

	for tableName, tableSchema := range schemas {
		for _, col := range tableSchema.Columns {
			if col.Name == ident.Name.String() {
				// Found a table with this column in its schema
				if foundInTable != "" {
					return nil, fmt.Errorf("ambiguous column name: %s exists in multiple tables", ident.Name.String())
				}
				foundInTable = tableName

				// Now get the value from the row data if it exists
				if tableData, ok := row[tableName]; ok && tableData != nil {
					if valAny, dataOk := tableData.Values[ident.Name.String()]; dataOk {
						val, err := utils.FromAny(valAny)
						if err != nil {
							return nil, err
						}
						foundValue = val
						valueFound = true
					}
				}
				break // Move to next table
			}
		}
	}

	if foundInTable == "" {
		return nil, fmt.Errorf("column %s not found in any table in FROM clause", ident.Name.String())
	}

	if !valueFound {
		return nil, nil // Column is in a schema, but no value in this row, return null
	}

	return foundValue, nil
}

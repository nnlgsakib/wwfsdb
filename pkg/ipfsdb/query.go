package ipfsdb

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	shell "github.com/ipfs/go-ipfs-api"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
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

	stmt, err := ssql.Parse(queryString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	selectStmt, ok := stmt.(*ast.SelectStmt)
	if !ok {
		return nil, fmt.Errorf("query is not a SELECT statement")
	}

	return QueryDB(sh, db, selectStmt)
}

// QueryDB retrieves rows from an in-memory database object
func QueryDB(sh *shell.Shell, db *pb.Database, s *ast.SelectStmt) ([]map[string]interface{}, error) {

	// Pre-calculate all table schemas involved in the query.
	tableNames := getTableNamesFromClause(s.From)
	schemas := make(map[string]*pb.Schema)
	for _, name := range tableNames {
		tableCID, ok := db.Tables[name]
		if !ok {
			return nil, fmt.Errorf("table %s not found", name)
		}
		table, err := LoadTable(sh, tableCID)
		if err != nil {
			return nil, err
		}
		schema, err := LoadSchema(sh, table.SchemaCid)
		if err != nil {
			return nil, err
		}
		schemas[name] = schema
	}

	// 1. Execute the FROM clause (including any JOINs) to get a set of combined rows.
	combinedRows, err := executeFromClause(sh, db, s.From, schemas)
	if err != nil {
		return nil, fmt.Errorf("error executing FROM/JOIN clause: %w", err)
	}

	// 2. Filter the combined rows using the WHERE clause.
	filteredRows := make([]CombinedRow, 0)
	for _, row := range combinedRows {
		include, err := evaluateExpression(row, s.Where, schemas)
		if err != nil {
			return nil, err
		}
		if include {
			filteredRows = append(filteredRows, row)
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
		return applyLimitAndOffsetToMap(results, s.Limit, s.Offset), nil
	}

	// 4. Handle ORDER BY for non-aggregate queries
	if s.OrderBy != nil {
		applyOrderBy(filteredRows, s.OrderBy, schemas)
	}

	// 5. Handle OFFSET and LIMIT for non-aggregate queries
	finalRows := applyLimitAndOffset(filteredRows, s.Limit, s.Offset)

	// 6. Project the final columns for the result set.
	results, err := projectColumns(sh, db, s.From, finalRows, s.Columns, schemas)
	if err != nil {
		return nil, fmt.Errorf("error projecting columns: %w", err)
	}

	return results, nil
}

func isAggregateQuery(s *ast.SelectStmt) bool {
	for _, col := range s.Columns {
		if _, ok := col.(*ast.AggregateFunctionExpr); ok {
			return true
		}
	}
	return false
}

func applyOrderBy(rows []CombinedRow, orderByExprs []*ast.OrderByExpression, schemas map[string]*pb.Schema) {
	sort.SliceStable(rows, func(i, j int) bool {
		for _, expr := range orderByExprs {
			valI, _ := evaluateExpressionValue(rows[i], expr.Column, schemas)
			valJ, _ := evaluateExpressionValue(rows[j], expr.Column, schemas)

			if valI == nil && valJ == nil {
				continue
			}
			if valI == nil {
				return expr.Direction != "DESC"
			}
			if valJ == nil {
				return expr.Direction == "DESC"
			}

			numI, isNumI := getNumericValue(valI)
			numJ, isNumJ := getNumericValue(valJ)
			if isNumI && isNumJ {
				if numI != numJ {
					if expr.Direction == "DESC" {
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
					if expr.Direction == "DESC" {
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

func applyLimitAndOffset(rows []CombinedRow, limitExpr, offsetExpr ast.Expression) []CombinedRow {
	offset := 0
	if offsetExpr != nil {
		offsetVal, err := evaluateExpressionValue(nil, offsetExpr, nil)
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

	limit := len(slicedRows)
	if limitExpr != nil {
		limitVal, err := evaluateExpressionValue(nil, limitExpr, nil)
		if err == nil {
			if l, ok := getIntValue(limitVal); ok {
				limit = l
			}
		}
	}

	if limit > len(slicedRows) {
		limit = len(slicedRows)
	}
	return slicedRows[:limit]
}

func applyLimitAndOffsetToMap(rows []map[string]interface{}, limitExpr, offsetExpr ast.Expression) []map[string]interface{} {
	offset := 0
	if offsetExpr != nil {
		offsetVal, err := evaluateExpressionValue(nil, offsetExpr, nil)
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

	limit := len(slicedRows)
	if limitExpr != nil {
		limitVal, err := evaluateExpressionValue(nil, limitExpr, nil)
		if err == nil {
			if l, ok := getIntValue(limitVal); ok {
				limit = l
			}
		}
	}

	if limit > len(slicedRows) {
		limit = len(slicedRows)
	}
	return slicedRows[:limit]
}

func applyOrderByToMap(results []map[string]interface{}, orderByExprs []*ast.OrderByExpression) {
	sort.SliceStable(results, func(i, j int) bool {
		for _, expr := range orderByExprs {
			var colName string

			if agg, ok := expr.Column.(*ast.AggregateFunctionExpr); ok {
				if ident, ok := agg.Argument.(*ast.Identifier); ok {
					colName = fmt.Sprintf("%s(%s)", strings.ToUpper(agg.Name), ident.Name)
				}
			} else if ident, ok := expr.Column.(*ast.Identifier); ok {
				colName = ident.Name
			} else {
				continue
			}

			valI, okI := results[i][colName]
			valJ, okJ := results[j][colName]

			if !okI || !okJ || valI == nil || valJ == nil {
				if valI == nil && valJ != nil {
					return expr.Direction != "DESC"
				}
				if valJ == nil && valI != nil {
					return expr.Direction == "DESC"
				}
				continue
			}

			numI, isNumI := getNumericValue(valI)
			numJ, isNumJ := getNumericValue(valJ)
			if isNumI && isNumJ {
				if numI != numJ {
					if expr.Direction == "DESC" {
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
					if expr.Direction == "DESC" {
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

func executeAggregateQuery(rows []CombinedRow, s *ast.SelectStmt, schemas map[string]*pb.Schema) ([]map[string]interface{}, error) {
	// Handle non-grouped aggregate query
	if s.GroupBy == nil {
		resultRow := make(map[string]interface{})
		for _, colExpr := range s.Columns {
			aggExpr, ok := colExpr.(*ast.AggregateFunctionExpr)
			if !ok {
				return nil, fmt.Errorf("cannot mix aggregate and non-aggregate columns without GROUP BY")
			}

			var argName string
			if ident, ok := aggExpr.Argument.(*ast.Identifier); ok {
				argName = ident.Name
			}
			resultColName := fmt.Sprintf("%s(%s)", strings.ToUpper(aggExpr.Name), argName)

			switch strings.ToUpper(aggExpr.Name) {
			case "COUNT":
				resultRow[resultColName] = len(rows)
			case "SUM", "AVG", "MIN", "MAX":
				if argName == "*" {
					return nil, fmt.Errorf("%s requires a column argument", strings.ToUpper(aggExpr.Name))
				}
				var total float64
				var min float64
				var max float64
				count := 0
				for i, row := range rows {
					val, err := evaluateIdentifier(row, aggExpr.Argument.(*ast.Identifier), schemas)
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

				switch strings.ToUpper(aggExpr.Name) {
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
				return nil, fmt.Errorf("unsupported aggregate function: %s", aggExpr.Name)
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
			if ident, ok := expr.(*ast.Identifier); ok {
				keyValues[ident.Name] = val
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

		for _, colExpr := range s.Columns {
			if aggExpr, ok := colExpr.(*ast.AggregateFunctionExpr); ok {
				var argName string
				if ident, ok := aggExpr.Argument.(*ast.Identifier); ok {
					argName = ident.Name
				}
				resultColName := fmt.Sprintf("%s(%s)", strings.ToUpper(aggExpr.Name), argName)

				switch strings.ToUpper(aggExpr.Name) {
				case "COUNT":
					resultRow[resultColName] = len(group.Rows)
				case "SUM", "AVG", "MIN", "MAX":
					if argName == "*" {
						return nil, fmt.Errorf("%s requires a column argument", strings.ToUpper(aggExpr.Name))
					}
					var total float64
					var min float64
					var max float64
					count := 0
					for i, row := range group.Rows {
						val, err := evaluateIdentifier(row, aggExpr.Argument.(*ast.Identifier), schemas)
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

					switch strings.ToUpper(aggExpr.Name) {
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
		finalResults = append(finalResults, resultRow)
	}

	return finalResults, nil
}

// executeFromClause is the core of the JOIN implementation. It recursively processes the FromClause.
func executeFromClause(sh *shell.Shell, db *pb.Database, from ast.FromClause, schemas map[string]*pb.Schema) ([]CombinedRow, error) {
	switch f := from.(type) {
	case *ast.TableIdentifier:
		// Base case: load all rows from a single table by iterating through its pages.
		tableCID, ok := db.Tables[f.Name]
		if !ok {
			return nil, fmt.Errorf("table %s not found in database", f.Name)
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
				results = append(results, CombinedRow{f.Name: row})
			}
		}
		return results, nil

	case *ast.JoinClause:
		// Recursive step: perform a join.
		leftRows, err := executeFromClause(sh, db, f.Left, schemas)
		if err != nil {
			return nil, err
		}
		rightRows, err := executeFromClause(sh, db, f.Right, schemas)
		if err != nil {
			return nil, err
		}

		var joinedRows []CombinedRow
		leftMatched := make(map[int]bool)

		for i, lRow := range leftRows {
			matchFound := false
			for _, rRow := range rightRows {
				tempRow := make(CombinedRow)
				for k, v := range lRow {
					tempRow[k] = v
				}
				for k, v := range rRow {
					tempRow[k] = v
				}

				match, err := evaluateExpression(tempRow, f.On, schemas)
				if err != nil {
					return nil, fmt.Errorf("error evaluating ON condition: %w", err)
				}

				if match {
					matchFound = true
					leftMatched[i] = true
					joinedRows = append(joinedRows, tempRow)
				}
			}
			if !matchFound && f.Type == "LEFT" {
				tempRow := make(CombinedRow)
				for k, v := range lRow {
					tempRow[k] = v
				}
				rightTableNames := getTableNamesFromClause(f.Right)
				for _, name := range rightTableNames {
					tempRow[name] = nil
				}
				joinedRows = append(joinedRows, tempRow)
			}
		}

		return joinedRows, nil
	}
	return nil, fmt.Errorf("unsupported FROM clause type: %T", from)
}

// projectColumns creates the final result set based on the selected columns.
func projectColumns(sh *shell.Shell, db *pb.Database, from ast.FromClause, rows []CombinedRow, columns []ast.SelectExpr, schemas map[string]*pb.Schema) ([]map[string]interface{}, error) {
	results := make([]map[string]interface{}, 0, len(rows))

	for _, combinedRow := range rows {
		resultRow := make(map[string]interface{})
		for _, colExpr := range columns {
			switch c := colExpr.(type) {
			case *ast.StarExpr:
				for tableName, rowData := range combinedRow {
					if rowData == nil {
						for _, schemaCol := range schemas[tableName].Columns {
							resultRow[tableName+"."+schemaCol.Name] = nil
						}
						continue
					}
					for colName, valAny := range rowData.Values {
						val, _ := FromAny(valAny)
						resultRow[tableName+"."+colName] = val
					}
				}
			case *ast.ColumnExpr:
				if c.Name == "*" {
					tableName := c.TableQualifier
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
						val, _ := FromAny(valAny)
						resultRow[tableName+"."+colName] = val
					}
				} else {
					val, err := evaluateIdentifier(combinedRow, &ast.Identifier{Name: c.Name, TableQualifier: c.TableQualifier}, schemas)
					if err != nil {
						return nil, err
					}
					finalColName := c.Name
					if c.TableQualifier != "" {
						finalColName = c.TableQualifier + "." + c.Name
					}
					resultRow[finalColName] = val
				}
			}
		}
		results = append(results, resultRow)
	}
	return results, nil
}

// getTableNamesFromClause recursively finds all table names involved in a FromClause.
func getTableNamesFromClause(from ast.FromClause) []string {
	switch f := from.(type) {
	case *ast.TableIdentifier:
		return []string{f.Name}
	case *ast.JoinClause:
		return append(getTableNamesFromClause(f.Left), getTableNamesFromClause(f.Right)...)
	}
	return nil
}

func evaluateExpression(row CombinedRow, expr ast.Expression, schemas map[string]*pb.Schema) (bool, error) {
	if expr == nil {
		return true, nil
	}

	switch e := expr.(type) {
	case *ast.BinaryExpr:
		left, err := evaluateExpression(row, e.Left, schemas)
		if err != nil {
			return false, err
		}
		right, err := evaluateExpression(row, e.Right, schemas)
		if err != nil {
			return false, err
		}

		switch e.Operator {
		case "AND":
			return left && right, nil
		case "OR":
			return left || right, nil
		default:
			return false, fmt.Errorf("unsupported binary operator: %s", e.Operator)
		}

	case *ast.ComparisonExpr:
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
	case *ast.LikeExpr:
		left, err := evaluateExpressionValue(row, e.Left, schemas)
		if err != nil {
			return false, err
		}
		pattern, err := evaluateExpressionValue(row, e.Pattern, schemas)
		if err != nil {
			return false, err
		}

		leftStr, okLeft := left.(string)
		patternStr, okPattern := pattern.(string)
		if !okLeft || !okPattern {
			return false, fmt.Errorf("LIKE operator requires string operands, got %T and %T", left, pattern)
		}

		matchPattern := strings.ReplaceAll(patternStr, "#", "*")
		matchPattern = strings.ReplaceAll(matchPattern, "_", "?")

		return filepath.Match(matchPattern, leftStr)

	case *ast.InExpr:
		left, err := evaluateExpressionValue(row, e.Left, schemas)
		if err != nil {
			return false, err
		}

		for _, valExpr := range e.Values {
			right, err := evaluateExpressionValue(row, valExpr, schemas)
			if err != nil {
				return false, err
			}
			if left == right {
				return true, nil
			}
		}
		return false, nil

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

func evaluateExpressionValue(row CombinedRow, expr ast.Expression, schemas map[string]*pb.Schema) (interface{}, error) {
	switch e := expr.(type) {
	case *ast.Identifier:
		return evaluateIdentifier(row, e, schemas)
	case *ast.Literal:
		return e.Value, nil
	case *ast.NumberLiteral:
		return e.Value, nil
	case *ast.BooleanLiteral:
		return e.Value, nil
	case *ast.PrefixExpression:
		right, err := evaluateExpressionValue(row, e.Right, schemas)
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
	default:
		return nil, fmt.Errorf("unsupported expression value type: %T", e)
	}
}

// evaluateIdentifier resolves an identifier (e.g., `users.id` or `id`) against a combined row.
// It is now schema-aware.
func evaluateIdentifier(row CombinedRow, ident *ast.Identifier, schemas map[string]*pb.Schema) (interface{}, error) {
	// Case 1: Identifier is fully qualified (e.g., users.id)
	if ident.TableQualifier != "" {
		tableSchema, ok := schemas[ident.TableQualifier]
		if !ok {
			return nil, fmt.Errorf("table %s not found in FROM clause", ident.TableQualifier)
		}

		// Check if column exists in schema
		foundInSchema := false
		for _, col := range tableSchema.Columns {
			if col.Name == ident.Name {
				foundInSchema = true
				break
			}
		}
		if !foundInSchema {
			return nil, fmt.Errorf("column %s not found in table %s", ident.Name, ident.TableQualifier)
		}

		tableData, ok := row[ident.TableQualifier]
		if !ok {
			return nil, fmt.Errorf("table %s not found in FROM clause", ident.TableQualifier)
		}
		if tableData == nil { // This happens in a LEFT JOIN where there was no match
			return nil, nil
		}

		valAny, ok := tableData.Values[ident.Name]
		if !ok {
			return nil, nil // Column is in schema but not in this specific row's data, return null
		}
		return FromAny(valAny)
	}

	// Case 2: Identifier is unqualified (e.g., id). We need to find which table it belongs to.
	var foundValue interface{}
	var foundInTable string
	var valueFound bool

	for tableName, tableSchema := range schemas {
		for _, col := range tableSchema.Columns {
			if col.Name == ident.Name {
				// Found a table with this column in its schema
				if foundInTable != "" {
					return nil, fmt.Errorf("ambiguous column name: %s exists in multiple tables", ident.Name)
				}
				foundInTable = tableName

				// Now get the value from the row data if it exists
			if tableData, ok := row[tableName]; ok && tableData != nil {
				if valAny, dataOk := tableData.Values[ident.Name]; dataOk {
					val, err := FromAny(valAny)
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
		return nil, fmt.Errorf("column %s not found in any table in FROM clause", ident.Name)
	}

	if !valueFound {
		return nil, nil // Column is in a schema, but no value in this row, return null
	}

	return foundValue, nil
}

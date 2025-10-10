package ipfsdb

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	shell "github.com/ipfs/go-ipfs-api"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
)

// CombinedRow represents a row resulting from a join, mapping table names to their respective row data.
type CombinedRow map[string]*pb.Row

// Query retrieves rows from a table
func Query(ipfsAPI, dbName string, columns []ast.SelectColumn, from ast.FromClause, where ast.Expression) ([]map[string]interface{}, error) {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return nil, err
	}

	return QueryDB(sh, db, columns, from, where)
}

// QueryDB retrieves rows from an in-memory database object
func QueryDB(sh *shell.Shell, db *pb.Database, columns []ast.SelectColumn, from ast.FromClause, where ast.Expression) ([]map[string]interface{}, error) {
	// Pre-calculate all table schemas involved in the query.
	tableNames := getTableNamesFromClause(from)
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
	combinedRows, err := executeFromClause(sh, db, from)
	if err != nil {
		return nil, fmt.Errorf("error executing FROM/JOIN clause: %w", err)
	}

	// 2. Filter the combined rows using the WHERE clause.
	filteredRows := make([]CombinedRow, 0)
	for _, row := range combinedRows {
		include, err := evaluateExpression(row, where, schemas)
		if err != nil {
			return nil, err
		}
		if include {
			filteredRows = append(filteredRows, row)
		}
	}

	// 3. Project the final columns for the result set.
	results, err := projectColumns(sh, db, from, filteredRows, columns, schemas)
	if err != nil {
		return nil, fmt.Errorf("error projecting columns: %w", err)
	}

	return results, nil
}

// executeFromClause is the core of the JOIN implementation. It recursively processes the FromClause.
func executeFromClause(sh *shell.Shell, db *pb.Database, from ast.FromClause) ([]CombinedRow, error) {
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
				// It's better to log this error than to fail the whole query
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
		leftRows, err := executeFromClause(sh, db, f.Left)
		if err != nil {
			return nil, err
		}
		rightRows, err := executeFromClause(sh, db, f.Right)
		if err != nil {
			return nil, err
		}

		var joinedRows []CombinedRow
		leftMatched := make(map[int]bool) // Keep track of matched left rows for LEFT JOIN

		// Pre-calculate schemas for expression evaluation
		// This is a simplified approach; a full query planner would do this more elegantly.
		tableNames := getTableNamesFromClause(from)
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

		// Nested loop join algorithm.
		for i, lRow := range leftRows {
			matchFound := false
			for _, rRow := range rightRows {
				// Create a temporary merged row for evaluating the ON condition.
				tempRow := make(CombinedRow)
				for k, v := range lRow {
					tempRow[k] = v
				}
				for k, v := range rRow {
					tempRow[k] = v
				}

				// Evaluate the ON condition.
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
			// For LEFT JOIN, if no match was found for the left row, add it with nulls for the right side.
			if !matchFound && f.Type == "LEFT" {
				// Create a row with nil values for the right table(s).
				tempRow := make(CombinedRow)
				for k, v := range lRow {
					tempRow[k] = v
				}
				// Get the tables from the right side of the join to add nil placeholders.
				rightTableNames := getTableNamesFromClause(f.Right)
				for _, name := range rightTableNames {
					tempRow[name] = nil // Represent a row of nulls
				}
				joinedRows = append(joinedRows, tempRow)
			}
		}

		return joinedRows, nil
	}
	return nil, fmt.Errorf("unsupported FROM clause type: %T", from)
}

// projectColumns creates the final result set based on the selected columns.
func projectColumns(sh *shell.Shell, db *pb.Database, from ast.FromClause, rows []CombinedRow, columns []ast.SelectColumn, schemas map[string]*pb.Schema) ([]map[string]interface{}, error) {
	results := make([]map[string]interface{}, 0, len(rows))

	for _, combinedRow := range rows {
		resultRow := make(map[string]interface{})
		for _, col := range columns {
			// Handle SELECT *
			if col.Name == "*" && col.TableQualifier == "" {
				for tableName, rowData := range combinedRow {
					if rowData == nil { // Handle case for LEFT JOIN with no match
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
				continue
			}

			// Handle SELECT table.*
			if col.Name == "*" {
				tableName := col.TableQualifier
				rowData, ok := combinedRow[tableName]
				if !ok {
					return nil, fmt.Errorf("table %s not found in query", tableName)
				}
				if rowData == nil { // Handle case for LEFT JOIN with no match
					for _, schemaCol := range schemas[tableName].Columns {
						resultRow[tableName+"."+schemaCol.Name] = nil
					}
					continue
				}
				for colName, valAny := range rowData.Values {
					val, _ := FromAny(valAny)
					resultRow[tableName+"."+colName] = val
				}
				continue
			}

			// Handle specific column (e.g., users.id or just id)
			val, err := evaluateIdentifier(combinedRow, &ast.Identifier{Name: col.Name, TableQualifier: col.TableQualifier}, schemas)
			if err != nil {
				return nil, err
			}
			// Determine the final column name in the result set.
			finalColName := col.Name
			if col.TableQualifier != "" {
				finalColName = col.TableQualifier + "." + col.Name
			}
			resultRow[finalColName] = val
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

		// Handle nil for LEFT JOIN cases
		if left == nil || right == nil {
			switch e.Operator {
			case "=":
				return left == right, nil
			case "!=":
				return left != right, nil
			default:
				return false, nil // Other comparisons with NULL are false
			}
		}

		// Try numeric comparison first
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

		// Try boolean comparison
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

		// Fallback to string comparison
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

		matchPattern := strings.ReplaceAll(patternStr, "_#", "?")
		matchPattern = strings.ReplaceAll(matchPattern, "#", "*")

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

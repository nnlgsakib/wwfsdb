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

// Query retrieves rows from a table
func Query(ipfsAPI, dbName, tableName string, columns []string, where ast.Expression) ([]map[string]interface{}, error) {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return nil, err
	}

	return QueryDB(sh, db, tableName, columns, where)
}

// QueryDB retrieves rows from an in-memory database object
func QueryDB(sh *shell.Shell, db *pb.Database, tableName string, columns []string, where ast.Expression) ([]map[string]interface{}, error) {
	// 2. Get the table CID from the database
	tableCID, ok := db.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table %s not found in database", tableName)
	}

	// 3. Load the table
	table, err := LoadTable(sh, tableCID)
	if err != nil {
		return nil, err
	}

	// 4. Find candidate rows (either from index or full scan)
	rowCIDs, err := findCandidateRows(sh, table, where)
	if err != nil {
		return nil, err
	}

	// 5. Load the schema
	schema, err := LoadSchema(sh, table.SchemaCid)
	if err != nil {
		return nil, err
	}

	// 6. Iterate over the candidate rows and filter based on the WHERE clause
	results := make([]map[string]interface{}, 0)
	for _, rowCID := range rowCIDs {
		row, err := LoadRow(sh, rowCID)
		if err != nil {
			return nil, err
		}

		include, err := evaluateExpression(row, where)
		if err != nil {
			return nil, err
		}

		if include {
			// Filter columns
			resultRow := make(map[string]interface{})
			if len(columns) == 1 && columns[0] == "*" {
				for _, col := range schema.Columns {
					val, err := FromAny(row.Values[col.Name])
					if err != nil {
						return nil, err
					}
					resultRow[col.Name] = val
				}
			} else {
				for _, colName := range columns {
					val, err := FromAny(row.Values[colName])
					if err != nil {
						return nil, err
					}
					resultRow[colName] = val
				}
			}
			results = append(results, resultRow)
		}
	}

	return results, nil
}

// findCandidateRows tries to use an index to narrow down the list of rows to scan.
// If it can't use an index, it returns all row CIDs for a full table scan.
func findCandidateRows(sh *shell.Shell, table *pb.Table, where ast.Expression) ([]string, error) {
	// Check if we can use an index. For now, we only support simple `col = val` queries.
	if comp, ok := where.(*ast.ComparisonExpr); ok && comp.Operator == "=" {
		if ident, ok := comp.Left.(*ast.Identifier); ok {
			if indexCID, ok := table.Indexes[ident.Name]; ok {
				// Index exists for this column. Let's try to use it.
				if lit, ok := comp.Right.(*ast.Literal); ok {
					// Load the index
					index, err := LoadIndex(sh, indexCID)
					if err != nil {
						return nil, fmt.Errorf("failed to load index %s: %w", indexCID, err)
					}

					// The key needs to be validated and cast just like during insertion
					// For now, we'll just use the literal string value. This is a simplification
					// and might not work for all types without proper casting.
					key := lit.Value
					if node, ok := index.Nodes[key]; ok {
						return node.Cids, nil // Found candidate rows from index!
					} else {
						return []string{}, nil // Value not in index, so no results
					}
				}
			}
		}
	}

	// If we reach here, we couldn't use an index, so return all rows for a full scan.
	return table.Rows, nil
}

func evaluateExpression(row *pb.Row, expr ast.Expression) (bool, error) {
	if expr == nil {
		return true, nil
	}

	switch e := expr.(type) {
	case *ast.BinaryExpr:
		left, err := evaluateExpression(row, e.Left)
		if err != nil {
			return false, err
		}
		right, err := evaluateExpression(row, e.Right)
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
		left, err := evaluateExpressionValue(row, e.Left)
		if err != nil {
			return false, err
		}
		right, err := evaluateExpressionValue(row, e.Right)
		if err != nil {
			return false, err
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
		left, err := evaluateExpressionValue(row, e.Left)
		if err != nil {
			return false, err
		}
		pattern, err := evaluateExpressionValue(row, e.Pattern)
		if err != nil {
			return false, err
		}

		leftStr, okLeft := left.(string)
		patternStr, okPattern := pattern.(string)
		if !okLeft || !okPattern {
			return false, fmt.Errorf("LIKE operator requires string operands, got %T and %T", left, pattern)
		}

		matchPattern := strings.ReplaceAll(patternStr, "%", "*")
		matchPattern = strings.ReplaceAll(matchPattern, "_", "?")

		return filepath.Match(matchPattern, leftStr)

	case *ast.InExpr:
		left, err := evaluateExpressionValue(row, e.Left)
		if err != nil {
			return false, err
		}

		for _, valExpr := range e.Values {
			right, err := evaluateExpressionValue(row, valExpr)
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

func evaluateExpressionValue(row *pb.Row, expr ast.Expression) (interface{}, error) {
	switch e := expr.(type) {
	case *ast.Identifier:
		anyVal, ok := row.Values[e.Name]
		if !ok {
			return nil, fmt.Errorf("column %s not found in row", e.Name)
		}
		return FromAny(anyVal)
	case *ast.Literal:
		return e.Value, nil
	case *ast.NumberLiteral:
		return e.Value, nil
	case *ast.BooleanLiteral:
		return e.Value, nil
	case *ast.PrefixExpression:
		right, err := evaluateExpressionValue(row, e.Right)
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

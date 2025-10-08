package ipfsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	shell "github.com/ipfs/go-ipfs-api"
	ssql "github.com/nnlgsakib/wwfsdb/pkg/ssql"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
)

// Insert adds a new row to a table
func Insert(ipfsAPI, dbName, tableName string, values []string) error {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return err
	}

	// 2. Get the table CID from the database
	tableCID, ok := db.Tables[tableName]
	if !ok {
		return fmt.Errorf("table %s not found in database %s", tableName, dbName)
	}

	// 3. Load the table
	table, err := LoadTable(sh, tableCID)
	if err != nil {
		return err
	}

	// 4. Create the new row
	row := make(map[string]interface{})
	schema, err := LoadSchema(sh, table.SchemaCID)
	if err != nil {
		return err
	}

	if len(values) != len(schema.Columns) {
		return fmt.Errorf("incorrect number of values for insert statement")
	}

	for i, col := range schema.Columns {
		// Validate and cast the value based on the column type
		val, err := ValidateAndCastValue(values[i], col.Type)
		if err != nil {
			return fmt.Errorf("validation error for column '%s': %w", col.Name, err)
		}
		row[col.Name] = val
	}

	// 5. Add the new row to IPFS
	rowCID, err := AddObject(sh, row)
	if err != nil {
		return err
	}

	// 6. Update the table with the new row CID
	table.Rows = append(table.Rows, rowCID)

	// 7. Update indexes
	table, err = UpdateIndexesOnInsert(sh, table, rowCID, row)
	if err != nil {
		return fmt.Errorf("failed to update indexes on insert: %w", err)
	}

	// 7. Update the table in IPFS
	newTableCID, err := AddObject(sh, table)
	if err != nil {
		return err
	}

	// 8. Update the database with the new table CID
	db.Tables[tableName] = newTableCID

	// 9. Update the database in IPFS
	newDbCID, err := AddObject(sh, db)
	if err != nil {
		return err
	}

	// 10. Update the cache
	UpdateCache(dbName, newDbCID)

	// 11. Update the IPNS record in the background
	publishAsync(sh, dbName, newDbCID)
	return nil
}

// Query retrieves rows from a table
func Query(ipfsAPI, dbName, tableName string, columns []string, where ast.Expression) ([]map[string]interface{}, error) {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return nil, err
	}

	// 2. Get the table CID from the database
	tableCID, ok := db.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table %s not found in database %s", tableName, dbName)
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
	schema, err := LoadSchema(sh, table.SchemaCID)
	if err != nil {
		return nil, err
	}

	// 6. Iterate over the candidate rows and filter based on the WHERE clause
	var results []map[string]interface{}
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
					resultRow[col.Name] = row[col.Name]
				}
			} else {
				for _, colName := range columns {
					resultRow[colName] = row[colName]
				}
			}
			results = append(results, resultRow)
		}
	}

	return results, nil
}

// findCandidateRows tries to use an index to narrow down the list of rows to scan.
// If it can't use an index, it returns all row CIDs for a full table scan.
func findCandidateRows(sh *shell.Shell, table *Table, where ast.Expression) ([]string, error) {
	// Check if we can use an index. For now, we only support simple `col = val` queries.
	if comp, ok := where.(*ast.ComparisonExpr); ok && comp.Operator == "=" {
		if ident, ok := comp.Left.(*ast.Identifier); ok {
			if indexCID, ok := table.Indexes[ident.Name]; ok {
				// Index exists for this column. Let's try to use it.
				if lit, ok := comp.Right.(*ast.Literal); ok {
					// Load the index
					indexData, err := sh.Cat(indexCID)
					if err != nil {
						return nil, fmt.Errorf("failed to load index %s: %w", indexCID, err)
					}
					defer indexData.Close()
					var index Index
					if err := json.NewDecoder(indexData).Decode(&index); err != nil {
						return nil, fmt.Errorf("failed to decode index %s: %w", indexCID, err)
					}

					// The key needs to be validated and cast just like during insertion
					// For now, we'll just use the literal string value. This is a simplification
					// and might not work for all types without proper casting.
					key := lit.Value
					if cids, ok := index[key]; ok {
						return cids, nil // Found candidate rows from index!
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

func evaluateExpression(row map[string]interface{}, expr ast.Expression) (bool, error) {
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
	case json.Number:
		f, err := val.Float64()
		if err == nil {
			return f, true
		}
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}

func evaluateExpressionValue(row map[string]interface{}, expr ast.Expression) (interface{}, error) {
	switch e := expr.(type) {
	case *ast.Identifier:
		return row[e.Name], nil
	case *ast.Literal:
		return e.Value, nil
	case *ast.NumberLiteral:
		return e.Value, nil
	case *ast.BooleanLiteral:
		return e.Value, nil
	default:
		return nil, fmt.Errorf("unsupported expression value type: %T", e)
	}
}

// LoadDatabase loads a database from IPFS
func LoadDatabase(sh *shell.Shell, dbName string) (*Database, error) {
	var dbCID string
	var err error

	// 1. Try to load the database CID from cache
	cachedCID, ok := ReadCache(dbName)
	if ok {
		dbCID = cachedCID
	} else {
		// 2. If not in cache, read the registry from LevelDB to get the program ID
		entryData, err := GetFromCache([]byte("registry:" + dbName))
		if err != nil {
			return nil, fmt.Errorf("database %s not found in registry", dbName)
		}

		var entry RegistryEntry
		if err := json.Unmarshal(entryData, &entry); err != nil {
			return nil, err
		}

		// 3. Resolve the IPNS name to get the database CID
		dbCID, err = sh.Resolve(entry.ProgramID)
		if err != nil {
			return nil, err
		}

		// 4. Update the cache with the resolved CID
		UpdateCache(dbName, dbCID)
	}

	// 5. Cat the database object
	data, err := sh.Cat(dbCID)
	if err != nil {
		return nil, err
	}
	defer data.Close()

	// 6. Decode the database object
	var db Database
	if err := json.NewDecoder(data).Decode(&db); err != nil {
		return nil, err
	}

	return &db, nil
}

// LoadTable loads a table from IPFS
func LoadTable(sh *shell.Shell, tableCID string) (*Table, error) {
	data, err := sh.Cat(tableCID)
	if err != nil {
		return nil, err
	}
	defer data.Close()

	var table Table
	if err := json.NewDecoder(data).Decode(&table); err != nil {
		return nil, err
	}

	return &table, nil
}

// LoadSchema loads a schema from IPFS
func LoadSchema(sh *shell.Shell, schemaCID string) (*ssql.Schema, error) {
	data, err := sh.Cat(schemaCID)
	if err != nil {
		return nil, err
	}
	defer data.Close()

	var schema ssql.Schema
	if err := json.NewDecoder(data).Decode(&schema); err != nil {
		return nil, err
	}

	return &schema, nil
}

// LoadRow loads a row from IPFS
func LoadRow(sh *shell.Shell, rowCID string) (map[string]interface{}, error) {
	data, err := sh.Cat(rowCID)
	if err != nil {
		return nil, err
	}
	defer data.Close()

	var row map[string]interface{}
	if err := json.NewDecoder(data).Decode(&row); err != nil {
		return nil, err
	}

	return row, nil
}

// AddObject adds a generic object to IPFS and returns its CID
func AddObject(sh *shell.Shell, obj interface{}) (string, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	return sh.Add(bytes.NewReader(data))
}

// publishAsync updates the IPNS record for a database in the background
func publishAsync(sh *shell.Shell, dbName, cid string) {
	go func() {
		if err := Publish(sh, dbName, cid); err != nil {
			fmt.Fprintf(os.Stderr, "Error publishing to IPNS: %v\n", err)
		}
	}()
}

// Publish updates the IPNS record for a database
func Publish(sh *shell.Shell, dbName, cid string) error {
	entryData, err := GetFromCache([]byte("registry:" + dbName))
	if err != nil {
		return fmt.Errorf("database %s not found in registry", dbName)
	}

	var entry RegistryEntry
	if err := json.Unmarshal(entryData, &entry); err != nil {
		return err
	}

	_, err = sh.PublishWithDetails(cid, entry.KeyName, 0, 0, false)
	return err
}

// CreateDatabase creates a new database
func CreateDatabase(ipfsAPI, dbName string) (string, error) {
	sh := shell.NewShell(ipfsAPI)

	// 1. Create a new database
	db := &Database{
		Tables: make(map[string]string),
	}

	// 2. Add the database to IPFS
	dbCID, err := AddObject(sh, db)
	if err != nil {
		return "", err
	}

	// 3. Create a new IPNS key
	key, err := sh.KeyGen(context.Background(), dbName, shell.KeyGen.Size(2048))
	if err != nil {
		return "", err
	}

	// 4. Publish the database CID to the new key in the background
	go func() {
		_, err := sh.PublishWithDetails(dbCID, key.Name, 0, 0, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error publishing to IPNS for new database: %v\n", err)
		}
	}()

	// 5. Save the database info to the registry
	entry := RegistryEntry{
		DbName:    dbName,
		ProgramID: key.Id,
		KeyName:   key.Name,
	}
	entryData, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}

	err = PutToCache([]byte("registry:"+dbName), entryData)
	if err != nil {
		return "", err
	}

	// Update the cache with the new database CID
	UpdateCache(dbName, dbCID)

	return key.Id, nil
}

// ExecuteQuery parses and executes a query
func ExecuteQuery(ipfsAPI, dbName, query string) (string, error) {
	// Use the new parser
	stmt, err := ssql.Parse(query)
	if err != nil {
		// Also try parsing as a multi-statement script for CREATE TABLE files
		if stmts, err2 := ssql.ParseMultiple(query); err2 == nil && len(stmts) > 0 {
			// For now, we only support multi-statement scripts for CREATE TABLE
			var results []string
			for _, s := range stmts {
				if ct, ok := s.(*ast.CreateTableStmt); ok {
					_, err := Migrate(ipfsAPI, dbName, ct.Name, &ct.Schema)
					if err != nil {
						return "", err
					}
					results = append(results, fmt.Sprintf("Table '%s' created successfully in database '%s'.", ct.Name, dbName))
				} else {
					return "", fmt.Errorf("unsupported statement in multi-statement query: %T", s)
				}
			}
			return strings.Join(results, "\n"), nil
		}
		return "", err // Return original error if multi-parse also fails
	}

	switch s := stmt.(type) {
	case *ast.CreateDatabaseStmt:
		_, err := CreateDatabase(ipfsAPI, s.Name)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Database '%s' created successfully.", s.Name), nil
	case *ast.CreateTableStmt:
		_, err := Migrate(ipfsAPI, dbName, s.Name, &s.Schema)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Table '%s' created successfully in database '%s'.", s.Name, dbName), nil
	case *ast.DropTableStmt:
		err := Drop(ipfsAPI, dbName, s.Name)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Table '%s' dropped successfully.", s.Name), nil
	case *ast.SelectStmt:
		rows, err := Query(ipfsAPI, dbName, s.Table, s.Columns, s.Where)
		if err != nil {
			return "", err
		}

		jsonResult, err := json.Marshal(rows)
		if err != nil {
			return "", err
		}

		return string(jsonResult), nil

	case *ast.InsertStmt:
		err = Insert(ipfsAPI, dbName, s.Table, s.Values)
		if err != nil {
			return "", err
		}

		return "INSERT successful", nil

	case *ast.UpdateStmt:
		err = Update(ipfsAPI, dbName, s.Table, s.Set.Column, s.Set.Value, s.Where)
		if err != nil {
			return "", err
		}

		return "UPDATE successful", nil

	case *ast.DeleteStmt:
		err = Delete(ipfsAPI, dbName, s.Table, s.Where)
		if err != nil {
			return "", err
		}

		return "DELETE successful", nil

	case *ast.CreateIndexStmt:
		err = CreateIndex(ipfsAPI, dbName, s.Table, s.Column)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Index on column '%s' for table '%s' created successfully.", s.Column, s.Table), nil
	default:
		return "", fmt.Errorf("unsupported query type: %T", s)
	}
}

// Migrate adds a new table to the database
func Migrate(ipfsAPI, dbName, tableName string, schema *ssql.Schema) (string, error) {
	sh := shell.NewShell(ipfsAPI)

	// 1. Add the schema to IPFS
	schemaCID, err := AddObject(sh, schema)
	if err != nil {
		return "", err
	}

	// 2. Create a new table
	table := &Table{
		SchemaCID: schemaCID,
		Rows:      []string{},
		Indexes:   make(map[string]string),
	}

	// 3. Add the table to IPFS
	tableCID, err := AddObject(sh, table)
	if err != nil {
		return "", err
	}

	// 4. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return "", err
	}

	// 5. Add the new table to the database
	db.Tables[tableName] = tableCID

	// 6. Update the database in IPFS
	newDbCID, err := AddObject(sh, db)
	if err != nil {
		return "", err
	}

	// 7. Update the cache
	UpdateCache(dbName, newDbCID)

	// 8. Update the IPNS record in the background
	publishAsync(sh, dbName, newDbCID)

	return newDbCID, nil
}

// Drop removes a table from the database
func Drop(ipfsAPI, dbName, tableName string) error {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return err
	}

	// 2. Check if the table exists
	if _, ok := db.Tables[tableName]; !ok {
		return fmt.Errorf("table %s not found in database %s", tableName, dbName)
	}

	// 3. Remove the table from the database
	delete(db.Tables, tableName)

	// 4. Update the database in IPFS
	newDbCID, err := AddObject(sh, db)
	if err != nil {
		return err
	}

	// 5. Update the cache
	UpdateCache(dbName, newDbCID)

	// 6. Update the IPNS record in the background
	publishAsync(sh, dbName, newDbCID)
	return nil
}

// Update modifies a row in a table
func Update(ipfsAPI, dbName, tableName, setColumn, setValue string, where ast.Expression) error {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return err
	}

	// 2. Get the table CID from the database
	tableCID, ok := db.Tables[tableName]
	if !ok {
		return fmt.Errorf("table %s not found in database %s", tableName, dbName)
	}

	// 3. Load the table
	table, err := LoadTable(sh, tableCID)
	if err != nil {
		return err
	}

	// 4. Load the schema to get column types
	schema, err := LoadSchema(sh, table.SchemaCID)
	if err != nil {
		return err
	}

	var columnType string
	for _, col := range schema.Columns {
		if col.Name == setColumn {
			columnType = col.Type
			break
		}
	}

	if columnType == "" {
		return fmt.Errorf("column '%s' not found in table '%s'", setColumn, tableName)
	}

	// 5. Validate and cast the new value
	castedValue, err := ValidateAndCastValue(setValue, columnType)
	if err != nil {
		return fmt.Errorf("validation error for column '%s': %w", setColumn, err)
	}

	// 6. Find and update rows
	var updated bool
	for i, rowCID := range table.Rows {
		row, err := LoadRow(sh, rowCID)
		if err != nil {
			return err
		}

		include, err := evaluateExpression(row, where)
		if err != nil {
			return err
		}

					if include {
						// Capture old state for index update
						oldRow := make(map[string]interface{})
						for k, v := range row {
							oldRow[k] = v
						}
		
						// First, update indexes as if the old row is being deleted
						table, err = UpdateIndexesOnDelete(sh, table, rowCID, oldRow)
						if err != nil {
							return fmt.Errorf("failed to update indexes on delete part of update: %w", err)
						}
		
						// Update the row data with the new value
						row[setColumn] = castedValue
		
						// Add the updated row to IPFS to get a new CID
						newRowCID, err := AddObject(sh, row)
						if err != nil {
							return err
						}
		
						// Now, update indexes as if the new row is being inserted
						table, err = UpdateIndexesOnInsert(sh, table, newRowCID, row)
						if err != nil {
							return fmt.Errorf("failed to update indexes on insert part of update: %w", err)
						}
		
						// Update the table with the new row CID
						table.Rows[i] = newRowCID
						updated = true
					}	}

	if !updated {
		return fmt.Errorf("no rows found to update")
	}

	// 10. Update the table in IPFS
	newTableCID, err := AddObject(sh, table)
	if err != nil {
		return err
	}

	// 11. Update the database with the new table CID
	db.Tables[tableName] = newTableCID

	// 12. Update the database in IPFS
	newDbCID, err := AddObject(sh, db)
	if err != nil {
		return err
	}

	// 13. Update the cache
	UpdateCache(dbName, newDbCID)

	// 14. Update the IPNS record in the background
	publishAsync(sh, dbName, newDbCID)
	return nil
}

// Delete removes a row from a table
func Delete(ipfsAPI, dbName, tableName string, where ast.Expression) error {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return err
	}

	// 2. Get the table CID from the database
	tableCID, ok := db.Tables[tableName]
	if !ok {
		return fmt.Errorf("table %s not found in database %s", tableName, dbName)
	}

	// 3. Load the table
	table, err := LoadTable(sh, tableCID)
	if err != nil {
		return err
	}

	// 4. Find the row to delete
	var newRows []string
	var deleted bool
	for _, rowCID := range table.Rows {
		row, err := LoadRow(sh, rowCID)
		if err != nil {
			return err
		}

		include, err := evaluateExpression(row, where)
		if err != nil {
			return err
		}

		if include {
			deleted = true
			// Update indexes before deleting the row
			table, err = UpdateIndexesOnDelete(sh, table, rowCID, row)
			if err != nil {
				return fmt.Errorf("failed to update indexes on delete: %w", err)
			}
		} else {
			newRows = append(newRows, rowCID)
		}
	}

	if !deleted {
		return fmt.Errorf("no rows found to delete")
	}

	// 5. Update the table with the new row list
	table.Rows = newRows

	// 6. Update the table in IPFS
	newTableCID, err := AddObject(sh, table)
	if err != nil {
		return err
	}

	// 7. Update the database with the new table CID
	db.Tables[tableName] = newTableCID

	// 8. Update the database in IPFS
	newDbCID, err := AddObject(sh, db)
	if err != nil {
		return err
	}

	// 9. Update the cache
	UpdateCache(dbName, newDbCID)

	// 10. Update the IPNS record in the background
	publishAsync(sh, dbName, newDbCID)
	return nil
}

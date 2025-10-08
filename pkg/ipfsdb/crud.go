package ipfsdb

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strings"

    shell "github.com/ipfs/go-ipfs-api"
    ssql "github.com/nnlgsakib/wwfsdb/pkg/ssql"
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
		row[col.Name] = values[i]
	}

	// 5. Add the new row to IPFS
	rowCID, err := AddObject(sh, row)
	if err != nil {
		return err
	}

	// 6. Update the table with the new row CID
	table.Rows = append(table.Rows, rowCID)

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
func Query(ipfsAPI, dbName, tableName, whereColumn, whereValue string) ([]map[string]interface{}, error) {
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

	// 4. Load the schema
	schema, err := LoadSchema(sh, table.SchemaCID)
	if err != nil {
		return nil, err
	}

	// 5. Iterate over the rows and filter based on the WHERE clause
	var results []map[string]interface{}
	for _, rowCID := range table.Rows {
		row, err := LoadRow(sh, rowCID)
		if err != nil {
			return nil, err
		}

		if whereColumn == "" || (row[whereColumn] != nil && row[whereColumn].(string) == whereValue) {
			// Convert row to include only the fields from the schema
			schemaRow := make(map[string]interface{})
			for _, col := range schema.Columns {
				schemaRow[col.Name] = row[col.Name]
			}
			results = append(results, schemaRow)
		}
	}

	return results, nil
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
	query = strings.TrimSpace(query)
	queryParts := strings.Split(query, " ")
	queryType := strings.ToUpper(queryParts[0])

	switch queryType {
	case "CREATE":
		if len(queryParts) < 2 {
			return "", fmt.Errorf("invalid CREATE statement")
		}
		createType := strings.ToUpper(queryParts[1])
		switch createType {
		case "DATABASE":
        dbName, err := ssql.ParseCreateDatabase(query)
			if err != nil {
				return "", err
			}
			_, err = CreateDatabase(ipfsAPI, dbName)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Database '%s' created successfully.", dbName), nil
		case "TABLE":
        schema, tableName, err := ssql.ParseCreateTable(query)
			if err != nil {
				return "", err
			}
			_, err = Migrate(ipfsAPI, dbName, tableName, schema)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Table '%s' created successfully in database '%s'.", tableName, dbName), nil
		default:
			return "", fmt.Errorf("unsupported CREATE statement: %s", query)
		}
    case "SELECT":
        tableName, where, err := ssql.ParseSelect(query)
		if err != nil {
			return "", err
		}

		var whereColumn, whereValue string
		if where != nil {
			whereColumn = where.Column
			whereValue = where.Value
		}

		rows, err := Query(ipfsAPI, dbName, tableName, whereColumn, whereValue)
		if err != nil {
			return "", err
		}

		jsonResult, err := json.Marshal(rows)
		if err != nil {
			return "", err
		}

		return string(jsonResult), nil

    case "INSERT":
        tableName, values, err := ssql.ParseInsert(query)
		if err != nil {
			return "", err
		}

		err = Insert(ipfsAPI, dbName, tableName, values)
		if err != nil {
			return "", err
		}

		return "INSERT successful", nil

    case "UPDATE":
        tableName, update, where, err := ssql.ParseUpdate(query)
		if err != nil {
			return "", err
		}

		err = Update(ipfsAPI, dbName, tableName, update.Column, update.Value, where.Column, where.Value)
		if err != nil {
			return "", err
		}

		return "UPDATE successful", nil

    case "DELETE":
        tableName, where, err := ssql.ParseDelete(query)
		if err != nil {
			return "", err
		}

		err = Delete(ipfsAPI, dbName, tableName, where.Column, where.Value)
		if err != nil {
			return "", err
		}

		return "DELETE successful", nil

	default:
		return "", fmt.Errorf("unsupported query type: %s", queryType)
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
func Update(ipfsAPI, dbName, tableName, setColumn, setValue, whereColumn, whereValue string) error {
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

	// 4. Find the row to update
	var updated bool
	for i, rowCID := range table.Rows {
		row, err := LoadRow(sh, rowCID)
		if err != nil {
			return err
		}

		if row[whereColumn] != nil && row[whereColumn].(string) == whereValue {
			// 5. Update the row
			row[setColumn] = setValue

			// 6. Add the updated row to IPFS
			newRowCID, err := AddObject(sh, row)
			if err != nil {
				return err
			}

			// 7. Update the table with the new row CID
			table.Rows[i] = newRowCID
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf("no rows found to update")
	}

	// 8. Update the table in IPFS
	newTableCID, err := AddObject(sh, table)
	if err != nil {
		return err
	}

	// 9. Update the database with the new table CID
	db.Tables[tableName] = newTableCID

	// 10. Update the database in IPFS
	newDbCID, err := AddObject(sh, db)
	if err != nil {
		return err
	}

	// 11. Update the cache
	UpdateCache(dbName, newDbCID)

	// 12. Update the IPNS record in the background
	publishAsync(sh, dbName, newDbCID)
	return nil
}

// Delete removes a row from a table
func Delete(ipfsAPI, dbName, tableName, whereColumn, whereValue string) error {
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

		if row[whereColumn] != nil && row[whereColumn].(string) == whereValue {
			deleted = true
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


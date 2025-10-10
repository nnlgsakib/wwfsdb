package ipfsdb

import (
	"bytes"
	"fmt"

	shell "github.com/ipfs/go-ipfs-api"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	ssql "github.com/nnlgsakib/wwfsdb/pkg/ssql"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
	"google.golang.org/protobuf/proto"
)

// Migrate adds a new table to the database
func Migrate(ipfsAPI, dbName, tableName string, schema *ssql.Schema) (string, error) {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return "", err
	}

	newDb, err := MigrateDB(sh, db, tableName, schema)
	if err != nil {
		return "", err
	}

	// 6. Update the database in IPFS
	newDbCID, err := AddObject(sh, newDb)
	if err != nil {
		return "", err
	}

	// 7. Update the cache
	UpdateCache(dbName, newDbCID)

	// 8. Update the IPNS record in the background
	PublishAsync(sh, dbName, newDbCID)

	return newDbCID, nil
}

// MigrateDB adds a new table to an in-memory database object
func MigrateDB(sh *shell.Shell, db *pb.Database, tableName string, schema *ssql.Schema) (*pb.Database, error) {
	newDb := proto.Clone(db).(*pb.Database)

	// Check if table already exists
	if _, ok := newDb.Tables[tableName]; ok {
		return nil, fmt.Errorf("table '%s' already exists in database", tableName)
	}

	// 2. Convert ssql.Schema to pb.Schema and add to IPFS
	pbSchema := &pb.Schema{}
	for _, col := range schema.Columns {
		pbSchema.Columns = append(pbSchema.Columns, &pb.Column{Name: col.Name, Type: col.Type})
	}
	schemaCID, err := AddObject(sh, pbSchema)
	if err != nil {
		return nil, err
	}

	// 3. Create a new table
	table := &pb.Table{
		SchemaCid: schemaCID,
		PageCids:  []string{},
		Indexes:   make(map[string]string),
	}

	// 4. Add the table to IPFS
	tableCID, err := AddObject(sh, table)
	if err != nil {
		return nil, err
	}

	if db.Tables == nil {
		db.Tables = make(map[string]string)
	}

	// 5. Add the new table to the database
	db.Tables[tableName] = tableCID

	return db, nil
}

// LoadTable loads a table from IPFS
func LoadTable(sh *shell.Shell, tableCID string) (*pb.Table, error) {
	data, err := sh.Cat(tableCID)
	if err != nil {
		return nil, err
	}
	defer data.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(data)

	var table pb.Table
	if err := proto.Unmarshal(buf.Bytes(), &table); err != nil {
		return nil, err
	}

	return &table, nil
}

// Drop removes a table from the database
func Drop(ipfsAPI, dbName, tableName string) error {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return err
	}

	newDb, err := DropDB(sh, db, tableName)
	if err != nil {
		return err
	}

	// 4. Update the database in IPFS
	newDbCID, err := AddObject(sh, newDb)
	if err != nil {
		return err
	}

	// 5. Update the cache
	UpdateCache(dbName, newDbCID)

	// 6. Update the IPNS record in the background
	PublishAsync(sh, dbName, newDbCID)
	return nil
}

// DropDB removes a table from an in-memory database object
func DropDB(sh *shell.Shell, db *pb.Database, tableName string) (*pb.Database, error) {
	newDb := proto.Clone(db).(*pb.Database)

	// 2. Check if the table exists
	if _, ok := newDb.Tables[tableName]; !ok {
		return nil, fmt.Errorf("table %s not found in database", tableName)
	}

	// 3. Remove the table from the database
	delete(newDb.Tables, tableName)

	return newDb, nil
}

// AlterTableDB modifies an existing table's schema.
func AlterTableDB(sh *shell.Shell, db *pb.Database, tableName string, action ast.AlterTableAction) (*pb.Database, error) {
	newDb := proto.Clone(db).(*pb.Database)

	tableCID, ok := newDb.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table %s not found in database", tableName)
	}

	table, err := LoadTable(sh, tableCID)
	if err != nil {
		return nil, err
	}

	schema, err := LoadSchema(sh, table.SchemaCid)
	if err != nil {
		return nil, err
	}

	newSchema := proto.Clone(schema).(*pb.Schema)

	switch act := action.(type) {
	case *ast.AddColumnClause:
		for _, c := range newSchema.Columns {
			if c.Name == act.Column.Name {
				return nil, fmt.Errorf("column '%s' already exists in table '%s'", act.Column.Name, tableName)
			}
		}
		newSchema.Columns = append(newSchema.Columns, &pb.Column{Name: act.Column.Name, Type: act.Column.Type})

	case *ast.DropColumnClause:
		found := false
		newColumns := []*pb.Column{}
		for _, c := range newSchema.Columns {
			if c.Name == act.ColumnName {
				found = true
			} else {
				newColumns = append(newColumns, c)
			}
		}
		if !found {
			return nil, fmt.Errorf("column '%s' not found in table '%s'", act.ColumnName, tableName)
		}
		newSchema.Columns = newColumns

	case *ast.RenameColumnClause:
		found := false
		for _, c := range newSchema.Columns {
			if c.Name == act.OldName {
				c.Name = act.NewName
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("column '%s' not found in table '%s'", act.OldName, tableName)
		}
		// IMPORTANT: This just renames the column in the schema.
		// Existing data rows will still have the old column name as the key.
		// A full data migration is required to update existing rows, which is a more complex operation.

	default:
		return nil, fmt.Errorf("unsupported ALTER TABLE action")
	}

	// Save the new schema to IPFS
	newSchemaCID, err := AddObject(sh, newSchema)
	if err != nil {
		return nil, err
	}

	// Update the table to point to the new schema
	table.SchemaCid = newSchemaCID

	// Save the updated table to IPFS
	newTableCID, err := AddObject(sh, table)
	if err != nil {
		return nil, err
	}

	// Update the database to point to the new table version
	newDb.Tables[tableName] = newTableCID

	return newDb, nil
}

// LoadSchema loads a schema from IPFS
func LoadSchema(sh *shell.Shell, schemaCID string) (*pb.Schema, error) {
	data, err := sh.Cat(schemaCID)
	if err != nil {
		return nil, err
	}
	defer data.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(data)

	var schema pb.Schema
	if err := proto.Unmarshal(buf.Bytes(), &schema); err != nil {
		return nil, err
	}

	return &schema, nil
}

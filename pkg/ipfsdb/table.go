package ipfsdb

import (
	"bytes"
	"fmt"

	shell "github.com/ipfs/go-ipfs-api"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	"github.com/blastrain/vitess-sqlparser/sqlparser"
	"google.golang.org/protobuf/proto"
)

// Migrate adds a new table to the database
func Migrate(ipfsAPI, dbName, tableName string, stmt sqlparser.Statement) (string, error) {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return "", err
	}

	newDb, err := MigrateDB(sh, db, tableName, stmt)
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
func MigrateDB(sh *shell.Shell, db *pb.Database, tableName string, stmt sqlparser.Statement) (*pb.Database, error) {
	newDb := proto.Clone(db).(*pb.Database)

	// Check if table already exists
	if _, ok := newDb.Tables[tableName]; ok {
		return nil, fmt.Errorf("table '%s' already exists in database", tableName)
	}

	// 2. Convert sqlparser.Statement to pb.Schema and add to IPFS
	pbSchema, err := extractSchemaFromDDL(stmt)
	if err != nil {
		return nil, err
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
func Drop(ipfsAPI, dbName string, ddl *sqlparser.DDL) error {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return err
	}

	newDb, err := DropDB(sh, db, ddl)
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
func DropDB(sh *shell.Shell, db *pb.Database, ddl *sqlparser.DDL) (*pb.Database, error) {
	newDb := proto.Clone(db).(*pb.Database)

	tableName := ddl.Table.Name.String()
	// 2. Check if the table exists
	if _, ok := newDb.Tables[tableName]; !ok {
		return nil, fmt.Errorf("table %s not found in database", tableName)
	}

	// 3. Remove the table from the database
	delete(newDb.Tables, tableName)

	return newDb, nil
}

// AlterTableDB modifies an existing table's schema.
func AlterTableDB(sh *shell.Shell, db *pb.Database, tableName string, ddl *sqlparser.DDL) (*pb.Database, error) {
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

	// TODO: Implement logic to handle different alteration types from the vitess-sqlparser AST

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

func RenameTableDB(sh *shell.Shell, db *pb.Database, ddl *sqlparser.DDL) (*pb.Database, error) {
	newDb := proto.Clone(db).(*pb.Database)

	oldTableName := ddl.Table.Name.String()
	newTableName := ddl.NewName.Name.String()

	tableCID, ok := newDb.Tables[oldTableName]
	if !ok {
		return nil, fmt.Errorf("table %s not found in database", oldTableName)
	}

	if _, ok := newDb.Tables[newTableName]; ok {
		return nil, fmt.Errorf("table %s already exists in database", newTableName)
	}

	delete(newDb.Tables, oldTableName)
	newDb.Tables[newTableName] = tableCID

	return newDb, nil
}

package ipfsdb

import (
	"fmt"

	"github.com/blastrain/vitess-sqlparser/sqlparser"
	shell "github.com/ipfs/go-ipfs-api"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// CreateIndex builds and saves a new index for a specific column in a table.
func CreateIndex(ipfsAPI, dbName string, ddl *sqlparser.DDL) error {
	// TODO: The vitess-sqlparser does not fully parse CREATE INDEX statements to extract the column name.
	// This needs to be fixed by either switching to a different parser or by manually parsing the column name from the DDL statement.
	return fmt.Errorf("CREATE INDEX is not yet fully supported")
}

// CreateIndexDB builds and saves a new index for a specific column in a table.
// It operates on an in-memory database object and returns the modified object.
func CreateIndexDB(sh *shell.Shell, db *pb.Database, tableName string, columnName string) (*pb.Database, error) {
	newDb := proto.Clone(db).(*pb.Database)

	tableCID, ok := newDb.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table %s not found", tableName)
	}
	table, err := LoadTable(sh, tableCID)
	if err != nil {
		return nil, err
	}

	if _, ok := table.Indexes[columnName]; ok {
		return nil, fmt.Errorf("index for column %s on table %s already exists", columnName, tableName)
	}

	schema, err := LoadSchema(sh, table.SchemaCid)
	if err != nil {
		return nil, err
	}
	columnExists := false
	for _, col := range schema.Columns {
		if col.Name == columnName {
			columnExists = true
			break
		}
	}
	if !columnExists {
		return nil, fmt.Errorf("column %s not found in table %s", columnName, tableName)
	}

	// Build the index using a ProllyTree
	prollyTree, err := NewProllyTree(sh)
	if err != nil {
		return nil, fmt.Errorf("failed to create new prolly tree: %w", err)
	}

	for _, pageCID := range table.PageCids {
		page, err := LoadPage(sh, pageCID)
		if err != nil {
			return nil, fmt.Errorf("failed to load page %s: %w", pageCID, err)
		}
		for _, row := range page.Rows {
			val, ok := row.Values[columnName]
			if !ok {
				continue
			}

			key, err := valueToString(val)
			if err != nil {
				return nil, fmt.Errorf("failed to convert value to string for index key: %w", err)
			}

			prollyTree, err = prollyTree.Put(key, pageCID)
			if err != nil {
				return nil, fmt.Errorf("failed to put value into prolly tree: %w", err)
			}
		}
	}

	if table.Indexes == nil {
		table.Indexes = make(map[string]string)
	}
	table.Indexes[columnName] = prollyTree.RootCID

	newTableCID, err := AddObject(sh, table)
	if err != nil {
		return nil, err
	}

	newDb.Tables[tableName] = newTableCID
	return newDb, nil
}

// UpdateIndexesOnInsert updates all relevant indexes when a new row is added.
// This version is more efficient as it batches IPFS writes.
func UpdateIndexesOnInsert(sh *shell.Shell, table *pb.Table, pageCID string, rowData *pb.Row) (*pb.Table, error) {
	if len(table.Indexes) == 0 {
		return table, nil // No indexes to update
	}

	newTable := proto.Clone(table).(*pb.Table)

	for colName, indexCID := range newTable.Indexes {
		val, ok := rowData.Values[colName]
		if !ok {
			continue
		}

		prollyTree := LoadProllyTree(sh, indexCID)

		key, err := valueToString(val)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value to string for index key: %w", err)
		}

		prollyTree, err = prollyTree.Put(key, pageCID)
		if err != nil {
			return nil, fmt.Errorf("failed to put value into prolly tree: %w", err)
		}
		newTable.Indexes[colName] = prollyTree.RootCID
	}

	return newTable, nil
}

// UpdateIndexesOnDelete updates all relevant indexes when a row is deleted.
func UpdateIndexesOnDelete(sh *shell.Shell, table *pb.Table, pageCID string, page *pb.Page, rowToDelete *pb.Row) (*pb.Table, error) {
	if len(table.Indexes) == 0 {
		return table, nil
	}

	newTable := proto.Clone(table).(*pb.Table)

	for colName, indexCID := range newTable.Indexes {
		val, ok := rowToDelete.Values[colName]
		if !ok {
			continue
		}

		key, err := valueToString(val)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value to string for index key: %w", err)
		}

		prollyTree := LoadProllyTree(sh, indexCID)
		prollyTree, err = prollyTree.Delete(key, pageCID)
		if err != nil {
			return nil, fmt.Errorf("failed to delete value from prolly tree: %w", err)
		}
		newTable.Indexes[colName] = prollyTree.RootCID
	}

	return newTable, nil
}

func valueToString(any *anypb.Any) (string, error) {
	val, err := FromAny(any)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", val), nil
}

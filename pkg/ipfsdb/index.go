package ipfsdb

import (
	"bytes"
	"fmt"

	shell "github.com/ipfs/go-ipfs-api"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// LoadIndex loads an index from IPFS
func LoadIndex(sh *shell.Shell, indexCID string) (*pb.Index, error) {
	data, err := sh.Cat(indexCID)
	if err != nil {
		return nil, err
	}
	defer data.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(data)

	var index pb.Index
	if err := proto.Unmarshal(buf.Bytes(), &index); err != nil {
		return nil, err
	}

	return &index, nil
}

// CreateIndex builds and saves a new index for a specific column in a table.
func CreateIndex(ipfsAPI, dbName, tableName, columnName string) error {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return err
	}

	newDb, err := CreateIndexDB(sh, db, tableName, columnName)
	if err != nil {
		return err
	}

	// 8. Update the database with the new table CID
	newDbCID, err := AddObject(sh, newDb)
	if err != nil {
		return err
	}

	// 9. Update cache and publish
	UpdateCache(dbName, newDbCID)
	PublishAsync(sh, dbName, newDbCID)

	return nil
}

// CreateIndexDB builds and saves a new index for a specific column in a table.
// It operates on an in-memory database object and returns the modified object.
func CreateIndexDB(sh *shell.Shell, db *pb.Database, tableName, columnName string) (*pb.Database, error) {
	// Make a deep copy of the database to avoid modifying the original in-place
	// This is crucial for transactional integrity.
	newDb := proto.Clone(db).(*pb.Database)

	tableCID, ok := newDb.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table %s not found", tableName)
	}
	table, err := LoadTable(sh, tableCID)
	if err != nil {
		return nil, err
	}

	// Check if index already exists
	if _, ok := table.Indexes[columnName]; ok {
		return nil, fmt.Errorf("index for column %s on table %s already exists", columnName, tableName)
	}

	// Load schema and validate column
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

	// Build the index from existing rows
	newIndex := &pb.Index{Nodes: make(map[string]*pb.IndexNode)}
	for _, rowCID := range table.Rows {
		row, err := LoadRow(sh, rowCID)
		if err != nil {
			return nil, fmt.Errorf("failed to load row %s: %w", rowCID, err)
		}
		val, ok := row.Values[columnName]
		if !ok {
			continue // This row doesn't have a value for the indexed column
		}

		key, err := valueToString(val)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value to string for index key: %w", err)
		}

		if _, ok := newIndex.Nodes[key]; !ok {
			newIndex.Nodes[key] = &pb.IndexNode{}
		}
		newIndex.Nodes[key].Cids = append(newIndex.Nodes[key].Cids, rowCID)
	}

	// Save the new index to IPFS (this is still needed here as index is a separate object)
	indexCID, err := AddObject(sh, newIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to save index to IPFS: %w", err)
	}

	// Update the table metadata with the new index CID
	if table.Indexes == nil {
		table.Indexes = make(map[string]string)
	}
	table.Indexes[columnName] = indexCID

	// Save the updated table back to IPFS (this is still needed here as table is a separate object)
	newTableCID, err := AddObject(sh, table)
	if err != nil {
		return nil, err
	}

	// Update the database with the new table CID
	newDb.Tables[tableName] = newTableCID

	return newDb, nil // Return the modified DB object
}

// UpdateIndexesOnInsert updates all relevant indexes when a new row is added.
func UpdateIndexesOnInsert(sh *shell.Shell, table *pb.Table, rowCID string, rowData *pb.Row) (*pb.Table, error) {
	if len(table.Indexes) == 0 {
		return table, nil // No indexes to update
	}

	for colName, indexCID := range table.Indexes {
		val, ok := rowData.Values[colName]
		if !ok {
			continue // This row doesn't have a value for the indexed column
		}

		// Load the existing index
		index, err := LoadIndex(sh, indexCID)
		if err != nil {
			return nil, fmt.Errorf("failed to load index %s: %w", indexCID, err)
		}

		// Add the new row CID to the index
		key, err := valueToString(val)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value to string for index key: %w", err)
		}

		// Ensure the nodes map is initialized
		if index.Nodes == nil {
			index.Nodes = make(map[string]*pb.IndexNode)
		}

		if _, ok := index.Nodes[key]; !ok {
			index.Nodes[key] = &pb.IndexNode{}
		}
		index.Nodes[key].Cids = append(index.Nodes[key].Cids, rowCID)

		// Save the updated index back to IPFS
		newIndexCID, err := AddObject(sh, index)
		if err != nil {
			return nil, fmt.Errorf("failed to save updated index for column %s: %w", colName, err)
		}

		// Update the table's metadata with the new index CID
		table.Indexes[colName] = newIndexCID
	}

	return table, nil
}

// UpdateIndexesOnDelete updates all relevant indexes when a row is deleted.
func UpdateIndexesOnDelete(sh *shell.Shell, table *pb.Table, rowCID string, rowData *pb.Row) (*pb.Table, error) {
	if len(table.Indexes) == 0 {
		return table, nil // No indexes to update
	}

	for colName, indexCID := range table.Indexes {
		val, ok := rowData.Values[colName]
		if !ok {
			continue // This row doesn't have a value for the indexed column
		}

		// Load the existing index
		index, err := LoadIndex(sh, indexCID)
		if err != nil {
			return nil, fmt.Errorf("failed to load index %s: %w", indexCID, err)
		}

		// Remove the row CID from the index
		key, err := valueToString(val)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value to string for index key: %w", err)
		}

		// Ensure the nodes map is not nil before accessing
		if index.Nodes == nil {
			continue // Nothing to delete
		}

		if node, ok := index.Nodes[key]; ok {
			newCIDs := []string{}
			for _, cid := range node.Cids {
				if cid != rowCID {
					newCIDs = append(newCIDs, cid)
				}
			}
			if len(newCIDs) == 0 {
				delete(index.Nodes, key)
			} else {
				node.Cids = newCIDs
			}
		}

		// Save the updated index back to IPFS
		newIndexCID, err := AddObject(sh, index)
		if err != nil {
			return nil, fmt.Errorf("failed to save updated index for column %s: %w", colName, err)
		}

		// Update the table's metadata with the new index CID
		table.Indexes[colName] = newIndexCID
	}

	return table, nil
}

func valueToString(any *anypb.Any) (string, error) {
	val, err := FromAny(any)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", val), nil
}


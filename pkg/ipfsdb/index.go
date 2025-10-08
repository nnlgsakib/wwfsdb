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

	// 1. Load the database and table
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return err
	}
	tableCID, ok := db.Tables[tableName]
	if !ok {
		return fmt.Errorf("table %s not found", tableName)
	}
	table, err := LoadTable(sh, tableCID)
	if err != nil {
		return err
	}

	// 2. Check if index already exists
	if _, ok := table.Indexes[columnName]; ok {
		return fmt.Errorf("index for column %s on table %s already exists", columnName, tableName)
	}

	// 3. Load schema and validate column
	schema, err := LoadSchema(sh, table.SchemaCid)
	if err != nil {
		return err
	}
	columnExists := false
	for _, col := range schema.Columns {
		if col.Name == columnName {
			columnExists = true
			break
		}
	}
	if !columnExists {
		return fmt.Errorf("column %s not found in table %s", columnName, tableName)
	}

	// 4. Build the index from existing rows
	newIndex := &pb.Index{Nodes: make(map[string]*pb.IndexNode)}
	for _, rowCID := range table.Rows {
		row, err := LoadRow(sh, rowCID)
		if err != nil {
			return fmt.Errorf("failed to load row %s: %w", rowCID, err)
		}
		val, ok := row.Values[columnName]
		if !ok {
			// Column value doesn't exist for this row, skip it
			continue
		}

		// Keys in a JSON map must be strings. We format the value to a string to use as a key.
		key, err := valueToString(val)
		if err != nil {
			return fmt.Errorf("failed to convert value to string for index key: %w", err)
		}

		if _, ok := newIndex.Nodes[key]; !ok {
			newIndex.Nodes[key] = &pb.IndexNode{}
		}
		newIndex.Nodes[key].Cids = append(newIndex.Nodes[key].Cids, rowCID)
	}

	// 5. Save the new index to IPFS
	indexCID, err := AddObject(sh, newIndex)
	if err != nil {
		return fmt.Errorf("failed to save index to IPFS: %w", err)
	}

	// 6. Update the table metadata with the new index CID
	if table.Indexes == nil {
		table.Indexes = make(map[string]string)
	}
	table.Indexes[columnName] = indexCID

	// 7. Save the updated table back to IPFS
	newTableCID, err := AddObject(sh, table)
	if err != nil {
		return err
	}

	// 8. Update the database with the new table CID
	db.Tables[tableName] = newTableCID
	newDbCID, err := AddObject(sh, db)
	if err != nil {
		return err
	}

	// 9. Update cache and publish
	UpdateCache(dbName, newDbCID)
	publishAsync(sh, dbName, newDbCID)

	return nil
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


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

	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return err
	}

	newDb, err := CreateIndexDB(sh, db, tableName, columnName)
	if err != nil {
		return err
	}

	newDbCID, err := AddObject(sh, newDb)
	if err != nil {
		return err
	}

	UpdateCache(dbName, newDbCID)
	PublishAsync(sh, dbName, newDbCID)

	return nil
}

// CreateIndexDB builds and saves a new index for a specific column in a table.
// It operates on an in-memory database object and returns the modified object.
func CreateIndexDB(sh *shell.Shell, db *pb.Database, tableName, columnName string) (*pb.Database, error) {
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

	// Build the index by iterating through all pages and rows
	newIndex := &pb.Index{Nodes: make(map[string]*pb.IndexNode)}
	for _, pageCID := range table.PageCids {
		page, err := LoadPage(sh, pageCID)
		if err != nil {
			return nil, fmt.Errorf("failed to load page %s: %w", pageCID, err)
		}
		for _, row := range page.Rows {
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

			// Add the page CID to the index if it's not already there.
			found := false
			for _, cid := range newIndex.Nodes[key].Cids {
				if cid == pageCID {
					found = true
					break
				}
			}
			if !found {
				// Note: In a paged system, we can't point to a row CID.
				// For now, we'll point to the page CID. The query planner will need to scan this page.
				// A future optimization (Prolly-Trees) would solve this more elegantly.
				newIndex.Nodes[key].Cids = append(newIndex.Nodes[key].Cids, pageCID)
			}
		}
	}

	indexCID, err := AddObject(sh, newIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to save index to IPFS: %w", err)
	}

	if table.Indexes == nil {
		table.Indexes = make(map[string]string)
	}
	table.Indexes[columnName] = indexCID

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
	// A map to hold loaded and modified indexes to avoid loading the same index multiple times
	loadedIndexes := make(map[string]*pb.Index)

	for colName, indexCID := range newTable.Indexes {
		val, ok := rowData.Values[colName]
		if !ok {
			continue // This row doesn't have a value for the indexed column
		}

		index, ok := loadedIndexes[colName]
		if !ok {
			var err error
			index, err = LoadIndex(sh, indexCID)
			if err != nil {
				return nil, fmt.Errorf("failed to load index %s: %w", indexCID, err)
			}
			loadedIndexes[colName] = index
		}

		key, err := valueToString(val)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value to string for index key: %w", err)
		}

		if index.Nodes == nil {
			index.Nodes = make(map[string]*pb.IndexNode)
		}

		if _, ok := index.Nodes[key]; !ok {
			index.Nodes[key] = &pb.IndexNode{}
		}
		// Add the page CID to the index if it's not already there.
		found := false
		for _, cid := range index.Nodes[key].Cids {
			if cid == pageCID {
				found = true
				break
			}
		}
		if !found {
			index.Nodes[key].Cids = append(index.Nodes[key].Cids, pageCID)
		}
	}

	// Now, save all modified indexes and update the table's index CIDs
	for colName, index := range loadedIndexes {
		newIndexCID, err := AddObject(sh, index)
		if err != nil {
			return nil, fmt.Errorf("failed to save updated index for column %s: %w", colName, err)
		}
		newTable.Indexes[colName] = newIndexCID
	}

	return newTable, nil
}

// UpdateIndexesOnDelete updates all relevant indexes when a row is deleted.
// This version is more efficient as it batches IPFS writes.
// Note: This function's correctness depends on the caller providing the correct state of the page.
// When called from UpdateDB, the page object might be in a post-update state, which can lead to incorrect index changes.
func UpdateIndexesOnDelete(sh *shell.Shell, table *pb.Table, pageCID string, page *pb.Page, rowToDelete *pb.Row) (*pb.Table, error) {
	if len(table.Indexes) == 0 {
		return table, nil
	}

	newTable := proto.Clone(table).(*pb.Table)
	loadedIndexes := make(map[string]*pb.Index)

	for colName, indexCID := range newTable.Indexes {
		val, ok := rowToDelete.Values[colName]
		if !ok {
			continue
		}

		key, err := valueToString(val)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value to string for index key: %w", err)
		}

		// Check if any other row in the page has the same value for this column.
		hasOtherRowsWithValue := false
		for _, r := range page.Rows {
			if r != rowToDelete { // Don't compare the row with itself
				if otherVal, ok := r.Values[colName]; ok {
					if otherKey, _ := valueToString(otherVal); otherKey == key {
						hasOtherRowsWithValue = true
						break
					}
				}
			}
		}

		// If no other row has this value, we can remove the page CID from the index for this key.
		if !hasOtherRowsWithValue {
			index, ok := loadedIndexes[colName]
			if !ok {
				var err error
				index, err = LoadIndex(sh, indexCID)
				if err != nil {
					return nil, fmt.Errorf("failed to load index %s: %w", indexCID, err)
				}
				loadedIndexes[colName] = index
			}

			if node, ok := index.Nodes[key]; ok {
				newCIDs := []string{}
				for _, cid := range node.Cids {
					if cid != pageCID {
						newCIDs = append(newCIDs, cid)
					}
				}
				if len(newCIDs) == 0 {
					delete(index.Nodes, key)
				} else {
					node.Cids = newCIDs
				}
			}
		}
	}

	// Save all modified indexes
	for colName, index := range loadedIndexes {
		newIndexCID, err := AddObject(sh, index)
		if err != nil {
			return nil, fmt.Errorf("failed to save updated index for column %s: %w", colName, err)
		}
		newTable.Indexes[colName] = newIndexCID
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
package ipfsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	shell "github.com/ipfs/go-ipfs-api"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	ssql "github.com/nnlgsakib/wwfsdb/pkg/ssql"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
	"google.golang.org/protobuf/proto"
)

// CreateDatabase creates a new database
func CreateDatabase(ipfsAPI, dbName string) (string, error) {
	sh := shell.NewShell(ipfsAPI)

	// 1. Create a new database
	db := &pb.Database{
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
	entry := &pb.RegistryEntry{
		DbName:    dbName,
		ProgramId: key.Id,
		KeyName:   key.Name,
	}
	entryData, err := proto.Marshal(entry)
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

// LoadDatabase loads a database from IPFS
func LoadDatabase(sh *shell.Shell, dbName string) (*pb.Database, error) {
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

		var entry pb.RegistryEntry
		if err := proto.Unmarshal(entryData, &entry); err != nil {
			return nil, err
		}

		// 3. Resolve the IPNS name to get the database CID
		dbCID, err = sh.Resolve(entry.ProgramId)
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

	buf := new(bytes.Buffer)
	buf.ReadFrom(data)

	// 6. Decode the database object
	var db pb.Database
	if err := proto.Unmarshal(buf.Bytes(), &db); err != nil {
		return nil, err
	}

	return &db, nil
}

// AddObject adds a generic object to IPFS and returns its CID
func AddObject(sh *shell.Shell, obj proto.Message) (string, error) {
	data, err := proto.Marshal(obj)
	if err != nil {
		return "", err
	}

	return sh.Add(bytes.NewReader(data))
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

		// Use json.Marshal to correctly handle types
		jsonResult, err := json.Marshal(rows)
		if err != nil {
			return "", fmt.Errorf("failed to marshal result to JSON: %w", err)
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

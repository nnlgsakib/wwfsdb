package ipfsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/nnlgsakib/wwfsdb/pkg/auth"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	ssql "github.com/nnlgsakib/wwfsdb/pkg/ssql"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
	"google.golang.org/protobuf/proto"
)

// CreateDatabase creates a new database
func CreateDatabase(ipfsAPI, dbName string) (map[string]string, error) {
	sh := shell.NewShell(ipfsAPI)

	// 1. Generate key pair for owner
	privKey, err := auth.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}
	pubKeyStr := auth.EncodePublicKey(&privKey.PublicKey)
	privKeyStr := auth.EncodePrivateKey(privKey)

	// 2. Create a new database
	db := &pb.Database{
		Tables:         make(map[string]string),
		OwnerPublicKey: pubKeyStr,
	}

	// 3. Add the database to IPFS
	dbCID, err := AddObject(sh, db)
	if err != nil {
		return nil, err
	}

	// 4. Create a new IPNS key
	key, err := sh.KeyGen(context.Background(), dbName, shell.KeyGen.Size(2048))
	if err != nil {
		return nil, err
	}

	// 5. Publish the database CID to the new key in the background
	go func() {
		_, err := sh.PublishWithDetails(dbCID, key.Name, 0, 0, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error publishing to IPNS for new database: %v\n", err)
		}
	}()

	// 6. Save the database info to the registry
	entry := &pb.RegistryEntry{
		DbName:    dbName,
		ProgramId: key.Id,
		KeyName:   key.Name,
	}
	entryData, err := proto.Marshal(entry)
	if err != nil {
		return nil, err
	}

	err = PutToCache([]byte("registry:"+dbName), entryData)
	if err != nil {
		return nil, err
	}

	// 7. Update the cache with the new database CID
	UpdateCache(dbName, dbCID)

	result := map[string]string{
		"program_id":  key.Id,
		"private_key": privKeyStr,
	}

	return result, nil
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

	return sh.Add(bytes.NewReader(data), shell.CidVersion(1))
}

// ExecuteQuery is the top-level function for single, auto-committed queries.
// It loads the database, executes the statement, and saves the result.
func ExecuteQuery(ipfsAPI, dbName, query, signature string) (string, error) {
	sh := shell.NewShell(ipfsAPI)

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

	// Handle CREATE DATABASE separately as it doesn't operate on an existing DB
	if s, ok := stmt.(*ast.CreateDatabaseStmt); ok {
		result, err := CreateDatabase(ipfsAPI, s.Name)
		if err != nil {
			return "", err
		}
		jsonResult, err := json.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("failed to marshal create database result: %w", err)
		}
		return string(jsonResult), nil
	}

	// Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return "", err
	}

	// Verify signature for write operations
	if !IsReadQuery(stmt) {
		err := VerifySignature(db, dbName, query, signature)
		if err != nil {
			return "", fmt.Errorf("unauthorized: %w", err)
		}
	}

	// Execute the statement on the loaded database state
	newDbState, result, err := ExecuteOnDB(sh, dbName, db, stmt)
	if err != nil {
		return "", err
	}

	// For read queries, just return the result
	if _, ok := stmt.(*ast.SelectStmt); ok {
		return result, nil
	}

	// For write queries, save the new state and publish
	newDbCID, err := AddObject(sh, newDbState)
	if err != nil {
		return "", err
	}

	UpdateCache(dbName, newDbCID)
	PublishAsync(sh, dbName, newDbCID)

	return result, nil
}

// ExecuteOnDB executes a statement against an in-memory database object.
// It returns the potentially modified database state and a result string.
// It does NOT save or publish the result.
func ExecuteOnDB(sh *shell.Shell, dbName string, db *pb.Database, stmt ast.Statement) (*pb.Database, string, error) {
	switch s := stmt.(type) {
	case *ast.CreateTableStmt:
		newDb, err := MigrateDB(sh, db, s.Name, &s.Schema)
		if err != nil {
			return nil, "", err
		}
		return newDb, fmt.Sprintf("Table '%s' created successfully in database '%s'.", s.Name, dbName), nil
	case *ast.DropTableStmt:
		newDb, err := DropDB(sh, db, s.Name)
		if err != nil {
			return nil, "", err
		}
		return newDb, fmt.Sprintf("Table '%s' dropped successfully.", s.Name), nil
	case *ast.SelectStmt:
	
rows, err := QueryDB(sh, db, s.Table, s.Columns, s.Where)
		if err != nil {
			return nil, "", err
		}
		jsonResult, err := json.Marshal(rows)
		if err != nil {
			return nil, "", fmt.Errorf("failed to marshal result to JSON: %w", err)
		}
		return db, string(jsonResult), nil // db is not modified
	case *ast.InsertStmt:
		newDb, err := InsertDB(sh, db, s.Table, s.Values)
		if err != nil {
			return nil, "", err
		}
		return newDb, "INSERT successful", nil
	case *ast.UpdateStmt:
		newDb, affectedRows, err := UpdateDB(sh, db, s.Table, s.Set.Column, s.Set.Value, s.Where)
		if err != nil {
			return nil, "", err
		}
		return newDb, fmt.Sprintf("UPDATE successful. %d rows affected.", affectedRows), nil
	case *ast.DeleteStmt:
		newDb, affectedRows, err := DeleteDB(sh, db, s.Table, s.Where)
		if err != nil {
			return nil, "", err
		}
		return newDb, fmt.Sprintf("DELETE successful. %d rows affected.", affectedRows), nil
	case *ast.CreateIndexStmt:
		newDb, err := CreateIndexDB(sh, db, s.Table, s.Column)
		if err != nil {
			return nil, "", err
		}
		return newDb, fmt.Sprintf("Index on column '%s' for table '%s' created successfully.", s.Column, s.Table), nil
	// These are handled by the dispatcher, but we can add a safeguard.
	case *ast.BeginStmt, *ast.CommitStmt, *ast.RollbackStmt:
		return nil, "", fmt.Errorf("transaction control statements cannot be executed in this context")
	// CreateDatabase is also handled separately.
	case *ast.CreateDatabaseStmt:
		return nil, "", fmt.Errorf("CREATE DATABASE cannot be run within a database context or transaction")
	default:
		return nil, "", fmt.Errorf("unsupported query type: %T", s)
	}
}

// IsReadQuery checks if a statement is a read-only query.
func IsReadQuery(stmt ast.Statement) bool {
	_, ok := stmt.(*ast.SelectStmt)
	return ok
}

// VerifySignature checks if a signature is valid for a given query and database.
func VerifySignature(db *pb.Database, dbName, query, signature string) error {
	if db.OwnerPublicKey == "" {
		// For backward compatibility, if no public key is set, allow the operation.
		return nil
	}
	if signature == "" {
		return fmt.Errorf("signature required for write operations")
	}

	message := fmt.Sprintf("%s:%s", dbName, query)
	valid, err := auth.Verify(db.OwnerPublicKey, message, signature)
	if err != nil {
		return fmt.Errorf("error verifying signature: %w", err)
	}
	if !valid {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

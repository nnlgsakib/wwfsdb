package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/nnlgsakib/wwfsdb/pkg/auth"
	"github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
	"google.golang.org/protobuf/proto"
)

// --- Service Definition ---

// WWFS is the service that exposes database operations via JSON-RPC.
type WWFS struct {
	Dispatcher *Dispatcher
}

// --- Method: wwfs_executeQuery ---

// ExecuteQueryArgs holds the arguments for the ExecuteQuery method.
type ExecuteQueryArgs struct {
	DbName    string `json:"db_name"`
	Query     string `json:"query"`
	SessionID string `json:"session_id,omitempty"`
	Signature string `json:"signature,omitempty"`
}

// ExecuteQueryResult holds the result for the ExecuteQuery method.
type ExecuteQueryResult struct {
	Result    string `json:"result"`
	SessionID string `json:"session_id,omitempty"`
}

// ExecuteQuery is the RPC method that executes a SQL query against the database.
func (h *WWFS) ExecuteQuery(r *http.Request, args *ExecuteQueryArgs, reply *ExecuteQueryResult) error {
	stmt, err := ssql.Parse(args.Query)
	if err != nil {
		return fmt.Errorf("parser error: %w", err)
	}

	// Handle transaction control statements directly
	switch stmt.(type) {
	case *ast.BeginStmt:
		sessionID, err := h.Dispatcher.BeginTransaction(args.DbName)
		if err != nil {
			return err
		}
		reply.SessionID = sessionID
		reply.Result = "Transaction started"
		return nil
	case *ast.CommitStmt:
		if args.SessionID == "" {
			return fmt.Errorf("no transaction in progress to commit")
		}
		result, err := h.Dispatcher.CommitTransaction(args.SessionID)
		if err != nil {
			return err
		}
		reply.Result = result
		return nil
	case *ast.RollbackStmt:
		if args.SessionID == "" {
			return fmt.Errorf("no transaction in progress to rollback")
		}
		err := h.Dispatcher.RollbackTransaction(args.SessionID)
		if err != nil {
			return err
		}
		reply.Result = "Transaction rolled back"
		return nil
	}

	// If in a transaction, dispatch to the transaction handler
	if args.SessionID != "" {
		result, err := h.Dispatcher.ExecuteInTransaction(args.SessionID, stmt, args.Query, args.Signature)
		if err != nil {
			return err
		}
		reply.Result = result
		return nil
	}

	// --- Original non-transactional execution path ---
	// For read-only queries or single auto-committed statements
	if _, ok := stmt.(*ast.SelectStmt); !ok {
		// This is a write operation outside a transaction, use the old queue system
		job := Job{
			DbName:    args.DbName,
			Query:     args.Query,
			Signature: args.Signature,
			Reply:     reply,
			ErrChan:   make(chan error),
		}
		h.Dispatcher.Dispatch(job)
		return <-job.ErrChan
	}

	// For SELECT statements, execute immediately
	result, err := ipfsdb.ExecuteQuery(h.Dispatcher.ipfsApi, args.DbName, args.Query, "")
	if err != nil {
		return err
	}
	reply.Result = result
	return nil
}

// --- Method: wwfs_getTableSchema ---

type GetTableSchemaArgs struct {
	DbName    string `json:"db_name"`
	TableName string `json:"table_name"`
}

type GetTableSchemaResult struct {
	Schema *pb.Schema `json:"schema"`
}

func (h *WWFS) GetTableSchema(r *http.Request, args *GetTableSchemaArgs, reply *GetTableSchemaResult) error {
	sh := shell.NewShell(h.Dispatcher.ipfsApi)
	db, err := ipfsdb.LoadDatabase(sh, args.DbName)
	if err != nil {
		return err
	}
	tableCID, ok := db.Tables[args.TableName]
	if !ok {
		return fmt.Errorf("table %s not found", args.TableName)
	}
	table, err := ipfsdb.LoadTable(sh, tableCID)
	if err != nil {
		return err
	}
	schema, err := ipfsdb.LoadSchema(sh, table.SchemaCid)
	if err != nil {
		return err
	}
	reply.Schema = schema
	return nil
}

// --- Method: wwfs_export ---
type ExportArgs struct {
	DbName    string `json:"db_name"`
	TableName string `json:"table_name,omitempty"` // if empty, export whole db
}

type ExportResult struct {
	JsonData string `json:"json_data"`
}

func (h *WWFS) Export(r *http.Request, args *ExportArgs, reply *ExportResult) error {
	sh := shell.NewShell(h.Dispatcher.ipfsApi)
	db, err := ipfsdb.LoadDatabase(sh, args.DbName)
	if err != nil {
		return err
	}

	// Full DB Export
	if args.TableName == "" {
		allData := make(map[string][]map[string]interface{})
		for tableName, tableCID := range db.Tables {
			table, err := ipfsdb.LoadTable(sh, tableCID)
			if err != nil {
				continue
			} // skip tables that fail to load
			var tableRows []map[string]interface{}
			for _, pageCID := range table.PageCids {
				page, err := ipfsdb.LoadPage(sh, pageCID)
				if err != nil {
					continue
				}
				for _, row := range page.Rows {
					rowData := make(map[string]interface{})
					for colName, valAny := range row.Values {
						val, _ := ipfsdb.FromAny(valAny)
						rowData[colName] = val
					}
				tableRows = append(tableRows, rowData)
				}
			}
			allData[tableName] = tableRows
		}
		jsonData, err := json.Marshal(allData)
		if err != nil {
			return err
		}
		reply.JsonData = string(jsonData)
		return nil
	}

	// Single Table Export
	tableCID, ok := db.Tables[args.TableName]
	if !ok {
		return fmt.Errorf("table %s not found", args.TableName)
	}
	table, err := ipfsdb.LoadTable(sh, tableCID)
	if err != nil {
		return err
	}
	var allRows []map[string]interface{}
	for _, pageCID := range table.PageCids {
		page, err := ipfsdb.LoadPage(sh, pageCID)
		if err != nil {
			continue
		}
		for _, row := range page.Rows {
			rowData := make(map[string]interface{})
			for colName, valAny := range row.Values {
				val, _ := ipfsdb.FromAny(valAny)
				rowData[colName] = val
			}
			allRows = append(allRows, rowData)
		}
	}
	jsonData, err := json.Marshal(allRows)
	if err != nil {
		return err
	}
	reply.JsonData = string(jsonData)
	return nil
}

// --- Method: wwfs_getContentCID ---
type GetContentCIDArgs struct {
	DbName    string `json:"db_name"`
	TableName string `json:"table_name,omitempty"`
}

type GetContentCIDResult struct {
	CID string `json:"cid"`
}

func (h *WWFS) GetContentCID(r *http.Request, args *GetContentCIDArgs, reply *GetContentCIDResult) error {
	sh := shell.NewShell(h.Dispatcher.ipfsApi)
	// Loading the database ensures the cache is populated
	db, err := ipfsdb.LoadDatabase(sh, args.DbName)
	if err != nil {
		return err
	}

	// DB export
	if args.TableName == "" {
		dbCID, ok := ipfsdb.ReadCache(args.DbName)
		if !ok {
			return fmt.Errorf("could not find database CID in cache")
		}
		reply.CID = dbCID
		return nil
	}

	// Table export
	tableCID, ok := db.Tables[args.TableName]
	if !ok {
		return fmt.Errorf("table %s not found", args.TableName)
	}
	reply.CID = tableCID
	return nil
}

// --- Method: wwfs_forkDatabase ---
type ForkDatabaseArgs struct {
	NewDbName   string `json:"new_db_name"`
	SourceRootCID string `json:"source_root_cid"`
}

// ForkDatabaseResult holds the result for the ForkDatabase method.
// It's the same as creating a new database.
type ForkDatabaseResult struct {
	ProgramID  string `json:"program_id"`
	PrivateKey string `json:"private_key"`
}

func (h *WWFS) ForkDatabase(r *http.Request, args *ForkDatabaseArgs, reply *ForkDatabaseResult) error {
	sh := shell.NewShell(h.Dispatcher.ipfsApi)

	// 1. Cat the source database object
	data, err := sh.Cat(args.SourceRootCID)
	if err != nil {
		return fmt.Errorf("failed to cat source database %s: %w", args.SourceRootCID, err)
	}
	defer data.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(data)

	var sourceDb pb.Database
	if err := proto.Unmarshal(buf.Bytes(), &sourceDb); err != nil {
		return fmt.Errorf("failed to decode source database: %w", err)
	}

	// 2. Generate a new key pair for the new owner
	privKey, err := auth.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate key pair for fork: %w", err)
	}
	pubKeyStr := auth.EncodePublicKey(&privKey.PublicKey)
	privKeyStr := auth.EncodePrivateKey(privKey)

	// 3. Create a new database object, copying the tables map
	newDb := &pb.Database{
		Tables:         sourceDb.Tables,
		OwnerPublicKey: pubKeyStr,
	}

	// 4. Add the new database object to IPFS
	newDbCID, err := ipfsdb.AddObject(sh, newDb)
	if err != nil {
		return fmt.Errorf("failed to save forked database object: %w", err)
	}

	// 5. Create a new IPNS key for the new database name
	key, err := sh.KeyGen(context.Background(), args.NewDbName, shell.KeyGen.Size(2048))
	if err != nil {
		return fmt.Errorf("failed to generate IPNS key for fork: %w", err)
	}

	// 6. Publish the new database CID to the new key
	go func() {
		_, err := sh.PublishWithDetails(newDbCID, key.Name, 0, 0, false)
		if err != nil {
			fmt.Printf("Warning: failed to publish forked database to IPNS: %v\n", err)
		}
	}()

	// 7. Save the new database info to the local registry
	entry := &pb.RegistryEntry{
		DbName:    args.NewDbName,
		ProgramId: key.Id,
		KeyName:   key.Name,
	}
	entryData, err := proto.Marshal(entry)
	if err != nil {
		return err
	}
	if err := ipfsdb.PutToCache([]byte("registry:"+args.NewDbName), entryData); err != nil {
		return fmt.Errorf("failed to save forked database to registry: %w", err)
	}

	// 8. Update the cache with the new database CID
	ipfsdb.UpdateCache(args.NewDbName, newDbCID)

	// 9. Return the new program ID and private key
	reply.ProgramID = key.Id
	reply.PrivateKey = privKeyStr

	return nil
}
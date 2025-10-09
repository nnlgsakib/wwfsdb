package rpc

import (
	"fmt"
	"net/http"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
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
		result, err := h.Dispatcher.ExecuteInTransaction(args.SessionID, stmt)
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
			DbName:  args.DbName,
			Query:   args.Query,
			Reply:   reply,
			ErrChan: make(chan error),
		}
		h.Dispatcher.Dispatch(job)
		return <-job.ErrChan
	}

	// For SELECT statements, execute immediately
	result, err := ipfsdb.ExecuteQuery(h.Dispatcher.ipfsApi, args.DbName, args.Query)
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
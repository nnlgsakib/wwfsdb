package rpc

import (
	"fmt"
	"net/http"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
)

// --- Service Definition ---

// WWFS is the service that exposes database operations via JSON-RPC.
type WWFS struct {
	Dispatcher *Dispatcher
}

// --- Method: wwfs_executeQuery ---

// ExecuteQueryArgs holds the arguments for the ExecuteQuery method.
type ExecuteQueryArgs struct {
	DbName string `json:"db_name"`
	Query  string `json:"query"`
}

// ExecuteQueryResult holds the result for the ExecuteQuery method.
type ExecuteQueryResult struct {
	Result string `json:"result"`
}

// ExecuteQuery is the RPC method that executes a SQL query against the database.
func (h *WWFS) ExecuteQuery(r *http.Request, args *ExecuteQueryArgs, reply *ExecuteQueryResult) error {
	job := Job{
		DbName:  args.DbName,
		Query:   args.Query,
		Reply:   reply,
		ErrChan: make(chan error),
	}

	h.Dispatcher.Dispatch(job)

	// Wait for the job to complete
	err := <-job.ErrChan
	return err
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

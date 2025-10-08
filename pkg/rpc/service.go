package rpc

import (
	"net/http"
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

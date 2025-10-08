package rpc

import (
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"
	"github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
)

// --- Job Queue / Dispatcher ---

// Job represents a query to be executed.
type Job struct {
	DbName  string
	Query   string
	Reply   *ExecuteQueryResult
	ErrChan chan error
}

// Dispatcher manages the job queues for all databases.
type Dispatcher struct {
	queues  map[string]chan Job
	mu      sync.Mutex
	ipfsApi string
}

// NewDispatcher creates a new dispatcher.
func NewDispatcher(ipfsApi string) *Dispatcher {
	return &Dispatcher{
		queues:  make(map[string]chan Job),
		ipfsApi: ipfsApi,
	}
}

// Dispatch sends a job to the appropriate database queue.
func (d *Dispatcher) Dispatch(job Job) {
	d.mu.Lock()
	queue, ok := d.queues[job.DbName]
	if !ok {
		queue = make(chan Job, 100) // Buffered channel
		d.queues[job.DbName] = queue
		go d.worker(queue)
	}
	d.mu.Unlock()

	queue <- job
}

// worker processes jobs from a single database queue.
func (d *Dispatcher) worker(queue chan Job) {
	for job := range queue {
		result, err := ipfsdb.ExecuteQuery(d.ipfsApi, job.DbName, job.Query)
		if err != nil {
			job.ErrChan <- err
		} else {
			job.Reply.Result = result
			job.ErrChan <- nil
		}
	}
}

// --- Server Setup ---

// NewServer creates a new JSON-RPC server.
func NewServer(ipfsApi string) http.Handler {
	dispatcher := NewDispatcher(ipfsApi)

	wwfsService := &WWFS{
		Dispatcher: dispatcher,
	}

	rpcServer := rpc.NewServer()
	rpcServer.RegisterCodec(json.NewCodec(), "application/json")
	rpcServer.RegisterService(wwfsService, "wwfs")

	router := mux.NewRouter()
	router.Handle("/rpc", rpcServer)

	return router
}

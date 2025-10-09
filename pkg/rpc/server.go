package rpc

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"
	shell "github.com/ipfs/go-ipfs-api"
	"github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
)

// --- Job Queue / Dispatcher ---

// Job represents a query to be executed.
type Job struct {
	DbName  string
	Query   string
	Reply   *ExecuteQueryResult
	ErrChan chan error
}

// Transaction holds the state for a series of operations.
type Transaction struct {
	id         string
	dbName     string
	dbState    *pb.Database // In-memory state of the database
	sh         *shell.Shell
	mu         sync.Mutex
	lastAccess time.Time
}

// Dispatcher manages active transactions and job queues.
// Dispatcher manages active transactions and legacy job queues.
type Dispatcher struct {
	transactions map[string]*Transaction // map[sessionID]*Transaction
	queues       map[string]chan Job     // For non-transactional writes
	mu           sync.Mutex
	ipfsApi      string
}

// NewDispatcher creates a new dispatcher.
func NewDispatcher(ipfsApi string) *Dispatcher {
	d := &Dispatcher{
		transactions: make(map[string]*Transaction),
		queues:       make(map[string]chan Job),
		ipfsApi:      ipfsApi,
	}
	go d.cleanupStaleTransactions()
	return d
}

func (d *Dispatcher) newSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (d *Dispatcher) BeginTransaction(dbName string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	sh := shell.NewShell(d.ipfsApi)

	// Load the initial state of the database.
	db, err := ipfsdb.LoadDatabase(sh, dbName)
	if err != nil {
		return "", fmt.Errorf("could not load database '%s': %w", dbName, err)
	}

	sessionID := d.newSessionID()
	txn := &Transaction{
		id:         sessionID,
		dbName:     dbName,
		dbState:    db,
		sh:         sh,
		lastAccess: time.Now(),
	}

	d.transactions[sessionID] = txn
	return sessionID, nil
}

func (d *Dispatcher) CommitTransaction(sessionID string) (string, error) {
	d.mu.Lock()
	txn, ok := d.transactions[sessionID]
	if !ok {
		d.mu.Unlock()
		return "", fmt.Errorf("transaction not found")
	}
	delete(d.transactions, sessionID)
	d.mu.Unlock()

	txn.mu.Lock()
	defer txn.mu.Unlock()

	// Add the final database state to IPFS
	finalCID, err := ipfsdb.AddObject(txn.sh, txn.dbState)
	if err != nil {
		return "", fmt.Errorf("failed to save committed state to IPFS: %w", err)
	}

	// Update cache and publish to IPNS
	ipfsdb.UpdateCache(txn.dbName, finalCID)
	ipfsdb.PublishAsync(txn.sh, txn.dbName, finalCID)

	return fmt.Sprintf("Commit successful. New database version: %s", finalCID), nil
}

func (d *Dispatcher) RollbackTransaction(sessionID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.transactions[sessionID]; !ok {
		return fmt.Errorf("transaction not found")
	}
	delete(d.transactions, sessionID)
	return nil
}

func (d *Dispatcher) ExecuteInTransaction(sessionID string, stmt ast.Statement) (string, error) {
	d.mu.Lock()
	txn, ok := d.transactions[sessionID]
	if !ok {
		d.mu.Unlock()
		return "", fmt.Errorf("transaction not found")
	}
	d.mu.Unlock()

	txn.mu.Lock()
	defer txn.mu.Unlock()

	txn.lastAccess = time.Now()

	// Execute the statement against the transaction's in-memory database state.
	newDbState, result, err := ipfsdb.ExecuteOnDB(txn.sh, txn.dbName, txn.dbState, stmt)
	if err != nil {
		return "", err
	}

	// Update the transaction's database state
	txn.dbState = newDbState

	return result, nil
}

func (d *Dispatcher) cleanupStaleTransactions() {
	for {
		time.Sleep(5 * time.Minute)
		d.mu.Lock()
		for id, txn := range d.transactions {
			if time.Since(txn.lastAccess) > (15 * time.Minute) {
				delete(d.transactions, id)
			}
		}
		d.mu.Unlock()
	}
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

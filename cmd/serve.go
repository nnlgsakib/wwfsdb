package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
	"github.com/spf13/cobra"
)

var port string

type QueryRequest struct {
	DbName string `json:"db_name"`
	Query  string `json:"query"`
}

type QueryResponse struct {
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start an HTTP server to listen for queries",
	Run: func(cmd *cobra.Command, args []string) {
		http.HandleFunc("/query", handleQuery)

		fmt.Println("Server listening on port", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			fmt.Println("Error starting server:", err)
		}
	},
}

func handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(QueryResponse{Error: err.Error()})
		return
	}

	result, err := ipfsdb.ExecuteQuery(ipfsApi, req.DbName, req.Query)
	if err != nil {
		json.NewEncoder(w).Encode(QueryResponse{Error: err.Error()})
		return
	}

	json.NewEncoder(w).Encode(QueryResponse{Result: result})
}

func init() {
	serveCmd.Flags().StringVarP(&port, "port", "p", "8080", "Port to listen on")
	rootCmd.AddCommand(serveCmd)
}
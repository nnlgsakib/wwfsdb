package cmd

import (
	"fmt"
	"net/http"

	"github.com/nnlgsakib/wwfsdb/pkg/rpc"
	"github.com/spf13/cobra"
)

var port string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a JSON-RPC server to listen for queries",
	Run: func(cmd *cobra.Command, args []string) {
		server := rpc.NewServer(ipfsApi)

		fmt.Println("JSON-RPC server listening on port", port)
		if err := http.ListenAndServe(":"+port, server); err != nil {
			fmt.Println("Error starting server:", err)
		}
	},
}

func init() {
	serveCmd.Flags().StringVarP(&port, "port", "p", "8080", "Port to listen on")
	rootCmd.AddCommand(serveCmd)
}
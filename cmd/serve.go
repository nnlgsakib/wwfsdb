package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
	"github.com/nnlgsakib/wwfsdb/pkg/rpc"
	"github.com/spf13/cobra"
)

var port string
var ipfsApi string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a JSON-RPC server to listen for queries",
	Run: func(cmd *cobra.Command, args []string) {
		if err := ipfsdb.InitCache(); err != nil {
			fmt.Println("Error initializing cache:", err)
			os.Exit(1)
		}
		defer ipfsdb.CloseCache()

		server := rpc.NewServer(ipfsApi)

		fmt.Println("JSON-RPC server listening on port", port)
		if err := http.ListenAndServe(":"+port, server); err != nil {
			fmt.Println("Error starting server:", err)
		}
	},
}

func init() {
	serveCmd.Flags().StringVarP(&port, "port", "p", "8080", "Port to listen on")
	serveCmd.Flags().StringVar(&ipfsApi, "api", "localhost:5001", "IPFS API endpoint")
	rootCmd.AddCommand(serveCmd)
}

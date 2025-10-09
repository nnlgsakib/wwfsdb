package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
	"github.com/nnlgsakib/wwfsdb/pkg/rpc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a JSON-RPC server to listen for queries",
	Run: func(cmd *cobra.Command, args []string) {
		cachePath := viper.GetString("cache-path")
		if err := ipfsdb.InitCache(cachePath); err != nil {
			fmt.Println("Error initializing cache:", err)
			os.Exit(1)
		}
		defer ipfsdb.CloseCache()

		ipfsApi := viper.GetString("ipfs-api")
		server := rpc.NewServer(ipfsApi)

		port := viper.GetString("port")
		fmt.Println("JSON-RPC server listening on port", port)
		if err := http.ListenAndServe(":"+port, server); err != nil {
			fmt.Println("Error starting server:", err)
		}
	},
}

func init() {
	serveCmd.Flags().StringP("port", "p", "8080", "Port to listen on")
	serveCmd.Flags().String("api", "localhost:5001", "IPFS API endpoint")
	rootCmd.AddCommand(serveCmd)
}

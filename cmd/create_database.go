package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nnlgsakib/wwfsdb/pkg/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// createDatabaseCmd represents the createdatabase command
var createDatabaseCmd = &cobra.Command{
	Use:   "createdatabase [sql_query]",
	Short: "Create a new database",
	Long:  `Create a new database.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		queryString := args[0]

		c := client.NewClient(viper.GetString("rpc-server"))
		// For CREATE DATABASE, dbName and privateKey are not needed in the client call itself
		result, err := c.ExecuteQuery("", queryString, "", "")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}

		var createResult map[string]string
		if err := json.Unmarshal([]byte(result.Result), &createResult); err != nil {
			fmt.Fprintln(os.Stderr, "Error parsing create database result:", err)
			// Fallback to printing the raw result if JSON parsing fails
			fmt.Println(result.Result)
			return
		}

		fmt.Printf("Database created successfully.\n")
		fmt.Printf("Program ID (IPNS): %s\n", createResult["program_id"])
		fmt.Printf("\nIMPORTANT: Save this private key. It is required for all future write operations.\n")
		fmt.Printf("Private Key: %s\n", createResult["private_key"])
	},
}

func init() {
	rootCmd.AddCommand(createDatabaseCmd)
}

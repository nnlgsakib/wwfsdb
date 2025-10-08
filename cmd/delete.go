package cmd

import (
    "fmt"
    "os"

    "github.com/nnlgsakib/wwfsdb/pkg/client"
    "github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete [db_name] [sql_query]",
	Short: "Execute a DELETE statement",
	Long:  `Executes a DELETE statement against the database to remove rows based on the WHERE clause.`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		dbName := args[0]
		queryString := args[1]

		fmt.Printf("Executing on database '%s': \"%s\"\n", dbName, queryString)

		c := client.NewClient(rpcServerAddr)
		result, err := c.ExecuteQuery(dbName, queryString)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}

		fmt.Println(result)
		fmt.Printf("\nDatabase '%s' has been updated. Note: IPNS propagation can take some time.\n", dbName)
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}

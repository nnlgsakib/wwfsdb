package cmd

import (
	"fmt"
	"os"

	"github.com/nnlgsakib/wwfsdb/pkg/client"
	"github.com/spf13/cobra"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update [db_name] [sql_query]",
	Short: "Execute an UPDATE statement",
	Long:  `Executes an UPDATE statement against the database to modify rows based on the WHERE clause.`,
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
	rootCmd.AddCommand(updateCmd)
}

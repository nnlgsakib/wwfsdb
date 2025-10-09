package cmd

import (
    "fmt"
    "os"

    "github.com/nnlgsakib/wwfsdb/pkg/client"
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

// dropCmd represents the drop command
var dropCmd = &cobra.Command{
	Use:   "drop [db_name] [sql_query]",
	Short: "Execute a DROP statement",
	Long:  `Executes a DROP statement against the database to remove a table.`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		dbName := args[0]
		queryString := args[1]

		fmt.Printf("Executing on database '%s': \"%s\"\n", dbName, queryString)

		c := client.NewClient(viper.GetString("rpc-server"))
		result, err := c.ExecuteQuery(dbName, queryString, "")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}

		fmt.Println(result)
		fmt.Printf("\nDatabase '%s' has been updated. Note: IPNS propagation can take some time.\n", dbName)
	},
}

func init() {
	rootCmd.AddCommand(dropCmd)
}

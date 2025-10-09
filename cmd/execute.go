package cmd

import (
	"fmt"
	"os"

	"github.com/nnlgsakib/wwfsdb/pkg/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// executeCmd represents the execute command
var executeCmd = &cobra.Command{
	Use:   "execute [db_name] [sql_query]",
	Short: "Execute an INSERT statement",
	Long:  `Executes an INSERT statement against the database to add a new row.`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		dbName := args[0]
		queryString := args[1]
		privateKey := viper.GetString("private-key")

		fmt.Printf("Executing on database '%s': \"%s\"\n", dbName, queryString)

		c := client.NewClient(viper.GetString("rpc-server"))
		result, err := c.ExecuteQuery(dbName, queryString, "", privateKey)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}

		fmt.Println(result)
		fmt.Printf("\nDatabase '%s' has been updated. Note: IPNS propagation can take some time.\n", dbName)
	},
}

func init() {
	rootCmd.AddCommand(executeCmd)
}

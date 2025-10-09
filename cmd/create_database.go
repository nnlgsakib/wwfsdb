package cmd

import (
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
		result, err := c.ExecuteQuery("", queryString, "") // dbName is not needed here
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}

		fmt.Println(result)
	},
}

func init() {
	rootCmd.AddCommand(createDatabaseCmd)
}

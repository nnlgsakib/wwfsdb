package cmd

import (
	"fmt"
	"os"

	"github.com/nnlgsakib/wwfsdb/pkg/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var deleteDatabaseCmd = &cobra.Command{
	Use:   "deletedatabase [db_name]",
	Short: "Delete a database",
	Long:  `Delete a database. This will remove the database from the local registry. The data will still exist on IPFS, but it will be garbage collected eventually if not pinned.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dbName := args[0]

		c := client.NewClient(viper.GetString("rpc-server"))
		result, err := c.DeleteDatabase(dbName)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}

		fmt.Println(result.Result)
	},
}

func init() {
	rootCmd.AddCommand(deleteDatabaseCmd)
}

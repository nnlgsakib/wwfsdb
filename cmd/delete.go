package cmd

import (
    "fmt"
    "os"

    "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
    ssql "github.com/nnlgsakib/wwfsdb/pkg/ssql"
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

        tableName, whereClause, err := ssql.ParseDelete(queryString)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }

		err = ipfsdb.Delete(ipfsApi, dbName, tableName, whereClause.Column, whereClause.Value)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("\nSuccessfully deleted data and updated database state!\n")
		fmt.Printf("Database '%s' has been updated locally. Run 'query' to see the changes immediately.\n", dbName)
		fmt.Println("Note: IPNS propagation can take some time for the changes to be reflected elsewhere.")
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}

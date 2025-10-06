package cmd

import (
    "fmt"
    "os"

    "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
    "github.com/nnlgsakib/wwfsdb/pkg/parser"
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

        tableName, updateClause, whereClause, err := parser.ParseUpdate(queryString)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }

		err = ipfsdb.Update(ipfsApi, dbName, tableName, updateClause.Column, updateClause.Value, whereClause.Column, whereClause.Value)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("\nSuccessfully updated data and updated database state!\n")
		fmt.Printf("Database '%s' has been updated locally. Run 'query' to see the changes immediately.\n", dbName)
		fmt.Println("Note: IPNS propagation can take some time for the changes to be reflected elsewhere.")
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

package cmd

import (
    "fmt"
    "os"

    "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
    ssql "github.com/nnlgsakib/wwfsdb/pkg/ssql"
    "github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
    "github.com/spf13/cobra"
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

		stmt, err := ssql.Parse(queryString)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}

		dropStmt, ok := stmt.(*ast.DropTableStmt)
		if !ok {
			fmt.Fprintln(os.Stderr, "Error: invalid DROP TABLE statement")
			return
		}
		tableName := dropStmt.Name

		err = ipfsdb.Drop(ipfsApi, dbName, tableName)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}

		fmt.Printf("\nSuccessfully dropped table and updated database state!\n")
		fmt.Printf("Database '%s' has been updated locally. Run 'query' to see the changes immediately.\n", dbName)
		fmt.Println("Note: IPNS propagation can take some time for the changes to be reflected elsewhere.")
	},
}

func init() {
	rootCmd.AddCommand(dropCmd)
}

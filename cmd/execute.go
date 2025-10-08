package cmd

import (
	"fmt"
	"os"

	    "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
	    ssql "github.com/nnlgsakib/wwfsdb/pkg/ssql"
	    "github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
	    "github.com/spf13/cobra"
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

		fmt.Printf("Executing on database '%s': \"%s\"\n", dbName, queryString)

		stmt, err := ssql.Parse(queryString)
		if err != nil {
			fmt.Printf("Error parsing query: %v\n", err)
			os.Exit(1)
		}

		insertStmt, ok := stmt.(*ast.InsertStmt)
		if !ok {
			fmt.Printf("Error: not an INSERT statement\n")
			os.Exit(1)
		}
		tableName := insertStmt.Table
		values := insertStmt.Values

		err = ipfsdb.Insert(ipfsApi, dbName, tableName, values)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("\nSuccessfully inserted data and updated database state!\n")
		fmt.Printf("Database '%s' has been updated locally. Run 'query' to see the changes immediately.\n", dbName)
		fmt.Println("Note: IPNS propagation can take some time for the changes to be reflected elsewhere.")
	},
}

func init() {
	rootCmd.AddCommand(executeCmd)
}

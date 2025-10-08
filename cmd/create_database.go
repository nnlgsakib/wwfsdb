package cmd

import (
    "fmt"

    "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
    ssql "github.com/nnlgsakib/wwfsdb/pkg/ssql"
    "github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
    "github.com/spf13/cobra"
)

// createDatabaseCmd represents the createdatabase command
var createDatabaseCmd = &cobra.Command{
	Use:   "createdatabase [sql_query]",
	Short: "Create a new database",
	Long:  `Create a new database.`, 
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		queryString := args[0]

		stmt, err := ssql.Parse(queryString)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		createDbStmt, ok := stmt.(*ast.CreateDatabaseStmt)
		if !ok {
			fmt.Printf("Error: invalid CREATE DATABASE statement\n")
			return
		}
		dbName := createDbStmt.Name

		programID, err := ipfsdb.CreateDatabase(ipfsApi, dbName)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("Database '%s' created successfully.\n", dbName)
		fmt.Printf("Program ID: %s\n", programID)
	},
}

func init() {
	rootCmd.AddCommand(createDatabaseCmd)
}

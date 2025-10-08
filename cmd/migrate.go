/*
Copyright © 2025 NNLGSakib
*/
package cmd

import (
    "fmt"
    "os"

    "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
    ssql "github.com/nnlgsakib/wwfsdb/pkg/ssql"
    "github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
    "github.com/spf13/cobra"
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate [db_name] [ssql_file]",
	Short: "Migrate a .ssql file to create a new table in a database",
	Long: `This command parses a .ssql file and adds a new table to an existing database.
`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		dbName := args[0]
		ssqlFile := args[1]

		fmt.Printf("Migrating tables from file '%s' to database '%s'\n", ssqlFile, dbName)

		// Parse the ssql file
		content, err := os.ReadFile(ssqlFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading file:", err)
			return
		}

		stmts, err := ssql.ParseMultiple(string(content))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error parsing ssql file:", err)
			return
		}

		for _, stmt := range stmts {
			createStmt, ok := stmt.(*ast.CreateTableStmt)
			if !ok {
				fmt.Fprintln(os.Stderr, "Error: file can only contain CREATE TABLE statements")
				continue
			}
			tableName := createStmt.Name
			schema := &createStmt.Schema
			dbCid, err := ipfsdb.Migrate(ipfsApi, dbName, tableName, schema)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error migrating table:", err)
				continue
			}
			fmt.Printf("Table '%s' created successfully in database '%s'.\n", tableName, dbName)
			fmt.Printf("Database CID: %s\n", dbCid)
		}
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

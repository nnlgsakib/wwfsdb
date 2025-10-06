/*
Copyright © 2025 NNLGSakib
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
	"github.com/nnlgsakib/wwfsdb/pkg/parser"
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
			fmt.Printf("Error reading file %s: %v\n", ssqlFile, err)
			os.Exit(1)
		}

		tables, err := parser.ParseMultipleCreateTables(string(content))
		if err != nil {
			fmt.Printf("Error parsing ssql file: %v\n", err)
			os.Exit(1)
		}

		for tableName, schema := range tables {
			dbCid, err := ipfsdb.Migrate(ipfsApi, dbName, tableName, schema)
			if err != nil {
				fmt.Printf("Error migrating table '%s': %v\n", tableName, err)
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

/*
Copyright © 2025 NNLGSakib
*/
package cmd

import (
    "fmt"
    "os"

    "github.com/nnlgsakib/wwfsdb/pkg/client"
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

		content, err := os.ReadFile(ssqlFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading file:", err)
			return
		}

		c := client.NewClient(rpcServerAddr)
		result, err := c.ExecuteQuery(dbName, string(content))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}

		fmt.Println(result)
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

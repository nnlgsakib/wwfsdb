package cmd

import (
    "fmt"

    "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
    ssql "github.com/nnlgsakib/wwfsdb/pkg/ssql"
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

        dbName, err := ssql.ParseCreateDatabase(queryString)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            return
        }

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

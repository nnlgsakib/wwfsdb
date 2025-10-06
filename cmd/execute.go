package cmd

import (
    "fmt"
    "os"
    "strings"

    "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
    ssql "github.com/nnlgsakib/wwfsdb/pkg/ssql"
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

		// --- 1. Detect query type and delegate ---
		queryType := strings.TrimSpace(strings.ToUpper(strings.Split(queryString, " ")[0]))
		if queryType != "INSERT" {
			fmt.Printf("Error: Unsupported query type '%s'. Only INSERT is supported.\n", queryType)
			os.Exit(1)
		}

        tableName, values, err := ssql.ParseInsert(queryString)
        if err != nil {
            fmt.Printf("Error parsing query: %v\n", err)
            os.Exit(1)
        }

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

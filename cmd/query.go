package cmd

import (
	"fmt"
	"os"
	"strings"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
	"github.com/spf13/cobra"
)

// queryCmd represents the query command
var queryCmd = &cobra.Command{
	Use:   "query [db_name] [query_string]",
	Short: "Execute a SELECT query against a database",
	Long: `Executes a SELECT query against a database stored in IPFS.
It resolves the database's permanent Program ID (IPNS Name) to get the latest state.`, 
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		dbName := args[0]
		queryString := args[1]

		fmt.Printf("Querying database '%s' with: \"%s\"\n", dbName, queryString)

		// --- 1. Parse the query string ---
		tableName, whereClause, err := ipfsdb.ParseSelect(queryString)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		var whereColumn, whereValue string
		if whereClause != nil {
			whereColumn = whereClause.Column
			whereValue = whereClause.Value
		}

		// --- 2. Execute the query using ipfsdb.Query ---
		rows, err := ipfsdb.Query(ipfsApi, dbName, tableName, whereColumn, whereValue)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// --- 3. Get the schema for printing headers ---
		sh := shell.NewShell(ipfsApi)
		db, err := ipfsdb.LoadDatabase(sh, dbName)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		tableCID, ok := db.Tables[tableName]
		if !ok {
			fmt.Printf("Error: could not find table %s\n", tableName)
			os.Exit(1)
		}
		table, err := ipfsdb.LoadTable(sh, tableCID)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		schema, err := ipfsdb.LoadSchema(sh, table.SchemaCID)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// --- 4. Print results ---
		var headers []string
		for _, col := range schema.Columns {
			headers = append(headers, col.Name)
		}
		fmt.Println(strings.Join(headers, "\t| "))
		fmt.Println(strings.Repeat("----", len(headers)*2))

		// Print rows
		if len(rows) == 0 {
			fmt.Println("(0 rows)")
		} else {
			for _, row := range rows {
				var rowValues []string
				for _, col := range schema.Columns {
					val, _ := row[col.Name]
					rowValues = append(rowValues, fmt.Sprintf("%v", val))
				}
				fmt.Println(strings.Join(rowValues, "\t| "))
			}
			fmt.Printf("\n(%d rows)\n", len(rows))
		}
	},
}

func init() {
	rootCmd.AddCommand(queryCmd)
}
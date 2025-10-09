package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/nnlgsakib/wwfsdb/pkg/client"
	ssql "github.com/nnlgsakib/wwfsdb/pkg/ssql"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

		// --- 1. Parse the query string to get table name and columns --- 
		stmt, err := ssql.Parse(queryString)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}

		_, ok := stmt.(*ast.SelectStmt)
		if !ok {
			fmt.Fprintln(os.Stderr, "Error: invalid SELECT statement")
			return
		}

		// --- 2. Execute the query using RPC client --- 
		c := client.NewClient(viper.GetString("rpc-server"))
		result, err := c.ExecuteQuery(dbName, queryString, "", "")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}

		var rows []map[string]interface{}
		if err := json.Unmarshal([]byte(result.Result), &rows); err != nil {
			fmt.Fprintln(os.Stderr, "Error unmarshalling result:", err)
			return
		}

		// --- 3. Get headers from the first row's keys --- 
		if len(rows) == 0 {
			fmt.Println("(0 rows)")
			return
		}
		
		var headers []string
		for key := range rows[0] {
			headers = append(headers, key)
		}
		sort.Strings(headers) // Sort for consistent order

		// --- 4. Print results --- 
		fmt.Println(strings.Join(headers, "\t| "))
		fmt.Println(strings.Repeat("----", len(headers)*2))

		// Print rows
		for _, row := range rows {
			var rowValues []string
			for _, colName := range headers {
				val, _ := row[colName]
				// To handle different types, we can marshal to JSON
				jsonVal, err := json.Marshal(val)
				if err != nil {
					rowValues = append(rowValues, fmt.Sprintf("ERR"))
				} else {
					rowValues = append(rowValues, string(jsonVal))
				}
			}
			fmt.Println(strings.Join(rowValues, "\t| "))
		}
		fmt.Printf("\n(%d rows)\n", len(rows))
	},
}

func init() {
	rootCmd.AddCommand(queryCmd)
}

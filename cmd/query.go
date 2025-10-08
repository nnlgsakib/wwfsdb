package cmd

import (
	"fmt"
	"os"
	"strings"

	    shell "github.com/ipfs/go-ipfs-api"
	    "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb"
	    ssql "github.com/nnlgsakib/wwfsdb/pkg/ssql"
	    "github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
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
		stmt, err := ssql.Parse(queryString)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}

		selectStmt, ok := stmt.(*ast.SelectStmt)
		if !ok {
			fmt.Fprintln(os.Stderr, "Error: invalid SELECT statement")
			return
		}
		tableName := selectStmt.Table
		whereClause := selectStmt.Where

		// --- 2. Execute the query using ipfsdb.Query ---
		rows, err := ipfsdb.Query(ipfsApi, dbName, tableName, selectStmt.Columns, whereClause)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}

		// --- 3. Get the schema for printing headers ---
		sh := shell.NewShell(ipfsApi)
		db, err := ipfsdb.LoadDatabase(sh, dbName)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}
		tableCID, ok := db.Tables[tableName]
		if !ok {
			fmt.Fprintln(os.Stderr, "Error: could not find table", tableName)
			return
		}
		table, err := ipfsdb.LoadTable(sh, tableCID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}
		schema, err := ipfsdb.LoadSchema(sh, table.SchemaCID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return
		}

		// --- 4. Print results ---
		var headers []string
		if len(selectStmt.Columns) == 1 && selectStmt.Columns[0] == "*" {
			for _, col := range schema.Columns {
				headers = append(headers, col.Name)
			}
		} else {
			headers = selectStmt.Columns
		}
		fmt.Println(strings.Join(headers, "\t| "))
		fmt.Println(strings.Repeat("----", len(headers)*2))

		// Print rows
		if len(rows) == 0 {
			fmt.Println("(0 rows)")
		} else {
			for _, row := range rows {
				var rowValues []string
				for _, colName := range headers {
					val, _ := row[colName]
					rowValues = append(rowValues, fmt.Sprintf("%v", val))
				}
				fmt.Println(strings.Join(rowValues, "\t| "))
			}
			fmt.Printf("\n(%d rows)\n", len(rows))
		}	},
}

func init() {
	rootCmd.AddCommand(queryCmd)
}

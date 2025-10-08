
package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nnlgsakib/wwfsdb/pkg/client"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
	"github.com/spf13/cobra"
)

var shellCmd = &cobra.Command{
	Use:   "shell [db_name]",
	Short: "Start an interactive SQL shell",
	Long: `Starts an interactive SQL shell to execute commands against a specified database.
Example:
  wwfsdb shell my_db`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dbName := args[0]
		c := client.NewClient(rpcServerAddr)

		fmt.Printf("Connected to database '%s'. Type 'exit' or 'quit' to leave.\n", dbName)

		scanner := bufio.NewScanner(os.Stdin)
		var query strings.Builder

		for {
			if query.Len() == 0 {
				fmt.Print("wwfsdb> ")
			} else {
				fmt.Print("     -> ")
			}

			if !scanner.Scan() {
				break
			}

			line := scanner.Text()
			line = strings.TrimSpace(line)

			if line == "exit" || line == "quit" {
				break
			}

			query.WriteString(line)
			query.WriteString(" ")

			if !strings.HasSuffix(line, ";") {
				continue
			}

			queryString := query.String()
			query.Reset()

			stmt, err := ssql.Parse(queryString)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				continue
			}

			resultString, err := c.ExecuteQuery(dbName, queryString)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				continue
			}

			// Handle different statement types
			switch s := stmt.(type) {
			case *ast.SelectStmt:
				var rows []map[string]interface{}
				if err := json.Unmarshal([]byte(resultString), &rows); err != nil {
					fmt.Fprintln(os.Stderr, "Error unmarshalling result:", err)
					continue
				}

				var headers []string
				if len(s.Columns) == 1 && s.Columns[0] == "*" {
					schema, err := c.GetTableSchema(dbName, s.Table)
					if err != nil {
						fmt.Fprintln(os.Stderr, "Error getting schema:", err)
						continue
					}
					for _, col := range schema.Columns {
						headers = append(headers, col.Name)
					}
				} else {
					headers = s.Columns
				}

				// Print results
				fmt.Println(strings.Join(headers, "\t| "))
				fmt.Println(strings.Repeat("----", len(headers)*2))

				if len(rows) == 0 {
					fmt.Println("(0 rows)")
				} else {
					for _, row := range rows {
						var rowValues []string
						for _, colName := range headers {
							val, _ := row[colName]
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
				}
			default:
				fmt.Println(resultString)
			}
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
}

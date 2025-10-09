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
	"github.com/spf13/viper"
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
		c := client.NewClient(viper.GetString("rpc-server"))
		privateKey := viper.GetString("private-key")
		var sessionID string
		inTransaction := false

		fmt.Printf("Connected to database '%s'. Type 'exit' or 'quit' to leave.\n", dbName)

		scanner := bufio.NewScanner(os.Stdin)
		var query strings.Builder

		for {
			prompt := "wwfsdb> "
			if inTransaction {
				prompt = fmt.Sprintf("wwfsdb (%s)*> ", dbName)
			} else if query.Len() > 0 {
				prompt = "     -> "
			}
			fmt.Print(prompt)

			if !scanner.Scan() {
				break
			}

			line := scanner.Text()
			trimmedLine := strings.TrimSpace(line)

			if trimmedLine == "exit" || trimmedLine == "quit" {
				break
			}

			query.WriteString(line)
			query.WriteString(" ")

			if !strings.HasSuffix(trimmedLine, ";") {
				continue
			}

			queryString := query.String()
			query.Reset()

			// --- Client-side check for transaction commands ---
			trimmedQuery := strings.TrimSpace(strings.ToUpper(strings.TrimRight(queryString, "; ")))

			stmt, err := ssql.Parse(queryString)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				continue
			}

			// --- Execute Query ---
			result, err := c.ExecuteQuery(dbName, queryString, sessionID, privateKey)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				if inTransaction {
					fmt.Fprintln(os.Stderr, "Transaction may be in an inconsistent state. It is recommended to ROLLBACK.")
				}
				continue
			}

			if trimmedQuery == "BEGIN" {
				inTransaction = true
				sessionID = result.SessionID
			} else if trimmedQuery == "COMMIT" || trimmedQuery == "ROLLBACK" {
				inTransaction = false
				sessionID = ""
			}

			// Handle different statement types
			switch s := stmt.(type) {
			case *ast.SelectStmt:
				var rows []map[string]interface{}
				if err := json.Unmarshal([]byte(result.Result), &rows); err != nil {
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
				fmt.Println(result.Result)
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
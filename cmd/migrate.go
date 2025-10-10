/*
Copyright © 2025 NNLGSakib
*/
package cmd

import (
    "fmt"
    "os"
    "regexp"
    "strings"

    "github.com/nnlgsakib/wwfsdb/pkg/client"
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate [db_name] [ssql_file]",
	Short: "Migrate a .ssql file to create tables in a database",
	Long: `This command parses a .ssql file and adds tables to an existing database.
It processes all CREATE TABLE statements in the file.`,
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

		// Split the content into individual statements based on semicolons
		statements := splitSQLStatements(string(content))

		privateKey := viper.GetString("private-key")
		c := client.NewClient(viper.GetString("rpc-server"))

		// Execute each statement
		for _, stmt := range statements {
			statementStr := strings.TrimSpace(stmt)
			if statementStr == "" {
				continue
			}

			fmt.Printf("Executing: %s\n", statementStr)
			
			result, err := c.ExecuteQuery(dbName, statementStr, "", privateKey)
			if err != nil {
				// Don't exit on error; just report it and continue with the next statement
				fmt.Fprintf(os.Stderr, "Error executing statement: %v\n", err)
				fmt.Fprintf(os.Stderr, "Statement: %s\n", statementStr)
				continue
			}

			fmt.Println(result)
		}
	},
}

// splitSQLStatements splits a string containing multiple SQL statements separated by semicolons
func splitSQLStatements(sql string) []string {
	// Remove comments and extra whitespace
	sql = removeSQLComments(sql)
	
	// Use regex to split on semicolons followed by optional whitespace/newlines
	re := regexp.MustCompile(`;[\s\n\r]*$|;[\s\n\r]+`)
	parts := re.Split(sql, -1)
	
	var statements []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			// Add back the semicolon that was removed by the split
			statements = append(statements, part+";")
		}
	}
	
	return statements
}

// removeSQLComments removes SQL comments from the given string
func removeSQLComments(sql string) string {
	// Remove line comments --
	lines := strings.Split(sql, "\n")
	var cleanLines []string
	
	for _, line := range lines {
		// Check for line comment
		if idx := strings.Index(line, "--"); idx != -1 {
			line = line[:idx]
		}
		cleanLines = append(cleanLines, line)
	}
	
	sql = strings.Join(cleanLines, "\n")
	
	// Remove block comments /* */
	re := regexp.MustCompile(`/\*.*?\*/`)
	sql = re.ReplaceAllString(sql, "")
	
	return sql
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

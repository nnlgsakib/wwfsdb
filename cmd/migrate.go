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
    	Use:   "migrate [db_name] [schema_file]",
    	Short: "Migrate a .ssql or .nsc file to create/update schemas in a database",
    	Long: `This command parses a schema file (.ssql or .nsc) and applies it to the database.
    For .ssql files, it processes all CREATE TABLE statements.
    For .nsc files, it parses the NSchema definition and updates the database models.`,
    	Args: cobra.ExactArgs(2),
    	Run: func(cmd *cobra.Command, args []string) {
    		dbName := args[0]
    		schemaFile := args[1]
    
    		fmt.Printf("Applying schema from file '%s' to database '%s'\n", schemaFile, dbName)
    
    		content, err := os.ReadFile(schemaFile)
    		if err != nil {
    			fmt.Fprintln(os.Stderr, "Error reading file:", err)
    			return
    		}
    
    		privateKey := viper.GetString("private-key")
    		c := client.NewClient(viper.GetString("rpc-server"))
    
    		// Check file type and dispatch accordingly
    		if strings.HasSuffix(schemaFile, ".nsc") {
    			// Handle NSchema file
    			if !strings.HasPrefix(string(content), "pragma nschema;") {
    				fmt.Fprintln(os.Stderr, "Error: .nsc file must start with 'pragma nschema;'")
    				return
    			}
    			fmt.Println("NSchema file detected. Sending to server for processing...")
    			// In the future, we might parse it client-side first.
    			// For now, send the whole file content to the RPC server.
    			result, err := c.ExecuteQuery(dbName, string(content), "", privateKey)
    			if err != nil {
    				fmt.Fprintf(os.Stderr, "Error executing NSchema migration: %v\n", err)
    				return
    			}
    			fmt.Println(result)
    
    		} else if strings.HasSuffix(schemaFile, ".ssql") {
    			// Handle SSQL file (existing logic)
    			statements := splitSQLStatements(string(content))
    			for _, stmt := range statements {
    				statementStr := strings.TrimSpace(stmt)
    				if statementStr == "" {
    					continue
    				}
    
    				fmt.Printf("Executing: %s\n", statementStr)
    				result, err := c.ExecuteQuery(dbName, statementStr, "", privateKey)
    				if err != nil {
    					fmt.Fprintf(os.Stderr, "Error executing statement: %v\n", err)
    					fmt.Fprintf(os.Stderr, "Statement: %s\n", statementStr)
    					continue
    				}
    				fmt.Println(result)
    			}
    		} else {
    			fmt.Fprintln(os.Stderr, "Error: unsupported schema file type. Please use .ssql or .nsc")
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

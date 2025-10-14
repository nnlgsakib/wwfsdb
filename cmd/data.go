package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/nnlgsakib/wwfsdb/pkg/client"
	"github.com/nnlgsakib/wwfsdb/pkg/core/proto"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	exportFormat string
	exportOutput string
	importFormat string
	importInput  string
)

// dataCmd represents the data command
var dataCmd = &cobra.Command{
	Use:   "data",
	Short: "Import and export data from a wwfsdb table",
	Long:  `Provides tools for bulk loading data into a table from a file (import) and saving a table's contents to a file (export).`,
}

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export [db_name] [table_name]",
	Short: "Export a full database or a single table to a file",
	Long: `Exports data to a file.
- To export a single table, provide the database name and table name.
- To export the entire database, provide only the database name.

Supported formats are 'dag-json' and 'car'.`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		dbName := args[0]
		tableName := ""
		if len(args) == 2 {
			tableName = args[1]
		}

		c := client.NewClient(viper.GetString("rpc-server"))

		if exportFormat == "dag-json" {
			if tableName == "" {
				fmt.Printf("Exporting entire database '%s' to '%s' in dag-json format...\n", dbName, exportOutput)
			} else {
				fmt.Printf("Exporting table '%s.%s' to '%s' in dag-json format...\n", dbName, tableName, exportOutput)
			}

			jsonData, err := c.Export(dbName, tableName)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error during export:", err)
				os.Exit(1)
			}

			// Pretty-print the JSON before writing
			var prettyBuffer bytes.Buffer
			if err := json.Indent(&prettyBuffer, []byte(jsonData), "", "  "); err != nil {
				fmt.Fprintln(os.Stderr, "Error formatting JSON:", err)
				os.Exit(1)
			}

			if err := os.WriteFile(exportOutput, prettyBuffer.Bytes(), 0644); err != nil {
				fmt.Fprintln(os.Stderr, "Error writing to output file:", err)
				os.Exit(1)
			}
			fmt.Printf("Successfully exported data to %s\n", exportOutput)

		} else if exportFormat == "car" {
			rootCID, err := c.GetContentCID(dbName, tableName)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error getting content CID for export:", err)
				os.Exit(1)
			}

			ipfsApi := viper.GetString("ipfs-api")
			sh := shell.NewShell(ipfsApi)

			fmt.Printf("Exporting DAG (root: %s) to CAR file '%s'...\n", rootCID, exportOutput)

			req := sh.Request("dag/export", rootCID)
			res, err := req.Send(context.Background())
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error sending DAG export request to IPFS:", err)
				os.Exit(1)
			}
			defer res.Close()

			if res.Error != nil {
				fmt.Fprintln(os.Stderr, "Error from IPFS API during DAG export:", res.Error)
				os.Exit(1)
			}

			file, err := os.Create(exportOutput)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error creating output file:", err)
				os.Exit(1)
			}
			defer file.Close()

			bytesWritten, err := io.Copy(file, res.Output)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error writing CAR data to file:", err)
				os.Exit(1)
			}
			fmt.Printf("Successfully exported CAR file to %s (%d bytes)\n", exportOutput, bytesWritten)

		} else {
			fmt.Fprintf(os.Stderr, "Error: unsupported format '%s'. Please use 'dag-json' or 'car'.\n", exportFormat)
			os.Exit(1)
		}
	},
}

var importCmd = &cobra.Command{
	Use:   "import [db_name] [table_name]",
	Short: "Import data from a file into a table or a full database",
	Long: `Imports data from a file.

- To import into a single table from a 'dag-json' file, provide the database name and table name.
  The JSON file should be an array of row objects.

- To import into a full database from a 'dag-json' file, provide only the database name.
  The JSON file should be an object where keys are table names and values are arrays of row objects.

- To import a database from a 'car' file, provide a new name for the database and the path to the .car file.
  This will create a new writable fork of the database.

Note: For JSON imports, this command performs one write operation per row and may be slow for very large datasets.`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		dbName := args[0]

		if importFormat == "car" {
			if len(args) != 1 {
				fmt.Fprintln(os.Stderr, "Error: when using --format car, you must provide exactly one argument: the new database name.")
				os.Exit(1)
			}
			importDatabaseFromCAR(dbName)
			return
		}

		// JSON import logic continues here
		privateKey := viper.GetString("private-key")
		if privateKey == "" {
			fmt.Fprintln(os.Stderr, "Error: a private key is required for JSON import operations. Use the --private-key flag or set it in your config.")
			os.Exit(1)
		}

		isDbImport := len(args) == 1

		fileContent, err := os.ReadFile(importInput)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input file:", err)
			os.Exit(1)
		}

		c := client.NewClient(viper.GetString("rpc-server"))

		if isDbImport {
			// --- Database Import from JSON ---
			var dbData map[string][]map[string]interface{}
			if err := json.Unmarshal(fileContent, &dbData); err != nil {
				fmt.Fprintln(os.Stderr, "Error parsing JSON for full database import:", err)
				fmt.Fprintln(os.Stderr, "Expected a JSON object with table names as keys and arrays of rows as values.")
				os.Exit(1)
			}

			fmt.Printf("Starting import of %d tables into database '%s'...\n", len(dbData), dbName)

			for tableName, rows := range dbData {
				fmt.Printf("--- Importing %d rows into table '%s' ---\n", len(rows), tableName)
				schema, err := c.GetTableSchema(dbName, tableName)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error getting schema for table '%s', skipping: %v\n", tableName, err)
					continue
				}
				importRows(c, dbName, tableName, privateKey, schema, rows)
			}

		} else {
			// --- Single Table Import from JSON ---
			tableName := args[1]
			var rows []map[string]interface{}
			if err := json.Unmarshal(fileContent, &rows); err != nil {
				fmt.Fprintln(os.Stderr, "Error parsing JSON for single table import:", err)
				fmt.Fprintln(os.Stderr, "Expected a JSON array of row objects.")
				os.Exit(1)
			}

			schema, err := c.GetTableSchema(dbName, tableName)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error getting table schema:", err)
				os.Exit(1)
			}
			fmt.Printf("Starting import of %d rows into table '%s.%s'...\n", len(rows), dbName, tableName)
			importRows(c, dbName, tableName, privateKey, schema, rows)
		}

		fmt.Printf("\nJSON Import finished.\n")
		fmt.Println("Note: The final database state is being published to IPNS, which can take some time to propagate.")
	},
}

func importDatabaseFromCAR(newDbName string) {
	fmt.Printf("Importing database from CAR file '%s' into new database '%s'...\n", importInput, newDbName)

	// 1. Create a multipart request body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	file, err := os.Open(importInput)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error opening CAR file:", err)
		os.Exit(1)
	}
	defer file.Close()

	part, err := writer.CreateFormFile("path", filepath.Base(importInput))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating form file:", err)
		os.Exit(1)
	}
	_, err = io.Copy(part, file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error copying file to form:", err)
		os.Exit(1)
	}
	writer.Close()

	// 2. Manually send the request to the IPFS daemon
	ipfsApi := viper.GetString("ipfs-api")
	url := fmt.Sprintf("http://%s/api/v0/dag/import", ipfsApi)

	fmt.Println("Uploading CAR file to IPFS node...")
	httpReq, err := http.NewRequest("POST", url, body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating HTTP request:", err)
		os.Exit(1)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	httpClient := &http.Client{}
	httpRes, err := httpClient.Do(httpReq)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error sending DAG import request to IPFS:", err)
		os.Exit(1)
	}
	defer httpRes.Body.Close()

	if httpRes.StatusCode != http.StatusOK {
		// Try to read the error message from the body
		var errRes struct {
			Message string
		}
		if json.NewDecoder(httpRes.Body).Decode(&errRes) == nil {
			fmt.Fprintf(os.Stderr, "Error from IPFS API during DAG import: %s\n", errRes.Message)
		} else {
			fmt.Fprintf(os.Stderr, "Error from IPFS API during DAG import: received status code %d\n", httpRes.StatusCode)
		}
		os.Exit(1)
	}

	// 3. Decode the response from `dag/import` to find the root CID.
	type DagImportRes struct {
		Root struct {
			Cid struct {
				Path string `json:"/"`
			}
		}
	}
	dec := json.NewDecoder(httpRes.Body)
	var r DagImportRes
	if err := dec.Decode(&r); err != nil {
		fmt.Fprintln(os.Stderr, "Error decoding IPFS response to get root CID:", err)
		os.Exit(1)
	}
	rootCID := r.Root.Cid.Path
	if rootCID == "" {
		fmt.Fprintln(os.Stderr, "Could not determine root CID from CAR file import.")
		os.Exit(1)
	}

	fmt.Printf("CAR file imported to IPFS. Root CID: %s\n", rootCID)

	// 4. Call the ForkDatabase RPC method
	fmt.Println("Forking database...")
	c := client.NewClient(viper.GetString("rpc-server"))
	forkResult, err := c.ForkDatabase(newDbName, rootCID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error forking database:", err)
		os.Exit(1)
	}

	// 5. Print success message with new details
	fmt.Printf("\nSuccessfully forked database. Your new database '%s' is ready.\n", newDbName)
	fmt.Printf("Program ID (IPNS): %s\n", forkResult.ProgramID)
	fmt.Printf("\nIMPORTANT: Save this new private key. It is required for all future write operations on the forked database.\n")
	fmt.Printf("Private Key: %s\n", forkResult.PrivateKey)
}

func importRows(c *client.RpcClient, dbName, tableName, privateKey string, schema *proto.Schema, rows []map[string]interface{}) {
	for i, rowMap := range rows {
		valueStrings := make([]string, len(schema.Columns))
		for j, col := range schema.Columns {
			val, ok := rowMap[col.Name]
			if !ok {
				fmt.Fprintf(os.Stderr, "Error in row %d: missing value for column '%s'\n", i+1, col.Name)
				os.Exit(1)
			}

			switch v := val.(type) {
			case string:
				escapedV := strings.ReplaceAll(v, "'", "''")
				valueStrings[j] = fmt.Sprintf("'%s'", escapedV)
			case float64:
				valueStrings[j] = strconv.FormatFloat(v, 'f', -1, 64)
			case bool:
				valueStrings[j] = strconv.FormatBool(v)
			case nil:
				fmt.Fprintf(os.Stderr, "Error in row %d: column '%s' is null, which is not yet supported.\n", i+1, col.Name)
				os.Exit(1)
			default:
				fmt.Fprintf(os.Stderr, "Error in row %d: unsupported data type '%T' for column '%s'.\n", i+1, v, col.Name)
				os.Exit(1)
			}
		}

		queryString := fmt.Sprintf("INSERT INTO %s VALUES (%s);", tableName, strings.Join(valueStrings, ", "))

		_, err := c.ExecuteQuery(dbName, queryString, "", privateKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error importing row %d: %v\nQuery: %s\n", i+1, err, queryString)
			os.Exit(1)
		}
		fmt.Printf("Imported row %d/%d\n", i+1, len(rows))
	}
}

func init() {
	rootCmd.AddCommand(dataCmd)

	dataCmd.AddCommand(exportCmd)
	exportCmd.Flags().StringVar(&exportFormat, "format", "dag-json", "Output format (dag-json or car)")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file path (required)")
	exportCmd.MarkFlagRequired("output")

	dataCmd.AddCommand(importCmd)
	importCmd.Flags().StringVar(&importFormat, "format", "dag-json", "Input format (only 'dag-json' is supported)")
	importCmd.Flags().StringVarP(&importInput, "input", "i", "", "Input file path (required)")
	importCmd.MarkFlagRequired("input")
}

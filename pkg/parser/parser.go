package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// Column represents a column in a table
type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Schema represents the schema of a table
type Schema struct {
	Columns []Column `json:"columns"`
}

// WhereClause represents a WHERE clause in a SQL statement
type WhereClause struct {
	Column string
	Value  string
}

// UpdateClause represents a SET clause in an UPDATE statement
type UpdateClause struct {
	Column string
	Value  string
}

// ParseCreateTable parses a CREATE TABLE statement and returns a Schema, table name, and an error.
func ParseCreateTable(sql string) (*Schema, string, error) {
	// Very basic regex to capture table name and columns. The 's' flag allows . to match newlines.
	re := regexp.MustCompile(`(?is)CREATE TABLE (\w+)\s*\((.*)\);`)
	matches := re.FindStringSubmatch(sql)

	if len(matches) != 3 {
		return nil, "", fmt.Errorf("invalid CREATE TABLE statement")
	}

	tableName := matches[1]
	columnsStr := matches[2]

	var columns []Column
	colPairs := strings.Split(columnsStr, ",")
	for _, pair := range colPairs {
		pair = strings.TrimSpace(pair)
		parts := strings.Fields(pair)
		if len(parts) >= 2 {
			columns = append(columns, Column{Name: parts[0], Type: parts[1]})
		}
	}

	if len(columns) == 0 {
		return nil, "", fmt.Errorf("no columns found in CREATE TABLE statement")
	}

	return &Schema{Columns: columns}, tableName, nil
}

func ParseMultipleCreateTables(sql string) (map[string]*Schema, error) {
	re := regexp.MustCompile(`(?is)CREATE TABLE (\w+)\s*\((.*?)\);`)
	matches := re.FindAllStringSubmatch(sql, -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf("no valid CREATE TABLE statements found")
	}

	tables := make(map[string]*Schema)

	for _, match := range matches {
		if len(match) != 3 {
			continue
		}

		tableName := match[1]
		columnsStr := match[2]

		var columns []Column
		colPairs := strings.Split(columnsStr, ",")
		for _, pair := range colPairs {
			pair = strings.TrimSpace(pair)
			parts := strings.Fields(pair)
			if len(parts) >= 2 {
				columns = append(columns, Column{Name: parts[0], Type: parts[1]})
			}
		}

		if len(columns) > 0 {
			tables[tableName] = &Schema{Columns: columns}
		}
	}

	if len(tables) == 0 {
		return nil, fmt.Errorf("no valid tables parsed from statements")
	}

	return tables, nil
}

// ParseDrop parses a DROP TABLE statement.
func ParseDrop(sql string) (string, error) {
	// Regex to capture table name.
	re := regexp.MustCompile(`(?i)DROP\s+TABLE\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)

	if len(matches) != 2 {
		return "", fmt.Errorf("invalid DROP TABLE statement")
	}

	tableName := matches[1]

	return tableName, nil
}

// ParseCreateDatabase parses a CREATE DATABASE statement.
func ParseCreateDatabase(sql string) (string, error) {
	// Regex to capture database name.
	re := regexp.MustCompile(`(?i)CREATE\s+DATABASE\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)

	if len(matches) != 2 {
		return "", fmt.Errorf("invalid CREATE DATABASE statement")
	}

	dbName := matches[1]

	return dbName, nil
}

// ParseSelect parses a SELECT statement.
func ParseSelect(sql string) (string, *WhereClause, error) {
	re := regexp.MustCompile(`(?i)SELECT\s+\*\s+FROM\s+(\w+)(?:\s+WHERE\s+(\w+)\s*=\s*'(.*)')?`)
	matches := re.FindStringSubmatch(sql)

	if len(matches) < 2 {
		return "", nil, fmt.Errorf("invalid SELECT statement")
	}

	tableName := matches[1]

	var where *WhereClause
	if len(matches) == 4 && matches[2] != "" {
		where = &WhereClause{Column: matches[2], Value: matches[3]}
	}

	return tableName, where, nil
}

// ParseInsert parses an INSERT statement.
func ParseInsert(sql string) (string, []string, error) {
    re := regexp.MustCompile(`(?i)INSERT\s+INTO\s+(\w+)\s+VALUES\s*\((.*)\);`)
    matches := re.FindStringSubmatch(sql)

    if len(matches) != 3 {
        return "", nil, fmt.Errorf("invalid INSERT statement")
    }

    tableName := matches[1]
    valuesStr := matches[2]

    // Support both quoted and unquoted tokens by splitting on commas
    // and trimming quotes/spaces, matching previous behavior in ipfsdb parser.
    parts := strings.Split(valuesStr, ",")
    var values []string
    for _, p := range parts {
        v := strings.TrimSpace(p)
        if strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'") && len(v) >= 2 {
            v = strings.Trim(v, "'")
        }
        values = append(values, v)
    }

    if len(values) == 0 {
        return "", nil, fmt.Errorf("no values found in INSERT statement")
    }

    return tableName, values, nil
}

// ParseUpdate parses an UPDATE statement.
func ParseUpdate(sql string) (string, *UpdateClause, *WhereClause, error) {
	re := regexp.MustCompile(`(?i)UPDATE\s+(\w+)\s+SET\s+(\w+)\s*=\s*'(.*)'\s+WHERE\s+(\w+)\s*=\s*'(.*)'`)
	matches := re.FindStringSubmatch(sql)

	if len(matches) != 6 {
		return "", nil, nil, fmt.Errorf("invalid UPDATE statement")
	}

	tableName := matches[1]
	update := &UpdateClause{Column: matches[2], Value: matches[3]}
	where := &WhereClause{Column: matches[4], Value: matches[5]}

	return tableName, update, where, nil
}

// ParseDelete parses a DELETE statement.
func ParseDelete(sql string) (string, *WhereClause, error) {
	re := regexp.MustCompile(`(?i)DELETE\s+FROM\s+(\w+)\s+WHERE\s+(\w+)\s*=\s*'(.*)'`)
	matches := re.FindStringSubmatch(sql)

	if len(matches) != 4 {
		return "", nil, fmt.Errorf("invalid DELETE statement")
	}

	tableName := matches[1]
	where := &WhereClause{Column: matches[2], Value: matches[3]}

	return tableName, where, nil
}

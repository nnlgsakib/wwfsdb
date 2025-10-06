
package ipfsdb

import (
	"fmt"
	"regexp"
	"strings"
)

// WhereClause represents a simple WHERE condition
type WhereClause struct {
	Column string
	Value  string
}

// UpdateClause represents a simple SET condition

type UpdateClause struct {
	Column string
	Value  string
}

// ParseSelect parses a SELECT statement, including an optional WHERE clause.
func ParseSelect(sql string) (string, *WhereClause, error) {
	// Regex to capture table name and an optional WHERE clause with a simple equality check.
	re := regexp.MustCompile(`(?i)SELECT\s+\*\s+FROM\s+(\w+)(?:\s+WHERE\s+(\w+)\s*=\s*'(.*)')?`)
	matches := re.FindStringSubmatch(sql)

	if len(matches) == 0 {
		return "", nil, fmt.Errorf("invalid or unsupported SELECT statement")
	}

	tableName := matches[1]

	// Check if a WHERE clause was matched
	if len(matches) > 2 && matches[2] != "" {
		where := &WhereClause{
			Column: matches[2],
			Value:  matches[3],
		}
		return tableName, where, nil
	}

	return tableName, nil, nil
}

// ParseInsert parses an INSERT INTO statement.
func ParseInsert(sql string) (string, []string, error) {
	re := regexp.MustCompile(`(?i)INSERT INTO (\w+)\s+VALUES\s*\((.*)\);`)
	matches := re.FindStringSubmatch(sql)

	if len(matches) != 3 {
		return "", nil, fmt.Errorf("invalid INSERT INTO statement")
	}

	tableName := matches[1]
	valuesStr := matches[2]

	var values []string
	rawValues := strings.Split(valuesStr, ",")
	for _, val := range rawValues {
		val = strings.TrimSpace(val)
		// Trim quotes for string literals
		if strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'") {
			val = strings.Trim(val, "'")
		}
		values = append(values, val)
	}

	if len(values) == 0 {
		return "", nil, fmt.Errorf("no values found in INSERT INTO statement")
	}

	return tableName, values, nil
}

// ParseUpdate parses an UPDATE statement.
func ParseUpdate(sql string) (string, *UpdateClause, *WhereClause, error) {
	// Regex to capture table name, SET clause, and a mandatory WHERE clause.
	re := regexp.MustCompile(`(?i)UPDATE\s+(\w+)\s+SET\s+(\w+)\s*=\s*'(.*)'\s+WHERE\s+(\w+)\s*=\s*'(.*)'`);
	matches := re.FindStringSubmatch(sql)

	if len(matches) != 6 {
		return "", nil, nil, fmt.Errorf("invalid or unsupported UPDATE statement; a WHERE clause is mandatory")
	}

	tableName := matches[1]
	update := &UpdateClause{
		Column: matches[2],
		Value:  matches[3],
	}
	where := &WhereClause{
		Column: matches[4],
		Value:  matches[5],
	}

	return tableName, update, where, nil
}

// ParseDelete parses a DELETE statement.
func ParseDelete(sql string) (string, *WhereClause, error) {
	// Regex to capture table name and a mandatory WHERE clause with a simple equality check.
	re := regexp.MustCompile(`(?i)DELETE\s+FROM\s+(\w+)\s+WHERE\s+(\w+)\s*=\s*'(.*)'`);
	matches := re.FindStringSubmatch(sql)

	if len(matches) != 4 {
		return "", nil, fmt.Errorf("invalid or unsupported DELETE statement; a WHERE clause is mandatory")
	}

	tableName := matches[1]
	where := &WhereClause{
		Column: matches[2],
		Value:  matches[3],
	}

	return tableName, where, nil
}

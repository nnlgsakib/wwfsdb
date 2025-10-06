package ssql

import (
	"fmt"

	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
)

// Type aliases re-exported at the facade level.
type (
	Column       = ast.Column
	Schema       = ast.Schema
	WhereClause  = ast.WhereClause
	UpdateClause = ast.UpdateClause
)

// ParseCreateDatabase parses a CREATE DATABASE statement and returns the database name.
func ParseCreateDatabase(sql string) (string, error) {
	stmt, err := parseCreateDatabase(sql)
	if err != nil { return "", err }
	return stmt.Name, nil
}

// ParseCreateTable parses a CREATE TABLE statement and returns a Schema and table name.
func ParseCreateTable(sql string) (*Schema, string, error) {
	stmt, err := parseCreateTable(sql)
	if err != nil { return nil, "", err }
	return &stmt.Schema, stmt.Name, nil
}

// ParseMultipleCreateTables parses multiple CREATE TABLE statements into map[name]*Schema.
func ParseMultipleCreateTables(sql string) (map[string]*Schema, error) {
	stmts, err := parseMultipleCreateTables(sql)
	if err != nil { return nil, err }
	out := make(map[string]*Schema)
	for _, s := range stmts { s := s; out[s.Name] = &s.Schema }
	if len(out) == 0 { return nil, fmt.Errorf("no valid tables parsed from statements") }
	return out, nil
}

// ParseDrop parses a DROP TABLE statement and returns the table name.
func ParseDrop(sql string) (string, error) {
	stmt, err := parseDrop(sql)
	if err != nil { return "", err }
	return stmt.Name, nil
}

// ParseSelect parses a SELECT statement and returns table and optional where.
func ParseSelect(sql string) (string, *WhereClause, error) {
	stmt, err := parseSelect(sql)
	if err != nil { return "", nil, err }
	return stmt.Table, stmt.Where, nil
}

// ParseInsert parses an INSERT statement and returns table and values.
func ParseInsert(sql string) (string, []string, error) {
	stmt, err := parseInsert(sql)
	if err != nil { return "", nil, err }
	return stmt.Table, stmt.Values, nil
}

// ParseUpdate parses an UPDATE statement and returns table, set clause, and where.
func ParseUpdate(sql string) (string, *UpdateClause, *WhereClause, error) {
	stmt, err := parseUpdate(sql)
	if err != nil { return "", nil, nil, err }
	return stmt.Table, &stmt.Set, &stmt.Where, nil
}

// ParseDelete parses a DELETE statement and returns table and where.
func ParseDelete(sql string) (string, *WhereClause, error) {
	stmt, err := parseDelete(sql)
	if err != nil { return "", nil, err }
	return stmt.Table, &stmt.Where, nil
}

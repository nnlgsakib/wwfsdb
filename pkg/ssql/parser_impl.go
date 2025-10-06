package ssql

import (
    "fmt"
    "regexp"
    "strings"

    "github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
)

// Initial regex-based parser implementation that returns AST statements.
// To be replaced by a robust SQL parser integration.

func parseCreateDatabase(sql string) (*ast.CreateDatabaseStmt, error) {
    re := regexp.MustCompile(`(?i)CREATE\s+DATABASE\s+(\w+)`)
    m := re.FindStringSubmatch(sql)
    if len(m) != 2 { return nil, fmt.Errorf("invalid CREATE DATABASE statement") }
    return &ast.CreateDatabaseStmt{Name: m[1]}, nil
}

func parseCreateTable(sql string) (*ast.CreateTableStmt, error) {
    re := regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(\w+)\s*\((.*)\);`)
    m := re.FindStringSubmatch(sql)
    if len(m) != 3 { return nil, fmt.Errorf("invalid CREATE TABLE statement") }
    table := m[1]
    cols := parseColumns(m[2])
    if len(cols) == 0 { return nil, fmt.Errorf("no columns found in CREATE TABLE statement") }
    return &ast.CreateTableStmt{Name: table, Schema: ast.Schema{Columns: cols}}, nil
}

func parseMultipleCreateTables(sql string) ([]*ast.CreateTableStmt, error) {
    re := regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(\w+)\s*\((.*?)\);`)
    matches := re.FindAllStringSubmatch(sql, -1)
    if len(matches) == 0 { return nil, fmt.Errorf("no valid CREATE TABLE statements found") }
    var out []*ast.CreateTableStmt
    for _, m := range matches {
        if len(m) != 3 { continue }
        table := m[1]
        cols := parseColumns(m[2])
        if len(cols) == 0 { continue }
        out = append(out, &ast.CreateTableStmt{Name: table, Schema: ast.Schema{Columns: cols}})
    }
    if len(out) == 0 { return nil, fmt.Errorf("no valid tables parsed from statements") }
    return out, nil
}

func parseDrop(sql string) (*ast.DropTableStmt, error) {
    re := regexp.MustCompile(`(?i)DROP\s+TABLE\s+(\w+)`)
    m := re.FindStringSubmatch(sql)
    if len(m) != 2 { return nil, fmt.Errorf("invalid DROP TABLE statement") }
    return &ast.DropTableStmt{Name: m[1]}, nil
}

func parseSelect(sql string) (*ast.SelectStmt, error) {
    re := regexp.MustCompile(`(?i)SELECT\s+\*\s+FROM\s+(\w+)(?:\s+WHERE\s+(\w+)\s*=\s*'(.*)')?`)
    m := re.FindStringSubmatch(sql)
    if len(m) < 2 { return nil, fmt.Errorf("invalid SELECT statement") }
    stmt := &ast.SelectStmt{Table: m[1]}
    if len(m) == 4 && m[2] != "" {
        stmt.Where = &ast.WhereClause{Column: m[2], Value: m[3]}
    }
    return stmt, nil
}

func parseInsert(sql string) (*ast.InsertStmt, error) {
    re := regexp.MustCompile(`(?i)INSERT\s+INTO\s+(\w+)\s+VALUES\s*\((.*)\);`)
    m := re.FindStringSubmatch(sql)
    if len(m) != 3 { return nil, fmt.Errorf("invalid INSERT statement") }
    parts := strings.Split(m[2], ",")
    var values []string
    for _, p := range parts {
        v := strings.TrimSpace(p)
        if strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'") && len(v) >= 2 { v = strings.Trim(v, "'") }
        values = append(values, v)
    }
    if len(values) == 0 { return nil, fmt.Errorf("no values found in INSERT statement") }
    return &ast.InsertStmt{Table: m[1], Values: values}, nil
}

func parseUpdate(sql string) (*ast.UpdateStmt, error) {
    re := regexp.MustCompile(`(?i)UPDATE\s+(\w+)\s+SET\s+(\w+)\s*=\s*'(.*)'\s+WHERE\s+(\w+)\s*=\s*'(.*)'`)
    m := re.FindStringSubmatch(sql)
    if len(m) != 6 { return nil, fmt.Errorf("invalid UPDATE statement") }
    return &ast.UpdateStmt{Table: m[1], Set: ast.UpdateClause{Column: m[2], Value: m[3]}, Where: ast.WhereClause{Column: m[4], Value: m[5]}}, nil
}

func parseDelete(sql string) (*ast.DeleteStmt, error) {
    re := regexp.MustCompile(`(?i)DELETE\s+FROM\s+(\w+)\s+WHERE\s+(\w+)\s*=\s*'(.*)'`)
    m := re.FindStringSubmatch(sql)
    if len(m) != 4 { return nil, fmt.Errorf("invalid DELETE statement") }
    return &ast.DeleteStmt{Table: m[1], Where: ast.WhereClause{Column: m[2], Value: m[3]}}, nil
}

func parseColumns(columnsStr string) []ast.Column {
    var columns []ast.Column
    colPairs := strings.Split(columnsStr, ",")
    for _, pair := range colPairs {
        pair = strings.TrimSpace(pair)
        parts := strings.Fields(pair)
        if len(parts) >= 2 {
            columns = append(columns, ast.Column{Name: parts[0], Type: parts[1]})
        }
    }
    return columns
}


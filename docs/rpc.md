# WWSFDB JSON-RPC API Reference

This document details the JSON-RPC 2.0 API for interacting with the `wwfsdb` server. This is the low-level interface used by the CLI.

**Endpoint:** `http://<server_address>:<port>/rpc`

---

## Method: `wwfs.ExecuteQuery`

This is the primary, universal method for executing all SQL statements, including data manipulation (DML), data definition (DDL), and transaction control.

### Request

The `params` array must contain a single JSON object with the following fields:

- `db_name` (string): The name of the database to execute the query against. This can be an empty string for `CREATE DATABASE` queries.
- `query` (string): The SQL query or command to execute.
- `session_id` (string, optional): The session ID for an ongoing transaction. Required for all queries within a `BEGIN`/`COMMIT` block.
- `signature` (string, optional): A hex-encoded ECDSA signature. **This is required for all write operations** on databases that have an owner.

#### Authentication Signature

For write queries (`INSERT`, `UPDATE`, `DELETE`, `CREATE TABLE`, etc.), a signature must be provided if the target database has an owner. The signature is created by:

1.  Constructing a message string in the format: `[db_name]:[query]`.
2.  Signing this message string with the database owner's private key using ECDSA with a P-256 curve and SHA-256 hash.
3.  Hex-encoding the resulting raw `r` and `s` values of the signature, concatenated together.

### Response

The `result` field of the JSON-RPC response will be a JSON object containing:

- `result` (string): The outcome of the query. The content varies:
    - For `SELECT`: A JSON string representing an array of row objects.
    - For `INSERT`, `UPDATE`, `DELETE`: A success message, often including the number of affected rows.
    - For `CREATE DATABASE`: A JSON string containing the new database's `program_id` and the all-important `private_key`.
    - For `BEGIN`: A confirmation message.
- `session_id` (string, optional): If the query was `BEGIN`, this field will contain the new session ID for the transaction.

### Examples

**1. Create a Database (No Auth Required)**

```json
// Request
{
    "jsonrpc": "2.0",
    "method": "wwfs.ExecuteQuery",
    "params": [{
        "db_name": "",
        "query": "CREATE DATABASE my_db"
    }],
    "id": 1
}

// Response
{
    "result": {
        "result": "{\"private_key\":\"a36d...\",\"program_id\":\"k51qzi...\"}",
        "session_id": ""
    },
    "error": null,
    "id": 1
}
```

**2. Create a Table (Auth Required)**

```json
// Request
{
    "jsonrpc": "2.0",
    "method": "wwfs.ExecuteQuery",
    "params": [{
        "db_name": "my_db",
        "query": "CREATE TABLE users (id INT);",
        "signature": "3045022100..."
    }],
    "id": 2
}

// Response
{
    "result": {
        "result": "Table 'users' created successfully in database 'my_db'.",
        "session_id": ""
    },
    "error": null,
    "id": 2
}
```

**3. Select Data (No Auth Required)**

```json
// Request
{
    "jsonrpc": "2.0",
    "method": "wwfs.ExecuteQuery",
    "params": [{
        "db_name": "my_db",
        "query": "SELECT * FROM users;"
    }],
    "id": 3
}

// Response
{
    "result": {
        "result": "[{{\"id\":1,\"name\":\"Alice\"}}]",
        "session_id": ""
    },
    "error": null,
    "id": 3
}
```

---

## Method: `wwfs.GetTableSchema`

Retrieves the schema (column names and types) for a given table.

### Request

The `params` array must contain a single JSON object with the following fields:

- `db_name` (string): The name of the database.
- `table_name` (string): The name of the table.

### Response

The `result` field will be a JSON object containing a `schema` object.

### Example

```json
// Request
{
    "jsonrpc": "2.0",
    "method": "wwfs.GetTableSchema",
    "params": [{
        "db_name": "my_db",
        "table_name": "users"
    }],
    "id": 4
}

// Response
{
    "result": {
        "schema": {
            "columns": [
                { "name": "id", "type": "INT" },
                { "name": "name", "type": "VARCHAR(50)" }
            ]
        }
    },
    "error": null,
    "id": 4
}
```

# WWSFDB CLI Reference

This document provides a comprehensive reference for the `wwfsdb` command-line interface.

## Global Flags

These flags can be used with any command.

- `--config [path]`
  - Specifies a path to a custom configuration file (e.g., `config.yaml`).

- `-r, --rpc-server [url]`
  - The URL of the `wwfsdb` JSON-RPC server to connect to.
  - **Default:** `http://localhost:8080/rpc`

- `--private-key [key]`
  - The hexadecimal private key associated with a database owner. **This is required for all write operations** (create table, insert, update, delete, etc.).

---

## Commands

### `serve`

Starts the JSON-RPC server to listen for database queries.

**Usage:**
```sh
wwfsdb serve [flags]
```

**Flags:**
- `-p, --port [port]`
  - The port for the server to listen on.
  - **Default:** `8080`
- `--api [host:port]`
  - The API address of the backing IPFS node.
  - **Default:** `localhost:5001`

**Example:**
```sh
# Start the server on port 8888
wwfsdb serve -p 8888
```

### `createdatabase`

Creates a new, empty decentralized database.

**Usage:**
```sh
wwfsdb createdatabase "CREATE DATABASE [database_name]"
```

**Important:** This is the only command that generates a private key. You must save this key, as it grants ownership and write access to the new database. It cannot be recovered if lost.

**Example:**
```sh
wwfsdb createdatabase "CREATE DATABASE my_app_db"

# Output:
# Database created successfully.
# Program ID (IPNS): k51qzi5uqu5dhmf3... 
#
# IMPORTANT: Save this private key. It is required for all future write operations.
# Private Key: a36d78ae4221369541cc271c0112121a37aad91f49d35403279e38bd7920653d
```

### `migrate`

Creates one or more tables in an existing database from a `.ssql` schema file.

**Usage:**
```sh
wwfsdb --private-key [key] migrate [database_name] [path_to_ssql_file]
```

**Example:**
```sh
# Create tables defined in schema.ssql in the 'my_app_db' database
wwfsdb --private-key <key> migrate my_app_db ./schema.ssql
```

### `execute`

Executes an `INSERT` statement to add a new row to a table.

**Usage:**
```sh
wwfsdb --private-key [key] execute [database_name] "INSERT INTO [table_name] VALUES (...)"
```

**Example:**
```sh
wwfsdb --private-key <key> execute my_app_db "INSERT INTO users VALUES (1, 'Alice', 'alice@email.com')"
```

### `query`

Executes a `SELECT` statement to retrieve data from a database. Does not require a private key.

**Usage:**
```sh
wwfsdb query [database_name] "SELECT ... FROM ..."
```

**Example:**
```sh
wwfsdb query my_app_db "SELECT name, email FROM users WHERE id = 1"
```

### `update`

Executes an `UPDATE` statement to modify existing rows in a table.

**Usage:**
```sh
wwfsdb --private-key [key] update [database_name] "UPDATE [table_name] SET ... WHERE ..."
```

**Example:**
```sh
wwfsdb --private-key <key> update my_app_db "UPDATE users SET email = 'new_email@email.com' WHERE id = 1"
```

### `delete`

Executes a `DELETE` statement to remove rows from a table.

**Usage:**
```sh
wwfsdb --private-key [key] delete [database_name] "DELETE FROM [table_name] WHERE ..."
```

**Example:**
```sh
wwfsdb --private-key <key> delete my_app_db "DELETE FROM users WHERE id = 1"
```

### `drop`

Executes a `DROP TABLE` statement to remove a table from a database.

**Usage:**
```sh
wwfsdb --private-key [key] drop [database_name] "DROP TABLE [table_name]"
```

**Example:**
```sh
wwfsdb --private-key <key> drop my_app_db "DROP TABLE users"
```

### `shell`

Starts an interactive shell session for a specific database. See `shell.md` for detailed instructions.

**Usage:**
```sh
wwfsdb shell [database_name]

# For write access
wwfsdb --private-key [key] shell [database_name]
```

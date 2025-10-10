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

### `data`

A group of commands for bulk importing and exporting data.

#### `data export`

Exports a single table or an entire database to a file.

**Usage:**
```sh
# Export an entire database
wwfsdb data export [database_name] [flags]

# Export a single table
wwfsdb data export [database_name] [table_name] [flags]
```

**Flags:**
- `-o, --output [path]` (required)
  - The path to the output file.
- `--format [format]`
  - The output format. Can be `dag-json` or `car`.
  - **Default:** `dag-json`

**Examples:**
```sh
# Export the 'users' table to a JSON file
wwfsdb data export my_app_db users -o users.json

# Export the entire 'my_app_db' database to a JSON file
wwfsdb data export my_app_db -o my_app_db.json

# Export the 'users' table to a CAR file
wwfsdb data export my_app_db users -o users.car --format car

# Export the entire 'my_app_db' database to a CAR file
wwfsdb data export my_app_db -o my_app_db.car --format car
```

#### `data import`

Imports data from a file into a single table or creates a new database from a CAR file.

**Usage:**
```sh
# Import into a single table from JSON
wwfsdb --private-key [key] data import [db_name] [table_name] --input [file.json]

# Import multiple tables into a database from JSON
wwfsdb --private-key [key] data import [db_name] --input [file.json]

# Import from a CAR file as a new, forked database
wwfsdb data import [new_db_name] --input [file.car] --format car
```

**Flags:**
- `-i, --input [path]` (required)
  - The path to the input file.
- `--format [format]`
  - The input format. Can be `dag-json` or `car`.
  - **Default:** `dag-json`

**Description:**

The import command has different behaviors based on the format and arguments:

- **Single Table (JSON):** Imports an array of row objects from a JSON file into an existing table. Requires a private key.
- **Full Database (JSON):** Imports data into multiple tables. The JSON file must be an object where keys are table names and values are arrays of row objects. Requires a private key and the tables must already exist.
- **Full Database (CAR):** Imports a database from a `.car` file. This creates a **new, writable fork** of the database under `[new_db_name]`. This command does *not* require a private key, as it will generate a new one for the forked database and provide it to you.

**Examples:**
```sh
# Import rows into the 'users' table from a JSON file
wwfsdb --private-key <key> data import my_app_db users -i new_users.json

# Import data for multiple tables from a single JSON file
wwfsdb --private-key <key> data import my_app_db -i full_backup.json

# Fork a database from a CAR file, creating 'my_forked_db'
wwfsdb data import my_forked_db -i original.car --format car
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

### `alter`

Executes an `ALTER TABLE` statement to modify a table's schema.

**Usage:**
```sh
wwfsdb --private-key [key] alter [database_name] "ALTER TABLE [table_name] [action]"
```

**Supported Actions:**

- **`ADD COLUMN`**: Adds a new column to a table. Existing rows will have a `null` value for this column until it is updated.
  ```sh
  wwfsdb --private-key <key> alter my_app_db "ALTER TABLE users ADD COLUMN age INT"
  ```

- **`DROP COLUMN`**: Removes a column from a table's schema. The data for this column in existing rows is not deleted from storage but becomes inaccessible.
  ```sh
  wwfsdb --private-key <key> alter my_app_db "ALTER TABLE users DROP COLUMN age"
  ```

- **`RENAME COLUMN`**: Renames an existing column.
  **Important**: This only changes the schema. Existing data will not be accessible under the new column name. A data migration is required to update existing rows.
  ```sh
  wwfsdb --private-key <key> alter my_app_db "ALTER TABLE users RENAME COLUMN name TO full_name"
  ```

### `shell`

Starts an interactive shell session for a specific database. See `shell.md` for detailed instructions.

**Usage:**
```sh
wwfsdb shell [database_name]

# For write access
wwfsdb --private-key [key] shell [database_name]
```
# WWSFDB: World Wide File System Database

WWSFDB is a decentralized database system built on top of IPFS (InterPlanetary File System). It provides a robust SQL interface for creating, managing, and querying databases, with all data stored and versioned on the decentralized web.

## Vision

The vision behind WWSFDB is to provide a robust, decentralized, and censorship-resistant database for the new web. By leveraging IPFS for storage and IPNS for addressing, WWSFDB offers a globally accessible database that is not reliant on any single server or provider.

## Core Concepts

- **Decentralized Storage**: All database objects (tables, rows, indexes) are stored as immutable objects in IPFS, ensuring data integrity and availability.
- **Immutable History**: Every write operation (INSERT, UPDATE, DELETE, etc.) creates a new version of the database, preserving the full history of changes.
- **IPNS for Versioning**: Each database has a stable address (an IPNS Program ID) that always points to its latest version, providing a consistent endpoint for clients.
- **Cryptographic Ownership**: Write operations are authorized using an ECDSA key pair. A private key is generated upon database creation and is required to sign and authorize any subsequent changes, ensuring that only the owner can modify the database.

## Features

- **Decentralized & Immutable**: Built on IPFS, ensuring high availability, data permanence, and a verifiable history.
- **Rich SQL Dialect**: Utilizes the Vitess SQL parser to support a wide range of SQL features, including `JOIN`s, `GROUP BY`, aggregate functions (`COUNT`, `SUM`, `AVG`, etc.), `UNION`, and subqueries.
- **Cryptographic Ownership**: Write operations are secured by ECDSA signatures, preventing unauthorized modifications.
- **ACID Transactions**: The interactive shell and RPC service support `BEGIN`, `COMMIT`, and `ROLLBACK` for transactional integrity.
- **Data Portability**: Easily import and export data in `dag-json` format or create full, portable database snapshots as `.car` files.
- **Database Forking**: Create new, independent, writable databases from any `.car` file snapshot.
- **Automatic Indexing**: High-performance Prolly-Tree indexes are automatically created and maintained for columns with `UNIQUE` constraints.
- **JSON-RPC API**: A JSON-RPC server allows for remote interaction with the database.
- **Interactive Shell**: A convenient interactive shell is available for direct database management and querying.

## Architecture

WWSFDB's architecture is composed of several key components:

- **SQL Parser**: Uses the battle-tested **Vitess SQL Parser** to understand complex SQL statements.
- **Core Engine**: Translates the SQL abstract syntax tree (AST) into operations on Protobuf-defined data structures.
- **IPFS Integration**: The database, tables, schemas, and data pages are stored as versioned, content-addressed objects in IPFS.
- **Prolly-Tree Indexing**: A Probabilistic B-Tree (Prolly-Tree) implementation is used for efficient, immutable indexing, particularly for enforcing `UNIQUE` constraints.
- **LevelDB Cache**: A local LevelDB cache stores IPNS pointers and other metadata to ensure fast lookups.
- **Protocol Buffers**: Data is serialized using Protocol Buffers for efficient, strongly-typed storage on IPFS.
- **Configuration**: **Viper** manages configuration via files, environment variables, or command-line flags.
- **JSON-RPC Server**: A Gorilla Mux-based JSON-RPC server exposes the database functionality for remote clients.
- **CLI:** A command-line interface built with **Cobra** provides a user-friendly way to interact with WWSFDB.

## Installation

To install WWSFDB, you need to have Go installed on your system.

1.  **Clone the repository:**
    ```sh
    git clone https://github.com/nnlgsakib/wwfsdb.git
    cd wwfsdb
    ```

2.  **Build and install the project:**
    ```sh
    go install .
    ```
    This will create and install the `wwfsdb` executable in your Go bin path.

## Configuration

WWSFDB can be configured via a YAML file, environment variables, or command-line flags.

- **Config File**: Create a file named `.wwfsdb.yaml` in your home directory (`~`) or the current project directory.
- **Environment Variables**: Set environment variables with the `WWSFDB_` prefix (e.g., `WWSFDB_PRIVATE_KEY=...`).

**Example `.wwfsdb.yaml`:**
```yaml
rpc-server: "http://localhost:8080/rpc"
ipfs-api: "localhost:5001"
private-key: "your-database-private-key"
```

## Usage (CLI Quickstart)

WWSFDB requires a running IPFS daemon.

### 1. Start the Server

First, start the WWSFDB JSON-RPC server. This server listens for commands and interacts with the IPFS network.

```sh
wwfsdb serve
```
*Flags: `--port 8080`, `--api localhost:5001`*

### 2. Create a Database

Create a new database with the `createdatabase` command.

```sh
wwfsdb createdatabase "CREATE DATABASE my_db"
```

This will output a **Program ID** (the database's permanent IPNS address) and a **Private Key**.

```
Database created successfully.
Program ID (IPNS): k51qzi5uqu5dk...
IMPORTANT: Save this private key. It is required for all future write operations.
Private Key: b12f76ea3dbea0a93c3ef82827f9ef6a085954b70dc66315a189c3aba30ef897
```
**Save the private key! You will need it for all write operations.**

### 3. Configure the Private Key

For subsequent write commands, you must provide the private key. You can do this with the `--private-key` flag or by adding it to your `.wwfsdb.yaml` file.

```sh
wwfsdb --private-key <your-key> [command]
```

### 4. Create Tables

Define your schema in a `.ssql` file. WWSFDB supports various data types (`INT`, `FLOAT`, `BOOL`, `TEXT`, `VARCHAR`, `TIMESTAMP`, etc.) and constraints like `NOT NULL` and `UNIQUE`.

**`schema.ssql`:**
```sql
CREATE TABLE users (
    id INT,
    username VARCHAR(50),
    email VARCHAR(100),
    created_at TIMESTAMP
);
```

Use the `migrate` command to create the table.

```sh
wwfsdb migrate my_db schema.ssql --private-key <your-key>
```

### 5. Write Data

Use `execute`, `update`, and `delete` to modify data.

```sh
# Insert a row
wwfsdb execute my_db "INSERT INTO users (id, username, email, created_at) VALUES (1, 'john_doe', 'john.doe@example.com', '2025-10-15T10:00:00Z')" --private-key <your-key>

# Update a row
wwfsdb update my_db "UPDATE users SET email = 'new.email@example.com' WHERE id = 1" --private-key <your-key>

# Delete a row
wwfsdb delete my_db "DELETE FROM users WHERE id = 1" --private-key <your-key>
```

### 6. Read Data

Query your data with the `query` command. This is a read-only operation and does not require a private key.

```sh
wwfsdb query my_db "SELECT username, email FROM users WHERE id = 1"
```

### 7. Interactive Shell

For a more interactive experience, use the `shell` command. Provide a private key to enable write operations within the shell.

```sh
wwfsdb shell my_db --private-key <your-key>
```

The shell supports multi-line statements and transactions.

```
Connected to database 'my_db'. Type 'exit' or 'quit' to leave.
wwfsdb> BEGIN;
wwfsdb (my_db)*> INSERT INTO users VALUES (2, 'jane_doe', 'jane@example.com', '2025-10-15T11:00:00Z');
wwfsdb (my_db)*> COMMIT;
COMMIT successful. New database version: bafy...
wwfsdb> SELECT * FROM users;
...
```

### 8. Data Management (Import/Export)

Export a single table to a `dag-json` file:
```sh
wwfsdb data export my_db users --format dag-json -o users.json
```

Import data from a `dag-json` file into a table:
```sh
wwfsdb data import my_db users --format dag-json -i users.json --private-key <your-key>
```

Create a full, portable snapshot of the entire database as a `.car` file:
```sh
wwfsdb data export my_db --format car -o my_db.car
```

Fork a database from a `.car` file. This creates a **new** database with a **new** private key.
```sh
wwfsdb data import my_forked_db --format car -i my_db.car
```

## Development Roadmap

Future development is focused on enhancing SQL compatibility, performance, and developer experience.
- **Full `ALTER TABLE` Support**: Implement the logic for adding, dropping, and modifying columns.
- **Explicit `CREATE INDEX`**: Allow users to create indexes on non-unique columns.
- **Performance Optimization**: Improve query execution speed, especially for large datasets and complex joins.
- **Expanded Data Types**: Add support for more complex data types.
- **Improved Tooling**: Enhance the developer experience with better error messages and more robust client libraries.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
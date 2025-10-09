# WWSFDB: World Wide File System Database

WWSFDB is a decentralized database system built on top of IPFS (InterPlanetary File System). It provides a simple, SQL-like interface for creating, managing, and querying databases, with all data stored and versioned on the decentralized web.

## Vision

The vision behind WWSFDB is to provide a robust, decentralized, and censorship-resistant database for the new web. By leveraging IPFS for storage and IPNS for addressing, WWSFDB offers a globally accessible database that is not reliant on any single server or provider.

## Features

- **Decentralized:** All data is stored on the IPFS network, ensuring high availability and data permanence.
- **SQL Interface:** Use a familiar SQL-like syntax to interact with your databases.
- **Versioning:** IPNS is used to maintain a stable address for your database while allowing for seamless updates.
- **JSON-RPC API:** A JSON-RPC server allows for remote interaction with the database.
- **Interactive Shell:** An interactive shell is available for easy database management.

## Architecture

WWSFDB's architecture is composed of several key components:

- **SQL Parser:** A custom-built parser for our SQL dialect, featuring a lexer and an AST.
- **IPFS Integration:** The database, tables, and rows are stored as objects in IPFS.
- **LevelDB Cache:** A local LevelDB cache is used to store mappings from database names to IPNS keys, ensuring fast lookups and resolving concurrency issues.
- **Protocol Buffers:** Data is serialized using Protocol Buffers for efficient storage and retrieval on IPFS.
- **JSON-RPC Server:** A Gorilla Mux-based JSON-RPC server exposes the database functionality for remote clients.
- **CLI:** A command-line interface built with Cobra provides a user-friendly way to interact with WWSFDB.

## Installation

To install WWSFDB, you need to have Go installed on your system.

1.  **Clone the repository:**
    ```sh
    git clone https://github.com/nnlgsakib/wwfsdb.git
    cd wwfsdb
    ```

2.  **Build the project:**
    ```sh
    go build .
    ```
    This will create the `wwfsdb` executable in the project directory.

## Usage

WWSFDB requires a running IPFS daemon. All commands are executed through the `wwfsdb` CLI.

### Starting the Server

First, start the WWSFDB JSON-RPC server. This server listens for commands and interacts with the IPFS network.

```sh
wwfsdb serve --port 8080 --api localhost:5001
```

### Creating a Database

Create a new database with the `createdatabase` command.

```sh
wwfsdb createdatabase "CREATE DATABASE my_db"
```

### Creating a Table

To create a table, define your schema in a `.ssql` file.

**`schema.ssql`:**
```sql
CREATE TABLE users (
    id INT,
    name STRING,
    email STRING
);
```

Then, use the `migrate` command to create the table in your database.

```sh
wwfsdb migrate my_db schema.ssql
```

### Inserting Data

Insert data into your tables using the `execute` command.

```sh
wwfsdb execute my_db "INSERT INTO users (id, name, email) VALUES (1, 'John Doe', 'john.doe@example.com')"
```

### Querying Data

Query your data with the `query` command.

```sh
wwfsdb query my_db "SELECT * FROM users"
```

You can also select specific columns and use `WHERE` clauses.

```sh
wwfsdb query my_db "SELECT name, email FROM users WHERE id = 1"
```

### Updating Data

Update existing rows with the `update` command.

```sh
wwfsdb update my_db "UPDATE users SET email = 'new.email@example.com' WHERE id = 1"
```

### Deleting Data

Delete rows using the `delete` command.

```sh
wwfsdb delete my_db "DELETE FROM users WHERE id = 1"
```

### Dropping a Table

Drop a table with the `drop` command.

```sh
wwfsdb drop my_db "DROP TABLE users"
```

### Interactive Shell

For a more interactive experience, use the `shell` command.

```sh
wwfsdb shell my_db
```

This will open a REPL where you can execute SQL commands directly.

```
Connected to database 'my_db'. Type 'exit' or 'quit' to leave.
wwfsdb> SELECT * FROM users;
...
```

## Development Roadmap

The development roadmap is tracked in the `todo.txt` file. Key future improvements include:

- **Comprehensive Testing Suite:** Building a full suite of unit and integration tests.
- **Improved CLI Help Text:** Adding detailed descriptions and examples to all CLI commands.
- **In-Code Documentation:** Adding more comments to clarify complex parts of the codebase.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

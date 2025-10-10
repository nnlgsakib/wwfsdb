# WWSFDB Interactive Shell

The `wwfsdb shell` provides an interactive Read-Eval-Print Loop (REPL) for executing queries against a specific database. This is often more convenient than using individual commands for each query.

## Starting the Shell

To start a shell session, use the `shell` command followed by the name of the database you want to connect to.

### Read-Only Access

For read-only operations (`SELECT`), you can start the shell without any special flags:

```sh
wwfsdb shell [database_name]
```

### Write Access

To perform write operations (`INSERT`, `UPDATE`, `DELETE`, `CREATE TABLE`, `ALTER TABLE`, etc.), you **must** provide the database owner's private key using the global `--private-key` flag.

```sh
wwfsdb --private-key [your_private_key] shell [database_name]
```

## Executing Queries

Once in the shell, you will see the `wwfsdb>` prompt. You can type any valid SQL statement and press Enter.

- **Semicolon:** Every statement must be terminated with a semicolon (`;`).
- **Multi-line Queries:** The shell supports multi-line statements. It will wait for you to enter a semicolon before executing the complete query. The prompt will change to `     ->` for continuation lines.

**Example:**
```
wwfsdb> SELECT
     ->   id,
     ->   name
     -> FROM
     ->   users
     -> WHERE
     ->   id > 5;
id	| name
----------------
6	| "Charlie"

(1 rows)
```

## Transactions

The shell fully supports transactions, allowing you to group multiple write operations into a single atomic unit.

1. **`BEGIN`**: Starts a new transaction. The prompt will change to `wwfsdb (database_name)*>` to indicate that you are in a transaction block.
2. **`COMMIT`**: Saves all changes made during the transaction to the database.
3. **`ROLLBACK`**: Discards all changes made during the transaction.

**Example:**
```
wwfsdb --private-key <key> shell my_app_db
Connected to database 'my_app_db'. Type 'exit' or 'quit' to leave.

wwfsdb> BEGIN;
Transaction started

wwfsdb (my_app_db)*> INSERT INTO users VALUES (10, 'David');
INSERT successful

wwfsdb (my_app_db)*> COMMIT;
Commit successful. New database version: bafy... 

wwfsdb> 
```

## Exiting the Shell

To exit the interactive shell, type `exit` or `quit` and press Enter.

```
wwfsdb> exit
```

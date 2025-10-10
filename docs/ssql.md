# WWSQL Language Reference

This document provides a comprehensive reference for WWSQL, the custom SQL dialect used by `wwfsdb`.

---

## Supported Data Types

WWSQL supports the following data types for table columns:

- `INT` / `INTEGER`: For whole numbers (stored as 64-bit integers).
- `FLOAT` / `REAL` / `DOUBLE`: For floating-point numbers (stored as 64-bit floats).
- `VARCHAR(n)`: For variable-length strings with a specified maximum length.
- `TEXT`: For variable-length strings of any length.
- `BOOLEAN` / `BOOL`: For `true` or `false` values.

---

## Data Definition Language (DDL)

DDL commands are used to define and manage the database structure.

### `CREATE DATABASE`

Initializes a new, empty database.

**Syntax:**
```sql
CREATE DATABASE database_name;
```

**Example:**
```sql
CREATE DATABASE my_app_db;
```

### `CREATE TABLE`

Creates a new table within a database.

**Syntax:**
```sql
CREATE TABLE table_name (
  column_1_name column_1_type,
  column_2_name column_2_type,
  ...
);
```

**Example:**
```sql
CREATE TABLE users (
  id INT,
  name VARCHAR(50),
  email TEXT,
  is_verified BOOLEAN
);
```

### `ALTER TABLE`

Modifies the schema of an existing table.

**Syntax:**
```sql
ALTER TABLE table_name <action>;
```

**Actions:**

- **`ADD COLUMN`**: Adds a new column. Existing rows will have a `null` value for this column.
  ```sql
  ALTER TABLE users ADD COLUMN signup_date VARCHAR(50);
  ```

- **`DROP COLUMN`**: Removes a column from the schema. The underlying data in existing rows is not modified but becomes inaccessible.
  ```sql
  ALTER TABLE users DROP COLUMN signup_date;
  ```

- **`RENAME COLUMN`**: Renames a column in the schema.
  **Important**: This action does not migrate existing data. Data stored under the old column name will become inaccessible. A manual data migration (e.g., `UPDATE table SET new_col = old_col`) is required before renaming.
  ```sql
  ALTER TABLE users RENAME COLUMN name TO full_name;
  ```

### `DROP TABLE`

Removes a table from the database.

**Syntax:**
```sql
DROP TABLE table_name;
```

**Example:**
```sql
DROP TABLE users;
```

### `CREATE INDEX`

Creates an index on a table column to speed up queries.

**Syntax:**
```sql
CREATE INDEX ON table_name (column_name);
```

**Example:**
```sql
CREATE INDEX ON users (email);
```

---

## Data Manipulation Language (DML)

DML commands are used to manage data within tables.

### `INSERT INTO`

Adds a new row to a table.

**Syntax:**
```sql
INSERT INTO table_name VALUES (value_1, value_2, ...);
```

**Example:**
```sql
INSERT INTO users VALUES (1, 'Alice', 'alice@example.com', true);
```

### `UPDATE`

Modifies existing rows in a table.

**Syntax:**
```sql
UPDATE table_name SET column_name = new_value WHERE condition;
```

**Example:**
```sql
UPDATE users SET email = 'new.email@example.com' WHERE id = 1;
```

### `DELETE FROM`

Removes rows from a table.

**Syntax:**
```sql
DELETE FROM table_name WHERE condition;
```

**Example:**
```sql
DELETE FROM users WHERE id = 1;
```

---

## Data Query Language (DQL)

DQL commands are used to retrieve data.

### `SELECT`

Retrieves data from one or more tables.

**Syntax:**
```sql
SELECT column_1, table_2.column_2, ...
FROM table_1
[JOIN table_2 ON condition]
[WHERE condition];
```

**Clauses:**

- **`SELECT`**: Can be `*` (all columns), `table.*` (all columns from a specific table), or a comma-separated list of columns (`col1, col2, ...`).
- **`FROM`**: Specifies the primary table.
- **`JOIN`**: Supports `INNER JOIN` and `LEFT JOIN` to combine rows from two tables based on a related column.
- **`WHERE`**: Filters results based on a condition. Supported operators include: `=`, `!=`, `>`, `<`, `>=`, `<=`, `AND`, `OR`, `LIKE`, `IN`.

**Examples:**

```sql
-- Select specific columns with a filter
SELECT id, name FROM users WHERE is_verified = true;

-- Select all columns from a table
SELECT * FROM users;

-- Select with a JOIN
SELECT users.name, orders.order_id
FROM users
LEFT JOIN orders ON users.id = orders.user_id;
```

---

## Transaction Control Language (TCL)

TCL commands are used to manage transactions.

### `BEGIN`

Starts a new transaction block. All subsequent queries will be part of this transaction until a `COMMIT` or `ROLLBACK` is issued.

**Syntax:**
```sql
BEGIN;
```

### `COMMIT`

Saves all changes made during the current transaction, making them permanent.

**Syntax:**
```sql
COMMIT;
```

### `ROLLBACK`

Discards all changes made during the current transaction.

**Syntax:**
```sql
ROLLBACK;
```

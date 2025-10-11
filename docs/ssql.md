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

## Advanced Literal Formats

WWSQL supports several advanced literal formats for enhanced expressiveness:

### Scientific Notation Numbers

Numbers can be expressed in scientific notation using `e` or `E`:

```sql
-- Examples of scientific notation
INSERT INTO measurements VALUES (1, 1.23e-4);     -- 0.000123
INSERT INTO measurements VALUES (2, 2.5E+5);      -- 250000.0
INSERT INTO measurements VALUES (3, 6.022e23);    -- 602200000000000000000000.0
```

### Hexadecimal Literals

Integer literals can be expressed in hexadecimal format using `0x` or `0X` prefix:

```sql
-- Examples of hexadecimal literals
INSERT INTO colors VALUES (1, 0xFF0000);    -- Red (255, 0, 0 in RGB)
INSERT INTO colors VALUES (2, 0x00FF00);    -- Green (0, 255, 0 in RGB)
INSERT INTO colors VALUES (3, 0x0000FF);    -- Blue (0, 0, 255 in RGB)
INSERT INTO flags VALUES (4, 0xDEADBEEF);   -- Large hex value
```

### Binary Literals

Integer literals can be expressed in binary format using `0b` or `0B` prefix:

```sql
-- Examples of binary literals
INSERT INTO permissions VALUES (1, 0b11111111);  -- 255 in decimal
INSERT INTO permissions VALUES (2, 0b10101010);  -- 170 in decimal
INSERT INTO flags VALUES (3, 0b1);              -- 1 in decimal
```

### Date/Time Keywords

The following keywords are reserved for date/time operations:
- `DATE` - For date values
- `TIME` - For time values  
- `TIMESTAMP` - For timestamp values

These can be used as quoted identifiers when needed:

```sql
-- Using reserved keywords as column names
CREATE TABLE events ("DATE" VARCHAR(50), "TIME" VARCHAR(50), "TIMESTAMP" FLOAT);
```

### Unicode Identifiers

WWSQL supports Unicode characters in string literals:

```sql
-- Examples of Unicode in string literals
INSERT INTO users VALUES (1, 'José María', 'café_Москва');
INSERT INTO products VALUES (2, 'Über Product', 'Nação');
```

### Double-Quoted Identifiers

Identifiers can be enclosed in double quotes to allow special characters or reserved words:

```sql
-- Using double-quoted identifiers
CREATE TABLE "My Special Table" ("First Name" VARCHAR(50), "Last-Name" VARCHAR(50));
INSERT INTO "My Special Table" VALUES ('John', 'Doe');
```

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
SELECT column_1, AGG_FUNC(column_2), ...
FROM table_1
[JOIN table_2 ON condition]
[WHERE condition]
[GROUP BY column_1, ...]
[ORDER BY column_1 [ASC|DESC], ...]
[LIMIT number]
[OFFSET number];
```

**Clauses:**

- **`SELECT`**: Can be `*` (all columns), `table.*` (all columns from a specific table), or a comma-separated list of columns (`col1, col2, ...`). Also supports aggregate functions:
  - `COUNT(* | column)`: Counts rows or non-null values.
  - `SUM(column)`: Calculates the sum of a numeric column.
  - `AVG(column)`: Calculates the average of a numeric column.
  - `MIN(column)`: Finds the minimum value in a column.
  - `MAX(column)`: Finds the maximum value in a column.
- **`FROM`**: Specifies the primary table.
- **`JOIN`**: Supports `INNER JOIN` and `LEFT JOIN` to combine rows from two tables based on a related column.
- **`WHERE`**: Filters results based on a condition. Supported operators include: `=`, `!=`, `>`, `<`, `>=`, `<=`, `AND`, `OR`, `LIKE`, `IN`.
- **`GROUP BY`**: Groups rows that have the same values in specified columns into summary rows. It is almost always used with aggregate functions.
- **`ORDER BY`**: Sorts the result set based on one or more columns, in ascending (`ASC`, default) or descending (`DESC`) order.
- **`LIMIT`**: Constrains the number of rows returned by the query.
- **`OFFSET`**: Skips a specified number of rows before beginning to return rows from the query.

**Examples:**

```sql
-- Select specific columns with a filter
SELECT id, name FROM users WHERE is_verified = true;

-- Select all columns from a table, sorted by name
SELECT * FROM users ORDER BY name DESC;

-- Select with a JOIN
SELECT users.name, orders.order_id
FROM users
LEFT JOIN orders ON users.id = orders.user_id;

-- Paginate through results
SELECT id, name FROM users
ORDER BY id
LIMIT 10 OFFSET 20; -- Returns rows 21-30

-- Count users by country and show the top 5
SELECT country, COUNT(*) AS user_count
FROM users
GROUP BY country
ORDER BY user_count DESC
LIMIT 5;

-- Calculate the average, min, and max order amount per user
SELECT user_id, AVG(amount), MIN(amount), MAX(amount)
FROM orders
GROUP BY user_id;
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

---

## Formatting and Comments

WWSQL supports various formatting options and comments for improved code readability:

### Whitespace and Indentation

WWSQL handles various whitespace styles including spaces, tabs, and mixed indentation:

```sql
-- Spaces indentation
SELECT id,
    name,
        email
FROM users;

-- Tabs indentation  
SELECT id,
	name,
		email
FROM users;

-- Mixed indentation
SELECT 
        id,
        name,
        email
    FROM 
        users;
```

### Comments

WWSQL supports both single-line and multi-line comments:

```sql
-- This is a single-line comment
SELECT id, name -- This is an inline comment
FROM users;

/* This is a 
   multi-line comment */
SELECT * FROM users;

/* Multi-line comment
   spanning
   multiple lines */
INSERT INTO users VALUES (1, 'John', 'john@example.com', true);
```

### Complex Formatting

WWSQL can handle complex formatting with comments and newlines:

```sql
-- Complex query with various formatting features
SELECT 
    id,                    -- User ID
    name,                  /* User name */
    email 
FROM users 
WHERE id > 0              -- Active users only
      AND name IS NOT NULL
ORDER BY name ASC;
```

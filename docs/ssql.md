# WWSQL Language Reference

This document provides a comprehensive reference for WWSQL, the SQL dialect used by `wwfsdb`, powered by the Vitess SQL parser.

---

## Supported Data Types

WWSQL supports the following data types for table columns:

| Data Type | Description | Example Value |
| --- | --- | --- |
| `INT` / `INTEGER` | For whole numbers (stored as 64-bit integers). | `100` |
| `FLOAT` / `REAL` / `DOUBLE` | For floating-point numbers (stored as 64-bit floats). | `123.45` |
| `DECIMAL` | For fixed-point numbers. Stored as float. | `10.99` |
| `VARCHAR(n)` | For variable-length strings with a specified maximum length. | `'Hello'` |
| `TEXT` | For variable-length strings of any length. | `'This is a long text.'` |
| `BOOLEAN` / `BOOL` | For `true` or `false` values. | `true` |
| `DATE` | For date values. Expected format: `YYYY-MM-DD`. | `'2025-10-15'` |
| `TIME` | For time values. Expected format: `HH:MM:SS`. | `'14:30:00'` |
| `TIMESTAMP` | For timestamp values. Expected format: RFC3339. | `'2025-10-15T14:30:00Z'` |

---

## Literals and Identifiers

### Numeric Literals
WWSQL supports standard integer and float literals, as well as more advanced formats.

- **Scientific Notation**: `1.23e-4`, `2.5E+5`
- **Hexadecimal**: `0xFF0000`, `0xDEADBEEF`
- **Binary**: `0b11111111`, `0b1010`

### String Literals
String literals must be enclosed in single quotes (e.g., `'hello world'`). Unicode characters are fully supported.

### Quoted Identifiers
Identifiers (like table or column names) can be enclosed in double quotes to allow for special characters, spaces, or to use reserved keywords as names.

```sql
CREATE TABLE "My Orders" ("item-name" VARCHAR(100), "quantity" INT);
```

---

## Data Definition Language (DDL)

DDL commands are used to define and manage the database structure.

### `CREATE DATABASE`

Initializes a new, empty database and returns its permanent Program ID (IPNS) and a private key for write access.

**Syntax:**
```sql
CREATE DATABASE database_name;
```

### `CREATE TABLE`

Creates a new table within a database. Column constraints `NOT NULL` and `UNIQUE` are supported.

**Syntax:**
```sql
CREATE TABLE table_name (
  column_1_name column_1_type [CONSTRAINT],
  ...
);
```

**Example:**
```sql
CREATE TABLE users (
  id INT,
  username VARCHAR(50) NOT NULL UNIQUE,
  email TEXT,
  is_verified BOOLEAN
);
```
**Note**: Columns with a `UNIQUE` constraint will have an index automatically created for them.

### `DROP TABLE`

Removes a table from the database.

**Syntax:**
```sql
DROP TABLE table_name;
```

### `RENAME TABLE`

Renames an existing table.

**Syntax:**
```sql
ALTER TABLE old_table_name RENAME TO new_table_name;
```

### `ALTER TABLE`

**Note**: `ALTER TABLE` for modifying columns (`ADD`, `DROP`, `RENAME`) is parsed but not yet fully implemented. Only `RENAME TABLE` is currently functional.

### `CREATE INDEX`

**Note**: The `CREATE INDEX` command is not yet supported. However, indexes are created automatically for all columns with a `UNIQUE` constraint to ensure efficient uniqueness checks.

---

## Data Manipulation Language (DML)

DML commands are used to manage data within tables. All DML operations require a valid private key.

### `INSERT INTO`

Adds new rows to a table.

**Syntax:**
```sql
-- Insert values in the order of the table schema
INSERT INTO table_name VALUES (value_1, value_2, ...);

-- Insert values for specific columns
INSERT INTO table_name (column_1, column_2) VALUES (value_1, value_2);
```

**Example:**
```sql
INSERT INTO users (id, username, is_verified) VALUES (1, 'alice', true);
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
DELETE FROM users WHERE is_verified = false;
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
[JOIN_TYPE] JOIN table_2 ON condition
[WHERE condition]
[GROUP BY column_1, ...]
[HAVING condition]
[ORDER BY column_1 [ASC|DESC], ...]
[LIMIT number]
[OFFSET number];
```

**Clauses:**

- **`SELECT`**: Can be `*` (all columns), `table.*`, or a comma-separated list of columns. Also supports aggregate functions:
  - `COUNT(* | [DISTINCT] column)`
  - `SUM([DISTINCT] column)`
  - `AVG([DISTINCT] column)`
  - `MIN(column)`
  - `MAX(column)`
- **`FROM`**: Specifies the primary table for the query.
- **`JOIN`**: Supports `INNER JOIN`, `LEFT JOIN`, `RIGHT JOIN`, and `NATURAL JOIN`. `FULL OUTER JOIN` is also supported through simulation.
- **`WHERE`**: Filters results based on a condition. Supported operators include: `=`, `!=`, `>`, `<`, `>=`, `<=`, `AND`, `OR`, `IS NULL`, `IS NOT NULL`. Subqueries with `EXISTS` are also supported.
- **`GROUP BY`**: Groups rows that have the same values in specified columns into summary rows, often used with aggregate functions.
- **`HAVING`**: Filters groups based on a condition after aggregation.
- **`ORDER BY`**: Sorts the result set.
- **`LIMIT`**: Constrains the number of rows returned.
- **`OFFSET`**: Skips a specified number of rows before returning results.

**Examples:**

```sql
-- Select specific columns with a filter
SELECT id, username FROM users WHERE is_verified = true;

-- Select with a JOIN
SELECT users.username, orders.order_id
FROM users
LEFT JOIN orders ON users.id = orders.user_id;

-- Paginate through results
SELECT id, username FROM users ORDER BY id LIMIT 10 OFFSET 20; -- Returns rows 21-30

-- Count users by country and show the top 5 with more than 10 users
SELECT country, COUNT(*) AS user_count
FROM users
GROUP BY country
HAVING user_count > 10
ORDER BY user_count DESC
LIMIT 5;
```

### `UNION`

Combines the result sets of two or more `SELECT` statements. `UNION` removes duplicate rows, while `UNION ALL` includes all rows.

```sql
SELECT name FROM employees
UNION
SELECT name FROM contractors;
```

### Advanced Query Examples

The following examples demonstrate complex queries using a sample e-commerce schema with four tables: `customers`, `products`, `orders`, and `order_items`.

#### Example 1: Customer Spending Report

This query calculates the total spending and order count for each customer, including customers who have not placed any orders.

*Features demonstrated: `LEFT JOIN`, `GROUP BY`, aggregate functions (`COUNT`, `SUM`), `ORDER BY`.*

```sql
SELECT 
    c.customer_id,
    c.name,
    c.email,
    COUNT(DISTINCT o.order_id) AS total_orders,
    SUM(oi.quantity * oi.price_per_unit) AS total_spent
FROM customers c
LEFT JOIN orders o ON c.customer_id = o.customer_id
LEFT JOIN order_items oi ON o.order_id = oi.order_id
GROUP BY c.customer_id, c.name, c.email
ORDER BY total_spent DESC;
```

#### Example 2: Finding Customers Who Ordered the Most Expensive Product

This query uses nested subqueries to first find the most expensive product, then find the customers who ordered it.

*Features demonstrated: Nested subqueries, `IN` operator, `ORDER BY` with `LIMIT`.*

```sql
SELECT 
    c.customer_id,
    c.name,
    c.email
FROM customers c
WHERE c.customer_id IN (
    SELECT o.customer_id
    FROM orders o
    JOIN order_items oi ON o.order_id = oi.order_id
    WHERE oi.product_id = (
        SELECT product_id
        FROM products
        ORDER BY price DESC
        LIMIT 1
    )
);
```

#### Example 3: Category Revenue Using a Derived Table

This query first creates a temporary "derived table" of all individual sale line items and then uses it to calculate the total revenue per product category.

*Features demonstrated: Subquery in `FROM` clause (derived table), `JOIN`, `SUM`, `GROUP BY`.*

```sql
SELECT
    category_sales.category,
    SUM(category_sales.line_total) AS total_revenue
FROM
    (SELECT
        p.category,
        oi.quantity * oi.price_per_unit AS line_total
    FROM products p
    JOIN order_items oi ON p.product_id = oi.product_id) AS category_sales
GROUP BY
    category_sales.category;
```

#### Example 4: Product Performance with Correlated Subquery

This query lists all products and uses a correlated subquery to calculate the total quantity sold for each, filtering out products that have never been sold.

*Features demonstrated: Correlated subquery in `SELECT` and `WHERE` clauses, `IS NOT NULL`.*

```sql
SELECT 
    p.product_id,
    p.name,
    p.category,
    p.price,
    (SELECT SUM(quantity) 
     FROM order_items oi 
     WHERE oi.product_id = p.product_id) AS total_quantity_sold
FROM products p
WHERE (SELECT SUM(quantity) 
       FROM order_items oi 
       WHERE oi.product_id = p.product_id) IS NOT NULL
ORDER BY total_quantity_sold DESC;
```

#### Example 5: Comprehensive Business Intelligence Report

This "ultimate" query combines many features to generate a full sales analysis report for each customer, including lifetime value, category-specific spending, and more.

*Features demonstrated: Multiple `LEFT JOIN`s, complex `GROUP BY`, `HAVING`, conditional aggregation with `CASE`, multiple aggregate functions.*

```sql
SELECT 
    c.customer_id,
    c.name AS customer_name,
    c.registration_date,
    COUNT(DISTINCT o.order_id) AS total_orders,
    SUM(oi.quantity) AS total_items_purchased,
    SUM(oi.quantity * oi.price_per_unit) AS lifetime_value,
    AVG(o.total_amount) AS avg_order_value,
    MAX(o.order_date) AS last_purchase_date,
    SUM(CASE WHEN p.category = 'Electronics' THEN oi.quantity * oi.price_per_unit ELSE 0 END) AS electronics_spending,
    SUM(CASE WHEN p.category = 'Furniture' THEN oi.quantity * oi.price_per_unit ELSE 0 END) AS furniture_spending
FROM customers c
LEFT JOIN orders o ON c.customer_id = o.customer_id
LEFT JOIN order_items oi ON o.order_id = oi.order_id
LEFT JOIN products p ON oi.product_id = p.product_id
GROUP BY c.customer_id, c.name, c.registration_date
HAVING COUNT(DISTINCT o.order_id) > 0
ORDER BY lifetime_value DESC;
```

---

## Transaction Control Language (TCL)

TCL commands are used to manage transactions within the interactive shell or over RPC.

### `BEGIN`

Starts a new transaction block.

### `COMMIT`

Saves all changes made during the current transaction.

### `ROLLBACK`

Discards all changes made during the current transaction.

**Example in the shell:**
```sql
BEGIN;
INSERT INTO users VALUES (10, 'temp_user', 'temp@email.com', false);
ROLLBACK; -- The user is not saved
```

---

## Comments

WWSQL supports both single-line and multi-line comments for improved code readability.

```sql
-- This is a single-line comment.
SELECT * FROM users; -- This is an inline comment.

/* 
  This is a multi-line comment.
*/
SELECT id, username FROM users;
```
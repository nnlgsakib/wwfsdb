
-- Schema for a complex e-commerce application

CREATE TABLE customers (
    customer_id INT,
    name VARCHAR(255),
    email VARCHAR(255),
    registration_date DATE
);

CREATE TABLE products (
    product_id INT,
    name VARCHAR(255),
    category VARCHAR(100),
    price DECIMAL(10, 2)
);

CREATE TABLE orders (
    order_id INT,
    customer_id INT,
    order_date DATE,
    total_amount DECIMAL(10, 2)
);

CREATE TABLE order_items (
    order_item_id INT,
    order_id INT,
    product_id INT,
    quantity INT,
    price_per_unit DECIMAL(10, 2)
);

-- Insert sample data

INSERT INTO customers (customer_id, name, email, registration_date) VALUES (1, 'John Doe', 'john.doe@email.com', '2023-01-15');
INSERT INTO customers (customer_id, name, email, registration_date) VALUES (2, 'Jane Smith', 'jane.smith@email.com', '2023-02-20');
INSERT INTO customers (customer_id, name, email, registration_date) VALUES (3, 'Peter Jones', 'peter.jones@email.com', '2023-03-10');

INSERT INTO products (product_id, name, category, price) VALUES (101, 'Laptop', 'Electronics', 1200.00);
INSERT INTO products (product_id, name, category, price) VALUES (102, 'Smartphone', 'Electronics', 800.00);
INSERT INTO products (product_id, name, category, price) VALUES (201, 'Office Chair', 'Furniture', 150.00);
INSERT INTO products (product_id, name, category, price) VALUES (202, 'Desk', 'Furniture', 300.00);

INSERT INTO orders (order_id, customer_id, order_date, total_amount) VALUES (1001, 1, '2023-04-01', 1350.00);
INSERT INTO orders (order_id, customer_id, order_date, total_amount) VALUES (1002, 2, '2023-04-05', 800.00);
INSERT INTO orders (order_id, customer_id, order_date, total_amount) VALUES (1003, 1, '2023-04-10', 450.00);

INSERT INTO order_items (order_item_id, order_id, product_id, quantity, price_per_unit) VALUES (1, 1001, 101, 1, 1200.00);
INSERT INTO order_items (order_item_id, order_id, product_id, quantity, price_per_unit) VALUES (2, 1001, 201, 1, 150.00);
INSERT INTO order_items (order_item_id, order_id, product_id, quantity, price_per_unit) VALUES (3, 1002, 102, 1, 800.00);
INSERT INTO order_items (order_item_id, order_id, product_id, quantity, price_per_unit) VALUES (4, 1003, 201, 3, 150.00);


-- Complex queries to test subquery features

-- Test 1: Subquery in FROM clause (Derived Table)
-- Find the total revenue generated per product category.
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


-- Test 2: Subquery in WHERE clause with IN operator
-- Find the names of all customers who have ordered a 'Laptop'.
SELECT name
FROM customers
WHERE customer_id IN (
    SELECT o.customer_id
    FROM orders o
    JOIN order_items oi ON o.order_id = oi.order_id
    WHERE oi.product_id = (SELECT product_id FROM products WHERE name = 'Laptop')
);


-- Test 3: Subquery in WHERE clause with a comparison operator
-- Find the order with the highest total amount.
SELECT order_id, total_amount
FROM orders
WHERE total_amount = (
    SELECT MAX(total_amount)
    FROM orders
);


-- Test 4: Scalar subquery in the SELECT clause
-- List all products and include a count of how many times each has been ordered.
SELECT
    p.name,
    p.price,
    (SELECT COUNT(*)
     FROM order_items oi
     WHERE oi.product_id = p.product_id) AS times_ordered
FROM
    products p;




SELECT
    c.name AS customer_name,
    c.email,
    (SELECT SUM(o.total_amount) FROM orders o WHERE o.customer_id = c.customer_id) AS total_spent,
    (SELECT COUNT(*) FROM orders o WHERE o.customer_id = c.customer_id) AS total_orders,
    (SELECT p.category
     FROM products p
     JOIN order_items oi ON p.product_id = oi.product_id
     JOIN orders o ON oi.order_id = o.order_id
     WHERE o.customer_id = c.customer_id
     GROUP BY p.category
     ORDER BY COUNT(*) DESC, p.category ASC
     LIMIT 1) AS favorite_category
FROM
    customers c
WHERE
    (SELECT SUM(o.total_amount) FROM orders o WHERE o.customer_id = c.customer_id) > (
        SELECT AVG(total_spent)
        FROM (
            SELECT SUM(o.total_amount) AS total_spent
            FROM orders o
            GROUP BY o.customer_id
        ) avg_spending
    )
ORDER BY
    total_spent DESC
LIMIT 5;
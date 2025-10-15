
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





-- ============================================
-- COMPLEX SQL TEST SUITE FOR E-COMMERCE DB
-- ============================================

-- TEST 1: Multiple JOINs with Aggregation
-- Find total revenue per customer with product details
SELECT 
    c.customer_id,
    c.name,
    c.email,
    COUNT(DISTINCT o.order_id) AS total_orders,
    COUNT(oi.order_item_id) AS total_items_purchased,
    SUM(oi.quantity * oi.price_per_unit) AS total_spent
FROM customers c
LEFT OUTER JOIN orders o ON c.customer_id = o.customer_id
LEFT OUTER JOIN order_items oi ON o.order_id = oi.order_id
GROUP BY c.customer_id, c.name, c.email
ORDER BY total_spent DESC;

-- TEST 2: Subquery in WHERE clause
-- Find customers who spent more than the average order amount
SELECT 
    c.customer_id,
    c.name,
    SUM(o.total_amount) AS total_spent
FROM customers c
JOIN orders o ON c.customer_id = o.customer_id
GROUP BY c.customer_id, c.name
HAVING SUM(o.total_amount) > (
    SELECT AVG(total_amount) FROM orders
);

-- TEST 3: Correlated Subquery
-- Find products that have been ordered and their total quantity sold
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

-- TEST 4: UNION query combining different data sources
-- Get all Electronics products and all orders over $1000
SELECT 
    'Product' AS type,
    product_id AS id,
    name AS description,
    price AS amount
FROM products
WHERE category = 'Electronics'
UNION
SELECT 
    'Order' AS type,
    order_id AS id,
    'Order Total' AS description,
    total_amount AS amount
FROM orders
WHERE total_amount > 1000
ORDER BY amount DESC;

-- TEST 5: Complex JOIN with WHERE and GROUP BY
-- Find customers who bought products from multiple categories
SELECT 
    c.customer_id,
    c.name,
    COUNT(DISTINCT p.category) AS categories_purchased,
    SUM(oi.quantity * oi.price_per_unit) AS total_spent
FROM customers c
JOIN orders o ON c.customer_id = o.customer_id
JOIN order_items oi ON o.order_id = oi.order_id
JOIN products p ON oi.product_id = p.product_id
GROUP BY c.customer_id, c.name
HAVING COUNT(DISTINCT p.category) > 1
ORDER BY categories_purchased DESC;

-- TEST 6: RIGHT OUTER JOIN with aggregation
-- Show all products and how many times they've been ordered
SELECT 
    p.product_id,
    p.name,
    p.category,
    COUNT(oi.order_item_id) AS times_ordered,
    SUM(oi.quantity) AS total_quantity_sold
FROM order_items oi
RIGHT OUTER JOIN products p ON oi.product_id = p.product_id
GROUP BY p.product_id, p.name, p.category
ORDER BY times_ordered DESC;

-- TEST 7: Complex WHERE with OR and IS NOT NULL
-- Find orders with Electronics OR orders over $500
SELECT 
    o.order_id,
    o.customer_id,
    o.order_date,
    o.total_amount,
    p.category
FROM orders o
JOIN order_items oi ON o.order_id = oi.order_id
JOIN products p ON oi.product_id = p.product_id
WHERE p.category = 'Electronics' OR o.total_amount > 500
GROUP BY o.order_id, o.customer_id, o.order_date, o.total_amount, p.category
ORDER BY o.order_date;

-- TEST 8: Nested subqueries
-- Find customers who ordered the most expensive product
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

-- TEST 9: Aggregate functions with multiple tables
-- Calculate revenue by category
SELECT 
    p.category,
    COUNT(DISTINCT o.order_id) AS total_orders,
    SUM(oi.quantity) AS total_units_sold,
    SUM(oi.quantity * oi.price_per_unit) AS total_revenue,
    AVG(oi.price_per_unit) AS avg_price_per_unit
FROM products p
LEFT OUTER JOIN order_items oi ON p.product_id = oi.product_id
LEFT OUTER JOIN orders o ON oi.order_id = o.order_id
GROUP BY p.category
ORDER BY total_revenue DESC;

-- TEST 10: UNION ALL with different projections
-- Combine customer summary and product summary
SELECT 
    'CUSTOMER' AS entity_type,
    name AS entity_name,
    registration_date AS date_field,
    0 AS numeric_field
FROM customers
UNION
SELECT 
    'PRODUCT' AS entity_type,
    name AS entity_name,
    '2023-01-01' AS date_field,
    price AS numeric_field
FROM products
ORDER BY entity_type, entity_name;

-- TEST 11: Complex JOIN with calculated fields
-- Customer purchase analysis with profit margin
SELECT 
    c.customer_id,
    c.name,
    COUNT(DISTINCT o.order_id) AS order_count,
    SUM(oi.quantity) AS total_items,
    SUM(oi.quantity * oi.price_per_unit) AS total_paid,
    SUM(oi.quantity * p.price) AS total_list_price,
    SUM(oi.quantity * p.price) - SUM(oi.quantity * oi.price_per_unit) AS discount_given
FROM customers c
JOIN orders o ON c.customer_id = o.customer_id
JOIN order_items oi ON o.order_id = oi.order_id
JOIN products p ON oi.product_id = p.product_id
GROUP BY c.customer_id, c.name
ORDER BY total_paid DESC;

-- TEST 12: NATURAL JOIN test
-- Join order_items with orders using common column
SELECT 
    o.order_id,
    o.customer_id,
    o.order_date,
    COUNT(oi.order_item_id) AS item_count
FROM orders o
NATURAL JOIN order_items oi
GROUP BY o.order_id, o.customer_id, o.order_date;

-- TEST 13: EXISTS subquery
-- Find customers who have placed orders
SELECT 
    c.customer_id,
    c.name,
    c.email
FROM customers c
WHERE EXISTS (
    SELECT 1
    FROM orders o
    WHERE o.customer_id = c.customer_id
);

-- TEST 14: NOT IN subquery
-- Find products that have never been ordered
SELECT 
    p.product_id,
    p.name,
    p.category,
    p.price
FROM products p
WHERE p.product_id NOT IN (
    SELECT DISTINCT product_id
    FROM order_items
);

-- TEST 15: Complex aggregation with HAVING
-- Find product categories with average price over $500
SELECT 
    p.category,
    COUNT(p.product_id) AS product_count,
    AVG(p.price) AS avg_price,
    MIN(p.price) AS min_price,
    MAX(p.price) AS max_price
FROM products p
GROUP BY p.category
HAVING AVG(p.price) > 500;

-- TEST 16: Multiple LEFT JOINs with filtering
-- Customer order history with product details
SELECT 
    c.name AS customer_name,
    o.order_id,
    o.order_date,
    p.name AS product_name,
    p.category,
    oi.quantity,
    oi.price_per_unit,
    oi.quantity * oi.price_per_unit AS line_total
FROM customers c
LEFT OUTER JOIN orders o ON c.customer_id = o.customer_id
LEFT OUTER JOIN order_items oi ON o.order_id = oi.order_id
LEFT OUTER JOIN products p ON oi.product_id = p.product_id
WHERE o.order_date IS NOT NULL
ORDER BY c.name, o.order_date, p.name;

-- TEST 17: UNION with aggregations
-- Combine top customers by order count and top customers by spending
SELECT 
    'By Order Count' AS metric,
    c.customer_id,
    c.name,
    COUNT(o.order_id) AS value
FROM customers c
JOIN orders o ON c.customer_id = o.customer_id
GROUP BY c.customer_id, c.name
ORDER BY value DESC
LIMIT 2
UNION
SELECT 
    'By Total Spent' AS metric,
    c.customer_id,
    c.name,
    SUM(o.total_amount) AS value
FROM customers c
JOIN orders o ON c.customer_id = o.customer_id
GROUP BY c.customer_id, c.name
ORDER BY value DESC
LIMIT 2;

-- TEST 18: Self-referencing query with subquery
-- Find customers who spent more than customer 1
SELECT 
    c.customer_id,
    c.name,
    SUM(o.total_amount) AS total_spent
FROM customers c
JOIN orders o ON c.customer_id = o.customer_id
GROUP BY c.customer_id, c.name
HAVING SUM(o.total_amount) > (
    SELECT SUM(total_amount)
    FROM orders
    WHERE customer_id = 1
);

-- TEST 19: Complex LIKE and OR conditions
-- Find products or customers with specific patterns
SELECT 
    'Product' AS type,
    name AS entity_name,
    'N/A' AS email
FROM products
WHERE name LIKE '%Chair%' OR name LIKE '%Desk%'
UNION
SELECT 
    'Customer' AS type,
    name AS entity_name,
    email
FROM customers
WHERE name LIKE '%John%' OR email LIKE '%smith%'
ORDER BY type, entity_name;

-- TEST 20: Multi-level aggregation with arithmetic
-- Product performance analysis
SELECT 
    p.category,
    p.name,
    COUNT(DISTINCT oi.order_id) AS times_ordered,
    SUM(oi.quantity) AS total_quantity,
    AVG(oi.price_per_unit) AS avg_selling_price,
    p.price AS list_price,
    p.price - AVG(oi.price_per_unit) AS avg_discount,
    SUM(oi.quantity * oi.price_per_unit) AS total_revenue
FROM products p
LEFT OUTER JOIN order_items oi ON p.product_id = oi.product_id
GROUP BY p.product_id, p.category, p.name, p.price
HAVING COUNT(DISTINCT oi.order_id) > 0
ORDER BY total_revenue DESC;

-- TEST 21: Date-based filtering with joins
-- Orders from a specific month
SELECT 
    o.order_id,
    c.name AS customer_name,
    o.order_date,
    o.total_amount,
    COUNT(oi.order_item_id) AS item_count
FROM orders o
JOIN customers c ON o.customer_id = c.customer_id
LEFT OUTER JOIN order_items oi ON o.order_id = oi.order_id
WHERE o.order_date LIKE '2023-04%'
GROUP BY o.order_id, c.name, o.order_date, o.total_amount
ORDER BY o.order_date;

-- TEST 22: IN with subquery and multiple conditions
-- Customers who bought both Electronics and Furniture
SELECT 
    c.customer_id,
    c.name
FROM customers c
WHERE c.customer_id IN (
    SELECT o.customer_id
    FROM orders o
    JOIN order_items oi ON o.order_id = oi.order_id
    JOIN products p ON oi.product_id = p.product_id
    WHERE p.category = 'Electronics'
)
AND c.customer_id IN (
    SELECT o.customer_id
    FROM orders o
    JOIN order_items oi ON o.order_id = oi.order_id
    JOIN products p ON oi.product_id = p.product_id
    WHERE p.category = 'Furniture'
);

-- TEST 23: Aggregation with NULL handling
-- All customers with their order statistics (including those without orders)
SELECT 
    c.customer_id,
    c.name,
    COUNT(o.order_id) AS order_count,
    SUM(o.total_amount) AS total_spent,
    AVG(o.total_amount) AS avg_order_value,
    MAX(o.order_date) AS last_order_date
FROM customers c
LEFT OUTER JOIN orders o ON c.customer_id = o.customer_id
GROUP BY c.customer_id, c.name
ORDER BY order_count DESC, total_spent DESC;

-- TEST 24: Complex UNION with filtering and ordering
-- Top items by different metrics
SELECT 
    'Most Expensive' AS category,
    name,
    price AS metric_value
FROM products
ORDER BY price DESC
LIMIT 3
UNION
SELECT 
    'Most Ordered' AS category,
    p.name,
    SUM(oi.quantity) AS metric_value
FROM products p
JOIN order_items oi ON p.product_id = oi.product_id
GROUP BY p.product_id, p.name
ORDER BY metric_value DESC
LIMIT 3;

-- TEST 25: Ultimate complex query - Complete business intelligence report
-- Comprehensive sales analysis with customer segmentation
SELECT 
    c.customer_id,
    c.name AS customer_name,
    c.registration_date,
    COUNT(DISTINCT o.order_id) AS total_orders,
    COUNT(DISTINCT p.category) AS categories_shopped,
    SUM(oi.quantity) AS total_items_purchased,
    SUM(oi.quantity * oi.price_per_unit) AS lifetime_value,
    AVG(o.total_amount) AS avg_order_value,
    MAX(o.order_date) AS last_purchase_date,
    SUM(CASE WHEN p.category = 'Electronics' THEN oi.quantity * oi.price_per_unit ELSE 0 END) AS electronics_spending,
    SUM(CASE WHEN p.category = 'Furniture' THEN oi.quantity * oi.price_per_unit ELSE 0 END) AS furniture_spending
FROM customers c
LEFT OUTER JOIN orders o ON c.customer_id = o.customer_id
LEFT OUTER JOIN order_items oi ON o.order_id = oi.order_id
LEFT OUTER JOIN products p ON oi.product_id = p.product_id
GROUP BY c.customer_id, c.name, c.registration_date
HAVING COUNT(DISTINCT o.order_id) > 0
ORDER BY lifetime_value DESC, total_orders DESC;
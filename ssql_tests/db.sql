-- Complex SQL Schema using new features

-- Users table with VARCHAR(36) for UUID, UNIQUE email, and TIMESTAMP
CREATE TABLE users (
    id VARCHAR(36),
    username VARCHAR(50) NOT NULL,
    email VARCHAR(100) NOT NULL UNIQUE,
    full_name VARCHAR(100),
    created_at TIMESTAMP NOT NULL,
    last_login_time TIME,
    PRIMARY KEY (id)
);

-- Products table with DECIMAL, UNIQUE SKU, and CHECK constraint
CREATE TABLE products (
    product_id VARCHAR(36),
    name VARCHAR(100) NOT NULL,
    sku VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    price DECIMAL(10, 2) NOT NULL,
    stock_quantity INT NOT NULL,
    release_date DATE,
    PRIMARY KEY (product_id)
);

-- Orders table with DATE, TIME, and a mix of data types
CREATE TABLE orders (
    order_id VARCHAR(36),
    user_id VARCHAR(36) NOT NULL,
    order_date DATE NOT NULL,
    order_time TIME NOT NULL,
    total_amount DECIMAL(12, 2) NOT NULL,
    status VARCHAR(20) NOT NULL,
    PRIMARY KEY (order_id)
);

-- Order items table to link products and orders
CREATE TABLE order_items (
    order_item_id VARCHAR(36),
    order_id VARCHAR(36) NOT NULL,
    product_id VARCHAR(36) NOT NULL,
    quantity INT NOT NULL,
    price_per_unit DECIMAL(10, 2) NOT NULL,
    PRIMARY KEY (order_item_id)
);

-- Sample Inserts

-- Insert into users
INSERT INTO users (id, username, email, full_name, created_at, last_login_time) VALUES 
('a1b2c3d4-e5f6-7890-1234-567890abcdef', 'john_doe', 'john.doe@example.com', 'John Doe', '2023-10-27T10:00:00Z', '10:00:00'),
('b2c3d4e5-f6a7-8901-2345-67890abcdef1', 'jane_smith', 'jane.smith@example.com', 'Jane Smith', '2023-10-27T11:30:00Z', '11:30:00');

-- Insert into products
INSERT INTO products (product_id, name, sku, description, price, stock_quantity, release_date) VALUES
('c3d4e5f6-a7b8-9012-3456-7890abcdef12', 'Laptop Pro', 'LP-2023', 'A powerful new laptop', 1499.99, 50, '2023-09-01'),
('d4e5f6a7-b8c9-0123-4567-890abcdef123', 'Wireless Mouse', 'WM-2023', 'An ergonomic wireless mouse', 49.99, 200, '2023-08-15');

-- Insert into orders
INSERT INTO orders (order_id, user_id, order_date, order_time, total_amount, status) VALUES
('e5f6a7b8-c9d0-1234-5678-90abcdef1234', 'a1b2c3d4-e5f6-7890-1234-567890abcdef', '2023-10-27', '12:00:00', 1549.98, 'shipped');

-- Insert into order_items
INSERT INTO order_items (order_item_id, order_id, product_id, quantity, price_per_unit) VALUES
('f6a7b8c9-d0e1-2345-6789-0abcdef12345', 'e5f6a7b8-c9d0-1234-5678-90abcdef1234', 'c3d4e5f6-a7b8-9012-3456-7890abcdef12', 1, 1499.99),
('a7b8c9d0-e1f2-3456-7890-bcdef1234567', 'e5f6a7b8-c9d0-1234-5678-90abcdef1234', 'd4e5f6a7-b8c9-0123-4567-890abcdef123', 1, 49.99);


-- Constraint Violation Examples (commented out)

-- VIOLATES NOT NULL on email
-- INSERT INTO users (id, username, created_at) VALUES ('a1b2c3d4-e5f6-7890-1234-567890abcdef', 'test_user', '2023-10-27T10:00:00Z');

-- VIOLATES UNIQUE on email
-- INSERT INTO users (id, username, email, created_at) VALUES ('b2c3d4e5-f6a7-8901-2345-67890abcdef1', 'another_user', 'john.doe@example.com', '2023-10-27T11:00:00Z');

-- VIOLATES CHECK on price
-- INSERT INTO products (product_id, name, sku, price, stock_quantity) VALUES ('c3d4e5f6-a7b8-9012-3456-7890abcdef12', 'Defective Laptop', 'LP-2024', -10.00, 5);

-- VIOLATES NOT NULL on price
-- INSERT INTO products (product_id, name, sku, stock_quantity) VALUES ('d4e5f6a7-b8c9-0123-4567-890abcdef123', 'Widget', 'W-2024', 100);


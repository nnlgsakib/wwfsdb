-- Mobile Banking System Database Schema
-- This schema is designed for a production-level mobile banking application.

-- Users Table: Stores information about the application users.
CREATE TABLE users (
    user_id VARCHAR(36) PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    email VARCHAR(100) NOT NULL UNIQUE,
    full_name VARCHAR(100) NOT NULL,
    phone_number VARCHAR(20) UNIQUE,
    address TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP
);

-- Accounts Table: Stores information about user bank accounts.
CREATE TABLE accounts (
    account_id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    account_number VARCHAR(20) NOT NULL UNIQUE,
    account_type VARCHAR(20) NOT NULL, -- e.g., 'Checking', 'Savings'
    balance DECIMAL(15, 2) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    status VARCHAR(20) NOT NULL, -- e.g., 'Active', 'Inactive', 'Closed'
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(user_id)
);

-- Transactions Table: Stores the history of all transactions.
CREATE TABLE transactions (
    transaction_id VARCHAR(36) PRIMARY KEY,
    from_account_id VARCHAR(36),
    to_account_id VARCHAR(36),
    amount DECIMAL(15, 2) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    transaction_type VARCHAR(20) NOT NULL, -- e.g., 'Transfer', 'Deposit', 'Withdrawal'
    description TEXT,
    status VARCHAR(20) NOT NULL, -- e.g., 'Pending', 'Completed', 'Failed'
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (from_account_id) REFERENCES accounts(account_id),
    FOREIGN KEY (to_account_id) REFERENCES accounts(account_id)
);

-- Beneficiaries Table: Stores information about saved beneficiaries for fund transfers.
CREATE TABLE beneficiaries (
    beneficiary_id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    nickname VARCHAR(50) NOT NULL,
    account_number VARCHAR(20) NOT NULL,
    bank_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(user_id)
);

-- Cards Table: Stores information about debit/credit cards linked to accounts.
CREATE TABLE cards (
    card_id VARCHAR(36) PRIMARY KEY,
    account_id VARCHAR(36) NOT NULL,
    card_number VARCHAR(20) NOT NULL UNIQUE,
    card_type VARCHAR(20) NOT NULL, -- e.g., 'Debit', 'Credit'
    expiry_date VARCHAR(7) NOT NULL, -- MM/YYYY
    cvv_hash VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL, -- e.g., 'Active', 'Inactive', 'Lost'
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (account_id) REFERENCES accounts(account_id)
);

-- Loans Table: Stores information about loans taken by users.
CREATE TABLE loans (
    loan_id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    loan_type VARCHAR(50) NOT NULL, -- e.g., 'Personal', 'Home', 'Car'
    amount DECIMAL(15, 2) NOT NULL,
    interest_rate DECIMAL(5, 2) NOT NULL,
    term_months INT NOT NULL,
    status VARCHAR(20) NOT NULL, -- e.g., 'Active', 'Paid Off'
    start_date DATE NOT NULL,
    end_date DATE,
    FOREIGN KEY (user_id) REFERENCES users(user_id)
);

-- Loan Payments Table: Stores the history of loan payments.
CREATE TABLE loan_payments (
    payment_id VARCHAR(36) PRIMARY KEY,
    loan_id VARCHAR(36) NOT NULL,
    amount DECIMAL(15, 2) NOT NULL,
    payment_date DATE NOT NULL,
    status VARCHAR(20) NOT NULL, -- e.g., 'Completed', 'Failed'
    FOREIGN KEY (loan_id) REFERENCES loans(loan_id)
);

-- Support Tickets Table: Stores customer support tickets.
CREATE TABLE support_tickets (
    ticket_id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    subject VARCHAR(100) NOT NULL,
    description TEXT NOT NULL,
    status VARCHAR(20) NOT NULL, -- e.g., 'Open', 'In Progress', 'Closed'
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(user_id)
);

-- App Settings Table: Stores user-specific application settings.
CREATE TABLE app_settings (
    setting_id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    setting_name VARCHAR(50) NOT NULL,
    setting_value VARCHAR(255) NOT NULL,
    UNIQUE (user_id, setting_name),
    FOREIGN KEY (user_id) REFERENCES users(user_id)
);

-- Security Logs Table: Logs security-related events.
CREATE TABLE security_logs (
    log_id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36),
    action VARCHAR(100) NOT NULL,
    ip_address VARCHAR(45),
    device_info TEXT,
    timestamp TIMESTAMP NOT NULL
);

-- Sample Data Insertion

-- Users
INSERT INTO users (user_id, username, password_hash, email, full_name, phone_number, address, created_at) VALUES
('1', 'nlg', 'hashed_password_1', 'johndoe@example.com', 'nlg 1', '123-456-7890', '123 Main St, Anytown, USA', '2023-01-15T10:00:00Z'),
('2', 'nlg2', 'hashed_password_2', 'janesmith@example.com', 'Jane Smith', '098-765-4321', '456 Oak Ave, Otherville, USA', '2023-02-20T11:30:00Z');

-- Accounts
INSERT INTO accounts (account_id, user_id, account_number, account_type, balance, currency, status, created_at) VALUES
('101', '1', '1234567890', 'Savings', 5000.00, 'USD', 'Active', '2023-01-15T10:00:00Z'),
('102', '1', '0987654321', 'Checking', 1500.50, 'USD', 'Active', '2023-01-15T10:05:00Z'),
('103', '2', '1122334455', 'Savings', 10000.00, 'USD', 'Active', '2023-02-20T11:30:00Z');

-- Transactions
INSERT INTO transactions (transaction_id, from_account_id, to_account_id, amount, currency, transaction_type, description, status, created_at) VALUES
('1001', '102', '101', 500.00, 'USD', 'Transfer', 'Monthly savings transfer', 'Completed', '2023-03-01T09:00:00Z'),
('1002', '103', NULL, 200.00, 'USD', 'Withdrawal', 'ATM withdrawal', 'Completed', '2023-03-05T14:20:00Z');

-- Beneficiaries
INSERT INTO beneficiaries (beneficiary_id, user_id, nickname, account_number, bank_name, created_at) VALUES
('201', '1', 'Mom', '5566778899', 'AnyBank', '2023-02-01T18:00:00Z');

-- Cards
INSERT INTO cards (card_id, account_id, card_number, card_type, expiry_date, cvv_hash, status, created_at) VALUES
('301', '102', '1111-2222-3333-4444', 'Debit', '12/26', 'hashed_cvv_1', 'Active', '2023-01-15T10:10:00Z');

-- Loans
INSERT INTO loans (loan_id, user_id, loan_type, amount, interest_rate, term_months, status, start_date) VALUES
('401', '2', 'Car Loan', 15000.00, 3.5, 60, 'Active', '2023-03-01');

-- Loan Payments
INSERT INTO loan_payments (payment_id, loan_id, amount, payment_date, status) VALUES
('501', '401', 300.00, '2023-04-01', 'Completed');

-- Support Tickets
INSERT INTO support_tickets (ticket_id, user_id, subject, description, status, created_at) VALUES
('601', '1', 'Card not working', 'My debit card is not working at ATMs.', 'Open', '2023-03-10T17:00:00Z');

-- App Settings
INSERT INTO app_settings (setting_id, user_id, setting_name, setting_value) VALUES
('701', '1', 'theme', 'dark'),
('702', '1', 'notifications_enabled', 'true');

-- Security Logs
INSERT INTO security_logs (log_id, user_id, action, ip_address, device_info, timestamp) VALUES
('801', '1', 'login', '192.168.1.10', 'iPhone 14 Pro', '2023-04-01T10:00:00Z'),
('802', '2', 'failed_login_attempt', '10.0.0.5', 'Android 13', '2023-04-01T10:05:00Z');

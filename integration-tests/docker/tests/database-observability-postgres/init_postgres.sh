#!/bin/bash
set -e

# Wait for PostgreSQL to be ready
echo "Waiting for PostgreSQL to be ready..."
until PGPASSWORD=rootpassword psql -h postgres -U root -d testdb -c "SELECT 1" > /dev/null 2>&1; do
    sleep 1
done

echo "PostgreSQL is ready. Enabling pg_stat_statements and creating test tables..."

PGPASSWORD=rootpassword psql -h postgres -U root -d testdb <<EOF
-- Enable pg_stat_statements extension
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Create products table
CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    quantity INT DEFAULT 0
);

-- Create orders table
CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    product_id INT,
    quantity INT NOT NULL,
    total DECIMAL(10, 2) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert test data into products
INSERT INTO products (name, price, quantity) VALUES
    ('Widget', 19.99, 100),
    ('Gadget', 29.99, 50),
    ('Gizmo', 39.99, 25);

-- Insert test data into orders
INSERT INTO orders (product_id, quantity, total, status) VALUES
    (1, 2, 39.98, 'completed'),
    (2, 1, 29.99, 'pending'),
    (3, 3, 119.97, 'shipped');

-- Run some queries to populate pg_stat_statements
SELECT * FROM products WHERE id = 1;
SELECT * FROM products WHERE price > 20;
SELECT * FROM orders WHERE status = 'pending';
SELECT p.name, o.total FROM products p JOIN orders o ON p.id = o.product_id;
EOF

echo "Test tables and data created successfully!"

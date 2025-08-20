#!/bin/sh
# Redis initialization script to populate test data
# This ensures keyspace metrics like redis_db_keys and redis_db_keys_expiring appear

# Wait for Redis to be ready
until redis-cli -h redis -p 6379 ping > /dev/null 2>&1; do
    echo "Waiting for Redis to be ready..."
    sleep 1
done

echo "Populating Redis with test data for metrics..."

# Add some test keys to database 0 (default)
redis-cli -h redis -p 6379 SET test_key_1 "value1"
redis-cli -h redis -p 6379 SET test_key_2 "value2"
redis-cli -h redis -p 6379 SET test_key_3 "value3"

# Add some keys with expiration to generate redis_db_keys_expiring metrics
redis-cli -h redis -p 6379 SET expiring_key_1 "expires1" EX 3600
redis-cli -h redis -p 6379 SET expiring_key_2 "expires2" EX 7200

# Add keys to database 1 as well
redis-cli -h redis -p 6379 -n 1 SET db1_key_1 "db1_value1"
redis-cli -h redis -p 6379 -n 1 SET db1_key_2 "db1_value2" EX 1800

echo "Redis test data populated successfully"
echo "Keys in DB 0: $(redis-cli -h redis -p 6379 DBSIZE)"
echo "Keys in DB 1: $(redis-cli -h redis -p 6379 -n 1 DBSIZE)"

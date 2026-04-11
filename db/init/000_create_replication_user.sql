-- Create replication user for streaming replication
CREATE ROLE IF NOT EXISTS replicator WITH REPLICATION LOGIN PASSWORD 'replicator_pass';

-- Create replication slot for slave1
SELECT pg_create_physical_replication_slot('slave1_slot');

-- Create replication slot for slave2
SELECT pg_create_physical_replication_slot('slave2_slot');

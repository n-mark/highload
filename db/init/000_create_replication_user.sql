-- Create replication user for streaming replication
DO
$$
BEGIN
   IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'replicator') THEN
      CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD 'replicator_pass';
   END IF;
END
$$;

-- Create replication slot for slave1
SELECT pg_create_physical_replication_slot('slave1_slot');

-- Create replication slot for slave2
SELECT pg_create_physical_replication_slot('slave2_slot');

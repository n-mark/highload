ALTER SYSTEM SET synchronous_commit = on;
ALTER SYSTEM SET synchronous_standby_names = 'ANY 2 (*)';
SELECT pg_reload_conf();
-- Enable Citus extension on the coordinator
CREATE EXTENSION IF NOT EXISTS citus;

-- =========================================================
-- messages table
-- =========================================================
-- Sharding strategy:
--   conversation_id = lower(from_user_id::text) || ':' || greater(to_user_id::text)
--   (canonical sorted pair — always smallest UUID first)
--
-- Why this key?
--   • Both sides of a conversation always land on the same shard → efficient
--     range queries for /dialog/{user_id}/list (single shard scan).
--   • "Lady Gaga effect": even if one user sends millions of messages, those
--     messages are spread across ALL shards because each unique conversation
--     partner produces a different shard key. No single shard is overloaded.
--   • Resharding: Citus supports online shard rebalancing via
--     citus_rebalance_start() without downtime (see README).

CREATE TABLE IF NOT EXISTS messages (
    id              uuid        NOT NULL DEFAULT uuidv7(),
    conversation_id text        NOT NULL,   -- shard distribution column
    from_user_id    uuid        NOT NULL,
    to_user_id      uuid        NOT NULL,
    text            text        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (conversation_id, id)       -- distribution col must be in PK
);

-- NOTE: create_distributed_table() and indexes are called AFTER workers
-- are registered in docker-compose citus-setup service.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS is_celebrity boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS followers_count integer NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_users_is_celebrity ON users (is_celebrity) WHERE is_celebrity;

-- Recalculate follower counts and celebrity flags.
-- Call with: SELECT recalc_celebrity_flags(10000);
CREATE OR REPLACE FUNCTION recalc_celebrity_flags(threshold integer)
RETURNS void AS $$
BEGIN
    UPDATE users u
    SET followers_count = COALESCE(sub.cnt, 0),
        is_celebrity = COALESCE(sub.cnt, 0) >= threshold
    FROM (
        SELECT friend_id, COUNT(*) AS cnt
        FROM friendship
        GROUP BY friend_id
    ) sub
    WHERE u.id = sub.friend_id;

    -- Users with zero followers also need is_celebrity = false (already default, but safe)
    UPDATE users
    SET followers_count = 0,
        is_celebrity = false
    WHERE id NOT IN (SELECT friend_id FROM friendship);
END;
$$ LANGUAGE plpgsql;

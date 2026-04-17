CREATE TABLE IF NOT EXISTS friendship (
    user_id uuid NOT NULL,
    friend_id uuid NOT NULL,
    created_at timestamp NOT NULL DEFAULT now(),

    CONSTRAINT pk_friendship PRIMARY KEY (user_id, friend_id),
    CONSTRAINT fk_friendship_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_friendship_friend FOREIGN KEY (friend_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT chk_not_self CHECK (user_id <> friend_id)
);

CREATE INDEX IF NOT EXISTS idx_friendship_friend_id ON friendship (friend_id);

CREATE TABLE IF NOT EXISTS post (
    post_id uuid PRIMARY KEY DEFAULT uuidv7(),
    author_id uuid NOT NULL,
    content text NOT NULL,
    created_at timestamp NOT NULL DEFAULT now(),

    CONSTRAINT fk_post_author FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_post_author_id ON post (author_id);
CREATE INDEX IF NOT EXISTS idx_post_created_at ON post (created_at DESC);
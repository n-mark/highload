CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username varchar(255) UNIQUE NOT NULL,
    email varchar(255) UNIQUE NOT NULL,
    password text NOT NULL,
    phone varchar(50)
);

CREATE TABLE IF NOT EXISTS profile (
    profile_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    userid uuid NOT NULL,
    name varchar(255),
    surname varchar(255),
    date_of_birth timestamp,
    gender varchar(20),
    city varchar(255),
    bio text,
    interests text,

    CONSTRAINT fk_user
        FOREIGN KEY(userid)
        REFERENCES users(id)
        ON DELETE CASCADE,

    CONSTRAINT uq_user UNIQUE (userid)
);
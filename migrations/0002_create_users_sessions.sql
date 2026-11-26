-- Users and sessions. Plural because these tables are ours; there is no auth
-- library whose names we have to keep.
--
-- Sessions carry a hash of the token, never the token itself: a database read
-- should not yield a usable credential. The cookie is the only place the raw
-- token exists.
CREATE TABLE users (
    id      bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sub     text NOT NULL UNIQUE,
    email   text NOT NULL,
    name    text NOT NULL,
    created timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash text NOT NULL UNIQUE,
    user_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created    timestamptz NOT NULL DEFAULT now(),
    expires    timestamptz NOT NULL,
    ip         text NOT NULL DEFAULT '',
    user_agent text NOT NULL DEFAULT ''
);

-- A cleanup job, if one exists, scans by expiry. The unique token_hash index
-- is the authentication lookup.
CREATE INDEX idx_sessions_expires ON sessions (expires);

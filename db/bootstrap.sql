-- Cluster-level bootstrap: the application role, and nothing else.
--
-- Roles are cluster-scoped and migrations are per-database, so this is not a
-- migration. It is run once per cluster, before the first migration, and it is
-- the one step applied outside the deployment pipeline (#5).
--
-- Idempotent: safe against a cluster that already has the role, the tables, or
-- both. Docker runs it from the initdb hook on an empty volume; everywhere else
-- it is `make db-bootstrap`, which is the same SQL rather than a second copy.
--
-- The password is a development default. Production sets it from a secret and
-- never from this file (#5).

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'godough_app') THEN
        CREATE ROLE godough_app LOGIN PASSWORD 'snippet_app'
            NOSUPERUSER NOCREATEDB NOCREATEROLE;
    END IF;
END
$$;

GRANT USAGE ON SCHEMA public TO godough_app;

-- For tables the owner creates later. This is why the bootstrap runs before the
-- first migration in a fresh cluster.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO godough_app;

-- For tables that already exist, so re-running is not a no-op that leaves a
-- half-granted database behind.
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO godough_app;

-- Cluster-level bootstrap: the application role, and nothing else.
--
-- Roles are cluster-scoped and migrations are per-database, so this is not a
-- migration. It is run once per cluster, before the first migration, and it is
-- the one step applied outside the deployment pipeline (#5).
--
-- Idempotent: safe against a cluster that already has the role, the tables, or
-- both. Docker runs it from the initdb hook on an empty volume; everywhere else
-- it is `make db-bootstrap` or `make rds-bootstrap`, which run this same SQL
-- rather than a second copy.
--
-- The password never lives in this file. It comes in as the psql variable
-- app_password, which `make db-bootstrap` leaves unset so the development
-- default applies, and which `make rds-bootstrap` fills from Secrets Manager
-- (#114). Nothing here is a secret; the value is bound at run time.

\if :{?app_password}
\else
\set app_password snippet_app
\endif

-- CREATE ROLE has no IF NOT EXISTS, and a DO block cannot see a psql variable
-- inside its dollar quotes. So the statement is generated and then executed,
-- which also means a re-run after a rotation puts the database back in line with
-- the secret instead of silently keeping the old password.
SELECT CASE
    WHEN EXISTS (SELECT FROM pg_roles WHERE rolname = 'godough_app')
        THEN format('ALTER ROLE godough_app PASSWORD %L', :'app_password')
    ELSE format('CREATE ROLE godough_app LOGIN PASSWORD %L NOSUPERUSER NOCREATEDB NOCREATEROLE', :'app_password')
END
\gexec

GRANT USAGE ON SCHEMA public TO godough_app;

-- For tables the owner creates later. This is why the bootstrap runs before the
-- first migration in a fresh cluster.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO godough_app;

-- For tables that already exist, so re-running is not a no-op that leaves a
-- half-granted database behind.
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO godough_app;

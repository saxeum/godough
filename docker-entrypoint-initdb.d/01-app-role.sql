-- Runs once, on an empty data volume (see docker-compose.yml).
-- godough (POSTGRES_USER) is the superuser and owner of the schema;
-- godough_app is what the server connects as, and owns nothing.

CREATE ROLE godough_app LOGIN PASSWORD 'snippet_app'
    NOSUPERUSER NOCREATEDB NOCREATEROLE;

-- Tables are created by godough in migrations, after this script runs, so
-- grant through default privileges instead of naming each table.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO godough_app;

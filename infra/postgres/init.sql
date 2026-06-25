-- Runs only on first container boot (POSTGRES_DB is already created by entrypoint).
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

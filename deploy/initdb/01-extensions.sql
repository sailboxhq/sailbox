-- Extensions installed at database initialization time (not in migrations).
-- These require PG server-level support and cannot be added at runtime
-- on standard PG builds.

-- pgcrypto: gen_random_uuid() — PG 13+ has it built-in, but kept for compatibility
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- pgmq and pg_cron require custom PG builds (tembo, supabase-postgres, etc).
-- Uncomment when deploying on a PG build that includes them:
-- CREATE EXTENSION IF NOT EXISTS pgmq;
-- CREATE EXTENSION IF NOT EXISTS pg_cron;

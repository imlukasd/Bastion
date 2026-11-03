-- Bastion Tickets baseline (P1-02 D8). Runs as tickets_owner, like every migration.

-- Only the owner uses schema public (it holds public.schema_migrations); each
-- module gets its own schema with explicit grants (TD §1.4, §8.3).
REVOKE ALL ON SCHEMA public FROM PUBLIC;

-- Functions the owner creates, SECURITY DEFINER ones included (TD §8.3), are
-- executable only by the roles a later migration grants EXECUTE to.
ALTER DEFAULT PRIVILEGES REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;

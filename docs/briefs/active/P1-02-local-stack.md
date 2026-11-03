# P1-02 — Local stack: Compose with two PostgreSQL databases and Redis; migrations

Executor: strong · Owner: Claude · Started: 2026-11-03 · Approved: 2026-11-03
Skip: G2 (reason: D11)

## Goal
The item's `done_when`, one clause per line:
1. `task up` starts PostgreSQL >= 17 with databases `bastion_pay` and `bastion_tickets` (`track_commit_timestamp = on`) and Redis >= 7.2.
2. `task migrate` applies the `db/pay` and `db/tickets` migrations.
3. Application roles `pay_app` and `tickets_app` exist.
4. An integration test connects to both databases and to Redis.

## Spec
- TD §1.3: "`timestamptz` everywhere; `track_commit_timestamp = on` in both databases (W3 detection)."
- TD §1.4: "Databases `bastion_pay` and `bastion_tickets`; one schema per module; statuses `UPPER_SNAKE`; ledger kinds `lower_snake`."
- TD §2 layout: "`db/{pay,tickets}/{migrations,queries}/`", "`deploy/  scripts/  test/{integration,…}/`"; `internal/platform/db` is "Package db opens PostgreSQL pools, runs transactions and maps SQLSTATEs to errors (TD §1.5)" (its doc.go).
- TD §8.3 Roles: "`pay_owner` (schemas, `SECURITY DEFINER` functions, migrations); `pay_app` (`SELECT`, `INSERT`; `UPDATE` only on status, counter and lease columns; no `UPDATE`/`DELETE` on ledger tables, …); `pay_auditor` (read-only); `pay_ops` (`pay_app` + ops functions, all logged)." TD §19: "Roles: `tickets_owner`, `tickets_app`, `tickets_auditor`, `tickets_ops`, with the same rules as Pay."
- TD §33: "`PAY_DB_URL`, `TIX_DB_URL`, `BASTION_REDIS_URL` | — | Required".
- TD §34.1: "Integration | testcontainers | Modules with real stores, triggers, indexes, Lua | Every commit".
- TD §35.1: "`task up` / `task down`, `task migrate`, `task test`, …".
- ARCH §12.1 Local topology: "`postgres` (two databases), `redis`, `edge` | 1 each | default"; app services, simulators, observability (`observability` profile), toxiproxy (`chaos` profile), k6 (`load` profile).
- ARCH §12.2: "Databases | One server, two databases | Separate HA clusters per company".
- AD-03: "PostgreSQL as each company's system of record" (deferrable constraints, partial unique indexes).
- AD-15: "pgx + sqlc + SQL migrations".
- ENGINEERING E23: "Migrations are forward-only; an applied migration is never edited." E29: "SQL behavior is tested against real PostgreSQL (testcontainers), never a mocked database." E39: "A new third-party dependency needs a reason in LOG.md, or an AD when it is architectural." E33: no `time.Sleep` for synchronization. Lint forbids `context.Background`/`TODO` outside `scripts/` and `cmd/` (tests use `t.Context()`), and gosec G101 flags hard-coded credentials.
- INV M3 Detect: "`has_table_privilege` for `pay_app` and `tickets_app` must be false for `UPDATE`/`DELETE` on ledger tables" (later items; here it only fixes the role names).

## Decisions
| # | Question | Decision | By | Why |
| --- | --- | --- | --- | --- |
| D1 | PostgreSQL image | `postgres:18.6-alpine3.24` (exact tag). | writer | Latest stable major (19 is rc1 on 2026-11-03); meets ">= 17"; 18 also has built-in `uuidv7()`. The 18 image keeps data under `/var/lib/postgresql/18/docker`, so the volume mounts `/var/lib/postgresql`. |
| D2 | Redis image | `redis:8.10.3-alpine` (exact tag). | writer | Latest stable; meets ">= 7.2". Redis 8 is AGPLv3/RSALv2/SSPLv1; used only as a local, unmodified server, so the licence has no effect here. |
| D3 | Migration tool (AD-15 names none) | A small in-house runner `internal/platform/db/migrate` on pgx (~150 lines, no new dependency); not goose (`pressly/goose`) or tern (`jackc/tern`). | owner (2026-11-03) | The runner records a SHA-256 per applied file and refuses an edited one, so E23 is enforced, not only reviewed; forward-only by construction; one engine for `task migrate` and tests. goose/tern bring up/down files and no checksum. |
| D4 | Where roles and databases are created | `deploy/postgres/init/10-bastion.sql`, run as superuser by the image's `docker-entrypoint-initdb.d` on first start, and by the test helper through the same file. Creates `pay_owner`, `pay_app`, `tickets_owner`, `tickets_app`, `bastion_pay` (owner `pay_owner`), `bastion_tickets` (owner `tickets_owner`). | writer | Clause 1 needs the databases after `task up` alone; one file for Compose and tests. Init runs only on an empty volume: a changed file needs `task down -- -v` (documented in AGENTS.md). |
| D5 | Which roles now | Owner and app roles only. `*_auditor` and `*_ops` are created by the items that first use them (P1-14 auditor; ops with `bastionctl`). | writer | Nothing uses them yet; their grants depend on tables that do not exist. |
| D6 | Database isolation | `REVOKE ALL ON DATABASE … FROM PUBLIC`; `CONNECT` only to the product's own roles. `pay_*` cannot connect to `bastion_tickets` and vice versa (SQLSTATE 42501, spiked). | writer | ARCH §7.1 / ARCH §12.2: one company's database per product; no transaction spans both. Cheap, and a test proves it. |
| D7 | Local credentials | Password = role name (`pay_app`/`pay_app`, superuser `postgres`/`postgres`); ports bound to `127.0.0.1` only (5432, 6379; both free on 2026-11-03). Credentials appear only in `deploy/`, `Taskfile.yml` and as `role:role` built at run time in `dbtest` (no literal, so gosec G101 stays quiet). | writer | Local-only stack; production secrets come from configuration (E36, ARCH §12.2). |
| D8 | First migration | `0001_baseline.sql` per product, run as the owner: `REVOKE ALL ON SCHEMA public FROM PUBLIC;` and `ALTER DEFAULT PRIVILEGES REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;` | writer | Gives clause 2 a real migration and sets the privilege convention P1-05 needs: functions (including `SECURITY DEFINER` ones, TD §8.3) are executable only where granted; app roles see only module schemas granted to them. |
| D9 | `track_commit_timestamp` | Server flag `-c track_commit_timestamp=on` in Compose `command` and in the test container. It is cluster-wide, so both databases have it. | writer | Needs a restart to change; a flag avoids a custom image. A test asserts it, and a test checks that Compose and `dbtest` use the same images and flag. |
| D10 | Integration test infrastructure | testcontainers-go v0.44.0 (`modules/postgres`, `modules/redis`) in `internal/platform/db/dbtest`; Go clients pgx v5.12.0 (AD-15) and go-redis v9.23.0. Integration tests skip under `-short` (so `task check` needs no Docker) and fail, not skip, when Docker is missing in a full run. | writer | E29 and TD §34.1. Spiked: container with init script up in ~4.5 s; govulncheck on pgx + go-redis + testcontainers: "No vulnerabilities found"; testcontainers adds ~50 indirect test-only modules. |
| D11 | Skip G2 | Tests come with the code in the `feat` commits; G5 is kept. | writer | Not critical, no money/state/concurrency path; the tests check infrastructure the steps build. G5 kept because later critical items build on these roles and the runner. |
| D12 | Where `task migrate` gets its URLs | Taskfile `env` with local defaults, overridable from the environment: `PAY_MIGRATE_URL`, `TIX_MIGRATE_URL` (owner roles). Not part of TD §33 (app configuration); tooling only. | writer | Migrations run as `*_owner` (TD §8.3), the app as `*_app` via `PAY_DB_URL`/`TIX_DB_URL`. |
| D13 | Verify path | `go test ./internal/platform/db/...` covers `migrate` and `dbtest`; unchanged. | writer | — |

## Files
Create:
- `deploy/compose.yaml`
- `deploy/postgres/init/10-bastion.sql`
- `db/pay/migrations/0001_baseline.sql`
- `db/tickets/migrations/0001_baseline.sql`
- `internal/platform/db/migrate/migrate.go`
- `internal/platform/db/migrate/migrate_test.go`
- `internal/platform/db/migrate/migrate_integration_test.go`
- `internal/platform/db/dbtest/dbtest.go`
- `internal/platform/db/dbtest/stack_test.go`
- `scripts/migrate/main.go`
Modify:
- `Taskfile.yml`: `up`, `down`, `migrate` implemented
- `go.mod`, `go.sum`: pgx, go-redis, testcontainers-go
- `AGENTS.md`: Commands (local URLs, `task down -- -v`, `dbtest` for integration tests)
Delete:
- `deploy/.gitkeep`, `db/pay/migrations/.gitkeep`, `db/tickets/migrations/.gitkeep`

## Contracts

**Compose** (`deploy/compose.yaml`, project `name: bastion`, default profile):
- `postgres`: image `postgres:18.6-alpine3.24`; `command: ["postgres", "-c", "track_commit_timestamp=on"]`; env `POSTGRES_PASSWORD: postgres`; ports `127.0.0.1:5432:5432`; volumes `pgdata:/var/lib/postgresql`, `./postgres/init:/docker-entrypoint-initdb.d:ro`; healthcheck `pg_isready -h 127.0.0.1 -U postgres -d bastion_tickets` (TCP, so healthy only after init finished).
- `redis`: image `redis:8.10.3-alpine`; ports `127.0.0.1:6379:6379`; healthcheck `redis-cli ping`. No volume.
- Named volume `pgdata`. No other services (they arrive with their items).

**Init SQL** (`deploy/postgres/init/10-bastion.sql`, superuser, runs once):
```sql
CREATE ROLE pay_owner     LOGIN PASSWORD 'pay_owner';
CREATE ROLE pay_app       LOGIN PASSWORD 'pay_app';
CREATE ROLE tickets_owner LOGIN PASSWORD 'tickets_owner';
CREATE ROLE tickets_app   LOGIN PASSWORD 'tickets_app';
CREATE DATABASE bastion_pay     OWNER pay_owner;
CREATE DATABASE bastion_tickets OWNER tickets_owner;
REVOKE ALL ON DATABASE bastion_pay, bastion_tickets FROM PUBLIC;
GRANT CONNECT ON DATABASE bastion_pay     TO pay_app;
GRANT CONNECT ON DATABASE bastion_tickets TO tickets_app;
```
All roles `NOSUPERUSER NOCREATEDB NOCREATEROLE` (the defaults; tests assert them).

**Migrations**: files `db/<product>/migrations/NNNN_<slug>.sql` (4 digits, slug `[a-z0-9_]+`), run as the product's owner role. `0001_baseline.sql` = D8 (both products, identical statements, a header comment naming the product).

**Package `internal/platform/db/migrate`** (doc: forward-only SQL migrations, E23, AD-15):
```go
type Migration struct {
    Version  int      // from the NNNN prefix
    Name     string   // file name
    SQL      string
    Checksum [32]byte // SHA-256 of the file bytes
}

var (
    ErrBadName          = errors.New("migrate: file name is not NNNN_slug.sql")
    ErrDuplicateVersion = errors.New("migrate: two files share a version")
    ErrEdited           = errors.New("migrate: an applied migration was edited")
    ErrUnknownApplied   = errors.New("migrate: an applied migration has no file")
    ErrOutOfOrder       = errors.New("migrate: a pending migration is older than the latest applied one")
)

// Load reads every *.sql file at the root of fsys, sorted by version. Other files are ignored.
func Load(fsys fs.FS) ([]Migration, error)

// Up applies the pending migrations in version order and returns the versions it applied (empty when up to date).
func Up(ctx context.Context, conn *pgx.Conn, ms []Migration) ([]int, error)
```
`Up`: takes `pg_advisory_lock(7_405_001)` on `conn` for its duration (released by `pg_advisory_unlock` before returning); `CREATE TABLE IF NOT EXISTS public.schema_migrations (version integer PRIMARY KEY, name text NOT NULL, checksum bytea NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`; checks every applied row before applying anything (`ErrUnknownApplied`, `ErrEdited`, `ErrOutOfOrder`, wrapped with the version: `fmt.Errorf("%w: %04d", err, v)`); then each pending migration in its own transaction: `Exec(SQL)` (no arguments, so the simple protocol runs several statements), `INSERT` its row, commit. A failing migration rolls back entirely and `Up` returns `fmt.Errorf("migrate: %04d: %w", v, err)` with the versions applied before it.

**Command `scripts/migrate`**: `go run ./scripts/migrate -url <owner URL> -dir <migrations dir>`. Prints `migrate <dir>: applied 0001 0002` or `migrate <dir>: up to date`; exit 1 with the error on stderr otherwise.

**Taskfile**:
- `up`: `docker compose -f deploy/compose.yaml up -d --wait` (returns when both services are healthy).
- `down`: `docker compose -f deploy/compose.yaml down {{.CLI_ARGS}}` (`task down -- -v` also drops the data volume).
- `migrate`: runs `scripts/migrate` for `db/pay/migrations` with `$PAY_MIGRATE_URL`, then `db/tickets/migrations` with `$TIX_MIGRATE_URL`; defaults `postgres://pay_owner:pay_owner@localhost:5432/bastion_pay?sslmode=disable` and `postgres://tickets_owner:tickets_owner@localhost:5432/bastion_tickets?sslmode=disable`.

**Package `internal/platform/db/dbtest`** (doc: real PostgreSQL and Redis for integration tests, E29):
```go
const (
    PostgresImage = "postgres:18.6-alpine3.24"
    RedisImage    = "redis:8.10.3-alpine"
)

// Postgres is a running test server with both databases and their roles.
type Postgres struct {
    SuperuserURL    string // postgres, database postgres
    PayOwnerURL     string // pay_owner → bastion_pay
    PayAppURL       string // pay_app → bastion_pay
    TicketsOwnerURL string // tickets_owner → bastion_tickets
    TicketsAppURL   string // tickets_app → bastion_tickets
}

// URL returns a URL for role on database (role's password is its name).
func (p *Postgres) URL(role, database string) string

// StartPostgres starts PostgresImage with -c track_commit_timestamp=on and
// deploy/postgres/init/10-bastion.sql, then applies db/pay and db/tickets migrations as the owners.
func StartPostgres(t testing.TB) *Postgres
// StartPostgresUnmigrated is StartPostgres without the migrations.
func StartPostgresUnmigrated(t testing.TB) *Postgres
// StartRedis starts RedisImage and returns its redis:// URL.
func StartRedis(t testing.TB) string
```
Each `Start*`: `t.Skip` when `testing.Short()`; `t.Fatal` on any error (Docker missing included); container terminated in `t.Cleanup` with `context.WithoutCancel(t.Context())`. Repository files are found from the module root (directory of `go.mod`, located by walking up from this source file via `runtime.Caller`). Ryuk stays enabled (testcontainers default).

## Steps
1. Compose, init SQL, Taskfile `up`/`down`; delete `deploy/.gitkeep`. Verify: `task up` → exit 0; `docker compose -f deploy/compose.yaml exec -T postgres psql -U postgres -Atc "select current_setting('server_version_num'), current_setting('track_commit_timestamp'), (select string_agg(datname, ',' order by datname) from pg_database where datname like 'bastion_%'), (select string_agg(rolname, ',' order by rolname) from pg_roles where rolname ~ '^(pay|tickets)_')"` → `180006|on|bastion_pay,bastion_tickets|pay_app,pay_owner,tickets_app,tickets_owner`; `docker compose -f deploy/compose.yaml exec -T redis redis-cli INFO server | grep redis_version` → `redis_version:8.10.3`.
2. `migrate` package with `migrate_test.go` (Load) and `migrate_integration_test.go` (Up); go.mod adds pgx and testcontainers. Verify: `go test ./internal/platform/db/migrate/...` → ok; with `-short` the integration tests skip.
3. Baseline migrations (delete their `.gitkeep`), `scripts/migrate`, Taskfile `migrate`. Verify: `task migrate` → `migrate db/pay/migrations: applied 0001` and the same for tickets; again → `up to date` twice.
4. `dbtest` and `stack_test.go`; go.mod adds go-redis. Verify: `go test ./internal/platform/db/...` → ok.
5. AGENTS.md Commands; LOG.md dependency reasons (E39) at handoff. Verify: `task check` → `check OK`; `task test` → all packages ok.

## Test map
| Clause | Test (file › name) | Asserts |
| --- | --- | --- |
| 1 | verify step 1 (Compose) | Version 18.6, `track_commit_timestamp` on, both databases, Redis 8.10.3, from the running Compose stack |
| 1 | `dbtest/stack_test.go › TestStack_PostgresSettings` | In each database (as its app role): `server_version_num >= 170000`, `track_commit_timestamp = on`, `current_database()` is the expected name |
| 1 | `dbtest/stack_test.go › TestStack_RedisVersion` | `INFO server` `redis_version` >= 7.2 |
| 1 | `dbtest/stack_test.go › TestStack_ImagesMatchCompose` | `deploy/compose.yaml` contains `PostgresImage`, `RedisImage` and `track_commit_timestamp=on` (unit; runs under `-short`) |
| 2 | verify step 3 (`task migrate` twice) | Applied once, then up to date |
| 2 | `dbtest/stack_test.go › TestStack_MigrationsApplied` | `public.schema_migrations` in each database holds version 1 with the SHA-256 of `0001_baseline.sql`; `has_schema_privilege('pay_app','public','USAGE')` is false (D8); a function created by the owner is not executable by the app role (D8) |
| 2 | `migrate/migrate_test.go › TestLoad_*` | Sorted by version; `ErrBadName` (`1_x.sql`, `0001-x.sql`, `0001_X.sql`); `ErrDuplicateVersion`; non-`.sql` files ignored; checksum = SHA-256 of bytes |
| 2 | `migrate/migrate_integration_test.go › TestUp_AppliesInOrderOnce` | First run applies 1,2 and their effects exist; second run returns empty and changes nothing |
| 2 | `… › TestUp_RefusesEditedMigration` | Changed SQL of an applied file → `ErrEdited`, nothing applied |
| 2 | `… › TestUp_RefusesUnknownApplied` | Applied row without file → `ErrUnknownApplied` |
| 2 | `… › TestUp_RefusesOutOfOrder` | 1 and 3 applied, 2 added → `ErrOutOfOrder` |
| 2 | `… › TestUp_FailedMigrationRollsBack` | Migration 2 = valid statement then an invalid one → error naming 0002; 1 recorded, 2 not recorded, its first statement's effect absent |
| 3 | `dbtest/stack_test.go › TestStack_Roles` | `pay_app` and `tickets_app` exist, can log in to their own database, are not superuser / createdb / createrole; `pay_app` → `bastion_tickets` and `tickets_app` → `bastion_pay` fail with 42501 (D6) |
| 4 | `dbtest/stack_test.go › TestStack_ConnectsToBothDatabasesAndRedis` | `pay_app` on `bastion_pay` and `tickets_app` on `bastion_tickets` run `select 1`; Redis `SET`/`GET` round-trips a value |

The `migrate` integration tests start one `StartPostgresUnmigrated` server per test function and use a fresh database per test (`CREATE DATABASE … OWNER pay_owner` as superuser), with migrations from an in-memory `fstest.MapFS`.

## Out of scope
- pgx pool, transaction helper with retries, SQLSTATE mapping, config loading of `PAY_DB_URL`/`TIX_DB_URL`/`BASTION_REDIS_URL` (P1-03).
- `redisx` key namespaces (the first item that uses Redis: outbox P1-12 / waiting room).
- Module schemas, table grants, `*_auditor` and `*_ops` roles (P1-05 onward, P1-14).
- sqlc configuration and `db/*/queries` (first item with queries, P1-05/P1-06).
- App services, `edge`, simulators, observability and chaos profiles in Compose (their items; P2-04 for chaos).
- CI (deferred by the owner).

## Stop rules
Stop and report instead of choosing when:
- a decision is needed that this brief does not make;
- a file outside Files must change, a dependency not named here is needed, or a test seems wrong;
- the code or the spec contradicts this brief;
- an image tag in D1/D2 cannot be pulled or behaves differently from the spike (for example the 18 image's data path);
- a linter flags the contracts as written (gosec G101, gochecknoglobals on the sentinel errors, forbidigo) and the fix would change a contract;
- the runner needs a migration that cannot run in a transaction.

## Progress
- 2026-11-03 — brief drafted. Spikes (outside the repo): testcontainers v0.44.0 with `postgres:16-alpine` + the init SQL of D4: roles and databases created, `track_commit_timestamp` on, cross-database connect denied (42501), app role denied `CREATE` on `public` (42501), multi-statement `Exec` as owner works; `redis:7-alpine` reachable. govulncheck on pgx v5.12.0 + go-redis v9.23.0 + testcontainers v0.44.0: no vulnerabilities. Image tags of D1/D2 read from Docker Hub; pulling them failed (see Surprises).

## Surprises
- 2026-11-03: Docker Desktop (WSL2) cannot pull images: every blob download through its proxy `http.docker.internal:3128` ends in `EOF` (postgres:18-alpine, redis:8-alpine, testcontainers/ryuk). Locally cached images run. Step 1 needs the owner to fix image pulls first.

## Outcome
When done: the item report (docs/REVIEW.md › Item report).

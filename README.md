# Spill — Social Media Platform API

A fully‑featured social media REST API built with **Go**, **PostgreSQL 16**, and a clean **MVC** architecture. Passwordless authentication, full‑text search, presigned asset uploads, and trigger‑maintained counters — no external cache required.

---

## Quick Start

### Prerequisites

| Tool | Version |
|---|---|
| Go | ≥ 1.27 |
| Docker | ≥ 24 (for testcontainers and local Postgres) |
| (optional) Bruno | ≥ 1.x (API client, see `docs/api/`) |

### 1. Clone & configure

```bash
git clone https://github.com/TheAmgadX/Spill-Social-Media-Platform.git
cd Spill-Social-Media-Platform
cp .env.example .env
# Edit .env — fill in real values for DATABASE_URL, JWT_SECRET, etc.
```

### 2. Run the database migration

Apply the schema to your PostgreSQL database (Supabase or local):

```bash
# Using psql against the migration connection:
psql "$MIGRATION_DATABASE_URL" -f migrations/000001_init.up.sql
```

### 3. (Optional) Seed sample data

```bash
go run cmd/seed/main.go
```

### 4. Start the server

```bash
go run cmd/main.go
# → Listening on :8080
```

### 5. Run the test suite

Tests use [testcontainers‑go](https://github.com/testcontainers/testcontainers-go) to spin up an isolated Postgres 16 container automatically — **no manual database setup needed**.

```bash
go test -race -count=1 ./...
```

Expected output — 7 packages pass, 0 failures:

```
ok   internal/auth         1.0s
ok   internal/controllers  12.6s
ok   internal/middleware   1.0s
ok   internal/models       9.2s
ok   internal/routes       13.1s
ok   internal/services     12.5s
ok   internal/testhelper   12.4s
```

### 6. Run verification & performance benchmarks

```bash
go run cmd/verify/main.go
```

This runs the migration round‑trip (DOWN → UP), security checks, seeds benchmark data, and saves `EXPLAIN ANALYZE` outputs to `docs/perf/`.

---

## Environment Variables

See [`.env.example`](.env.example) for every variable with placeholder values. Key groups:

| Group | Variables |
|---|---|
| **App** | `APP_ENV`, `HTTP_PORT` |
| **Database** | `DATABASE_URL` (runtime, pooler port 6543), `MIGRATION_DATABASE_URL` (migrations, port 5432) |
| **Supabase** | `SUPABASE_URL`, `SUPABASE_SERVICE_ROLE_KEY` |
| **Storage** | `STORAGE_BUCKET`, `STORAGE_PUBLIC_BASE_URL`, `UPLOAD_MAX_BYTES`, `UPLOAD_PENDING_TTL` |
| **Auth/JWT** | `JWT_SECRET`, `JWT_ISSUER`, `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`, `SIGNUP_TOKEN_TTL` |
| **OTP** | `OTP_HMAC_KEY`, `OTP_TTL`, `OTP_MAX_ATTEMPTS`, `OTP_RESEND_COOLDOWN` |
| **Mail** | `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`, `MAILER` |

---

## Architecture

The system follows an **MVC** pattern with PostgreSQL as the single data store. All search, counters, and integrity rules live in the database via generated columns, GIN indexes, and triggers.

```
Client → [Recover → Logger → Auth Middleware → Router] → Controller → Service → Model/Repo → PostgreSQL
                                                           ↓
                                                        View (JSON DTO)
```

See the full architecture diagrams (SVG/PNG) in [`assets/diagrams/`](assets/diagrams/):

| Diagram | File |
|---|---|
| Requirements mindmap | `01_requirements_mindmap` |
| High‑level architecture | `02_high_level_architecture` |
| Request lifecycle | `03_request_lifecycle` |
| API gateway flow | `04_api_gateway_flow` |
| Passwordless auth flow | `05_auth_passwordless_flow` |
| Post creation flow | `06_posts_creation_flow` |
| Interactions (like) flow | `07_interactions_like_flow` |
| Asset presigned upload | `08_assets_presigned_flow` |
| Search inverted index | `09_search_inverted_index` |
| Database ER diagram | `10_database_er_diagram` |
| Data interaction overview | `11_data_interaction_overview` |

The complete system design document is at [`docs/system-design.md`](docs/system-design.md).

---

## Folder Layout

```
/cmd/main.go                 # wiring: config, db, router
/cmd/seed/main.go            # seed script for demo data
/cmd/verify/main.go          # verification & perf benchmarks
/internal
  /auth                      # JWT generation/parsing, OTP hashing, age calc
  /config                    # env loading
  /controllers               # HTTP handlers (parse → validate → service → respond)
  /db                        # pgxpool connection helper
  /mailer                    # send OTP emails (SMTP / mailpit)
  /middleware                # logger, recover, auth (RequireAuth, OptionalAuth, RequireRole, MaxBytes)
  /models                    # domain structs + repository (SQL queries via pgx)
  /routes                    # route table → controllers (gorilla/mux router)
  /services                  # business logic (auth, posts, feed, search, assets, users, comments)
  /storage                   # object storage client (presign, head, delete)
  /testhelper                # testcontainers Postgres setup
  /views                     # response DTOs / mappers (PublicUserResponse, PrivateUserResponse, etc.)
/migrations
  000001_init.up.sql         # full schema: tables, indexes, triggers, RLS, constraints
  000001_init.down.sql       # clean rollback
/docs
  system-design.md           # full system design document
  auth.md                    # auth flow details
  openapi.yaml               # OpenAPI 3.0 specification
  traceability.md            # requirement → endpoint → test traceability matrix
  security-checklist.md      # security verification with evidence
  perf/                      # EXPLAIN ANALYZE outputs + README
  api/                       # Bruno API collection
/assets
  diagrams/                  # exported Mermaid diagrams (SVG + PNG)
/scripts
  export_diagrams.py         # script to re-export all Mermaid diagrams
.env.example                 # all env vars with placeholder values
```

---

## API Surface

| Area | Endpoints |
|---|---|
| **Auth** | `POST /auth/request-code`, `/auth/verify`, `/auth/signup`, `/auth/refresh`, `/auth/logout` |
| **Users** | `GET /users/{username}`, `GET/PATCH /me`, `PUT/DELETE /users/{id}/follow`, `GET /users/{id}/followers`, `/following` |
| **Posts** | `POST /posts`, `GET/PATCH/DELETE /posts/{id}`, `GET /feed`, `GET /users/{id}/posts` |
| **Interactions** | `PUT/DELETE /posts/{id}/like`, `POST /posts/{id}/share`, `GET/POST /posts/{id}/comments`, `DELETE /comments/{id}` |
| **Assets** | `POST /assets`, `POST /assets/{id}/complete` |
| **Search** | `GET /search?q=&type=users\|posts\|comments` |

All routes are also mounted under `/api/v1/` prefix.

Full OpenAPI spec: [`docs/openapi.yaml`](docs/openapi.yaml)

---

## Documentation

| Document | Description |
|---|---|
| [`docs/system-design.md`](docs/system-design.md) | Complete system design with Mermaid diagrams |
| [`docs/traceability.md`](docs/traceability.md) | Requirement → code → test mapping (27/27 met) |
| [`docs/security-checklist.md`](docs/security-checklist.md) | 8-point security audit with evidence |
| [`docs/perf/README.md`](docs/perf/README.md) | Query performance analysis with EXPLAIN ANALYZE |
| [`docs/openapi.yaml`](docs/openapi.yaml) | OpenAPI 3.0 specification |
| [`docs/auth.md`](docs/auth.md) | Authentication flow documentation |
| [`docs/api/`](docs/api/) | Bruno API collection with chained auth flow |
| [`REMAINING_TASKS.md`](REMAINING_TASKS.md) | Full-stack class project roadmap and remaining tasks |

---

## License

See repository for license details.

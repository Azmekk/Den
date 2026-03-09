# Den — Development Guide

## Database Migrations

Uses `migrate` CLI installed locally.

**Run all up migrations:**

```sh
migrate -path src/db/migrations -database "postgres://den:changeme@localhost:5440/den?sslmode=disable" up
```

**Roll back one migration:**

```sh
migrate -path src/db/migrations -database "postgres://den:changeme@localhost:5440/den?sslmode=disable" down 1
```

**Roll back all migrations:**

```sh
migrate -path src/db/migrations -database "postgres://den:changeme@localhost:5440/den?sslmode=disable" down -all
```

## sqlc Workflow

Config at project root: `sqlc.yaml`

- Queries in `src/db/queries/`
- Schema from `src/db/migrations/`
- Generated code in `src/internal/db/`

```sh
sqlc generate   # run from project root
```

## Build

**Frontend:**

```sh
cd src/web && bun run build
```

**Backend:**

```sh
cd src && go build -o ../bin/den .
```

## Dev Server

```sh
cd src && go run .
```

## Frontend Dev

- Bun as package manager
- `cd src/web && bun install` for deps
- `cd src/web && bun run dev` for dev server (Vite, proxies /api to :8080)
- Svelte 5 with runes, `.svelte.ts` store files
- Tailwind v4 with `@theme inline` in app.css (no tailwind.config.js)
- bits-ui for headless components, `cn()` utility in `src/web/src/lib/utils.ts`

## Docker

```sh
docker compose up -d postgres   # Start Postgres (port 5440)
docker compose up -d            # Start all services
docker compose down             # Stop everything
```

## Project Structure

- All source code lives under `src/` (Go module root: `github.com/Azmekk/den`)
- Go entrypoint: `src/main.go`
- Go packages: `src/internal/` (service, handler, middleware, router, httputil, ws, db)
- SvelteKit frontend: `src/web/`
- DB migrations: `src/db/migrations/` (through 000007)
- sqlc queries: `src/db/queries/`
- Infrastructure configs at project root

**Handlers:** admin, auth, channel, config, emote, message, user
**Services:** admin, auth, bucket, channel, emote, message, user, helpers
**WebSocket:** client, handler, hub
**Frontend stores:** auth, channels, config, emotes, messages, presence, typing, unread, users, websocket
**Frontend routes:** `/` (main chat), `/login`, `/register`, `/admin`

## Architecture Caveats

- Chi router: use `chi.URLParam(r, "id")` not `r.PathValue("id")`
- WebSocket auth via query param: `GET /api/ws?token=<JWT>`
- Message pagination: cursor-based with `before_time` + `before_id`
- Hub uses channel-based select loop (no mutexes)
- User colors generated client-side from username hash (no DB column)
- Admin settings (open_registration, instance_name) are in-memory only
- Emote tokens in messages: `<emote:uuid>`, mention tokens: `<mention:uuid>`
- S3 bucket storage is optional — upload features hidden when BUCKET\_\* env vars not set
- Postgres on port 5440 (5432-5434 occupied on host)
- `MSYS_NO_PATHCONV=1` needed for Docker volume mount commands in Git Bash

## After Every Run

After completing a run, always update **both** `docs/plan.md` **and** `docs/progress.md`:
- `docs/plan.md` — mark the completed run as done and update the next run section
- `docs/progress.md` — record what was completed, any deviations from the plan, and the exact starting point for the next run

**Never update one without the other.** Both files must stay in sync.

**Runs vs Deviations:** Only count work as a numbered run if the user explicitly says so. Ad-hoc fixes, improvements, or changes done outside a formal run should be logged as a "Deviation" in progress.md — not as a new run number. The run counter only advances when the user specifies it.

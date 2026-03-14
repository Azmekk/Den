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
- Tailwind v4 does not add `cursor: pointer` to buttons by default; a global rule in `app.css` handles this. For non-button clickable elements (e.g. `<div onclick>`), add the `cursor-pointer` Tailwind class.

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


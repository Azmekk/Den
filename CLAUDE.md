# Den — Development Guide

## Code Quality Standards

This project values **code readability**, **maintainability**, **best practices**, and **clean code**. All agents must:

- Write clear, well-structured code that is easy to read and understand
- Use full, descriptive variable and function names — no single-letter variables (e.g. `index` not `i`, `value` not `v`, `error` not `e`)
- Prefer proper, maintainable solutions over quick hacks or workarounds
- Proactively suggest fixes for existing code that is badly written, convoluted, or hard to follow
- Never write hackfix or band-aid solutions unless the user specifically authorizes or requests one

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
- Go packages: `src/internal/` (service, handler, middleware, router, httputil, ws, db, voice)
- SvelteKit frontend: `src/web/`
- DB migrations: `src/db/migrations/` (through 000015)
- sqlc queries: `src/db/queries/`
- Infrastructure configs at project root

**Handlers:** admin, auth, channel, config, emote, message, user
**Services:** admin, auth, bucket, channel, emote, message, user, helpers
**WebSocket:** client, handler, hub
**Frontend stores:** auth, channels, config, emotes, messages, presence, typing, unread, users, websocket
**Frontend routes:** `/` (main chat), `/login`, `/register`, `/admin`

## Authentication

- **Custom in-house auth** — no external auth providers
- Backend issues HS256 JWTs (15-min access tokens) with DB-backed refresh tokens (30-day)
- Registration: email/password with bcrypt hashing, optional email verification (when SMTP configured)
- TOTP-based 2FA with recovery codes (Google Authenticator compatible)
- SMTP-based password reset (optional — features disabled when SMTP not configured)
- Username validation: `a-zA-Z0-9._-`, 1-32 characters
- Tokens sent in `Authorization: Bearer <token>` header
- User cache (sync.Map with UUID keys, 2-min TTL) avoids DB lookup on every request
- **Banning**: Admin sets `banned` flag, revokes all refresh tokens, kicks from WebSocket
- Required env var: `JWT_SECRET`
- Optional env vars: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`, `FRONTEND_URL`

## Architecture Caveats

- Chi router: use `chi.URLParam(r, "id")` not `r.PathValue("id")`
- WebSocket auth via first message containing Supabase JWT
- Message pagination: cursor-based with `before_time` + `before_id`
- Hub uses channel-based select loop (no mutexes)
- User colors stored in DB `color` column, fallback to client-side hash
- Admin settings (open_registration, instance_name) are in-memory only
- Emote tokens in messages: `<emote:uuid>`, mention tokens: `<mention:uuid>`
- S3 bucket storage is optional — upload features hidden when BUCKET\_\* env vars not set
- Postgres on port 5440 (5432-5434 occupied on host)
- `MSYS_NO_PATHCONV=1` needed for Docker volume mount commands in Git Bash


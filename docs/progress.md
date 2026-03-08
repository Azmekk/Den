# Den — Progress

> This file is updated by Claude at the end of every run. Always paste both `docs/plan.md` and `docs/progress.md` at the start of a new run.

---

## Status

**Current run:** Complete
**Last completed run:** Run 5 — Main Chat UI
**Next run:** Run 6

---

## Completed Runs

### Run 1 — Skeleton & Database
- Scaffolded full project structure under `src/`
- Created `.gitignore`, `.env.example`, `docker-compose.yml`, `Dockerfile`, `nginx.conf`, `livekit.yaml`, `sqlc.yaml`
- Created 4 migration pairs (users, channels, dm_pairs, messages + pinned_messages view + GIN index)
- Go module with `lib/pq` dependency, embed.go, minimal main.go, package placeholders
- SvelteKit scaffolded with adapter-static, SPA mode layout
- All migrations verified up and down
- `go build ./...` and `bun run build` both pass

### Run 2 — Auth (Backend Only)
- Added `golang-jwt/jwt/v5`, `golang.org/x/crypto`, `google/uuid` dependencies
- Created migration 000005 for `refresh_tokens` table with indexes
- Extended `users.sql` queries: GetUserByUsername, CreateUser, CountUsers, UpdateUserPassword, SetUserAdmin
- Added `refresh_tokens.sql` queries: full CRUD + cleanup
- Ran `sqlc generate` → `src/internal/db/` generated code
- Implemented `src/internal/auth/` package: service, middleware, handlers, helpers
- Restructured `main.go` with explicit `http.NewServeMux()` and method-pattern routes
- All 6 endpoints verified with curl: register, login, refresh, logout, me, change-password
- First user auto-admin, JWT access tokens (5 min), refresh token rotation (7 day), HttpOnly cookies

### Run 3 — Channels, WebSocket & Messages (Backend Only)
- Added `gorilla/websocket` dependency
- Created `src/internal/httputil/` with shared HTTP helpers (WriteJSON, WriteError, DecodeJSON)
- Created `src/db/queries/channels.sql` and `src/db/queries/messages.sql`
- Ran `sqlc generate` → generated `channels.sql.go` and `messages.sql.go`
- Implemented `src/internal/channel/` package: Service with CRUD, HTTP handlers
- Implemented `src/internal/message/` package: Service with SendMessage, EditMessage, DeleteMessage, GetHistory
- Implemented `src/internal/ws/` package: Hub, Client, ServeWS handler
- Wired all routes in main.go
- `go build ./src/cmd/server` passes clean

### Run 4 — SvelteKit Frontend Scaffold & Auth UI
- Installed Tailwind CSS v4 (`tailwindcss`, `@tailwindcss/vite`) with Vite plugin
- Installed `clsx`, `tailwind-merge`, `bits-ui` for shadcn-svelte foundation
- Created dark theme CSS variables in `src/web/src/app.css` (custom palette with blue-purple primary accent)
- Created `cn()` utility in `src/web/src/lib/utils.ts`
- Created auth store (`src/web/src/lib/stores/auth.svelte.ts`) with reactive Svelte 5 runes: login, register, logout, refresh, init
- Created API helper (`src/web/src/lib/api.ts`) with automatic JWT injection and transparent token refresh on 401
- Created login page (`/login`) and register page (`/register`) with form validation and error display
- Root layout initializes auth state via refresh token on mount; shows loading state until ready
- Dashboard (`/`) shows sidebar with user info, logout button, and empty channel list placeholder
- Route guard on `/` redirects unauthenticated users to `/login`; auth pages redirect logged-in users to `/`
- Vite dev proxy configured to forward `/api` to `http://localhost:8080`
- `bun run build` and `cd src && go build .` both pass clean

### Run 5 — Main Chat UI
- Added `ListUsers` sqlc query and generated `users.sql.go`
- Created `UserService` and `UserHandler` with `GET /api/users` endpoint
- Added presence tracking to WS Hub: `onlineUsers` map, `presence_initial` on connect, `presence_update` on first connect/last disconnect
- Added `BroadcastExclude` to Hub for typing indicators (broadcasts to channel subscribers except sender)
- Added `typing_start` message handler in WS client
- Created 6 frontend stores: `websocket` (auto-reconnect with exponential backoff), `channels`, `messages` (with cursor pagination), `presence`, `typing` (3s auto-clear, 2s send throttle), `users`
- Created shared TypeScript types (`ChannelInfo`, `MessageInfo`, `UserInfo`)
- Created `ChannelSidebar` component: channel list sorted by position, user panel with logout
- Created `MessageArea` component: channel header, scrollable message list with auto-scroll-to-bottom, load-older-on-scroll-top with scroll position preservation, typing indicator, textarea input (Enter to send, Shift+Enter for newline)
- Created `MemberList` component: online/offline sections with hash-based colored avatars and presence dots
- Rewrote `+page.svelte` with three-column layout, WS lifecycle management, event listener wiring, auto-select first channel
- Updated Vite proxy config for WebSocket upgrade support
- `go build` and `bun run build` both pass clean

---

## Run Log

### Run 1 (2026-03-07)
- All files created per plan
- Postgres exposed on port 5440 (5432-5434 were occupied by existing instances)
- Replaced Makefile/gofer with CLAUDE.md documenting raw commands (Windows path issues with both Make and gofer task runners)
- SvelteKit 5 with Vite 7, adapter-static 3

### Run 2 (2026-03-07)
- All files created per plan
- Access token expiry set to 5 minutes (changed from planned 15 min for tighter security)
- `OPEN_REGISTRATION` env var defaults to true; set to `false` to close registration
- `JWT_SECRET` env var with dev fallback and warning log

### Run 3 — Channels, WebSocket & Messages (Backend Only)
- Added `gorilla/websocket` dependency
- Created `src/internal/httputil/` with shared HTTP helpers (WriteJSON, WriteError, DecodeJSON)
- Channel CRUD requires admin for create/update/delete; list/get requires auth
- WebSocket connects via `GET /api/ws?token=<JWT>` (query param auth since browsers can't set WS headers)
- Message cursor pagination uses `before_time` + `before_id` query params (both required together)
- Hub uses channel-based select loop for all operations — no mutexes needed
- sqlc named params used for GetMessagesByChannel to get proper Go types

### Run 4 (2026-03-08)
- Tailwind CSS v4 with `@tailwindcss/vite` plugin (no PostCSS config needed)
- Dark-only theme — no light mode toggle (per plan: single dark theme)
- Primary color set to `231 77% 60%` (blue-purple, similar to Discord's blurple)
- Auth store uses Svelte 5 runes (`$state`) via `.svelte.ts` file convention
- API helper does transparent 401 → refresh → retry flow
- No shadcn-svelte CLI used; bits-ui installed manually, `cn()` utility created by hand
- Vite proxy avoids CORS issues during development

### Chi Router + Layered Architecture Refactor (2026-03-08)
- Switched from `http.ServeMux` to `chi` router for cleaner route groups and middleware chaining
- Separated HTTP handlers from business logic into a layered architecture:
  - `src/internal/service/` — pure business logic (AuthService, ChannelService, MessageService)
  - `src/internal/handler/` — HTTP handlers (AuthHandler, ChannelHandler, MessageHandler)
  - `src/internal/middleware/auth.go` — RequireAuth (chi middleware signature), RequireAdmin, context accessors
  - `src/internal/router/router.go` — chi router with route groups (public, authenticated, admin)
- Consolidated duplicate `isUniqueViolation` helper (was in both auth and channel) into `service/helpers.go`
- Consolidated cookie helpers from `auth/helpers.go` into `httputil/httputil.go` (SetRefreshTokenCookie, ClearRefreshTokenCookie)
- Updated `ws/handler.go` to import `service.AuthService` instead of `auth.Service`
- Slimmed `main.go` down to config loading + service wiring only
- Moved Go module root to `src/` — `go.mod` and `main.go` live in `src/`, eliminating `cmd/server/` nesting
- Import paths shortened from `den/src/internal/...` to `den/internal/...`
- `embed.go` changed from `package src` to `package main` (same package as `main.go`)
- Deleted old packages: `internal/auth/`, `internal/channel/`, `internal/message/`
- Deleted placeholder packages: `internal/admin/`, `internal/dm/`, `internal/embed/`, `internal/voice/`
- Added chi standard middleware: RealIP, RequestID, Logger, Recoverer, Compress, Heartbeat(`/healthz`)
- Build output goes to `bin/` at project root (`cd src && go build -o ../bin/den .`)
- Added `.dockerignore` for build context efficiency
- `go build`, `go vet` both pass clean
- No changes to API contracts, frontend, or database layer

### Run 5 (2026-03-08)
- No `color` column in users table — plan referenced it but schema doesn't have it. Colors generated client-side via username hash instead.
- No virtual list — simple scrollable div with load-more-on-scroll-up is sufficient for current scale
- No optimistic message sending — WS broadcast used as single source of truth
- Presence tracked via WS only — no REST endpoint for online users; `presence_initial` on connect provides initial state
- Svelte 5 reactivity with Set/Map requires creating new instances on mutation (reassignment pattern)
- Hub refactored to use `removeClient` helper to DRY up cleanup logic across register/unregister/broadcast cases

---

## Known Deviations from Plan

- **No Makefile or gofer.json** — Both had Windows/MSYS path translation issues with Docker volume mounts. Commands documented in CLAUDE.md instead.
- **Postgres port 5440** instead of 5432 — ports 5432-5434 were already in use on host.
- **5-minute access tokens** instead of 15 — tighter security window, negligible UX impact with refresh rotation.
- **No shadcn-svelte CLI init** — bits-ui installed directly, `cn()` utility created manually. Components will be added as needed in future runs.
- **Layered architecture instead of per-domain packages** — Plan originally listed `internal/auth/`, `internal/channel/`, etc. Refactored to `service/`, `handler/`, `middleware/`, `router/` layers with chi router. Future features (admin, embed, voice) will add files to existing layers rather than creating new top-level packages.

---

## Notes for Next Run

- Postgres is running on port 5440, all migrations applied through 000005
- Auth is fully wired frontend-to-backend: login, register, refresh, logout all work through the Vite proxy
- `MSYS_NO_PATHCONV=1` prefix needed for Docker commands with volume mounts in Git Bash
- **Go module root is `src/`** — build with `cd src && go build -o ../bin/den .`, run with `cd src && go run .`
- **Architecture is now layered**: business logic in `service/`, HTTP handlers in `handler/`, middleware in `middleware/`, routing in `router/`. New features add files to existing layers.
- **Chi router** is used — route params accessed via `chi.URLParam(r, "id")`, not `r.PathValue("id")`
- Channel CRUD requires admin for create/update/delete; list/get requires auth
- WebSocket connects via `GET /api/ws?token=<JWT>` (query param auth since browsers can't set WS headers — browser WebSocket API doesn't support custom headers)
- Message cursor pagination uses `before_time` + `before_id` query params (both required together)
- Hub uses channel-based select loop for all operations — no mutexes needed. Hub now also handles presence tracking and `BroadcastExclude` for typing.
- Tailwind v4 uses `@theme inline` block in app.css for custom colors (no `tailwind.config.js`)
- shadcn-svelte components can be added incrementally (bits-ui is installed, cn utility exists)
- Vite dev server proxies `/api` → `http://localhost:8080` with `ws: true` for WebSocket upgrade support
- Frontend stores follow factory pattern with `$state` runes in `.svelte.ts` files
- User colors are generated client-side from username hash — no `color` column in DB
- Three-column layout: ChannelSidebar (w-60) | MessageArea (flex-1) | MemberList (w-60)

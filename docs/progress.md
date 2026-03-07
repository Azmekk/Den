# Den — Progress

> This file is updated by Claude at the end of every run. Always paste both `docs/plan.md` and `docs/progress.md` at the start of a new run.

---

## Status

**Current run:** Complete
**Last completed run:** Run 2 — Auth (Backend Only)
**Next run:** Run 3

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

---

## Known Deviations from Plan

- **No Makefile or gofer.json** — Both had Windows/MSYS path translation issues with Docker volume mounts. Commands documented in CLAUDE.md instead.
- **Postgres port 5440** instead of 5432 — ports 5432-5434 were already in use on host.
- **5-minute access tokens** instead of 15 — tighter security window, negligible UX impact with refresh rotation.

---

## Notes for Next Run

- Postgres is running on port 5440, all migrations applied through 000005
- Auth is backend-only; frontend integration needed in a future run
- `MSYS_NO_PATHCONV=1` prefix needed for Docker commands with volume mounts in Git Bash

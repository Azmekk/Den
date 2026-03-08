# Den — Self-Hostable Chat & Voice Platform
### Project Plan

---

## Overview

A lightweight, self-hostable chat and voice application for small communities (20–50 concurrent users). Simple to deploy, simple to administrate. Not trying to be Discord — trying to be the thing Discord replaced for small groups.

---

## Tech Stack

| Layer | Choice | Rationale |
|---|---|---|
| **Backend** | Go | Fast, single binary deploys, great concurrency primitives for WebSocket/WebRTC signaling |
| **Database** | PostgreSQL | Full-text search via GIN index, mature, straightforward |
| **Query layer** | sqlc | Type-safe SQL without ORM overhead |
| **Frontend** | SvelteKit | Reactive, lightweight, no virtual DOM overhead, excellent SSR story |
| **Package manager** | Bun | Fast installs, built-in bundler, replaces npm/node for the frontend |
| **UI components** | shadcn-svelte + Tailwind | Headless, copy-owned components for dialogs/dropdowns/tooltips; Tailwind for everything else |
| **Real-time** | WebSockets (native Go `gorilla/websocket`) | For chat message delivery and presence |
| **Voice** | LiveKit (self-hosted) | Open-source WebRTC SFU; handles voice mixing server-side, reasonable quality, Docker-friendly |
| **Auth** | bcrypt + JWT (short-lived) + refresh tokens | Simple, no external deps |
| **Image proxying** | Server-side oEmbed + URL validation | See image section below |
| **Deployment** | Docker Compose | Single-command self-host; nginx reverse proxy in front |

---

## Database Design

### Message Limits & Cleanup

**Recommended hard limit: 100,000 messages total across the entire instance.**

Rationale:
- At an average of 300 bytes per message, 100k messages ≈ 30 MB. Trivially small.
- At typical small-community cadence (a few hundred messages/day), 100k buys you 1–3 years of history.
- When the limit is hit, a background job deletes the oldest N messages (e.g. oldest 5,000) in a single sweep.
- Pinned messages are **exempt** from cleanup and must be explicitly unpinned before they can be purged.
- DMs count toward the global limit the same as channel messages. Keep it simple.

You can expose this limit as a configurable env variable (`MAX_MESSAGES=100000`) so the operator can tune it at deploy time.

**Character limit: 2,000 characters per message.** Discord uses 2,000. It's a reasonable ceiling — long-form content belongs elsewhere.

### Schema (abbreviated)

```sql
-- Users
CREATE TABLE users (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  username    TEXT NOT NULL UNIQUE,
  password    TEXT NOT NULL,           -- bcrypt hash
  color       TEXT NOT NULL DEFAULT '#5865F2',
  is_admin    BOOLEAN NOT NULL DEFAULT FALSE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Channels (text + voice unified, distinguished by type)
CREATE TABLE channels (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name        TEXT NOT NULL,
  type        TEXT NOT NULL CHECK (type IN ('text', 'voice')),
  position    INT NOT NULL DEFAULT 0,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Messages (channel messages + DMs in one table)
CREATE TABLE messages (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  channel_id  UUID REFERENCES channels(id) ON DELETE CASCADE,  -- NULL for DMs
  dm_pair_id  UUID REFERENCES dm_pairs(id) ON DELETE CASCADE,  -- NULL for channels
  author_id   UUID NOT NULL REFERENCES users(id),
  content     TEXT NOT NULL CHECK (char_length(content) <= 2000),
  is_pinned   BOOLEAN NOT NULL DEFAULT FALSE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  edited_at   TIMESTAMPTZ,
  CONSTRAINT one_target CHECK (
    (channel_id IS NULL) != (dm_pair_id IS NULL)
  )
);

-- GIN index for full-text search
CREATE INDEX messages_content_search ON messages USING GIN (
  to_tsvector('english', content)
);

-- Regular indexes for filtered queries
CREATE INDEX messages_author ON messages(author_id);
CREATE INDEX messages_created ON messages(created_at);
CREATE INDEX messages_channel ON messages(channel_id, created_at DESC);

-- DM pairs (canonical pair regardless of who initiated)
CREATE TABLE dm_pairs (
  id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_a  UUID NOT NULL REFERENCES users(id),
  user_b  UUID NOT NULL REFERENCES users(id),
  UNIQUE (user_a, user_b)
);

-- Pinned messages (pointer, not a copy)
-- The is_pinned flag on messages is sufficient; this view just makes querying easier
CREATE VIEW pinned_messages AS
  SELECT * FROM messages WHERE is_pinned = TRUE;
```

---

## Message Search

Expose a single search endpoint with optional filters. All filters are combinable:

```
GET /api/search?q=hello&author=uuid&after=2024-01-01&before=2024-06-01&channel=uuid
```

**Implementation:**

```sql
SELECT m.*, u.username, u.color
FROM messages m
JOIN users u ON u.id = m.author_id
WHERE
  ($1::uuid IS NULL OR m.channel_id = $1)
  AND ($2::uuid IS NULL OR m.author_id = $2)
  AND ($3::timestamptz IS NULL OR m.created_at >= $3)
  AND ($4::timestamptz IS NULL OR m.created_at <= $4)
  AND ($5::text IS NULL OR to_tsvector('english', m.content) @@ plainto_tsquery('english', $5))
ORDER BY m.created_at DESC
LIMIT 50;
```

The GIN index makes the `tsquery` fast. The other indexes make date/author filtering fast. For 100k messages and 50 users this is more than sufficient without any query tuning.

---

## Authentication

- Registration is open by default; the operator can toggle `OPEN_REGISTRATION=false` to require an invite (simple invite token, no complex flow).
- First registered user is automatically admin.
- Admins can promote/demote other users to admin via a simple toggle in the admin panel.
- JWT access tokens (15 min TTL) + refresh tokens stored in httpOnly cookies (7 day TTL).
- No OAuth, no magic links. Username + password only.
- Password change supported. No account recovery (self-hosted — the admin can reset via DB if needed).

---

## Channels & Voice

### Text Channels
- CRUD managed by admins only.
- Channels have a drag-to-reorder position field.
- Messages delivered in real-time via WebSocket. On connect, client subscribes to a channel and receives new messages as they arrive.
- On initial load, fetch the last 50 messages. Infinite scroll upward fetches earlier pages.

### Voice Channels
- Powered by **LiveKit** (self-hosted, runs as a Docker container alongside the app).
- LiveKit is an open-source WebRTC SFU — it handles the hard parts (mixing, TURN/STUN, etc).
- The Go backend mints short-lived LiveKit JWT tokens for each user joining a voice channel.
- The Svelte frontend uses the LiveKit JS SDK to connect.
- Voice channel presence (who's currently in a channel) is tracked via the Go backend over WebSocket and broadcast to all connected clients in real time.
- No recording, no screen share.

**Audio quality:** LiveKit defaults to Opus codec at 32kbps, which is indistinguishable from Discord for voice chat. No configuration needed.

---

## Image & Video Embedding

**The approach:** The client never uploads files. Instead, users paste URLs. The backend validates and enriches them.

### Flow

1. User pastes a URL into the message box.
2. Frontend detects the URL pattern and shows a small preview badge before sending.
3. On send, the backend receives the message content as plain text. A lightweight parser extracts URLs.
4. The backend fetches oEmbed metadata (or Open Graph tags as fallback) for recognized domains and caches the result (in-memory or a small `embeds` table with a TTL).
5. The cached embed metadata is returned alongside the message object. The client renders it.

### Recognized embed types

| Type | Detection | Render |
|---|---|---|
| Direct image URL | Ends in `.jpg/.png/.gif/.webp` | `<img>` tag, max height 400px |
| Imgur | `imgur.com/...` | Fetch oEmbed, render image |
| YouTube | `youtube.com/watch` or `youtu.be` | Render iframe embed (user clicks to activate) |
| Tenor/Giphy | Domain match | Render GIF via their oEmbed API |
| Generic URL | Anything else | Open Graph title + description card, no media |

### "Upload via Imgur" convenience feature

Add a paperclip/image button in the message bar that:
1. Opens a native file picker (images only, client-side only — nothing hits your server).
2. POSTs the selected image directly to the **Imgur API** from the client's browser using their free anonymous upload endpoint.
3. On success, inserts the returned Imgur URL into the message input.
4. User sends the message as normal.

This means zero file storage on your server. Imgur handles CDN, hosting, and legal. The only thing you store is the URL string in the message. Users can also just paste Imgur links manually if they prefer.

> **Note:** Imgur's anonymous upload API has rate limits (1,250 uploads/day per client IP, generous for a 50-user instance). No API key required for anonymous uploads, but registering a free Imgur app gives you a client ID and higher limits.

---

## Admin Panel

Simple web UI accessible to admins only:

- **User management:** List users, toggle admin, reset password (generates a temp password), deactivate account.
- **Channel management:** Create/rename/delete/reorder text and voice channels.
- **Message cleanup:** View current message count vs. limit. Trigger manual cleanup sweep. Configure auto-cleanup threshold.
- **Instance settings:** Toggle open registration, set instance name, set default theme color.

---

## User Customization

- **Username:** Set on registration, changeable in profile settings. Unique constraint enforced.
- **Display color:** A color picker in profile settings. Stored as a hex string. Used to colorize the username in chat — the same way IRC clients handled it. No role colors, no server-wide theming per user.
- No avatars (no file storage). The color + username initial is the avatar (generated CSS circle, like a Google account placeholder).

---

## Real-time Architecture (WebSocket)

A single persistent WebSocket connection per client handles:

- Incoming messages (channel and DM)
- Message edits and deletes
- Presence updates (who's online, who's in a voice channel)
- Typing indicators (ephemeral, never persisted)

The Go backend maintains an in-memory hub (`map[channelID][]conn`) for fan-out. With 50 concurrent users this is trivial — no Redis, no pub/sub infrastructure needed.

---

## UI & Styling

- **Tailwind** for all layout and styling. A single dark theme defined as ~10 CSS variables (`--bg-base`, `--bg-surface`, `--bg-elevated`, `--border`, `--text-primary`, `--text-muted`, `--accent`, `--danger`). Use these throughout rather than raw Tailwind color classes so palette tweaks are a one-line change.
- **shadcn-svelte** for interactive components that are painful to build accessibly from scratch. Copy in only what's needed — no locked-in dependency:
  - `Dialog` — admin panel modals, confirm prompts
  - `DropdownMenu` — right-click/long-press context menu on messages (edit, delete, pin)
  - `Tooltip` — timestamps, user info on hover
  - `Command` — search palette (Cmd+K style search across messages)
- **`@livekit/components-svelte`** for voice UI (mic toggle, speaker indicators, participant list). Minimal and override-friendly.
- **Virtual list** for the message area — a lightweight custom scroller or `svelte-virtual-list` to avoid DOM bloat on long history. Load 50 messages at a time, fetch earlier pages on scroll up.
- Everything else (message bubbles, sidebar, channel list, input bar, presence dots) is plain Tailwind.

---

## Mobile & Desktop Design

SvelteKit with a responsive layout:

- **Desktop:** Three-column layout — channel list, message area, member list (collapsible). Standard Discord-style sidebar.
- **Mobile:** Single-column with a slide-out drawer for the channel list. Member list hidden behind a tap. Voice join/leave as a persistent bottom bar when in a call.
- No native app. PWA manifest so it can be "installed" from the browser on mobile.
- Touch-friendly tap targets (min 44px). No hover-dependent interactions on critical paths.

---

## Deployment

`docker-compose.yml` with four services:

```yaml
services:
  postgres:
    image: postgres:16
    volumes: [pgdata:/var/lib/postgresql/data]
    env_file: .env

  livekit:
    image: livekit/livekit-server:latest
    command: --config /etc/livekit.yaml
    volumes: [./livekit.yaml:/etc/livekit.yaml]
    ports: ["7880:7880", "7881:7881", "50000-50200:50000-50200/udp"]

  app:
    build: .
    depends_on: [postgres, livekit]
    env_file: .env
    ports: ["8080:8080"]

  nginx:
    image: nginx:alpine
    volumes: [./nginx.conf:/etc/nginx/nginx.conf]
    ports: ["80:80", "443:443"]
    depends_on: [app]
```

- Single `.env` file for all configuration.
- `make migrate` runs DB migrations via `golang-migrate`.
- `make build` builds the frontend with `bun run build` and produces a single static Go binary with the Svelte output embedded via Go's `embed` package.
- The Dockerfile installs Bun to build the frontend, then compiles the Go binary — the final image has no Node/Bun runtime, just the binary.
- HTTPS via nginx + Certbot (Let's Encrypt). One command to provision.

---

## Project Structure

All source code lives under `src/`, which is also the Go module root. The Go backend uses a layered architecture with chi router:

```
src/
├── main.go                # Entrypoint (config + wiring only)
├── embed.go               # Embeds built frontend static files
├── internal/
│   ├── router/router.go   # Chi router setup + all route registration
│   ├── handler/           # HTTP handlers (thin layer, no business logic)
│   │   ├── auth.go        # AuthHandler — register, login, refresh, logout, me, change-password
│   │   ├── channel.go     # ChannelHandler — list, get, create, update, delete
│   │   └── message.go     # MessageHandler — get history
│   ├── service/           # Business logic (no HTTP concerns)
│   │   ├── auth.go        # AuthService — registration, login, JWT, refresh tokens
│   │   ├── channel.go     # ChannelService — channel CRUD
│   │   ├── message.go     # MessageService — send, edit, delete, history
│   │   └── helpers.go     # Shared helpers (isUniqueViolation)
│   ├── middleware/auth.go # RequireAuth, RequireAdmin, context accessors
│   ├── httputil/httputil.go # DecodeJSON, WriteJSON, WriteError, cookie helpers
│   ├── ws/                # WebSocket hub, client, handler
│   └── db/                # sqlc generated code
├── db/
│   ├── migrations/        # SQL migration files
│   └── queries/           # sqlc .sql query files
├── web/                   # SvelteKit frontend
│   ├── src/
│   │   ├── routes/
│   │   ├── lib/
│   │   │   ├── components/
│   │   │   └── stores/    # Svelte stores for WS state
│   │   └── app.html
│   ├── static/
│   └── bun.lockb
├── docs/
│   ├── plan.md            # This document
│   └── progress.md        # Updated by Claude after every run
├── docker-compose.yml
├── Dockerfile
├── livekit.yaml
└── .env.example
```

Future features (admin panel, embeds, voice) will slot into the existing layers:
- Business logic → `service/admin.go`, `service/embed.go`, `service/voice.go`
- HTTP handlers → `handler/admin.go`, `handler/embed.go`, `handler/voice.go`

---

## Feature Checklist & Claude CLI Build Plan

Each run should leave the repo in a working, committable state. Never start a run on a broken foundation.

**Before every run:**
- Paste `docs/plan.md` + `docs/progress.md` as context
- Be explicit about what is already complete and must not be modified
- After each run, read the full diff before moving on

**After every run:**
- Claude must update `docs/progress.md` with what was completed, any deviations from the plan, and the exact starting point for the next run

---

### Run 1 — Skeleton & Database
- [ ] Scaffold full repo structure (see Project Structure)
- [ ] `docker-compose.yml`, `Dockerfile`, `.env.example`
- [ ] All DB migrations (full schema from this plan)
- [ ] `golang-migrate` wired to `make migrate`
- [ ] Verify: `docker compose up` brings Postgres live, migrations run clean

### Run 2 — Auth (Backend only)
- [ ] Register, login, JWT issuance, refresh token rotation
- [ ] Auth middleware for protected routes
- [ ] First registered user is automatically admin
- [ ] Verify: `curl` register + login returns valid JWT

### Run 3 — Channels, WebSocket & Messages (Backend only)
- [ ] Channel CRUD (text channels first)
- [ ] WebSocket hub with channel subscription and fan-out
- [ ] Message send, receive, persist, paginate (50 at a time)
- [ ] Message edit and delete
- [ ] Verify: Two `wscat` connections receive each other's messages in real time

### Run 4 — SvelteKit Frontend Scaffold & Auth UI
- [ ] SvelteKit project init with Bun, Tailwind, shadcn-svelte
- [ ] CSS variables defined for the full dark theme palette
- [ ] Login and register pages
- [ ] JWT handling, refresh flow, protected route guards
- [ ] Verify: Can log in and land on an empty dashboard

### Run 5 — Main Chat UI
- [ ] Three-column layout (channel list, message area, member list)
- [ ] Virtual scrolling message list
- [ ] Message input bar
- [ ] Real-time WebSocket connection from client
- [ ] Presence dots (online/offline)
- [ ] Typing indicators
- [ ] Verify: Two browser tabs can chat in real time

### Run 6 — DMs & Pinned Messages
- [ ] DM pair creation and conversation view
- [ ] DM delivery over existing WebSocket
- [ ] Pin/unpin messages (admin or author)
- [ ] Pinned messages view per channel
- [ ] Verify: DMs work between two users, pinned messages persist across reload

### Run 7 — Search
- [ ] Backend search endpoint with all filter combinations (text, author, date, date range, channel)
- [ ] Frontend Command palette (Cmd+K) wired to search endpoint
- [ ] Verify: Search by text returns correct results, GIN index is being used (`EXPLAIN ANALYZE`)

### Run 8 — Admin Panel
- [ ] User management (list, toggle admin, reset password, deactivate)
- [ ] Channel management (create, rename, delete, reorder)
- [ ] Message cleanup controls (current count, manual sweep, threshold config)
- [ ] Instance settings (open registration toggle, instance name)
- [ ] Verify: Admin can promote another user who can then access the panel

### Run 9 — Embeds & Imgur Upload
- [ ] Backend URL extractor and oEmbed/OG fetcher with in-memory cache
- [ ] Embed metadata returned alongside message objects
- [ ] Frontend embed renderer (image, YouTube iframe, GIF, generic link card)
- [ ] Imgur client-side upload button (file picker → Imgur API → inserts URL)
- [ ] Verify: Pasting a YouTube URL renders an embed; Imgur button inserts a working image URL

### Run 10 — Voice Channels
- [ ] LiveKit token minting in Go backend
- [ ] Voice channel presence tracked over WebSocket
- [ ] `@livekit/components-svelte` integrated in frontend
- [ ] Join/leave voice, mic toggle, participant list
- [ ] Verify: Two users can join a voice channel and hear each other

### Run 11 — Polish & Deployment
- [ ] Mobile layout (slide-out drawer, bottom voice bar)
- [ ] Touch targets and mobile interaction pass
- [ ] PWA manifest
- [ ] User customization (color picker, username change)
- [ ] Message cleanup background job wired up
- [ ] nginx config + Certbot HTTPS setup documented
- [ ] Verify: Full flow works on a mobile browser; `docker compose up` on a fresh VPS serves over HTTPS

---

## What's Explicitly Out of Scope

- File uploads of any kind (images go through Imgur or external URLs)
- End-to-end encryption
- Role/permission systems beyond admin/non-admin
- Screen sharing
- Message reactions (not in spec, skip it)
- Threads or replies (keep it flat)
- Bots or webhooks
- Federation or multi-server

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
| **Object storage** | S3-compatible bucket (MinIO, R2, S3) | Emotes, profile pics, uploaded images; config via env vars |
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
  avatar_filename TEXT,               -- bucket object key (avatars/{uuid}.webp), NULL = no avatar
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

-- Custom emotes (admin-uploaded, stored as <emote:uuid> tokens in messages)
CREATE TABLE custom_emotes (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name        TEXT NOT NULL UNIQUE,            -- shortcode (alphanumeric + underscores, 2-32 chars)
  filename    TEXT NOT NULL,                   -- bucket object key (emotes/{uuid}.webp)
  uploaded_by UUID NOT NULL REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Tracks each user's last-read position per channel (for unread counts)
CREATE TABLE channel_reads (
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  channel_id  UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  last_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, channel_id)
);

-- Tracks which users are mentioned in which messages
CREATE TABLE message_mentions (
  message_id  UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (message_id, user_id)
);
CREATE INDEX message_mentions_user ON message_mentions(user_id);

-- Tracks uploaded media files (for ephemeral upload cleanup)
CREATE TABLE media_uploads (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  uploader_id UUID NOT NULL REFERENCES users(id),
  bucket_key  TEXT NOT NULL,              -- e.g. "videos/{uuid}.mp4"
  media_type  TEXT NOT NULL CHECK (media_type IN ('image', 'video')),
  expires_at  TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '24 hours'),  -- all inline uploads expire after 24h
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX media_uploads_expires ON media_uploads(expires_at);
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

**The approach:** Users can upload images directly (when bucket storage is configured) or paste external URLs. The backend validates, converts, and enriches them.

### Media Upload (requires bucket storage)

When bucket storage is configured (`BUCKET_*` env vars set):

1. A paperclip button appears in the message bar (hidden when uploads are disabled). On desktop, users can also drag and drop files onto the message area to upload.
2. User picks an image (PNG, JPG, GIF, WebP — max 25 MB) or video (MP4, WebM — max 100 MB).
3. Frontend POSTs the file to `POST /api/upload/image` or `POST /api/upload/video`.
4. **Images:** Backend validates, converts to WebP (animated GIFs kept as-is), stores in bucket under `images/{uuid}.webp`.
5. **Videos:** Backend validates format + size, stores as-is (no transcoding) under `videos/{uuid}.{ext}`.
6. Backend returns the public URL. Frontend inserts it into the message input.
7. User sends the message as normal — the URL is embedded like any other media URL.

**Ephemeral uploads:** All inline image and video uploads are auto-deleted from the bucket after 24 hours. A background goroutine runs periodically to purge expired uploads. After expiry, the message text retains the URL but the media will no longer load — the frontend shows a "media expired" placeholder. A `media_uploads` table tracks all uploaded files and their expiry.

**Not ephemeral:** Emotes and profile pictures are **permanent** — only inline image/video uploads are ephemeral.

**Profile pictures:** Max 5 MB upload. User can position/crop via a small UI menu before submitting. Server converts the cropped result to 128×128 WebP.

When bucket storage is **not** configured, the paperclip button is hidden. Users can still paste external image/video URLs manually.

### External URL Embeds

1. User pastes a URL into the message box.
2. Frontend detects the URL pattern and shows a small preview badge before sending.
3. On send, the backend receives the message content as plain text. A lightweight parser extracts URLs.
4. The backend fetches oEmbed metadata (or Open Graph tags as fallback) for recognized domains and caches the result (in-memory or a small `embeds` table with a TTL).
5. The cached embed metadata is returned alongside the message object. The client renders it.

### Recognized embed types

| Type | Detection | Render |
|---|---|---|
| Direct image URL | Ends in `.jpg/.png/.gif/.webp` | `<img>` tag, max height 400px |
| Direct video URL | Ends in `.mp4/.webm` | `<video>` tag, max height 400px |
| YouTube | `youtube.com/watch` or `youtu.be` | Render iframe embed (user clicks to activate) |
| Tenor/Giphy | Domain match | Render GIF via their oEmbed API |
| Generic URL | Anything else | Open Graph title + description card, no media |

---

## Custom Emotes

Custom server emotes work like Discord — admins upload small images and assign them a shortcode. Users type `:shortcode:` in the input, but what gets stored in the database is an **emote token** containing the emote's UUID, not the shortcode text.

### Embedding Format

When a user sends a message containing `:shortcode:`, the backend resolves the shortcode to a UUID and replaces it in the stored message content with an emote token:

```
<emote:550e8400-e29b-41d4-a716-446655440000>
```

This format is chosen because:
- **Rename-safe:** If an admin renames an emote, old messages still render correctly (they reference the UUID, not the name).
- **Collision-free:** The `<emote:uuid>` pattern cannot appear in normal user text (angle brackets in user content are escaped before emote resolution).
- **Parseable:** The frontend uses a simple regex (`/<emote:([0-9a-f-]{36})>/g`) to find and render emotes.

The backend performs this substitution on message create/edit. The frontend never stores raw shortcodes.

### Image Storage (bucket)

Emote images are stored in the S3-compatible bucket under `emotes/{uuid}.webp`. Uploaded images (PNG, GIF, WebP) are converted to WebP server-side before storing. Animated GIFs are kept as-is (`emotes/{uuid}.gif`). The Go backend serves them at `GET /api/emotes/{id}/image` which proxies or redirects to the bucket URL, with aggressive cache headers (emote images are immutable — a "change" is a delete + re-upload).

Emote upload is only available when bucket storage is configured. If no bucket is configured, the emote management UI is hidden and upload endpoints return 501.

- **Max file size:** 256 KB
- **Allowed formats:** PNG, GIF, WebP (converted to WebP on upload; animated GIFs kept as GIF)
- **Max dimensions:** 128×128 (server resizes/rejects on upload)

### Admin Flow

1. Admin uploads an image + shortcode via a dedicated emote management page (accessible from admin panel or sidebar).
2. Backend validates name (alphanumeric + underscores, 2–32 chars), checks uniqueness, validates image (size, format, dimensions).
3. Converts to WebP (or keeps as GIF if animated), uploads to bucket under `emotes/{uuid}.webp`, inserts row into `custom_emotes`.
4. Broadcasts an `emote_list_update` event over WebSocket so all connected clients refresh their cache.

### Deletion

When an emote is deleted, its image is removed from the bucket and the DB row is deleted. Existing messages that reference the UUID will render a placeholder (e.g., a small "deleted emote" icon or the text `:unknown:`). The `<emote:uuid>` token remains in the message content — no retroactive message rewriting.

### Client-Side Cache & Rendering

- On app load, the frontend fetches the full emote list (`GET /api/emotes`) — an array of `{id, name, url}`.
- This list is cached in a Svelte store and used for both rendering and autocomplete.
- WS `emote_list_update` events trigger a re-fetch.
- When rendering a message, the frontend replaces `<emote:uuid>` tokens with inline `<img>` elements (sized to line height for inline flow, larger if the message is emote-only).

### Autocomplete

When a user types `:` followed by 2+ characters in the message input, a popup shows matching emotes (filtered by shortcode as they type, with image previews). Selecting one inserts `:shortcode:` into the input text. The backend resolves this to `<emote:uuid>` on send.

---

## @Mentions, Notifications & Unread Messages

A full notification and unread tracking system. Users can mention others with `@username`. All users — online and offline — receive proper unread counts and mention notifications.

### Mention Parsing & Storage

- In the message input, users type `@username`. On send, the backend resolves `@username` to a **mention token**: `<mention:user-uuid>`.
- Like emotes, this is rename-safe — if a user changes their username, old mentions still resolve.
- The backend extracts all mentioned user IDs and stores them in a `message_mentions` join table for efficient querying.
- The frontend renders `<mention:uuid>` tokens as highlighted, clickable spans showing the current username.

### Database Schema

```sql
-- Tracks each user's last-read position per channel
CREATE TABLE channel_reads (
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  channel_id  UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  last_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, channel_id)
);

-- Tracks which users are mentioned in which messages (for badge counts)
CREATE TABLE message_mentions (
  message_id  UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (message_id, user_id)
);
CREATE INDEX message_mentions_user ON message_mentions(user_id);
```

### Unread Tracking

- **`channel_reads` table:** Stores the last time each user "read" each channel (updated when the user views a channel or scrolls to the bottom).
- **Unread count:** For any channel, unread = count of messages in that channel with `created_at > channel_reads.last_read_at` for that user.
- **On connect/reconnect:** The client fetches unread counts for all channels in a single API call (`GET /api/channels/unread`). This returns `[{channel_id, unread_count, mention_count}]`.
- **Real-time updates:** When a new message arrives via WS for a channel the user is NOT currently viewing, the frontend increments the local unread count. When the user switches to that channel, a `PUT /api/channels/{id}/read` call updates `last_read_at` and resets the count.

### Mention Notifications

- **Online users (connected via WS):**
  - When a message with mentions is sent, the WS broadcast to mentioned users includes a `mentioned: true` flag.
  - The frontend shows a **mention badge** (count) on the channel in the sidebar, visually distinct from the regular unread indicator.
  - A **notification sound** plays if the mentioned user is not currently viewing that channel. Users can mute sounds in their settings (stored in localStorage).

- **Offline users (not connected):**
  - The `message_mentions` table persists all mentions regardless of online status.
  - When an offline user reconnects, the `GET /api/channels/unread` endpoint returns mention counts derived from `message_mentions` rows joined against `channel_reads.last_read_at`.
  - No push notifications (out of scope) — offline users see their mentions when they next open the app.

### Mention Rendering

- Messages mentioning the **current user** are highlighted with a distinct background color (e.g., a subtle yellow/gold tint).
- The `@username` span is styled as a colored pill/badge (using the mentioned user's display color).
- Clicking a mention could scroll to that user in the member list (nice-to-have).

### Autocomplete

When a user types `@` in the message input, a popup shows matching users (all users, not just online — filtered as they type). Selecting one inserts `@username` into the input text. The backend resolves this to `<mention:uuid>` on send.

---

## Admin Panel

Simple web UI accessible to admins only:

- **User management:** List users, toggle admin, reset password (generates a temp password), deactivate account.
- **Channel management:** Create/rename/delete/reorder text and voice channels.
- **Message cleanup:** View current message count vs. limit. Trigger manual cleanup sweep. Configure auto-cleanup threshold.
- **Emote management:** Upload/delete custom emotes, view current emote list with previews.
- **Instance settings:** Toggle open registration, set instance name, set default theme color.
- **Storage management (when bucket configured):** View current bucket usage (total size of stored files), set a max storage limit via `MAX_BUCKET_STORAGE` env var (default unlimited), browse/delete uploaded files (images, videos, emotes), see file metadata (uploader, upload date, expiry). When the storage limit is reached, new uploads are rejected with a 507 error.

---

## User Customization

- **Username:** Set on registration, changeable in profile settings. Unique constraint enforced.
- **Display color:** A color picker in profile settings. Stored as a hex string. Used to colorize the username in chat — the same way IRC clients handled it. No role colors, no server-wide theming per user.
- **Profile picture (requires bucket storage):**
  - Users can upload a profile picture via profile settings.
  - Max size: 5 MB. Accepted formats: PNG, JPG, WebP.
  - User can position/crop via a small frontend UI menu before submitting.
  - Server converts the cropped result to 128×128 WebP on upload.
  - Stored in bucket under `avatars/{user-uuid}.webp`. The `avatar_filename` column in `users` tracks this.
  - Profile pictures are **permanent** (not ephemeral).
  - `GET /api/users/{id}/avatar` serves the image (redirect to bucket URL).
  - Fallback: CSS circle with username initial + user color (existing behavior, always works even without bucket).
  - Upload button is hidden in profile settings when bucket storage is not configured.

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

## Object Storage (Optional)

All uploaded media (emotes, profile pictures, inline images) is stored in an S3-compatible bucket. **Bucket storage is optional** — if the `BUCKET_*` env vars are not set, upload features are simply disabled/hidden in the UI and the app remains fully functional as a text-only chat.

### Environment Variables

All optional — if unset, upload features are disabled:

| Variable | Description | Default |
|---|---|---|
| `BUCKET_ENDPOINT` | S3-compatible endpoint URL | — |
| `BUCKET_NAME` | Bucket name | — |
| `BUCKET_REGION` | Region | `auto` |
| `BUCKET_ACCESS_KEY` | Access key | — |
| `BUCKET_SECRET_KEY` | Secret key | — |
| `BUCKET_PUBLIC_URL` | Public base URL for serving files | Falls back to endpoint |
| `MAX_BUCKET_STORAGE` | Max total storage in bucket (e.g. `1GB`, `500MB`) | Unlimited |

### Backend Behavior When Not Configured

- `GET /api/config` returns `{ "uploads_enabled": false }` (frontend uses this to hide upload UI)
- Emote upload endpoints return 501
- Profile picture upload returns 501
- Image upload returns 501
- Video upload returns 501
- Text chat, embeds via external URLs, and all other features work normally

### Media Pipeline

All uploads (emotes, profile pics, inline images, videos) go through a validation and storage pipeline:

1. **Validate** — check file size and format
2. **Resize** if needed (emotes: 128×128, profile pics: 128×128, inline images/videos: no resize)
3. **Convert to WebP** (images only) — normalizes formats and saves space. Videos are stored as-is (no server-side transcoding — too CPU-heavy for a small instance).
4. **Store in bucket** — under the appropriate prefix (`emotes/`, `avatars/`, `images/`, `videos/`)
5. **Track expiry** — inline image and video uploads are inserted into `media_uploads` with a 24h expiry

**Exception:** Animated GIFs are stored as-is (Go's stdlib can't encode animated WebP).

**Ephemeral cleanup job:** A background goroutine runs every hour, queries `media_uploads` for rows where `expires_at < NOW()` (both images and videos), deletes the files from the bucket, and removes the DB rows.

**Permanent vs ephemeral:** Emotes and profile pictures are permanent (no expiry, no cleanup). Only inline image/video uploads are ephemeral (24h).

**Go dependencies:**
- `golang.org/x/image/webp` for decoding
- `github.com/chai2010/webp` (or similar) for WebP encoding

**Max upload sizes (pre-conversion):**
- Emotes: 256 KB (permanent)
- Profile pics: 5 MB (permanent)
- Inline images: 25 MB (ephemeral, 24h)
- Videos: 100 MB (ephemeral, 24h)

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

### Run 6 — Admin Panel
- [ ] User management (list, toggle admin, reset password, deactivate)
- [ ] Channel management (create, rename, delete, reorder)
- [ ] Message cleanup controls (current count, manual sweep, threshold config)
- [ ] Instance settings (open registration toggle, instance name)
- [ ] Storage management UI (when bucket configured): current usage, file browser with delete, max storage limit display
- [ ] Verify: Admin can promote another user who can then access the panel

### Run 7 — Custom Emotes (bucket storage)
- [ ] Migration for `custom_emotes` table
- [ ] Backend: bucket service/client setup (S3-compatible, configured via `BUCKET_*` env vars)
- [ ] Backend: `GET /api/config` endpoint returning `{ "uploads_enabled": true/false }`
- [ ] Backend: emote CRUD endpoints (`POST /api/emotes` admin-only create with image upload, `DELETE /api/emotes/{id}`, `GET /api/emotes` public list, `GET /api/emotes/{id}/image` serve image)
- [ ] Backend: image validation on upload (256 KB max, PNG/GIF/WebP, 128×128 max dimensions)
- [ ] Backend: WebP conversion pipeline — convert uploaded PNG/WebP to WebP, keep animated GIFs as-is
- [ ] Backend: bucket storage under `emotes/{uuid}.webp` (or `.gif` for animated)
- [ ] Backend: upload endpoints return 501 when bucket is not configured
- [ ] Backend: on message create/edit, resolve `:shortcode:` → `<emote:uuid>` tokens before persisting
- [ ] Backend: escape angle brackets in user content before emote resolution (prevent collisions)
- [ ] Backend: WebSocket `emote_list_update` broadcast on emote create/delete
- [ ] Frontend: check `GET /api/config` to determine if uploads are enabled; hide emote upload UI if not
- [ ] Frontend: emote store — fetch full list on load, re-fetch on WS `emote_list_update`
- [ ] Frontend: message renderer parses `<emote:uuid>` tokens → inline `<img>` (line-height sized, larger if message is emote-only)
- [ ] Frontend: deleted emote fallback (`:unknown:` text or placeholder icon)
- [ ] Frontend: emote autocomplete popup on `:` + 2 chars — filtered list with image previews, inserts `:shortcode:`
- [ ] Frontend: emote management page (admin-only, hidden when uploads disabled) — upload form, list with previews, delete
- [ ] Verify: Upload emote as admin, send message with `:shortcode:`, renders as image; delete emote, old message shows placeholder; disable bucket config → upload UI hidden

### Run 8 — @Mentions, Notifications & Unread
- [ ] Migration for `channel_reads` and `message_mentions` tables
- [ ] Backend: on message create, resolve `@username` → `<mention:uuid>` tokens before persisting
- [ ] Backend: extract mentioned user IDs, insert into `message_mentions` table
- [ ] Backend: `GET /api/channels/unread` endpoint — returns `[{channel_id, unread_count, mention_count}]` per user
- [ ] Backend: `PUT /api/channels/{id}/read` endpoint — updates `channel_reads.last_read_at`
- [ ] Backend: WS broadcast includes `mentioned_user_ids` array on new messages
- [ ] Frontend: message renderer parses `<mention:uuid>` tokens → highlighted clickable spans with current username
- [ ] Frontend: self-mentions styled with distinct background color (gold/yellow tint)
- [ ] Frontend: `@` autocomplete popup — shows all users filtered as they type, inserts `@username`
- [ ] Frontend: unread count badge on channels in sidebar (derived from local tracking + API on connect)
- [ ] Frontend: mention count badge on channels (visually distinct from unread, e.g. red pill)
- [ ] Frontend: real-time unread increment when WS message arrives for non-active channel; reset on channel switch
- [ ] Frontend: notification sound on mention when not viewing that channel (with mute toggle in localStorage)
- [ ] Frontend: on reconnect, fetch `GET /api/channels/unread` to sync offline mentions and unread counts
- [ ] Verify: Mention a user → they see highlight, badge, and hear sound; go offline, get mentioned, reconnect → badge shows; unread counts track correctly across channel switches

### Run 9 — DMs & Pinned Messages
- [ ] DM pair creation and conversation view
- [ ] DM delivery over existing WebSocket
- [ ] Pin/unpin messages (admin or author)
- [ ] Pinned messages view per channel
- [ ] Verify: DMs work between two users, pinned messages persist across reload

### Run 10 — Search (DONE)
- [x] Backend search endpoint with all filter combinations (text, author, date, date range, channel)
- [x] Frontend Command palette (Cmd+K) wired to search endpoint
- [ ] Verify: Search by text returns correct results, GIN index is being used (`EXPLAIN ANALYZE`)

### Deviation — Enhanced Search: User Filter, Jump-to-Message, Jump to Latest (DONE)
- [x] Backend: `GetMessagesAroundTarget` SQL query (25 before + target + 25 after)
- [x] Backend: `GetMessagesAfterCursor` SQL query (forward pagination, 50 messages)
- [x] Backend: `GetMessagesAround` and `GetNewer` service methods
- [x] Backend: `GetMessagesAround` and `GetNewer` handler methods
- [x] Backend: Routes `GET /channels/{id}/messages/around` and `GET /channels/{id}/messages/newer`
- [x] Frontend: Message store — `fetchAround`, `fetchNewer`, `jumpToLatest`, `clearJumped`, `scrollTarget`, jumped/hasMoreAfter state
- [x] Frontend: `handleNewMessage` skips appending when channel is jumped
- [x] Frontend: CSS `highlight-flash` animation for scroll-to-message
- [x] Frontend: MessageArea — `data-message-id` attrs, scroll-to-message effect, forward pagination on scroll, "Jump to latest" button
- [x] Frontend: SearchPalette — user filter UI with dropdown, jump-to-message on result click
- [x] Frontend: +page.svelte — clear jumped state on channel switch

### Run 11 — Media Embeds & Media Upload (bucket storage) (DONE)
- [x] Frontend client-side URL detection and inline embed rendering (images, videos, YouTube thumbnails → iframe on click)
- [x] Backend: `POST /api/upload/image` — validate format via magic bytes (WebP/PNG/JPEG/GIF), SHA-256 dedup, store in bucket under `images/{uuid}.{ext}`, insert into `media_uploads` with 24h expiry (max 25 MB)
- [x] Backend: `POST /api/upload/video` — validate format (MP4/WebM via magic bytes), SHA-256 dedup, store in bucket under `videos/{uuid}.{ext}`, insert into `media_uploads` with 24h expiry (max 100 MB)
- [x] Backend: background cleanup goroutine — runs hourly, deletes expired images and videos from bucket + removes `media_uploads` DB rows
- [x] Migration 000009: `media_uploads` table with `content_hash` column for dedup, indexes on `expires_at` and `content_hash`
- [x] Frontend: paperclip button in message bar (only shown when uploads enabled via `configStore.uploadsEnabled`) + drag-and-drop onto message area
- [x] Frontend: WebP conversion utility (`convertToWebP`) — converts images client-side before upload, skips animated GIFs
- [x] Frontend: video player for uploaded/embedded videos (native `<video>` tag, max height 400px)
- [x] Frontend: "media expired" placeholder when image/video URL no longer resolves (onerror handler)
- [x] Backend: profile picture upload — `POST /api/users/me/avatar`, frontend crops via cropperjs v2 to 128×128, converts to WebP, server validates and stores under `avatars/{user-uuid}.{ext}` (permanent, max 5 MB)
- [x] Backend: `GET /api/users/{id}/avatar` — redirect to bucket URL (public route)
- [x] No new migration needed for `avatar_url` — column already exists from migration 000001
- [x] Frontend: avatar crop modal with cropperjs v2 (square aspect ratio, 128×128 output)
- [x] Frontend: avatar display in messages, member list, user profile popover, and sidebar (with fallback to initial+color circle)
- [x] Frontend: avatar upload trigger in ChannelSidebar settings (click avatar to change, hidden when uploads disabled)
- [x] WebSocket `user_updated` event extended with `avatar_url` field for real-time avatar updates

### Deviation — Fix Bucket Path Duplication + Hide Embedded URLs (DONE)
- [x] Added `UsePathStyle: true` to S3 client options in `service/bucket.go` (fixes R2 path duplication)
- [x] Added `embedUrls` derived Set in `MessageContent.svelte` to hide embedded URLs from message text

### Deviation — Emoji Picker, Message Edit/Delete, DM Sizing (DONE)
- [x] DM sidebar avatar sizing: `h-5 w-5 text-[10px]` → `h-6 w-6 text-xs` for better readability
- [x] Message context menu: Edit/Delete actions with permission checks (edit: author only, delete: author or admin)
- [x] Inline message editing: textarea replaces content, Enter to save, Escape to cancel, sends `edit_message` via WS
- [x] Delete confirmation dialog: modal with message preview, sends `delete_message` via WS
- [x] `unresolveContent()` utility: converts `<emote:uuid>` → `:name:`, `<mention:uuid>` → `@username`, unescapes HTML
- [x] Emoji/emote picker: bits-ui Popover with search, category tabs, custom emotes + unicode emojis
- [x] `unicode-emoji-json` package for lightweight emoji data (~40KB gzipped)
- [x] Emoji data helper: lazy-loaded categories, search function, shortcode generation
- [x] EmoteAutocomplete extended: `:shortcode` now matches both custom emotes and unicode emojis
- [x] Unicode emoji inserts raw character (not shortcode), custom emote inserts `:name:`

### Run 12 — Voice Channels ✅
- [x] LiveKit token minting in Go backend
- [x] Voice channel presence tracked over WebSocket
- [x] `livekit-client` integrated in frontend (used directly, not component library)
- [x] Join/leave voice, mic toggle, participant list
- [x] **Noise gate:** Volume threshold setting — mic only transmits when input exceeds threshold (prevents ambient pickup)
- [x] **Noise cancellation:** Browser-native via `noiseSuppression: true` on audio track constraints
- [x] **Echo cancellation:** Browser WebRTC AEC enabled by default (`echoCancellation: true` on audio track constraints)
- [x] Audio settings UI: toggles for noise gate, noise cancellation, echo cancellation; noise gate threshold slider
- [x] Verify: Two users can join a voice channel and hear each other; audio processing toggles work

### Run 13 — Polish & Deployment (DONE)
- [x] Mobile layout (slide-out drawer, bottom voice bar) — Sidebar sections + mobile drawers done in deviation
- [x] Touch targets and mobile interaction pass — 44px min touch targets on header buttons, member rows, voice channel items; `touch-action: manipulation` globally
- [x] ~~PWA manifest~~ — Dropped (Tauri desktop wrapper planned in Run 18)
- [x] User customization (color picker, display name change) — Done in deviation
- [x] Message cleanup background job wired up — `RunMessageCleanupLoop` in admin service, `MAX_MESSAGES` env var, hourly check
- [x] nginx config — Production-ready with gzip, security headers, WebSocket support, upload size limit (Certbot dropped — external reverse proxy handles TLS)
- [x] Verify: Full flow works on a mobile browser; `docker compose up` serves through nginx

### Deviation — Fix UserProfilePopover conflicts, display name/color real-time updates (DONE)
- [x] Remove UserProfilePopover from MemberList (conflicted with UserContextMenu + onclick)
- [x] Look up display name from usersStore in MessageArea (real-time updates)
- [x] Pass live display name to UserProfilePopover props in MessageArea

### Deviation — Fix DM Opening, Navigation & Token Refresh (DONE)
- [x] Fast DM opening via `findByUserId` — skip POST for existing conversations
- [x] Tab auto-switching on channel/DM selection
- [x] DM unread tracking (client-side) with badge indicators on individual DMs and Messages tab
- [x] Channel unread indicator on Server tab
- [x] Message button in chat user profiles (UserProfilePopover in MessageArea)
- [x] Token refresh on visibility change (sleep/tab switch recovery)
- [x] WebSocket `updateToken` method for fresh token on reconnect

### Deviation — Fix Biome --unsafe underscore-prefixed variables (DONE)
- [x] Removed `_` prefix from all script variables/functions that Biome incorrectly marked as unused (Svelte template references invisible to Biome)
- [x] 9 files fixed, `bun run build` passes clean

### Deviation — Sidebar Tabs + Mobile Drawers (DONE)
- [x] Created `layout.svelte.ts` store with sidebar/memberList open state + mutual exclusion + `sidebarTab` (server/messages)
- [x] Restructured `ChannelSidebar.svelte` with Server/Messages tab bar (full tab switching, not sections), added `onNavigate` prop
- [x] Changed `MemberList.svelte` — clicking user row opens profile popover (with "Message" button for DMs), auto-closes drawer on DM open
- [x] Added `onMessage` and `isSelf` props to `UserProfilePopover.svelte` — shows "Message" button inside profile drawer for non-self users
- [x] Added hamburger + members toggle buttons (`md:hidden`) in `MessageArea.svelte` header
- [x] Updated `+page.svelte` with responsive layout: static panels on desktop, full-height overlay drawers on mobile with fly/fade transitions

### Deviation — Fix UserProfilePopover not opening on click (DONE)
- [x] Replaced `Popover.Trigger` with `Popover.Anchor` + manual `onclick`/`onkeydown` handlers in `UserProfilePopover.svelte`
- [x] `e.stopPropagation()` prevents click from being swallowed by parent `ContextMenu.Trigger`
- [x] `bind:open` on `Popover.Root` preserves bits-ui positioning, focus, and dismiss behavior

### Deviation — @everyone Mention, Reserved Usernames, Display Name Update, User Profile Popover, User Color Picker (DONE)
- [x] Reserved usernames (everyone, here, channel, admin) blocked at registration
- [x] @everyone mention: backend token resolution + frontend rendering + notification
- [x] Display name update: PUT /api/users/me/display-name + sidebar edit UI
- [x] User color picker: migration 000008 + PUT /api/users/me/color + sidebar color swatches + native picker
- [x] User profile popover: bits-ui Popover on avatar/name in chat and member list
- [x] Shared userColor utility extracted to $lib/utils.ts (removed duplication from 4 components)
- [x] Real-time user_updated WS broadcast for display name and color changes

### Deviation — Fix Voice Audio: Noise Gate & Speaking Indicator (NOT WORKING)
- [x] Fix noise gate `gateOpen` initialization (`true` → `false`) so `onGateChange(true)` fires on first speech
- [x] Noise gate arming fall-through (already applied in Run 12)
- [x] `handleActiveSpeakers` excludes local user — speaking state driven exclusively by noise gate (already applied in Run 12)
- **⚠ None of these fixes resolved the issues. Noise gate, noise cancellation, echo cancellation, and speaking indicator are all still broken. Needs deeper investigation.**

### Deviation — Voice Fixes: Stereo, Noise Gate Scaling, Mic Level, Sound Guards
- [x] Stereo playback: shared AudioContext with ChannelSplitter(1) → ChannelMerger(2) upmix for remote audio
- [x] Noise gate scaling: RMS multiplier 1000→3000 for better threshold slider range (useful range now ~15–50 instead of 5–10)
- [x] Mic level indicator: `onLevelChange` callback in noise gate, reactive `micLevel` in voice store, level bar behind threshold slider
- [x] Sound guards: join/leave sounds only play when local user is in a voice channel
- **Working:** Noise gate, stereo playback, mic level indicator, sound guards
- **Still broken:** Echo cancellation (stereo upmix mutes `<audio>` element, breaking browser echo reference), noise suppression (needs testing with noise gate disabled — may work but masked by gate pipeline)

### Deviation — Fix Echo Cancellation via MediaStreamAudioDestinationNode
- [x] Route stereo upmix pipeline to `MediaStreamAudioDestinationNode` instead of `ctx.destination`
- [x] Set destination's `.stream` as `el.srcObject` (replaces original mono stream with stereo)
- [x] Leave `<audio>` element unmuted — browser echo canceller has proper reference signal
- [x] Clean up `MediaStreamAudioDestinationNode` in `handleTrackUnsubscribed`
- **Fixed:** Echo cancellation now works (unmuted `<audio>` element provides reference signal)
- **Needs verification:** Noise suppression with noise gate disabled, stereo still works through new pipeline

### Deviation — Composite Krisp + Noise Gate Processor (DONE)
- [x] Created `createCompositeProcessor()` in `noise-gate.ts` — chains Krisp → Noise Gate in single TrackProcessor
- [x] Removed `createSpeakingDetector()` — composite gate handles level monitoring + gain gating internally
- [x] Simplified voice store: single `noiseGateProcessor` ref replaces separate `krispProcessor` + `speakingDetector`
- [x] Three modes: composite (Krisp+gate), Krisp-only (wrapped), standalone gate (fallback)
- [x] Noise gate toggle always visible in AudioSettingsPopover (no longer hidden when Krisp active)
- [x] `bun run build` passes clean

### Run 14 — Avatar Cropper Fix & Old Avatar Cleanup ✅
- [x] Fix avatar cropper image positioning bug (image not positioned correctly in cropperjs modal)
- [x] Delete old avatar from bucket when uploading a new one (in `UpdateAvatar()`, delete previous bucket key before storing new file — prevents orphaned files when format changes e.g. PNG→WebP)
- [x] Verify: Cropper displays image correctly, old avatar file is removed from bucket after re-upload

### Run 15 — Bucket Storage Limit
- [ ] Migration: add `file_size BIGINT NOT NULL` column to `media_uploads` table
- [ ] Backend: record file size on every upload (images, videos — avatars and emotes are permanent and small, can be excluded or included)
- [ ] Backend: `GetTotalBucketUsage()` query — `SELECT COALESCE(SUM(file_size), 0) FROM media_uploads`
- [ ] Backend: pre-upload size check — sum current usage + incoming file size, reject with 413 if over `MAX_BUCKET_STORAGE` limit
- [ ] Admin UI: display current bucket usage and configured limit in admin panel (Storage tab or Settings tab)
- [ ] `MAX_BUCKET_STORAGE` env var (e.g. `1GB`, `500MB`) parsed at startup, default unlimited
- [ ] Verify: Upload works under limit, returns 413 when limit would be exceeded

### Run 16 — Admin-Configurable Message Limits ✅
- [x] **Message count limit:** Admin-configurable max total messages across the instance (replaces hardcoded concept from plan)
- [x] Background cleanup job: when limit is exceeded, delete oldest N messages (pinned messages exempt)
- [x] Admin UI: current message count vs limit, manual cleanup trigger, configurable threshold
- [x] **Message character limit:** Admin-configurable max characters per message (currently hardcoded at 2000)
- [x] Backend: read character limit from admin settings instead of constant in `message_service.go` and `dm_service.go`
- [x] Frontend: enforce character limit in message input (show counter, disable send when exceeded)
- [x] Persist both settings to DB (admin settings table or similar — currently in-memory only)
- [x] Verify: Admin can change both limits; message cleanup runs when count limit exceeded; oversized messages rejected

### Deviation — Fix Media Embeds & Server-Side URL Unfurling (DONE)
- [x] Server-side OG unfurl endpoint (`GET /api/unfurl?url=...`) with in-memory cache (10-min TTL)
- [x] Removed client-side YouTube/Tenor/Giphy pattern matching — all non-media URLs use unfurl API
- [x] Rich embed cards for URLs with OG metadata (title, description, image, site name)
- [x] YouTube detected via `og:site_name`, rendered as `youtube-nocookie.com` iframe
- [x] Video URLs with `og:video` render with `<video>` player + poster
- [x] Direct media URLs (`.jpg`, `.mp4`, etc.) still handled client-side
- [x] Video file URLs show clickable URL text + embed (not hidden like images)

### Run 17 — Admin Media Manager
- [ ] Backend: `GET /api/admin/media` — list all media uploads with metadata (uploader, filename, size, type, upload date, expiry)
- [ ] Backend: `DELETE /api/admin/media/{id}` — delete a specific media file from bucket + DB
- [ ] Backend: `GET /api/admin/media/stats` — total count, total size, breakdown by type
- [ ] Admin UI: new "Media" tab in admin panel — sortable/filterable table of uploads, delete button per item, bulk delete
- [ ] Admin UI: storage stats summary (total size, count, usage by type chart or breakdown)
- [ ] Verify: Admin can browse, inspect, and delete uploaded media files

### Run 18 — Tauri Desktop Wrapper
- [ ] Initialize Tauri project alongside existing SvelteKit frontend
- [ ] `tauri.conf.json` configuration (window size, title, app identifier)
- [ ] Auth token storage (system keychain via Tauri's secure storage vs localStorage)
- [ ] System tray with online/offline indicator
- [ ] Desktop notifications for mentions (native OS notifications)
- [ ] GitHub Actions workflow using `tauri-apps/tauri-action` for cross-platform builds (Windows, macOS, Linux)
- [ ] Auto-updater configuration
- [ ] Verify: App launches as native window, connects to server, chat works, notifications fire on mention

---

## What's Explicitly Out of Scope

- Media uploads are limited to profile pics (5 MB), emotes (256 KB), images (25 MB, ephemeral 24h), and videos (100 MB, ephemeral 24h); stored in optional S3-compatible bucket
- End-to-end encryption
- Role/permission systems beyond admin/non-admin
- Screen sharing
- Message reactions (emoji reactions on messages — custom emotes are for inline message content only)
- Threads or replies (keep it flat)
- Bots or webhooks
- Federation or multi-server

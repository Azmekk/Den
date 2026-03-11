# Den — Progress

> This file is updated by Claude at the end of every run. Always paste both `docs/plan.md` and `docs/progress.md` at the start of a new run.

---

## Status

**Current run:** Complete
**Last completed run:** Run 16 — Admin-Configurable Message Limits
**Last deviation:** Chat Timestamps & Date Separators
**Next run:** Run 17

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

### Run 6 — Admin Panel
- Added `DeleteUser`, `CountMessages`, `DeleteOldestMessages`, `CountChannels` sqlc queries and regenerated Go code
- Created `AdminService` (`src/internal/service/admin.go`): list users, toggle admin (prevents self-demotion), reset password (random 16-char hex + revoke tokens), delete user (prevents self-deletion), stats, message cleanup, settings
- Created `AdminHandler` (`src/internal/handler/admin.go`): 8 endpoints under `/api/admin/*`
- Extended `AuthService` with `SetOpenRegistration`/`IsOpenRegistration` and `SetInstanceName`/`GetInstanceName` for runtime settings toggle
- Wired admin routes in router under existing auth + admin middleware group
- Created admin panel frontend (`src/web/src/routes/admin/+page.svelte`) with 4 tabs: Users (toggle admin, reset password with modal, delete with confirmation), Channels (create/edit/delete), Messages (stats + cleanup), Settings (instance name, open registration toggle)
- Added gear icon in `ChannelSidebar` linking to `/admin` (visible only for admins)
- Added `AdminStats` and `AdminSettings` TypeScript types
- `go build` and `bun run build` both pass clean

### Run 7 — Custom Emotes
- Added S3-compatible bucket storage for custom emote images (via MinIO in dev)
- Created migration for `emotes` table with unique name constraint
- Added sqlc queries and generated Go code for emote CRUD
- Implemented `EmoteService` and `EmoteHandler` with upload (admin-only), list (authenticated), and delete (admin-only) endpoints
- Frontend emote picker in chat input with `:emote_name:` syntax rendering in messages
- Emote images served from S3 bucket via presigned URLs or direct bucket access
- `go build` and `bun run build` both pass clean

### Run 8 — @Mentions, Notifications & Unread
- Added `channel_reads` and `message_mentions` tables (migration 000007)
- @mention support: `@username` → `<mention:uuid>` token, stored in `message_mentions` table
- `MentionAutocomplete` component for @-mention suggestions in chat input
- Unread tracking per channel with mention counts; sidebar shows unread dots and mention badges
- Notification sound on mention when not in current channel
- `go build` and `bun run build` both pass clean

### Run 9 — DMs & Pinned Messages
- **DM Pairs**: sqlc queries for `dm_pairs` table (create/get/list), `DMService` with create-or-get, list conversations, send DM message, validate user membership
- **DM Messaging**: WebSocket `send_dm` message type, DM-aware `edit_message`/`delete_message` routing (sends to both users via `SendToUser` instead of global broadcast)
- **Pin/Unpin**: `SetMessagePinned` query, `PinMessage`/`UnpinMessage` service methods (author or admin), REST endpoints `PUT/DELETE /api/messages/{id}/pin`, broadcasts `pin_message`/`unpin_message` events
- **Pinned Messages Panel**: `GET /api/channels/{id}/pins` and `GET /api/dms/{id}/pins` endpoints, `PinnedMessagesPanel` slide-out component
- **Frontend DM Store**: `dms.svelte.ts` with conversations list, message history, cursor pagination, WS event handlers
- **Frontend Pin Store**: `pins.svelte.ts` with fetch/pin/unpin/toggle panel
- **ChannelSidebar**: Added "Direct Messages" section below channels
- **MemberList**: Click on user opens DM conversation
- **MessageArea**: Dual-mode (channel/DM), pin button in header, pin/unpin action on message hover
- **Mutual exclusion**: Selecting a channel deselects DM and vice versa; MemberList hidden in DM mode
- `MessageInfo.ChannelID` changed from `uuid.UUID` to `*uuid.UUID` (pointer) to support nullable channel_id for DM messages
- `EditMessage` and `DeleteMessage` signatures updated to return both `channelID` and `dmPairID` for correct WS routing
- No new migrations needed — `dm_pairs` table and `messages.dm_pair_id` column already exist from migration 000003/000004
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

### Run 6 (2026-03-09)
- Settings (open_registration, instance_name) are runtime-only in-memory toggles — not persisted to DB
- Admin routes nested inside existing admin middleware group (reuses `RequireAdmin` middleware)
- `CountChannels` query added to messages.sql (alongside `CountMessages`) since it's used only by admin stats
- Password reset generates 16 hex chars (8 random bytes) and revokes all refresh tokens for the user
- WS event listener registration moved before `websocket.connect()` in `+page.svelte` to avoid dropped messages on connect

### Run 7 (2026-03-09)
- Custom emotes with S3-compatible storage (MinIO for dev, any S3-compatible provider for prod)
- Emote images uploaded as multipart form data, stored in S3 bucket
- `:emote_name:` syntax parsed and rendered inline in message text
- Emote picker UI added to chat input area
- Admin-only upload/delete; all authenticated users can list and use emotes
- **Message ordering fix**: wrapped `GetMessagesByChannel` query in a SQL subquery to ensure chronological order — outer query re-sorts `ASC` after inner query limits `DESC` (ensures newest N messages are returned in display order)

### Run 8 (2026-03-09)
- @Mentions and unread tracking implemented (already done before this run, progress.md not updated)

### Deviation (2026-03-09) — Context Menus, New User Visibility, DM Pin Fix, Mention Autocomplete Avatars
Applied ahead of Run 10 as a deviation (not a numbered run):
- **DM Pin Permission Fix**: `PinMessage`/`UnpinMessage` in `service/message.go` now validate DM pair membership — only participants can pin/unpin in DMs (admin bypass disabled for DMs)
- **New User Broadcast**: `AuthHandler` now holds a `*ws.Hub` reference; on successful registration, broadcasts `user_registered` WS event to all connected clients
- **Frontend User Registration Listener**: `users.svelte.ts` gained `addUser()` method; `+page.svelte` listens for `user_registered` events and adds new users to the store immediately
- **Message Context Menu**: Created `MessageContextMenu.svelte` using bits-ui `ContextMenu` — right-click on any message shows "Pin Message" / "Unpin Message" option
- **User Context Menu**: Created `UserContextMenu.svelte` — right-click on a member shows "Message" option to open DM (skipped for self)
- **Removed Hover Pin Buttons**: Pin/unpin buttons removed from message hover UI in `MessageArea.svelte`; pinning now exclusively through context menu
- **Mention Autocomplete Avatars**: Added colored avatar circles (same `userColor` hash function) to `MentionAutocomplete.svelte` results
- **Bare `@` Trigger**: Mention autocomplete now triggers on bare `@` character (shows all users, capped at 8) instead of requiring at least 1 character after `@`
- bits-ui `ContextMenu` component used for the first time (was installed in Run 4 but unused until now)
- `go build` and `bun run build` both pass clean

### Run 9 (2026-03-09)
- DMs use existing `dm_pairs` table (migration 000003) with `CHECK (user_a < user_b)` canonical ordering
- `CreateDMPair` uses `LEAST/GREATEST` with `ON CONFLICT DO UPDATE` to always return existing pair (upsert pattern)
- DM messages routed via `SendToUser` (both sender and recipient) instead of channel broadcast
- Edit/delete of DM messages also route to both users via `ValidateUserInPair`
- Pin/unpin accessible to message author or admin; broadcasts globally so all connected clients update
- `DMMessageHandler` interface added to WS hub for DM operations
- Frontend mutual exclusion: `dmStore.select()` calls `channelStore.deselect()`, sidebar `selectChannel()` calls `dmStore.deselect()`
- `MessageInfo.channel_id` changed to optional (`*uuid.UUID` backend, `string?` frontend) to support DM messages where `channel_id` is null
- `pinned` field added to `MessageInfo` and rendered in message UI

### Deviation (2026-03-09) — @everyone Mention, Reserved Usernames, Display Name Update, User Profile Popover, User Color Picker
- **Reserved Usernames**: Added `reservedUsernames` map in `service/auth.go` blocking "everyone", "here", "channel", "admin" during registration
- **@everyone Mention (Backend)**: Updated `resolveMentions()` in `service/message.go` to detect `@everyone` and replace with `<mention:everyone>` token; returns `mentionedEveryone` bool; `SendMessage`/`EditMessage` envelopes include `"mentioned_everyone": true` when applicable; @everyone is not supported in DMs (no special handling needed — "everyone" isn't a real user)
- **@everyone Mention (Frontend)**: `MessageContent.svelte` tokenRegex extended to match `everyone` as mention value; renders as amber-highlighted `@everyone` span; `MentionAutocomplete.svelte` shows `@everyone` as first entry with amber icon (hidden in DMs via `isDM` prop); `+page.svelte` checks `data.mentioned_everyone` for notification sound + mention badge
- **Display Name Update (Backend)**: Added `UpdateUserDisplayName` sqlc query; `UserService.UpdateDisplayName()` method with 64-char limit; `UserHandler.UpdateDisplayName` handler at `PUT /api/users/me/display-name`; broadcasts `user_updated` WS event
- **User Color (Backend)**: Migration 000008 adds nullable `color VARCHAR(7)` column to users; `UpdateUserColor` sqlc query; `ListUsers` now includes color; `UserService.UpdateColor()` with hex validation; `UserHandler.UpdateColor` at `PUT /api/users/me/color`; broadcasts `user_updated` WS event; `PublicUserInfo` gains `Color` field
- **User Color (Frontend)**: Extracted `USER_COLORS`, `userColorFromHash()`, `getUserColor()` to `$lib/utils.ts`; removed duplicated color functions from `MessageArea`, `MemberList`, `MentionAutocomplete`, `ChannelSidebar`; added `color?: string` to `UserInfo` type
- **User Profile Popover**: New `UserProfilePopover.svelte` using bits-ui `Popover`; shows large avatar, display name, @username; integrated on avatar/name in `MessageArea` (non-grouped messages) and `MemberList` (avatar)
- **Settings UI**: `ChannelSidebar.svelte` user panel redesigned — shows user color avatar, display name + username, edit pencil opens Popover with display name input and color picker (12 swatches + native `<input type="color">`); `usersStore` gains `changeDisplayName()`, `changeColor()`, `updateUser()` methods
- **WS Event**: `+page.svelte` listens for `user_updated` events and updates local user store in real-time
- **UserHandler** now holds `*ws.Hub` reference (passed from router) for broadcasting profile changes
- `go build` and `bun run build` both pass clean; migration 000008 applied

### Deviation (2026-03-09) — Fix UserProfilePopover conflicts, display name/color real-time updates
- **MemberList Popover Fix**: Fixed `UserProfilePopover` in MemberList — changed outer `<button>` to `<div>` (avoids nested-button invalid HTML from Popover.Trigger), wrapped avatar in a `<div onclick={stopPropagation}>` so clicking the avatar opens the profile popover without triggering the DM open. Right-click context menu and row click for DM still work as before.
- **Live Display Name in Messages**: Added `getDisplayNameForMessage()` helper in `MessageArea.svelte` that looks up the user from `usersStore` by `msg.user_id` instead of using stale `msg.display_name` from when the message was sent. Fallback to `msg.display_name || msg.username` if user not found in store.
- **Live Data in UserProfilePopover**: Updated `UserProfilePopover` props in MessageArea to pass `displayName={getDisplayNameForMessage(msg)}` instead of `displayName={msg.display_name}`, so popovers show current display name.
- Color was already updating in real-time via `getColorForMessage()` which looks up the users store.
- `bun run build` passes clean

### Deviation (2026-03-09) — Fix UserProfilePopover not opening on click
- `Popover.Trigger` inside `ContextMenu.Trigger` doesn't receive clicks — ContextMenu intercepts `pointerdown` events
- Replaced `Popover.Trigger` with `Popover.Anchor` wrapping a `<div role="button" tabindex="0" class="contents">` with manual `onclick` (toggle + `stopPropagation`) and `onkeydown` (Enter/Space)
- `bind:open` on `Popover.Root` preserves bits-ui auto-positioning, focus management, and dismiss-on-outside-click
- Right-click context menu unaffected (separate event path from `onclick`)

### Deviation (2026-03-09) — Fix Biome --unsafe underscore-prefixed variables
- Biome `--unsafe` applied `noUnusedVariables` fixes that prefixed Svelte script variables with `_`, breaking template references (Biome can't see Svelte template usage)
- Removed `_` prefix from all affected variables/functions across 9 files:
  - `ConnectionBanner.svelte` (1 var)
  - `ChannelSidebar.svelte` (10 vars/functions)
  - `MemberList.svelte` (3 vars/functions)
  - `MessageArea.svelte` (20 vars/functions)
  - `MessageContent.svelte` (4 vars/functions)
  - `PinnedMessagesPanel.svelte` (3 vars/functions — found beyond original plan)
  - `admin/+page.svelte` (22 vars/functions)
  - `login/+page.svelte` (3 vars/functions)
  - `register/+page.svelte` (3 vars/functions)
- `bun run build` passes clean

### Deviation (2026-03-10) — Fix DM Opening, Navigation & Token Refresh
- **Fast DM Opening**: Added `findByUserId(userId)` to `dms.svelte.ts` — searches conversations by `other_user_id`. `MemberList.svelte` `openDM()` now checks for existing conversation first, skipping the POST request. UI switches immediately to Messages tab.
- **Tab Auto-Switching**: `selectChannel()` in `ChannelSidebar.svelte` sets `layoutStore.sidebarTab = 'server'`; `selectDM()` sets it to `'messages'`. DM opening from MemberList and MessageArea also switches tab.
- **DM Unread Tracking**: Added `dmUnreadCounts` Map to `dms.svelte.ts` with `incrementUnread`, `clearUnread`, `getDMUnread`, `hasAnyUnread` methods. `select()` auto-clears unread for selected DM. `+page.svelte` `handleNewDM` increments unread when incoming DM is not the active one.
- **Tab Notification Indicators**: Server tab shows red dot when `unreadStore.unreadCounts.size > 0`; Messages tab shows red dot when `dmStore.hasAnyUnread()`. Individual DM items show count badge + bold text (mirrors channel unread pattern).
- **Message Button in Chat Profiles**: Added `openDM()` function to `MessageArea.svelte`, passed `onMessage` and `isSelf` props to both `UserProfilePopover` instances on chat messages (avatar and username).
- **Token Refresh on Wake**: Added `visibilitychange` listener in `+page.svelte` `onMount` — when tab becomes visible, calls `auth.refresh()`, updates WebSocket token via new `updateToken()` method, reconnects WS if disconnected. Redirects to `/login` if refresh fails.
- **WebSocket `updateToken`**: Added `updateToken(newToken)` method to `websocket.svelte.ts` that updates the stored token without disconnecting (reconnect logic uses fresh token automatically).
- `bun run build` passes clean

### Run 10 — Search
- Added `SearchMessages` sqlc query with `sqlc.narg()` for nullable filter params (channel, author, after, before, text query)
- `SearchMessagesParams` uses `uuid.NullUUID`, `sql.NullTime`, `sql.NullString` for optional filters
- `JOIN channels c ON c.id = m.channel_id` naturally excludes DM messages (channel_id is NULL for DMs)
- `SearchResult` struct in service with `ChannelName` field for display
- `Search` handler at `GET /api/search` with query params: `q`, `channel`, `author`, `after` (RFC3339), `before` (RFC3339)
- Requires at least one filter (returns 400 if all empty)
- Route registered in authenticated group alongside other GET endpoints
- `SearchResult` TypeScript interface in `types.ts`
- `SearchPalette.svelte` component using bits-ui `Dialog` + `Command` (shouldFilter=false for server-side search)
- Debounced search (300ms), min 2 chars, AbortController via request counter for race conditions
- Results show channel pill, colored author name, truncated content, relative timestamp
- Clicking a result navigates to the channel and closes the palette
- Search button (magnifying glass icon) added to `MessageArea` header via `onSearchOpen` prop
- Global `Cmd/Ctrl+K` keyboard shortcut in `+page.svelte` toggles the search palette
- `go build` and `bun run build` both pass clean

### Deviation (2026-03-10) — Fix @-mention autocomplete broken
- **Missing Import**: Added `import { getUserColor } from '$lib/utils'` to `MentionAutocomplete.svelte` — was used on line 156 but never imported, causing a runtime crash when the autocomplete popup tried to render colored user avatars
- `bun run build` passes clean

### Deviation (2026-03-09) — Sidebar Tabs + Mobile Drawers
- **Layout Store**: Created `layout.svelte.ts` with `sidebarOpen`/`memberListOpen` state, mutual exclusion, `anyDrawerOpen` derived, `sidebarTab` (`'server' | 'messages'`) state for tab switching
- **ChannelSidebar Tabs**: Replaced combined Server+DM sections with a tab bar at top — "Server" tab (server icon, channel list) and "Messages" tab (chat icon, DM list); tabs fill full height; outer tag changed from `<aside>` to `<div>`; `onNavigate` callback prop
- **MemberList Rework**: Clicking a user row now opens `UserProfilePopover` (shows profile + "Message" button) instead of directly opening a DM; auto-closes member list drawer when DM is opened
- **UserProfilePopover**: Added `onMessage` and `isSelf` props; shows a "Message" button inside the profile drawer for non-self users; clicking it closes the popover and triggers the DM action
- **MessageArea Header**: Added hamburger button (left, `md:hidden`) and users/people button (right, `md:hidden`, hidden in DM mode)
- **+page.svelte Responsive Layout**: Desktop — static sidebar and member list; Mobile — full-height overlay drawers (inset-y-0) with fly/fade transitions, backdrop click to close, auto-close on navigation
- `bun run build` passes clean

---

### Run 10 (2026-03-10)
- Used `sqlc.narg()` for nullable params instead of plain `@param::type` (sqlc generates non-nullable types without it)
- bits-ui `Command` + `Dialog` components used for search palette (both already installed from Run 4)
- No additional dependencies needed
- Emote/mention tokens stripped in search result display via regex replacement
- `plainto_tsquery` used for safe user input (no special tsquery operators)

### Deviation (2026-03-10) — Enhanced Search: User Filter, Jump-to-Message, Jump to Latest
- **Backend SQL**: Added `GetMessagesAroundTarget` query (CTE with UNION ALL: 25 before + target + 25 after, ordered ASC) and `GetMessagesAfterCursor` query (forward pagination, 50 messages after cursor)
- **Backend Service**: `GetMessagesAround(ctx, channelID, targetMessageID)` returns messages + `hasMoreBefore`/`hasMoreAfter` booleans (based on whether 25 rows were returned in each direction); `GetNewer(ctx, channelID, afterTime, afterID)` returns newer messages + `hasMore`
- **Backend Handler/Routes**: `GET /channels/{id}/messages/around?message_id=` and `GET /channels/{id}/messages/newer?after_time=&after_id=` added to authenticated route group
- **Message Store**: Added jumped state tracking (`jumpedByChannel`, `hasMoreAfterByChannel`, `scrollTarget`, `loadingNewer`); `fetchAround()` loads messages around target and sets jumped state; `fetchNewer()` appends newer messages; `jumpToLatest()` clears jumped state and re-fetches latest; `clearJumped()` resets state for channel navigation; `handleNewMessage()` skips appending when channel is jumped
- **CSS**: Added `highlight-flash` keyframe animation (primary color fade out over 2s) in `app.css`
- **MessageArea**: Added `data-message-id` attributes to both grouped and non-grouped message divs; scroll-to-message `$effect` watches `scrollTarget`, scrolls element into view center and adds highlight animation; forward pagination triggers `fetchNewer` when scrolling near bottom in jumped mode; "Jump to latest" floating pill button shown when channel is jumped
- **SearchPalette**: Added user filter UI — "From: anyone" clickable text opens dropdown of all users (filterable), selecting shows pill with X to clear; search triggers with user filter alone (no text required); `handleSelect` checks if message exists in loaded messages (just scroll) or calls `fetchAround` (load around target); closes dialog after selection
- **+page.svelte**: Channel switch effect tracks previous channel ID and calls `clearJumped()` on the old channel so returning re-fetches latest
- `go build` and `bun run build` both pass clean

### Run 11 (2026-03-10) — Media Embeds & Media Upload
- **Migration 000009**: `media_uploads` table with `content_hash TEXT NOT NULL` for SHA-256 dedup, indexes on `expires_at` and `content_hash`
- **sqlc queries**: `media_uploads.sql` with InsertMediaUpload, GetMediaUploadByHash, ExtendMediaUploadExpiry, GetExpiredMediaUploads, DeleteMediaUploadsByIDs; `UpdateUserAvatarUrl` added to `users.sql`
- **MediaService** (`service/media.go`): UploadImage (WebP/PNG/JPEG/GIF, 25MB, SHA-256 dedup), UploadVideo (MP4/WebM, 100MB, SHA-256 dedup), UpdateAvatar (5MB, stored permanently under `avatars/{user-uuid}.{ext}`), CleanupExpired (hourly goroutine), format detection via magic bytes
- **MediaHandler** (`handler/media.go`): `POST /api/upload/image` and `POST /api/upload/video`, returns `{ "url": "..." }`
- **UserHandler**: Added `UploadAvatar` (`POST /api/users/me/avatar`) with `user_updated` WS broadcast; `GetAvatar` (`GET /api/users/{id}/avatar`) redirects to bucket URL
- **UserService**: Added `Queries()` accessor and `GetAvatarURL()` method
- **Router**: Added media upload routes (conditional on mediaSvc != nil), avatar routes (authenticated + public)
- **main.go**: Creates MediaService when bucket configured, starts cleanup goroutine with context
- **Frontend WebP conversion** (`media.ts`): `convertToWebP()` converts images client-side via Canvas API (quality 85%), `isAnimatedGif()` detects multi-frame GIFs (kept as-is), `isImageFile()`/`isVideoFile()` helpers
- **MessageContent.svelte**: Rewrote with URL detection — scans text parts for `https?://` URLs, renders inline embeds below message text. Image URLs → `<img>`, video URLs → `<video controls>`, YouTube → clickable thumbnail that loads iframe. `onerror` handlers show "Media expired" placeholder
- **MessageArea.svelte**: Added paperclip upload button (visible when `configStore.uploadsEnabled`), hidden file input, `uploadFile()` (converts images to WebP, POSTs to API, inserts URL into message input), drag-and-drop handlers with visual border highlight, avatar display in non-grouped messages (img with fallback div)
- **MemberList.svelte**: Avatar `<img>` with onerror fallback for both online and offline user lists
- **UserProfilePopover.svelte**: Added `avatarUrl` prop, shows avatar image with fallback to colored initial circle
- **AvatarCropModal.svelte**: Uses cropperjs v2 (web components, no CSS import needed), square aspect ratio selection, crops to 128×128 canvas, converts to WebP via `canvas.toBlob`, POSTs to `/api/users/me/avatar`, updates local user store
- **ChannelSidebar.svelte**: Clickable avatar in user panel (opens file picker when uploads enabled), shows current avatar with fallback; AvatarCropModal rendered at bottom
- **+page.svelte**: `handleUserUpdated` extended to handle `avatar_url` field in WS events
- **Design decisions**:
  - Frontend WebP conversion (no CGo dependency on backend) — backend validates format and stores as-is
  - SHA-256 content hash dedup — reuses existing bucket key and extends expiry for duplicate uploads
  - cropperjs v2 (web components) — no CSS import needed, `$toCanvas()` API for cropping
  - Client-side URL detection — no server-side oEmbed/OG fetching (plan deviation — simpler, no external HTTP calls)
  - `avatar_url` column already existed from migration 000001 (no new migration needed)
- `go build` and `bun run build` both pass clean

### Deviation (2026-03-10) — Fix Bucket Path Duplication + Hide Embedded URLs
- **Bucket `UsePathStyle`**: Added `UsePathStyle: true` to `s3.Options` in `service/bucket.go` — AWS SDK v2 defaults to virtual-hosted style addressing which doesn't work with S3-compatible services like R2 (prepends bucket name as path prefix to key, causing `den-dev/images/xxx.webp` instead of `images/xxx.webp`)
- **Hide embedded URLs from text**: Added `embedUrls` derived Set in `MessageContent.svelte` containing all URLs that render as media embeds; wrapped the URL `<a>` tag in `{#if !embedUrls.has(part.value)}` so embedded URLs (images, videos, YouTube, Tenor, Giphy) are hidden from the message text — only the embed preview shows below
- Existing misplaced objects in R2 (under `den-dev/images/...` key) need manual cleanup
- `go build` and `bun run build` both pass clean

### Deviation (2026-03-10) — Emoji Picker, Message Edit/Delete, DM Sizing
- **DM Sidebar Sizing**: `ChannelSidebar.svelte` DM avatar circles changed from `h-5 w-5 text-[10px]` to `h-6 w-6 text-xs` (20px → 24px) for better readability
- **MessageContextMenu**: Extended with `canEdit`, `canDelete`, `onEdit`, `onDelete` props. Edit shown for author only, Delete shown for author or admin (styled destructive red)
- **Inline Message Editing**: `editingMessageId`/`editContent` state in MessageArea. `startEdit()` calls `unresolveContent()` to convert stored tokens back to user-friendly text. Textarea replaces MessageContent, Enter saves via `edit_message` WS, Escape cancels
- **Delete Confirmation**: Modal overlay with message preview, Cancel/Delete buttons. Sends `delete_message` via WS
- **`unresolveContent()`**: New utility in `$lib/utils.ts` — reverse-resolves `<emote:uuid>` → `:name:`, `<mention:uuid>` → `@username`, `<mention:everyone>` → `@everyone`, unescapes `&lt;`/`&gt;`/`&amp;`
- **Emoji/Emote Picker**: New `EmotePicker.svelte` component using bits-ui `Popover`. Search bar, category sidebar tabs (9 unicode categories + custom), 8-column grid layout. Custom emotes shown first, then unicode. Lazy-loads emoji data on first open via dynamic import
- **Emoji Data Helper**: New `$lib/data/emoji-data.ts` — imports `unicode-emoji-json`, groups by category, generates shortcodes, caches result. `searchEmojis()` searches across all categories
- **EmoteAutocomplete Extended**: Now matches both custom emotes (priority) and unicode emojis when typing `:shortcode`. Custom emotes insert `:name:`, unicode emojis insert the raw character. Lazy-loads emoji data on first autocomplete trigger
- **Picker Integration**: Smiley face button between textarea and send button in MessageArea. `handlePickerSelect()` inserts at cursor position
- `unicode-emoji-json@0.8.0` added as dependency
- `bun run build` passes clean

### Deviation (2026-03-10) — Plan Update: Runs 14–18 & Run 12 Audio Features
- **Run 12 extended**: Added noise gate, noise cancellation, echo cancellation items + audio settings UI to existing Voice Channels run
- **Run 14 added**: Avatar Cropper Fix — fix cropperjs positioning bug + delete old avatars from bucket on re-upload
- **Run 15 added**: Bucket Storage Limit — `file_size` column, pre-upload size check, `MAX_BUCKET_STORAGE` env var, admin UI
- **Run 16 added**: Admin-Configurable Message Limits — message count limit with background cleanup + character limit per message, persisted to DB
- **Run 17 added**: Admin Media Manager — list/delete/stats endpoints + admin panel Media tab
- **Run 18 added**: Tauri Desktop Wrapper — Tauri init, secure auth storage, system tray, native notifications, GitHub Actions cross-platform builds
- **GIF picker skipped**: Tenor closed to new clients, existing URL embed system preferred
- **Desktop framework**: Tauri chosen (best CI/CD with official GitHub Action, cross-platform builds)
- No code changes — plan.md update only

### Run 12 (2026-03-10) — Voice Channels with LiveKit
- **SQL queries**: Added `ListVoiceChannels` (is_voice=true), `ListAllChannels` (no filter), made `CreateChannel` accept `is_voice` as `$4` parameter
- **VoiceService** (`service/voice.go`): `NewVoiceService(apiKey, apiSecret, url)` returns nil if any param empty; `GenerateToken()` uses `livekit/protocol/auth` to mint JWT with RoomJoin grant, 1hr TTL; `GetURL()` returns LiveKit WS URL
- **VoiceHandler** (`handler/voice.go`): `POST /api/voice/{channelId}/join` — validates channel exists and `is_voice=true`, mints token, returns `{token, url}`
- **ChannelService**: Added `IsVoice bool` to `ChannelInfo`, `ListVoice()`, `ListAll()`, `Create()` now accepts `isVoice` param
- **ChannelHandler**: Added `ListVoice`, `ListAll` handlers, `is_voice` in create request
- **ConfigHandler**: Added `voiceEnabled` bool, config response changed from `map[string]bool` to `map[string]any`
- **WebSocket voice presence**: Hub tracks `voiceUsers map[channelID]map[userID]bool`; `voiceJoin`/`voiceLeave` channels in select loop; removes user from previous channel on join; broadcasts `voice_state_update` globally; sends `voice_state_initial` on client register; auto-removes from voice on last connection disconnect
- **Client WS**: Added `voice_join`/`voice_leave` message types
- **Router**: Added `GET /api/channels/voice`, `POST /api/voice/{channelId}/join` (authenticated), `GET /api/admin/channels` (admin)
- **main.go**: Reads `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`, `LIVEKIT_URL` env vars; creates VoiceService (nil if unconfigured); passes to router
- **Frontend types**: Added `is_voice` to `ChannelInfo`, `voice_enabled` to `AppConfig`
- **Config store**: Added `voiceEnabled` state
- **Channels store**: Added `voiceChannels`, `fetchVoice()`, `sortedVoiceChannels`
- **Voice store** (`voice.svelte.ts`): Full LiveKit Room lifecycle (connect/disconnect/mute/unmute), voice states from WS events, noise gate/cancellation/echo settings persisted to localStorage, auto-play remote audio via hidden DOM container
- **Noise gate** (`voice/noise-gate.ts`): Web Audio API AnalyserNode, polls RMS every 50ms, 3 consecutive below-threshold checks to close gate, immediate open above threshold
- **AudioSettingsPopover**: bits-ui Popover with toggles for noise gate (+ threshold slider), noise cancellation, echo cancellation
- **VoiceConnectionBar**: Shown above user panel when connected — green dot, channel name, mic toggle, audio settings, disconnect button
- **ChannelSidebar**: Voice channels section with speaker icon, participant list (small avatar + username), VoiceConnectionBar above user panel
- **+page.svelte**: WS listeners for `voice_state_initial`/`voice_state_update`, `fetchVoice()` on mount, `voiceStore.leave()` on cleanup
- **Admin panel**: `is_voice` checkbox on channel creation form (disabled when editing), Type column in channel table, fetches from `/api/admin/channels` (shows both text + voice)
- **Go dependency**: `github.com/livekit/protocol v1.45.0`
- **Frontend dependency**: `livekit-client@2.17.2`
- Used `livekit-client` directly instead of `@livekit/components-svelte` (plan deviation — components lib is React-focused, direct SDK gives full control)
- `go build` and `bun run build` both pass clean

### Deviation (2026-03-10) — Fix Voice Audio: Noise Gate & Speaking Indicator
- **Noise gate `gateOpen` init**: Changed from `true` to `false` in `noise-gate.ts` — when `gateOpen` started as `true`, the first speech after joining never triggered `onGateChange(true)` because `!gateOpen` was false in the else branch. The speaking indicator wouldn't appear until the second speech burst (after a pause closed the gate and speaking reopened it).
- **Noise gate arming fall-through**: Already correctly implemented in Run 12 — when `armed` becomes true, code falls through to normal gating logic instead of returning early.
- **`handleActiveSpeakers` race condition**: Already correctly implemented in Run 12 — excludes local user (`myId`) from LiveKit's speaker set, preserves local user's state from noise gate callback. Prevents LiveKit's frequent `ActiveSpeakersChanged` events from overwriting the noise gate's speaking state.
- **Static/background noise**: Resolved by noise gate fix — gate now properly cuts audio below threshold (including static between speech). Browser constraints (`noiseSuppression`, `echoCancellation`) were already correctly passed.
- `bun run build` passes clean
- **⚠ None of these fixes resolved the issues. Noise gate, noise cancellation, echo cancellation, and speaking indicator are all still broken. Needs deeper investigation.**

### Deviation (2026-03-10) — Voice Fixes: Stereo, Noise Gate Scaling, Mic Level, Sound Guards
- **Stereo playback**: Remote audio routed through shared `AudioContext` with `ChannelSplitter(1)` → `ChannelMerger(2)`, duplicating mono to both L/R channels. `<audio>` element muted (LiveKit bookkeeping only). Shared context closed on leave/disconnect.
- **Noise gate scaling**: RMS multiplier changed from `1000` → `3000`. Old useful threshold range of 5–10 now maps to ~15–30. Default threshold kept at 20.
- **Mic level indicator**: Added `onLevelChange` callback to noise gate processor (called every 50ms with clamped 0–100 level). Voice store exposes reactive `micLevel`. `AudioSettingsPopover` renders a colored bar (green above threshold, yellow below) behind the threshold slider.
- **Sound guards**: `handleVoiceStateUpdate` returns early when `currentChannelId` is null — no more phantom join/leave sounds when not in voice.
- **Working:** Noise gate, stereo playback, mic level indicator, sound guards, speaking indicator
- **Still broken:** Echo cancellation (muted `<audio>` element breaks browser's echo reference signal), noise suppression (untested with noise gate disabled — may work but needs verification)
- `bun run build` passes clean

### Deviation (2026-03-10) — Fix Echo Cancellation via MediaStreamAudioDestinationNode
- **Problem**: Previous stereo fix muted the `<audio>` element and routed audio through `ctx.destination` directly. The browser's echo canceller correlates audio played through `<audio>` elements with mic input — a muted element provides no reference signal, so echoes passed through uncancelled.
- **Fix**: Changed `handleTrackSubscribed` to route the stereo upmix pipeline (`source → splitter → merger`) to a `MediaStreamAudioDestinationNode` instead of `ctx.destination`. The destination node's `.stream` is set as `el.srcObject`, replacing the original mono stream with stereo. The `<audio>` element is left unmuted and plays normally, giving the browser's echo canceller a proper reference signal.
- **Cleanup**: `handleTrackUnsubscribed` now also disconnects the stored `MediaStreamAudioDestinationNode` alongside the source node.
- **No changes** to `leave()`/`handleDisconnect()` — shared AudioContext cleanup was already correct.
- `bun run build` passes clean

### Deviation (2026-03-10) — Composite Krisp + Noise Gate Processor
- **Problem**: Krisp as sole track processor left residual crackles audible between speech — no gain gating to fully mute artifacts when user isn't speaking.
- **Composite processor** (`createCompositeProcessor` in `noise-gate.ts`): Chains Krisp → Noise Gate in a single `TrackProcessor`. On `init()`/`restart()`, calls `krispProcessor.init()` first, then feeds `krispProcessor.processedTrack` into the noise gate Web Audio pipeline (source → analyser → gain → destination). Composite's `processedTrack` is the gate output.
- **Removed `createSpeakingDetector()`**: No longer needed — the composite's noise gate handles both level monitoring and gain gating internally.
- **Voice store simplified**: Removed `krispProcessor` and `speakingDetector` variables, replaced by single `noiseGateProcessor` ref. Three modes: Krisp + noise gate (composite, recommended), Krisp only (wrapped with no-op setThreshold), noise gate only (standalone, unchanged).
- **UI**: Noise gate toggle now always visible in AudioSettingsPopover (was hidden when Krisp active). Users can disable gate for Krisp-only mode or enable both. Threshold slider shown when noise gate is enabled.
- `bun run build` passes clean

### Run 13 (2026-03-10) — Polish & Deployment
- **Touch targets**: Added `min-w-11 min-h-11` (44px) to all header icon buttons in MessageArea (hamburger, search, pinned, members). Added `min-h-11` to member list rows (online + offline). Bumped voice channel items from `py-1.5` to `py-2` with `min-h-11`. Added `touch-action: manipulation` globally in app.css to prevent 300ms tap delay.
- **Message cleanup background job**: Added `RunMessageCleanupLoop(ctx, maxMessages, batchSize, interval)` to `AdminService`. Runs hourly, counts messages, deletes oldest unpinned when count exceeds limit. Wired in `main.go` with `MAX_MESSAGES` env var (default 100,000). Deletes `count - max + batchSize/2` to avoid thrashing.
- **Nginx config**: Rewrote `nginx.conf` — added MIME types include, gzip compression, `client_max_body_size 50m`, security headers (`X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`), proper `/api/ws` WebSocket location with 24h timeouts, `X-Forwarded-Proto` trust from outer proxy, `real_ip_header`. HTTP-only (port 80), designed for external TLS-terminating reverse proxy.
- **Dropped**: PWA manifest (Tauri desktop wrapper planned in Run 18), Certbot (external reverse proxy handles TLS)
- `go build` and `bun run build` both pass clean

### Run 14 (2026-03-11) — Avatar Cropper Fix & Old Avatar Cleanup
- **Cropper CSS import**: Added `import 'cropperjs/dist/cropper.css'` to `AvatarCropModal.svelte` — cropperjs v2 web components need the stylesheet imported explicitly for proper rendering
- **Removed constraining CSS**: Removed `class="block max-w-full"` from the `<img>` element — cropperjs v2 manages image sizing internally, constraining classes break its positioning
- **Old avatar cleanup**: `UpdateAvatar()` in `media.go` now fetches the user's current `avatar_url` via `queries.GetUserByID()` before uploading. If an existing avatar exists, extracts the bucket key via new `KeyFromURL()` method and deletes it before uploading the new file
- **`KeyFromURL()` helper**: Added to `BucketService` in `bucket.go` — strips the `publicURL` prefix from a full URL to extract the bucket key
- No new dependencies, no migrations
- `go build` passes clean

### Run 16 (2026-03-11) — Admin-Configurable Message Limits
- **Migration 000010**: Created `admin_settings` single-row table with `CHECK (id = 1)` — stores `open_registration`, `instance_name`, `max_messages`, `max_message_chars`
- **sqlc queries**: `GetAdminSettings` and `UpdateAdminSettings` for the new table
- **AdminService refactor**: Added `sync.RWMutex`-protected `maxMessages` and `maxMessageChars` cache fields. `LoadSettings(ctx)` seeds from DB at startup. `GetSettings()` returns all 4 fields. `UpdateSettings()` writes to DB + updates cache + syncs AuthService. `RunMessageCleanupLoop` now reads `maxMessages` from cache each tick (0 = unlimited skip). Removed `maxMessages` parameter.
- **MessageService & DMService**: Added `getMaxChars func() int` field, replaced hardcoded `2000` with `s.getMaxChars()` in `SendMessage`, `EditMessage`, and `SendDMMessage`
- **ConfigHandler**: Added `getMaxChars func() int` field, exposes `max_message_chars` in `GET /api/config`
- **AdminHandler**: `UpdateSettings` now accepts `max_messages` and `max_message_chars` with validation (max_messages >= 0, max_message_chars 1–10000), writes to DB via service
- **main.go**: `adminSvc.LoadSettings()` called at startup before creating message/DM services. Removed `MAX_MESSAGES` env var block. Cleanup loop uses DB-backed limit.
- **Frontend types**: `AdminSettings` and `AppConfig` interfaces updated with new fields
- **Config store**: Added `maxMessageChars` state populated from `/api/config`
- **Admin page**: Two new setting cards (Max Messages, Max Message Characters) in Settings tab with number inputs
- **MessageArea**: Character counter shown below textarea when typing, turns red when over limit, send button disabled when exceeded
- All migrations applied through 000010, `sqlc generate` clean, backend and frontend build clean

### Deviation — Emote Upload Auto-Resize (2026-03-11)
- **Frontend emote upload now auto-resizes**: Static images are resized to 128x128 max and converted to WebP client-side via `convertToWebP()` before uploading. Animated GIFs pass through as-is (canvas can't handle animation). File input accepts `image/*` instead of just PNG/GIF/WebP. Label updated to reflect auto-resize behavior.
- **Backend unchanged**: Still validates ≤128x128, ≤256KB, PNG/GIF/WebP — acts as safety net.

### Deviation — Chat Timestamps & Date Separators (2026-03-11)
- **Replaced `formatTime()` with `formatTimestamp()`**: Today shows `HH:MM`, yesterday shows `Yesterday at HH:MM`, older shows `DD/MMM/YYYY HH:MM` (e.g. `09/MAR/2026 14:30`). Applied to both inline and hover timestamps.
- **Added date separator bars**: Horizontal `line — label — line` dividers between days. Today/Yesterday use labels, older dates use `DD/MMM/YYYY` format.
- **Message grouping respects day boundaries**: `isGrouped()` returns false when consecutive messages span different days.

## Known Deviations from Plan

- **No Makefile or gofer.json** — Both had Windows/MSYS path translation issues with Docker volume mounts. Commands documented in CLAUDE.md instead.
- **Postgres port 5440** instead of 5432 — ports 5432-5434 were already in use on host.
- **5-minute access tokens** instead of 15 — tighter security window, negligible UX impact with refresh rotation.
- **No shadcn-svelte CLI init** — bits-ui installed directly, `cn()` utility created manually. Components will be added as needed in future runs.
- **Layered architecture instead of per-domain packages** — Plan originally listed `internal/auth/`, `internal/channel/`, etc. Refactored to `service/`, `handler/`, `middleware/`, `router/` layers with chi router. Future features (admin, embed, voice) will add files to existing layers rather than creating new top-level packages.

---

## Notes for Next Run

- Postgres is running on port 5440, all migrations applied through 000010
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
- User colors stored in DB `users.color` column (nullable, `VARCHAR(7)`); fallback to client-side hash when NULL; shared utility in `$lib/utils.ts`
- Three-column layout: ChannelSidebar (w-60) | MessageArea (flex-1) | MemberList (w-60)
- Admin panel at `/admin` — admin-only, 4 tabs (users, channels, messages, settings)
- Admin settings (open_registration, instance_name) are in-memory only — reset on server restart
- Admin routes: `/api/admin/users`, `/api/admin/users/{id}/admin`, `/api/admin/users/{id}/reset-password`, `/api/admin/users/{id}` (DELETE), `/api/admin/stats`, `/api/admin/messages/cleanup`, `/api/admin/settings`

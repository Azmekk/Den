<p align="center">
  <img src="assets/den_logo.png" alt="Den" width="120" />
</p>

<h1 align="center">Den</h1>
<p align="center">A self-hostable chat &amp; voice platform for small communities.</p>

<p align="center">
  <a href="https://github.com/Azmekk/den/stargazers"><img src="https://img.shields.io/github/stars/Azmekk/den?style=flat" alt="Stars" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-source--available-blue" alt="License" /></a>
  <a href="https://github.com/Azmekk/den/pkgs/container/den"><img src="https://img.shields.io/badge/ghcr.io-den-blue?logo=docker" alt="GHCR" /></a>
</p>

---

## Features

- **Text channels** — real-time messaging with mentions, emotes, message pinning, and replies
- **Voice channels** — built-in WebRTC SFU with screen sharing support
- **Custom emotes** — upload and use custom emotes across channels
- **Admin panel** — manage users, channels, and instance settings
- **Desktop app** — native Electron client with auto-updates, tray support, and notifications
- **Simple self-hosting** — single Docker Compose file, minimal configuration

---

## Self-Hosting

### Quick Start

```sh
mkdir den && cd den
curl -LO https://raw.githubusercontent.com/Azmekk/den/master/docker-compose.yml \
     -LO https://raw.githubusercontent.com/Azmekk/den/master/.env.example
cp .env.example .env
# Edit .env — set Supabase credentials and Postgres password
docker compose up -d
```

Open `http://localhost:8080` — the first registered user becomes admin.

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | — | PostgreSQL connection string |
| `APP_PORT` | No | `8080` | HTTP listen port |
| `JWT_SECRET` | Yes | — | Secret for signing auth tokens |
| `MAX_MESSAGES` | No | `50` | Messages returned per page |
| `OPEN_REGISTRATION` | No | `true` | Allow public registration |
| `STUN_SERVERS` | No | `stun:stun.l.google.com:19302` | STUN server URLs (comma-separated) |
| `TURN_URL` | No | — | TURN server URL (for NAT traversal) |
| `TURN_USERNAME` | No | — | TURN server username |
| `TURN_CREDENTIAL` | No | — | TURN server credential |
| `BUCKET_ENDPOINT` | No | — | S3-compatible endpoint (enables uploads) |
| `BUCKET_NAME` | No | — | Bucket name |
| `BUCKET_REGION` | No | — | Bucket region |
| `BUCKET_ACCESS_KEY` | No | — | Bucket access key |
| `BUCKET_SECRET_KEY` | No | — | Bucket secret key |
| `BUCKET_PUBLIC_URL` | No | — | Public URL for serving uploaded files |

### Voice

Voice channels use a built-in WebRTC SFU (Selective Forwarding Unit) powered by Pion. Voice is enabled by default with Google's public STUN server. For production deployments behind NAT, configure a TURN server via the `TURN_*` environment variables.

### File Uploads (S3)

Emote and media uploads require an S3-compatible bucket (AWS S3, Cloudflare R2, MinIO, etc.). Set the `BUCKET_*` environment variables to enable. Upload features are hidden in the UI when not configured.

---

## Desktop App

Download the latest installer from the [Releases](https://github.com/Azmekk/den/releases) page.

Available for **Windows** (.exe), **macOS** (.dmg), and **Linux** (.AppImage, .deb).

The desktop app includes auto-updates — you'll be notified when a new version is available.

---

## Development

### Prerequisites

- [Go 1.23+](https://go.dev/)
- [Bun](https://bun.sh/)
- [Docker](https://www.docker.com/)

### Local Setup

```sh
# Start Postgres
docker compose up -d postgres

# Run migrations
migrate -path src/db/migrations \
  -database "postgres://den:changeme@localhost:5440/den?sslmode=disable" up

# Build & run
cd src/web && bun install && bun run build && cd ../..
cd src && go run .
```

---

If you find Den useful, consider giving it a **star** — it helps others discover the project.

## License

Den is [source-available](LICENSE). Free for personal use and self-hosting. See the LICENSE file for details.

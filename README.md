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
- **Voice channels** — powered by LiveKit with screen sharing support
- **Custom emotes** — upload and use custom emotes across channels
- **Admin panel** — manage users, channels, and instance settings
- **Desktop app** — native Electron client with auto-updates, tray support, and notifications
- **Simple self-hosting** — single Docker Compose file, minimal configuration

---

## Self-Hosting

### Quick Start

Create a `docker-compose.yml`:

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: den
      POSTGRES_PASSWORD: changeme
      POSTGRES_DB: den
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U den"]
      interval: 5s
      timeout: 5s
      retries: 5

  app:
    image: ghcr.io/azmekk/den:latest
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://den:changeme@postgres:5432/den?sslmode=disable
      APP_PORT: "8080"
      JWT_SECRET: change-me-in-production
    depends_on:
      postgres:
        condition: service_healthy

volumes:
  pgdata:
```

```sh
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
| `LIVEKIT_API_KEY` | No | — | LiveKit API key (enables voice) |
| `LIVEKIT_API_SECRET` | No | — | LiveKit API secret |
| `LIVEKIT_URL` | No | — | LiveKit server WebSocket URL |
| `BUCKET_ENDPOINT` | No | — | S3-compatible endpoint (enables uploads) |
| `BUCKET_NAME` | No | — | Bucket name |
| `BUCKET_REGION` | No | — | Bucket region |
| `BUCKET_ACCESS_KEY` | No | — | Bucket access key |
| `BUCKET_SECRET_KEY` | No | — | Bucket secret key |
| `BUCKET_PUBLIC_URL` | No | — | Public URL for serving uploaded files |

### Voice (LiveKit)

To enable voice channels, add a [LiveKit](https://livekit.io/) server and set the `LIVEKIT_*` environment variables. See the development `docker-compose.yml` for a working example with a self-hosted LiveKit instance.

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

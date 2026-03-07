# Den

A self-hostable chat and voice platform.

## Tech Stack

- **Backend:** Go
- **Frontend:** SvelteKit (static SPA)
- **Database:** PostgreSQL
- **Voice:** LiveKit
- **Proxy:** Nginx

## Getting Started

### Prerequisites

- [Go 1.25+](https://go.dev/)
- [Bun](https://bun.sh/)
- [Docker](https://www.docker.com/)

### Setup

1. Copy the example env file:
   ```sh
   cp .env.example .env
   ```

2. Start Postgres:
   ```sh
   docker compose up -d postgres
   ```

3. Run migrations:
   ```sh
   docker run --rm -v "$(pwd)/src/db/migrations:/migrations" migrate/migrate \
     -path=/migrations -database "postgres://den:changeme@localhost:5440/den?sslmode=disable" up
   ```

4. Build the frontend:
   ```sh
   cd src/web && bun install && bun run build
   ```

5. Run the server:
   ```sh
   go run ./src/cmd/server
   ```

### Full Stack (Docker)

```sh
docker compose up -d
```

This starts Postgres, LiveKit, the app, and Nginx.

## Project Structure

```
src/
  cmd/server/       # Go entrypoint
  internal/         # Go packages (auth, channel, message, dm, voice, admin)
  db/
    migrations/     # PostgreSQL migrations (golang-migrate)
    queries/        # sqlc queries
  web/              # SvelteKit frontend
```

## License

MIT

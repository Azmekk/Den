# Den — Development Commands

## Database Migrations

All migration commands use the `migrate/migrate` Docker image. Prefix with `MSYS_NO_PATHCONV=1` to prevent Git Bash path mangling.

**Run all up migrations:**
```sh
MSYS_NO_PATHCONV=1 docker run --rm -v "$(pwd)/src/db/migrations:/migrations" migrate/migrate -path=/migrations -database "postgres://den:changeme@host.docker.internal:5440/den?sslmode=disable" up
```

**Roll back one migration:**
```sh
MSYS_NO_PATHCONV=1 docker run --rm -v "$(pwd)/src/db/migrations:/migrations" migrate/migrate -path=/migrations -database "postgres://den:changeme@host.docker.internal:5440/den?sslmode=disable" down 1
```

**Roll back all migrations:**
```sh
MSYS_NO_PATHCONV=1 docker run --rm -v "$(pwd)/src/db/migrations:/migrations" migrate/migrate -path=/migrations -database "postgres://den:changeme@host.docker.internal:5440/den?sslmode=disable" down -all
```

## Build

**Frontend:**
```sh
cd src/web && bun run build
```

**Backend:**
```sh
go build -o den ./src/cmd/server
```

## Dev Server

```sh
go run ./src/cmd/server
```

## Docker

```sh
docker compose up -d postgres   # Start Postgres (port 5440)
docker compose up -d            # Start all services
docker compose down             # Stop everything
```

## Project Structure

- All source code lives under `src/`
- Go backend: `src/cmd/server/`, `src/internal/`
- SvelteKit frontend: `src/web/`
- DB migrations: `src/db/migrations/`
- sqlc queries: `src/db/queries/`
- Infrastructure configs at project root

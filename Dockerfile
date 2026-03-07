FROM oven/bun:1 AS frontend
WORKDIR /app/src/web
COPY src/web/package.json src/web/bun.lock* ./
RUN bun install --frozen-lockfile
COPY src/web/ .
RUN bun run build

FROM golang:1.23-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/src/web/build ./src/web/build
RUN CGO_ENABLED=0 go build -o /den ./src/cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=backend /den /den
COPY src/db/migrations /migrations
EXPOSE 8080
CMD ["/den"]

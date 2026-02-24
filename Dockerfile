# ---- Build stage ----
FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o /replog ./cmd/replog

# ---- Runtime stage ----
FROM gcr.io/distroless/static-debian12

COPY --from=builder /replog /replog

EXPOSE 8080

# Persistent data: SQLite database and avatar uploads.
VOLUME ["/data"]

ENV REPLOG_DB_PATH=/data/replog.db
ENV REPLOG_AVATAR_DIR=/data/avatars
ENV REPLOG_ADDR=:8080

ENTRYPOINT ["/replog"]

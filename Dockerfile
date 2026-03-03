# ---- Build stage ----
FROM golang:1.25-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
    -trimpath -o /replog ./cmd/replog

# ---- Runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /replog /replog

EXPOSE 8080

# Persistent data: SQLite database and avatar uploads.
VOLUME ["/data"]

ENV REPLOG_DB_PATH=/data/replog.db
ENV REPLOG_AVATAR_DIR=/data/avatars
ENV REPLOG_ADDR=:8080

USER nonroot:nonroot
ENTRYPOINT ["/replog"]

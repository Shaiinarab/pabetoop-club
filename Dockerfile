# syntax=docker/dockerfile:1
# Builds a single static binary that embeds the PocketBase framework.
# The Go version here must match the `go` directive in go.mod.
FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/pabetoop-club ./cmd/app

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S pabetoop \
    && adduser -S -D -H -G pabetoop pabetoop \
    && mkdir -p /app/pb_data /app/assets \
    && chown -R pabetoop:pabetoop /app

WORKDIR /app
COPY --from=builder --chown=pabetoop:pabetoop /out/pabetoop-club /app/pabetoop-club
COPY --chown=pabetoop:pabetoop assets /app/assets

USER pabetoop
VOLUME ["/app/pb_data"]
EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --spider -q http://127.0.0.1:8090/_healthz || exit 1

ENTRYPOINT ["/app/pabetoop-club", "serve", "--http=0.0.0.0:8090"]

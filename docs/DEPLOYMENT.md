# Deployment

The platform is one binary plus one directory of state (`pb_data/`). Everything
below assumes a single Linux host with Docker, or a single host with systemd if you
prefer no container runtime.

## 1. Before you start

- A domain pointing at the host, with TLS already terminable (Caddy or Nginx).
- Go 1.26+ to build, or Docker to build the image.
- A `.env` derived from `.env.example` with the `SITE_*` block filled in and
  `APP_ENV=production`, `SEED_DEMO=0`, `STUB=0`.

## 2. Docker Compose

```bash
cp .env.example .env        # edit the SITE_* block and payment settings
docker compose up -d --build
docker compose ps           # expect: healthy
curl -sf http://127.0.0.1:8090/_healthz && echo ok
```

State lives in the named volume `pabetoop_club_pb_data`, mounted at `/app/pb_data`.
The container runs as an unprivileged user with `no-new-privileges`.

## 3. systemd (no container runtime)

```ini
# /etc/systemd/system/pabetoop-club.service
[Unit]
Description=Pabetoop Club
After=network-online.target

[Service]
User=pabetoop
WorkingDirectory=/srv/pabetoop-club
EnvironmentFile=/srv/pabetoop-club/.env
Environment=APP_ENV=production
ExecStart=/srv/pabetoop-club/pabetoop-club serve --http=127.0.0.1:8090
Restart=on-failure
RestartSec=3
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ReadWritePaths=/srv/pabetoop-club/pb_data
ProtectHome=yes

[Install]
WantedBy=multi-user.target
```

```bash
go build -trimpath -ldflags="-s -w" -o /srv/pabetoop-club/pabetoop-club ./cmd/app
sudo systemctl daemon-reload && sudo systemctl enable --now pabetoop-club
```

Binding to `127.0.0.1` means only the reverse proxy can reach it — intended.

## 4. Reverse proxy

Caddy, minimal and automatic-TLS:

```caddyfile
club.example.org {
    encode zstd gzip
    header Strict-Transport-Security "max-age=31536000; includeSubDomains"
    reverse_proxy 127.0.0.1:8090
}
```

Nginx equivalent — the two lines that matter are the proxy pass and HSTS:

```nginx
server {
    listen 443 ssl http2;
    server_name club.example.org;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Proxy concerns the application deliberately does not handle: TLS, HSTS, request-size
limits, and coarse rate limiting.

## 5. Backups

The database is a single SQLite file in WAL mode. Take a consistent copy with the
SQLite backup API rather than `cp`:

```bash
sqlite3 pb_data/data.db ".backup '/backup/pabetoop-$(date +%F).db'"
```

Or copy the whole state directory read-only while the service is stopped, then zip it:

```bash
docker compose stop app
docker run --rm -v pabetoop_club_pb_data:/data:ro -v "$PWD/backups:/backup" alpine \
  tar czf /backup/pabetoop-$(date +%F).tgz -C /data .
docker compose start app
```

### Verify every backup

`tools/backup-check` is a standalone Rust utility that opens a ZIP, checks its
SHA-256 manifest, and confirms a PocketBase database is actually present. An
unverified archive is not a backup.

```bash
cd tools/backup-check && cargo build --release
./target/release/backup-check /path/to/backups/pabetoop-YYYY-MM-DD.zip
```

### Restore drill

Run this once before you need it, not during an incident:

1. `docker compose down` (keep the volume for now).
2. Move the current volume aside or copy the old `pb_data/` to `pb_data.old/`.
3. Extract the archive into a fresh `pb_data/`.
4. `docker compose up -d`, check `/_healthz`, log in as manager, open a member.
5. Only then delete `pb_data.old/`.

## 6. Upgrades

```bash
git pull
docker compose build && docker compose up -d
```

Schema changes are applied at bootstrap, so no manual migration step is required.
**Back up before every upgrade.** Roll back by checking out the previous tag and
rebuilding — a database written by a newer schema may not open under an older binary,
which is exactly why the backup comes first.

## 7. Verification gates

| Gate | Command |
|---|---|
| Unit and lifecycle tests | `go test ./...` |
| Full synthetic deployment | `docker compose -f docker-compose.test.yml up -d --build --wait` |
| End-to-end manager + guardian flow | `./scripts/test-deploy-smoke.sh` |
| Branding leak check | `go test ./internal/app -run Brand` |
| Liveness | `curl -sf http://127.0.0.1:8090/_healthz` |

## 8. Monitoring the minimum

- `/_healthz` must return 200 — this is what the container healthcheck uses.
- Watch free disk on the volume: SQLite WAL growth is the usual first symptom.
- Read the reminder scan's output once a week; it is the early warning for fee arrears.

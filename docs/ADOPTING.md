# Adopting this template

From "Use this template" to a running membership platform for *your* club or academy.
Every command below is the verified path for this repo — copy-paste order, not a sketch.

- **§1** re-brand it (one command, ~30 s)
- **§2** run it locally and see it serve your name
- **§3** members, subscriptions and payments
- **§4** deploy it
- **§5** what not to change
- **§6** the traps that actually bite

---

## 0. What "tagless" means here

No club, city or operator name is compiled in. There are two layers:

| Layer | What it is | Where it lives |
| --- | --- | --- |
| **Structural** | Go module path, binary name, image names, compose volumes | rewritten by `tools/instantiate.sh` |
| **Cosmetic** | The name the UI, the PWA manifest and receipts render | `.env` → read by `internal/brand` at startup |

The PWA manifest is **rendered per request** from the active brand profile, not served as a static
file. That matters: a re-branded deployment used to be able to install onto a family's phone still
under the previous club's name.

This template ships a test that **fails the build** if a client identifier reappears in the tree
(`internal/app/branding_test.go`). Leave it in place.

---

## 1. Re-brand it

Create your repo from the template (the green **Use this template** button), clone it, then:

```bash
./tools/instantiate.sh \
  --name "Riverside FC" \
  --name-fa "باشگاه فوتبال رودساید" \
  --short-fa "ر" \
  --discipline-fa "مدرسه فوتبال" \
  --tagline-fa "از پایه تا قهرمانی" \
  --module-path github.com/riverside/riverside-club \
  --binary riverside-club
```

`--module-path` defaults to the current one and `--binary` to the last path segment of the module.
`--dry-run` prints the plan and writes nothing.

The script refuses to run on a dirty tree (so a bad run is `git checkout .`), then **verifies its own
work**: it builds the tree and runs the branding leak test.

Afterwards it writes **gitignored** `.env` from `.env.example` carrying your names. Two cosmetic
things it deliberately leaves for you:

1. `assets/icons/icon-{192,512}.svg` are generated monograms — replace them with your crest.
2. `SITE_CITY_FA` is empty by default. Set it to put a city in the landing-page strapline, or leave
   it empty to omit that clause entirely.

---

## 2. Run it locally (from source)

```bash
cp .env.example .env        # already done by instantiate.sh, if you used it
go mod download
make run                    # loads .env, then starts :8090 in stub mode
```

- manager → <http://127.0.0.1:8090/manager>
- family portal → <http://127.0.0.1:8090/portal>
- health → <http://127.0.0.1:8090/_healthz>

`make run` sources `tools/load-env.sh`, so the branding you put in `.env` really reaches the process.
An explicitly exported variable still wins over `.env`, so `PORT=9000 make run` does what you expect.

To run the binary yourself:

```bash
ENV_FILE=./.env . ./tools/load-env.sh && SEED_DEMO=1 STUB=1 go run ./cmd/app serve --http=127.0.0.1:8090
```

### Confirm it is serving *your* brand

The manifest is the cheapest proof, because it is rendered from the brand profile:

```bash
curl -s http://127.0.0.1:8090/assets/manifest.webmanifest
```

If it still carries the template's name rather than yours, `.env` is not being read — see §6.

### The verification gate

```bash
make test                                        # Go tests + strict TS type-check
make test-up && make test-smoke && make test-down # end-to-end manager + guardian flow
```

`make test-smoke` runs the real deployment (isolated compose project, synthetic data) through a
manager and a guardian workflow. A green `go build` + `go vet` proves none of that — see §6.

---

## 3. Members, subscriptions and payments

### Try it with demo data

```bash
SEED_DEMO=1 make run
```

Creates 180 fictitious players, a demo manager and demo guardians — **never** combine this with real
member data. Credentials:

| Account | Login | Password |
| --- | --- | --- |
| Manager | `manager@example.test` | `ChangeMe123!` |
| Guardian 1 | mobile `09170000001` | `ChangeMe123!` |
| Guardians 2–3 | `0917000000{2,3}` | `DemoFamily0{2,3}!` |

### Turning payments on for real

Payments ship **stubbed**: `STUB=1`, `PAYMENT_PROVIDER=test`. The test provider moves no real money
and exists so the whole lifecycle — invoice, payment, receipt, reminder — is exercisable offline.

Keep `STUB=1` until **all three** are true:

1. you have provider credentials (`ZARINPAL_MERCHANT_ID`),
2. you have a **public HTTPS** callback URL (`ZARINPAL_CALLBACK_BASE_URL`),
3. the provider has approved your production access (`ZARINPAL_SANDBOX=0`).

Then set `STUB=0`. Flip it back the moment anything is uncertain; the stub path is not a mode of
last resort, it is a supported configuration.

### Reminders

Invoice reminders are driven by an automation hook, not a cron job you have to install. It runs on
the same binary. Nothing to schedule, nothing to forget to restart.

---

## 4. Deploy it

### Docker Compose

```bash
cp .env.example .env      # then edit the SITE_* block
docker compose up -d --build
```

### What production needs

- **`APP_ENV=production`** and a real `SITE_NAME_FA`. `SEED_DEMO=0`, `STUB` per above.
- **Backups.** PocketBase keeps everything in `pb_data/`. Its admin UI has a one-click download;
  from the CLI, stop the app and archive the directory — or use PocketBase's own backup zip.
- **Verify the backup, don't assume it.** `tools/backup-check` is a standalone Rust utility that
  opens a PocketBase backup zip and confirms it actually contains the database entry, that the entry
  count is sane, and prints a SHA-256:

  ```bash
  make backup-check                       # build it
  ./tools/backup-check/target/release/backup-check pb_data-backup.zip
  ```

  Run it in your backup job. A backup that has never been opened is a hypothesis.
- **TLS in front**, and keep the container non-root — the image already does.

### Health

`GET /_healthz` returns 200 when the server is up. `make health` curls it.

---

## 5. What not to change

- **The stack is pinned** — Go 1.26 + PocketBase 0.39, `html/template`, htmx 4, TypeScript 7 (`tsgo`),
  Bun/NPM for the client. Do not "downgrade for safety".
- **`internal/app/app.go` route registration.** PocketBase builds its mux from these registrations;
  see §6 trap 2. Every path is registered exactly once, and `internal/app/routes_test.go` enforces it.
- **`internal/app/branding_test.go`** — leave it. It is your de-identification net.
- **Never commit `.env` or `pb_data/`.** Both are gitignored; keep it that way.
- **The client is a separate build.** `client/` is a small TypeScript project; `make client` installs
  it, `make dev` watches it. The Go server runs without it (stub mode), which is why the Go tests
  never need `node_modules`.

---

## 6. The traps that actually bite

Each of these cost a real bug in this codebase's history.

**1. `make run` silently doing nothing.**
The binary is a PocketBase app: `./pabetoop-club` with **no subcommand** prints root help and exits 0.
It has to be `... serve --http=127.0.0.1:8090`. The Makefile target already does this — but if you
invoke the binary yourself, don't forget the subcommand. It looks like a working command: it produces
output and reports no error.

**2. A duplicate route registration panics on the first request, not at build time.**
`net/http`'s ServeMux panics on a duplicate pattern *while the mux is being built*, which is on the
first request — the health endpoint included. PocketBase catches that panic, so instead of dying
loudly the app limps on with the route unregistered. Build, vet and every other test stay green.

`internal/app/routes_test.go` fails on any duplicated method+path. Run it before you add a route:

```bash
go test ./internal/app -run TestNoDuplicateRouteRegistrations -v
```

**3. Sourcing `.env` breaks on any value containing a space.**
`. ./.env` makes the shell try to *execute* the second word: `SITE_NAME=Riverside FC` leaves
`SITE_NAME=Riverside` and runs `FC`. Use `tools/load-env.sh` (what `make run` does), or export the
values inline.

**4. A script with a shebang but no executable bit.**
Running `./tools/whatever.sh` the way the docs say gives "Permission denied". If you add one:

```bash
chmod +x tools/yours.sh && git update-index --chmod=+x tools/yours.sh
```

**5. A global proxy eats localhost.**
On a host with `HTTPS_PROXY` set, requests to `127.0.0.1` get captured and fail mysteriously. Always
`export NO_PROXY=127.0.0.1,localhost`. `make run`, `make health` and the smoke script set this for
themselves.

**6. "It builds" is not "it runs".**
Both failures above were invisible to `go build` and `go vet`. Boot it and read the bytes.

---

## 7. Go-live checklist

- [ ] `./tools/instantiate.sh` ran and exited 0
- [ ] `curl -s http://<host>/assets/manifest.webmanifest` shows **your** club name
- [ ] your crest replaces `assets/icons/icon-{192,512}.svg`
- [ ] `make test` green
- [ ] `make test-up && make test-smoke && make test-down` green
- [ ] `APP_ENV=production`, `SEED_DEMO=0`
- [ ] `STUB=0` only after provider credentials + public HTTPS callback + approval are all in place
- [ ] demo accounts disabled or deleted (`manager@example.test` first)
- [ ] `pb_data/` in your backup schedule, and `backup-check` run against a real backup
- [ ] TLS terminated somewhere in front
- [ ] `.env` and `pb_data/` not committed

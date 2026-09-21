# Pabetoop Club

**A white-label membership, subscription and billing platform for sports clubs and
academies.** One Go binary, one SQLite file, a Persian RTL web UI, and no club name
anywhere in the source.

[![CI](https://github.com/shaiinarab/pabetoop-club/actions/workflows/ci.yml/badge.svg)](https://github.com/shaiinarab/pabetoop-club/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-blue)

> **Taking this live for your own club?** → [`docs/ADOPTING.md`](docs/ADOPTING.md) walks the whole
> path end to end: re-brand it in one command, run it locally, put your members and payments in,
> deploy it, and the traps that actually bite.

---

## What problem this solves

A small academy — 50 to a few hundred members — runs on a spreadsheet and a chat
group. Fees get chased by hand, guardians phone the office to ask whether a payment
landed, and nobody can answer "which members are overdue?" without opening the sheet.

Pabetoop Club is the smallest system that actually fixes that:

- **Guardians** get a portal showing their player's status, subscription, invoices
  and receipts — installable as a PWA on the phone they already have.
- **The manager** gets one dashboard: search members, attach a guardian to a player,
  define fee plans, issue invoices, take payment, and see the audit trail.
- **The deployment owner** gets a single static binary with an embedded database.
  No Node runtime in production, no ORM, no microservices, no paid SaaS.

It is deliberately scoped to *membership and money*. It is not an ERP.

## Features

| Area | Status | Notes |
|---|---|---|
| Persian RTL UI | ✅ | Server-rendered HTML; no client framework required to operate |
| Family portal | ✅ | Player status, subscription window, invoices, test payments |
| Manager dashboard | ✅ | Member search (htmx), guardian linking, plan management |
| Fee plans & subscriptions | ✅ | Define a plan, activate a subscription, replace a prior one |
| Invoicing | ✅ | Issued only to a guardian already linked to the player |
| Payment lifecycle | ✅ | invoice → payment → verify → settle, with an audit trail |
| Zarinpal adapter | ⚙️ | Request/callback/verify implemented; needs merchant ID + public HTTPS |
| Dunning scan | ✅ | In-process daily scan, actionable queue, audited — sends nothing by itself |
| Audit log | ✅ | Every privileged mutation is recorded with actor and target |
| Security | ✅ | HttpOnly/SameSite cookies, CSRF tokens, login rate limiting, CSP-class headers |
| PWA | ✅ | Manifest generated per deployment; sensitive routes never cached |
| Backup integrity | ✅ | Standalone Rust tool: unzips, SHA-256 checks, confirms the PocketBase DB |
| Demo seed | ✅ | `SEED_DEMO=1` → 180 fictitious players, zero real data |
| White-label | ✅ | Six environment variables re-brand the entire system |
| GPS load / ACWR analytics | 🔬 | Tested maths in `internal/domain`; not wired to routes yet |

Legend: ✅ shipped · ⚙️ shipped, needs credentials · 🔬 extension kit, see `internal/README.md`

## Architecture

One process. The HTTP layer talks to the PocketBase framework embedded *in the same
binary*, which owns the SQLite file. Every privileged write goes through a Go handler
that authorizes first — the browser never gets direct data-API access.

```text
        Guardian / Manager browser
                   │  HTTPS · HttpOnly session · CSRF token
                   ▼
        ┌──────────────────────────────────────────┐
        │  Go single binary (cmd/app)              │
        │                                          │
        │  internal/app      routes, auth, billing │
        │  internal/brand    white-label identity  │  ← every user-facing string
        │  internal/middleware  (optional) RBAC    │
        │  internal/adapters    (optional) ports   │
        │  internal/domain      (optional) load    │
        │                                          │
        │  PocketBase 0.39 (embedded framework)    │
        │    schema · hooks · records · migrations │
        └────────────────┬─────────────────────────┘
                         ▼
                  pb_data/data.db   (SQLite, WAL)
```

Design rules that keep it maintainable:

1. **Server-rendered first.** htmx is vendored for partial swaps; there is no SPA to keep in sync.
2. **Deny by default.** Public routes read aggregate counts only; operational data needs a session.
3. **One identity surface.** `internal/brand` is the only place a club name can live, and a
   test asserts no former name can reappear in rendered HTML (`internal/app/branding_test.go`).
4. **Pure maths stays pure.** Load/readiness calculations live in dependency-free packages
   so they stay unit-testable.

More detail: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Quickstart

### Docker (recommended)

```bash
cp .env.example .env      # then edit the SITE_* block, see Re-branding below
docker compose up -d --build
# manager  → http://127.0.0.1:8090/manager   (SEED_DEMO=1 creates manager@example.test)
# portal   → http://127.0.0.1:8090/portal
# health   → http://127.0.0.1:8090/_healthz
```

### From source

Requires Go 1.26+ (Node 22+ only if you touch `client/`).

```bash
cp .env.example .env        # same file the Docker path reads — edit the SITE_* block
go mod download
make run                    # loads .env, then starts :8090 in stub mode
```

`make run` sources `tools/load-env.sh`, so the branding you put in `.env` actually reaches the
process. Do not shortcut this with `. ./.env` — that is a trap for any value containing a space:
`SITE_NAME=Riverside FC` makes the shell try to run `FC`, and the variable silently becomes
`Riverside`. To run the binary yourself:

```bash
ENV_FILE=./.env . ./tools/load-env.sh && SEED_DEMO=1 STUB=1 go run ./cmd/app serve --http=127.0.0.1:8090
```

### Verify it works

```bash
go test ./...                                   # unit + lifecycle tests
docker compose -f docker-compose.test.yml up -d --build --wait
./scripts/test-deploy-smoke.sh                  # end-to-end manager + guardian flow
docker compose -f docker-compose.test.yml down -v
```

## Configuration

Everything is environment-driven; the full annotated list is in [`.env.example`](.env.example).

| Variable | Default | Purpose |
|---|---|---|
| `SITE_NAME` | `Pabetoop Club` | Latin name (manifests, receipts, logs) |
| `SITE_NAME_FA` | `باشگاه پابهتوپ` | Persian display name in the UI |
| `SITE_SHORT_FA` | `پ` | Header monogram |
| `SITE_DISCIPLINE_FA` | `مدرسه فوتبال` | Discipline noun; empty renders the bare name |
| `SITE_CITY_FA` | *(empty)* | Hero strapline city; empty omits it |
| `SITE_TAGLINE_FA` | neutral | Hero strapline |
| `APP_ENV` | `development` | `production` hides the startup banner |
| `SEED_DEMO` | `0` | `1` creates 180 fictitious members |
| `STUB` | `1` | `1` keeps the deterministic payment double |
| `PAYMENT_PROVIDER` | `test` | `test` or the live gateway adapter |
| `ZARINPAL_MERCHANT_ID` | *(empty)* | Required for the live adapter |
| `ZARINPAL_SANDBOX` | `1` | Keep `1` until production approval |
| `ZARINPAL_CALLBACK_BASE_URL` | `http://localhost:8090` | Must be publicly reachable in production |

## Re-branding a fork

The platform ships **tagless**: no club, city or operator name is compiled in. To
stand up a copy for a different academy:

```bash
./tools/instantiate.sh --name "Riverside FC" --name-fa "باشگاه رودخانه" \
                       --short-fa "ر" --city-fa "شهر نمونه" --module-path github.com/you/riverside
```

The script rewrites the Go module path, the binary/service name and the `.env` brand
block, then runs the leak test. See [`docs/BRANDING.md`](docs/BRANDING.md) for the
full contract and how to add a locale.

## Security posture

- Session cookies are `HttpOnly` + `SameSite`; CSRF tokens are required on every mutation.
- Login attempts are rate-limited per identifier.
- Public routes expose aggregate counts only — never operational records.
- Browser writes to PocketBase collections are disabled; all mutations are server-side.
- The PWA service worker never caches `/portal`, `/manager`, `/pay/*` or `/payments/*`.
- Secrets come from the environment; `.env` is git-ignored and there is no secret in the repo.

This is a small-team platform, not a regulated financial system. Threat model and
residual risks: [`docs/SECURITY.md`](docs/SECURITY.md).

## Documentation map

| Document | Contents |
|---|---|
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Processes, packages, request flow, extension points |
| [`docs/BRANDING.md`](docs/BRANDING.md) | The white-label contract and how to add a locale |
| [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) | Docker, systemd, reverse proxy, backups, restore drill |
| [`docs/SECURITY.md`](docs/SECURITY.md) | Threat model, controls, residual risk |
| [`internal/README.md`](internal/README.md) | Package map with wired vs. optional status |
| [`AGENTS.md`](AGENTS.md) | Working contract for AI agents in this repo |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Development workflow and review gates |

## خلاصهٔ فارسی

پابه‌توپ کلاب یک پلتفرم **بدون‌نام تجاری (white-label)** برای مدیریت باشگاه‌ها و
مدرسه‌های ورزشی است: ثبت‌نام عضو، اتصال خانواده به ورزشکار، طرح شهریه، صدور صورتحساب،
پرداخت و پورتال نصب‌شدنی خانواده. کل سامانه یک باینری Go با پایگاه‌دادهٔ SQLite است و
رابط کاربری فارسی و راست‌به‌چپ دارد.

نام باشگاه، شهر و رشتهٔ ورزشی **هیچ‌کدام در کد نوشته نشده‌اند**؛ با شش متغیر محیطی
(`SITE_*`) می‌توانید کل سامانه را برای هر باشگاهی سفارشی کنید. راهنمای کامل در
`docs/BRANDING.md`.

## License

MIT — see [`LICENSE`](LICENSE).

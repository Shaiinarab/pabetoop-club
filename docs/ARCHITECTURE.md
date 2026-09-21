# Architecture

## 1. Shape of the system

One static Go binary. There is no separate application server, no reverse-proxy
routing table between services, and no database server:

```text
cmd/app ──► internal/app ──► PocketBase (embedded) ──► SQLite file
                │
                ├── internal/brand       identity strings (env-driven)
                ├── internal/middleware  optional RBAC helpers
                ├── internal/adapters    optional outbound ports
                └── internal/domain      optional pure calculations
```

PocketBase is used as a **library**, not as a service: `pocketbase.NewWithConfig`
gives us a router, a record layer, migrations and a hook system, and we attach our
own HTTP routes to it. Nothing about PocketBase's own admin UI or data API is
exposed to members.

## 2. Request flow

A privileged mutation follows the same path every time:

```text
browser
  │  POST /manager/... (session cookie + csrf_token field)
  ▼
securityHeaders          CSP, Referrer-Policy, Permissions-Policy, COOP, nosniff
  ▼
route handler            (internal/app/*.go)
  ├─ validateCSRF        constant-time compare of cookie and form token
  ├─ session check       PocketBase auth record present and authorised
  ├─ business rule       e.g. "invoice only for a linked guardian"
  ├─ app.Save(record)    record layer
  └─ writeAudit()        actor, action, target, metadata
  ▼
HTML fragment or redirect
```

Two consequences worth preserving:

- **Authorisation happens in Go, before the write.** Collections have browser writes
  disabled, so a handler is the only way in and the only place a rule can be forgotten.
- **Every privileged mutation is auditable.** `writeAudit` is called from the same
  function that performs the write, so a new privileged route that forgets the audit
  call is visible in review as an outlier.

## 3. Packages

| Package | Wired? | Responsibility |
|---|---|---|
| `cmd/app` | ✅ | Process entrypoint; calls `app.Run()`. |
| `internal/app` | ✅ | Schema, hooks, routes, auth, billing, payments, reminders, audit. |
| `internal/brand` | ✅ | The single source of every organisation-facing string. |
| `internal/middleware` | 🔬 | Role-based access control helpers (`RequireAuth`). |
| `internal/adapters` | 🔬 | Ports for payments, SMS and GPS ingestion, plus deterministic stubs. |
| `internal/domain` | 🔬 | Pure training-load maths (EWMA-based ACWR). |

🔬 packages compile and are tested, but the shipped routes do not call them yet.
They are **extension kits**: adopt them by wiring a route, or delete them. See
[`../internal/README.md`](../internal/README.md).

## 4. Data model

Created programmatically at bootstrap (`ensureSchoolCollections`), so a fresh
deploy needs no manual schema setup.

| Collection | Holds |
|---|---|
| `staff` | Auth records for managers; `role` distinguishes them |
| `guardians` | Auth records for family accounts; `full_name`, `mobile` |
| `players` | Member records; `first_name`, `last_name`, `status`, `date_of_birth` |
| `plans` | Fee plans: amount, period, label |
| `guardian_players` | Link table — which guardian may act for which player |
| `subscriptions` | A player on a plan for a window, with `status` |
| `invoices` | Amount, due date, `status` (`due`/`overdue`/`paid`) |
| `payments` | Attempts against invoices, with provider reference |
| `audit_log` | Append-only privileged-action trail |

Rules that hold everywhere:

1. An invoice can only be issued to a guardian **already linked** to the player.
2. Replacing a subscription closes the previous one rather than editing history.
3. Money is stored as integer rial — never floating point.
4. Timestamps are stored UTC; Jalali conversion happens at the UI boundary.

## 5. Rendering

HTML is produced by server-side functions in `internal/app` and rendered with the
active brand profile. htmx 4 is vendored under `assets/js/` and drives partial
swaps (member search, dashboard panels); it is an enhancement, not a dependency —
the pages work without it.

The client layer under `client/` holds `fa.ts` (Persian digit/date formatting) and
type declarations. It is optional: the app runs with no Node toolchain present.

## 6. Static assets and the PWA

- `assets/js/sw.js` is a hand-written service worker. It cache-firsts `/assets/*`
  and network-firsts everything else, and it **never** caches `/portal`, `/manager`,
  `/pay/*` or `/payments/*` — installing the app must not put fees or family data
  on disk.
- The manifest is no longer a static file. `manifestHandler` renders it from the
  brand profile on every request, so a re-branded deployment cannot ship the
  previous club's name into an installed app.

## 7. Extension points

| Goal | Where to start |
|---|---|
| Change any name, city or strapline | `internal/brand` + the `SITE_*` env block |
| Add a payment provider | `internal/adapters` interfaces, then a new case in the payment handler |
| Send SMS/email reminders | `internal/adapters` `SMSService`, called from the reminder scan |
| Add a role or restrict a route | `internal/middleware.RequireAuth` |
| Add load analytics | `internal/domain`, then a route that reads `gps_sessions` |
| Add a schema field | `ensureSchoolCollections` + a migration note in `db/migrations/` |

## 8. Deliberate non-goals

- No ORM, no service mesh, no message broker, no Redis.
- No multi-tenant hosting: one deployment serves one club. Re-branding is a fork,
  not a runtime tenant switch.
- No offline-first mutation queue. The PWA installs and surfaces read-only pages;
  writes always require the network.

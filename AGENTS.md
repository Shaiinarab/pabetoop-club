# AGENTS.md — working contract for this repository

This file is loaded automatically by agent tooling that supports `AGENTS.md`. Read
it before making changes.

## What this repository is

`pabetoop-club` is a **white-label template**. It is a Go + PocketBase membership,
subscription and billing platform for sports clubs and academies, with a Persian
RTL server-rendered UI.

Two properties define the project and both are enforced, not aspirational:

1. **Tagless.** No club, city, league or operator name is compiled in. Every
   organisation-facing string comes from `internal/brand`.
2. **Single-artifact.** One static binary plus one SQLite directory. No runtime
   dependency on Node, a database server, a queue or a paid SaaS.

## Commands

```bash
go build ./...                  # compile everything
go test ./...                   # unit + lifecycle tests
go vet ./...                    # static checks
gofmt -l . | grep -v vendor     # must print nothing
go test ./internal/app -run Brand   # the anti-leak branding gate

make build && STUB=1 ./pabetoop-club   # run locally on :8090 via Makefile

# full synthetic deployment
docker compose -f docker-compose.test.yml up -d --build --wait
./scripts/test-deploy-smoke.sh
docker compose -f docker-compose.test.yml down -v
```

Environment note: this module uses `-mod=mod` and needs network access for the first
`go mod download`. If a host has `GOFLAGS=-mod=vendor` set globally, override it per
command (`GOFLAGS=-mod=mod go build ./...`) rather than editing the host config.

## Hard rules

- **Never hard-code an organisation name.** Not in Go, HTML, SVG, the manifest or a
  fixture. `internal/app/branding_test.go` renders every page and fails on a legacy
  name; add new surfaces to `renderedPages()` when you add a page.
- **Authorise, then write, then audit.** Every privileged mutation calls
  `validateCSRF` (browser) or verifies the session, applies the business rule, saves,
  and writes an audit row. A privileged route with no audit call is a review failure.
- **Money is integer rial.** Never `float64`.
- **Timestamps are UTC in storage.** Persian/Jalali conversion happens at display time.
- **Public routes return aggregates only.** Anything that returns a member's record
  needs a session and an ownership check via the `guardian_players` link.
- **No new heavy dependency** without a reason recorded in the PR description. The
  whole point of this stack is that it stays small: stdlib `net/http`, PocketBase,
  htmx, and one calendar library.
- **No real personal data, ever** — not in tests, fixtures, screenshots or seeds. The
  demo seed is entirely fictitious and only pairs with the stub payment provider.
- **Delete dead code rather than commenting it out.** `internal/README.md` records
  the packages that are deliberately unwired extension kits; anything else that
  nothing imports should go.

## Editing the UI

The Persian copy lives in the page functions in `internal/app`. When you change
copy:

1. Read `internal/brand` first — if the string identifies the organisation, it
   belongs there as a profile field, not inline.
2. Keep the RTL layout intact (`dir="rtl"`, `lang="fa"`).
3. Keep Persian digits for display; store Latin digits.
4. Run the branding gate afterwards.

## Docs to keep in sync

| Change | Update |
|---|---|
| New env var | `.env.example` + the table in `README.md` |
| New route | `docs/ARCHITECTURE.md` request flow if privileged |
| New package wired/unwired | `internal/README.md` |
| New identity surface | `docs/BRANDING.md` §3 + `renderedPages()` |
| New security-relevant behaviour | `docs/SECURITY.md` |

## Review gate

Before calling any change complete:

- [ ] `go build ./...`, `go test ./...`, `go vet ./...` all pass
- [ ] `gofmt -l` is empty
- [ ] `go test ./internal/app -run Brand` passes
- [ ] Any new privileged route validates CSRF/session and writes an audit row
- [ ] No new hard-coded organisation name
- [ ] Docs above updated

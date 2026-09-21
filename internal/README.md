# Package map

Read the **Status** column before touching a package: this repository is a template,
and some packages are extension kits rather than live code.

| Package | Status | What it does |
|---|---|---|
| `cmd/app` | **wired** | Process entrypoint. Parses flags/env, calls `app.Run()`. |
| `internal/app` | **wired** | Everything that ships: schema creation, hooks, HTTP routes, sessions, CSRF, rate limiting, billing, payment lifecycle, reminder scan, audit log, demo seed, PWA manifest. |
| `internal/brand` | **wired** | The single source of every organisation-facing string. Read this before changing any user-visible copy. |
| `internal/middleware` | **optional** | `RequireAuth(allowed ...Role)` and `CurrentRole` for role-gated routes. Not currently referenced — the shipped manager routes check the session directly. Adopt it when you add a second role with different powers. |
| `internal/adapters` | **optional** | Interfaces for outbound services (`PaymentGateway`, `SMSService`, `GpsIngestor`) plus deterministic stubs. The live payment adapter lives in `internal/app`; these interfaces are here for the SMS and GPS work. |
| `internal/domain` | **optional** | Pure training-load maths: EWMA-based acute:chronic workload ratio. No PocketBase dependency, covered by its own tests. Wire it to a route to expose load analytics. |

## Wait — why does the template ship code nothing calls?

Because deleting a tested, dependency-free module is worse than labelling it. The
three **optional** packages are small, self-contained and honest about their state:
they compile, they are tested, and they are documented as unwired.

If you do not want them, delete them. Nothing in the shipped routes imports them, so
removal is `rm -rf` plus a `go mod tidy` and cannot break the running application.

## Conventions

- **Authorise before you write.** A handler checks the caller, then mutates, then
  writes an audit row. A privileged route without an audit call is a review failure.
- **Money is integer rial.** Never `float64`.
- **Timestamps are UTC.** Persian/Jalali formatting is a display concern.
- **Brand strings come from `internal/brand`.** A hard-coded name will fail
  `internal/app/branding_test.go`.
- **Public routes expose aggregates only.** If a route returns a record belonging to
  a member, it needs a session and an ownership check.

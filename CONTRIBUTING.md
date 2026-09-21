# Contributing

Thanks for looking. This is a small, opinionated platform; the fastest way to get a
change merged is to keep it small and prove it works.

## Setup

```bash
git clone https://github.com/shaiinarab/pabetoop-club
cd pabetoop-club
cp .env.example .env

go mod download
go test ./...                         # ~3s, no network, no database server
SEED_DEMO=1 STUB=1 go run ./cmd/app serve --http=127.0.0.1:8090
```

Open <http://127.0.0.1:8090>. The demo seed creates `manager@example.test`; the
smoke script and tests document the credentials they use.

Node 22+ is only needed if you change `client/` (TypeScript helpers). The Go
application never requires it.

## Before opening a pull request

```bash
go build ./...
go test ./...
go vet ./...
gofmt -l . | grep -v vendor           # must print nothing
go test ./internal/app -run Brand     # branding leak gate
```

For anything touching routes, auth or money, also run the end-to-end gate:

```bash
docker compose -f docker-compose.test.yml up -d --build --wait
./scripts/test-deploy-smoke.sh
docker compose -f docker-compose.test.yml down -v
```

## What a good change looks like

**Small.** One behaviour per pull request. A schema change plus a UI redesign is two
pull requests.

**Tested at the right level.** Business rules get unit tests next to the code. A new
privileged route gets a test that proves the unauthorised path is refused — the
refusal is the part most likely to regress.

**Auditable.** If it writes, it records. Follow the existing shape: validate CSRF or
session → check the business rule → `app.Save` → `writeAudit`.

**Tagless.** If your change introduces a club name, a city or an operator name, move
it into `internal/brand` as a profile field and add the surface to
`renderedPages()` in `internal/app/branding_test.go`.

**Honest about limits.** If something is not wired, not tested, or only works for one
provider, say so in `internal/README.md` or the relevant doc rather than leaving a
comment that will rot.

## Commit messages

Conventional-commit style, imperative mood, explaining *why*:

```
fix(billing): refuse invoice when guardian link was removed

The link table was checked at page render but not at submit, so a guardian
unlinked in another tab could still be invoiced.
```

## Reporting bugs and security issues

Bugs: open an issue with the route, what you expected, what happened, and the exact
command you ran.

Security: **do not open a public issue.** Use a private security advisory. See
[`docs/SECURITY.md`](docs/SECURITY.md) §6.

## Scope

In scope: membership, subscriptions, invoicing, payments, the family portal, the
manager dashboard, deployment ergonomics, and the extension kits.

Out of scope: multi-tenant hosting, offline write queues, an ORM, and anything that
adds a runtime service to the deployment. Those are deliberate non-goals — see
`docs/ARCHITECTURE.md` §8.

## License

By contributing you agree your contribution is licensed under the MIT License
(see [`LICENSE`](LICENSE)).

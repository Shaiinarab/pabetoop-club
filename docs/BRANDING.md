# Branding and white-labelling

This repository is a **template**: it must contain no club, city, league or
operator name. That is not a stylistic preference — a former deployment's identity
leaking into a template is a correctness bug, so it is enforced by a test.

## 1. The contract

Every organisation-facing string resolves through `internal/brand`:

```go
profile := brand.Active()
profile.TitleFA()          // "مدرسه فوتبال باشگاه پابهتوپ"
profile.HeroTagFA()        // "<city> · <tagline>"  (empty parts collapse away)
profile.ManagerTitleFA()   // "مدیریت باشگاه پابهتوپ"
```

No Go file, HTML page, static asset or manifest may contain a hard-coded name.
`internal/app/branding_test.go` renders every user-facing page and fails if any
legacy name from the source deployment reappears:

```
TestRenderedPagesCarryNoLegacyIdentity   ← the anti-leak gate
TestPagesFollowActiveBrand               ← changing the profile changes every page
TestBrandLabelsCollapse                  ← optional fields degrade cleanly
```

## 2. Configuration

| Variable | Example | Effect when empty |
|---|---|---|
| `SITE_NAME` | `Riverside FC` | Latin name keeps the template default |
| `SITE_NAME_FA` | `باشگاه رودساید` | Persian display name |
| `SITE_SHORT_FA` | `ر` | Header monogram |
| `SITE_DISCIPLINE_FA` | `آکادمی فوتبال` | Renders the bare name instead of "<discipline> <name>" |
| `SITE_CITY_FA` | `شهر نمونه` | City is omitted from the hero strapline |
| `SITE_TAGLINE_FA` | `رشد از پایه` | Strapline is omitted |

Setting a variable to an empty string is meaningful and distinct from leaving it
unset: unset keeps the template default, empty disables the field.

## 3. Identity surfaces

These are the only places a deployment's identity appears. All of them read the
profile — none of them store a name.

| Surface | Source |
|---|---|
| Page titles and H1s | `internal/app/app.go`, `auth.go`, `dashboard.go`, `payments_v2.go` |
| Header badge and monogram | `dashboard.go` → `pageShell` |
| Payment description sent to the gateway | `payments_v2.go` |
| Demo manager display name | `seed.go` (demo data only) |
| PWA manifest (name, short name, description) | `app.go` → `manifestHandler` |
| PWA icons | `assets/icons/icon-{192,512}.svg` — abstract, no lettering |
| Landing-page hero copy | `publicHomeHTML` |

The icons are intentionally text-free. A club that wants its crest replaces the two
SVG files; nothing else needs to change.

## 4. Instantiating a new deployment

```bash
./tools/instantiate.sh \
  --name      "Riverside FC" \
  --name-fa   "باشگاه رودساید" \
  --short-fa  "ر" \
  --discipline-fa "آکادمی فوتبال" \
  --city-fa   "شهر نمونه" \
  --tagline-fa "رشد از پایه" \
  --module-path github.com/yourorg/riverside
```

The script:

1. rewrites the Go module path across `go.mod` and every import,
2. renames the binary and Docker/compose identifiers,
3. writes the `SITE_*` block into `.env` (copying `.env.example` if needed),
4. runs `go build ./...` and the branding test as a post-condition.

It refuses to run in a dirty tree, so a botched instantiation is always revertible
with `git checkout .`.

## 5. Adding another locale

The UI is Persian-first by design (RTL, Jalali calendar, Persian digits). To ship a
second locale:

1. **Copy the Persian strings out of the page functions** into a locale map keyed by
   message id — start with `internal/brand` and grow a sibling `internal/i18n`
   package. Do not scatter translations through handlers.
2. **Add the direction** to `pageShell`: `dir="rtl"` / `dir="ltr"` and a `lang`
   attribute derived from the active locale.
3. **Keep the calendar at the boundary.** Jalali conversion lives in the UI layer
   (`client/src/fa.ts`, and the handlers that format dates). Storage stays UTC/ISO.
4. **Add the locale to the manifest** — `manifestHandler` already sets `lang`/`dir`.

This is deliberately left as an exercise rather than a half-built abstraction: the
platform ships one locale done properly instead of two done approximately.

## 6. What must never be committed

- A real club's name in `internal/brand` defaults or in `.env.example`.
- Real member, guardian or payment data — including in tests and fixtures.
- Merchant IDs, encryption keys or callback URLs tied to a live account.
- Screenshots showing a real dashboard.

Demo data (`SEED_DEMO=1`) is entirely fictitious and must only ever be combined with
the stub payment provider.

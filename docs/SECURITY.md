# Security

Scope: a single-deployment membership and billing platform for one sports club,
hosted by the club. It holds member names, guardian phone numbers, fee amounts and
payment references. It does **not** hold card data — card details never reach this
application.

## 1. Assets

| Asset | Sensitivity |
|---|---|
| Member and guardian records | Personal data (name, mobile, date of birth) |
| Subscription and invoice history | Financial history, private to each family |
| Payment references and statuses | Reconciliation data |
| Audit log | Integrity-relevant |
| Admin session cookie | Grants full manager authority |

## 2. Threat model

| Threat | Control |
|---|---|
| Guardian reads another family's data | Records are only reachable through handlers that check the `guardian_players` link |
| Anonymous visitor reads operational data | Public routes expose aggregate counts only; browser writes to collections are disabled |
| Cross-site request forgery | Double-submit token, constant-time compared, required on every mutation |
| Credential stuffing / brute force | Fixed-window limiter: 5 attempts / 10 min per identifier |
| Session theft via script | Session cookie is `HttpOnly` + `SameSite`; `Secure` when served over TLS |
| XSS through stored values | All interpolated values pass `html.EscapeString`; CSP `script-src 'self'` |
| Clickjacking | CSP `frame-ancestors 'none'` |
| MIME confusion | `X-Content-Type-Options: nosniff` |
| Data at rest in the PWA cache | Service worker refuses to cache `/portal`, `/manager`, `/pay/*`, `/payments/*` |
| Secret leakage | No secret in the repo; `.env` is git-ignored; environments supply values |
| Fake payment confirmation | Gateway callbacks are verified server-side before a payment is settled |

## 3. Headers set by the application

`internal/app/security.go`, applied to every route:

```
Content-Security-Policy: default-src 'self'; base-uri 'self'; form-action 'self';
  frame-ancestors 'none'; script-src 'self' 'unsafe-eval'; style-src 'self'
  'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: camera=(), microphone=(), geolocation=()
Cross-Origin-Opener-Policy: same-origin
X-Content-Type-Options: nosniff
```

## 4. Residual risks — read before deploying

These are known and accepted for the target scale. Each is a deployment decision,
not something the code will fix for you.

1. **The CSRF cookie is not `HttpOnly`.** The double-submit pattern requires the
   page to read it. It is a separate cookie from the session, so reading it grants
   nothing on its own, but the CSP `script-src` directive is what actually contains
   this. Narrow `'unsafe-eval'` if you can confirm htmx does not need it.
2. **The rate limiter is in-process and in-memory.** It resets on restart and is not
   shared between replicas. Deploy one instance, or move limiting to the proxy.
3. **No HSTS is set by the application.** Terminate TLS at a reverse proxy and set
   `Strict-Transport-Security` there (see `DEPLOYMENT.md`).
4. **CSP allows `'unsafe-inline'` styles.** Needed by the inline `<style>` blocks in
   the server-rendered pages. Moving that CSS into `assets/css/` would let you drop it.
5. **The demo seed creates known credentials** (`manager@example.test`). `SEED_DEMO`
   must be `0` in production; the manager password must be rotated on first login.
6. **The stub payment provider moves no money.** It exists so the whole lifecycle is
   testable. The live adapter requires a merchant ID, production approval and a
   publicly reachable HTTPS callback URL.
7. **No multi-tenancy isolation.** One deployment serves one club. Do not host two
   clubs on one instance expecting separation.

## 5. Operational requirements

- Serve exclusively over HTTPS; redirect HTTP at the proxy.
- Set `APP_ENV=production`, `STUB=0`, `SEED_DEMO=0`.
- Provide `PABETOOP_PB_ENCRYPTION_KEY` (32 characters, high entropy) if encrypted
  PocketBase settings are enabled.
- Back up `pb_data/` on a schedule and **verify the backup with the bundled tool**
  (`tools/backup-check`) — an unverified backup is not a backup.
- Rotate the manager password on handover, and remove staff records for people who
  leave the club.

## 6. Reporting

Open a private security advisory on the repository rather than a public issue.
Include the affected route, a reproduction, and the impact you believe it has.
There is no bug bounty.

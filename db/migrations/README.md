# Migrations

This directory is intentionally empty.

The schema is created **programmatically at bootstrap**: `ensureSchoolCollections`
in `internal/app/app.go` creates every collection if it is missing, so a fresh
deployment needs no migration step and no SQL client. PocketBase keeps its own
migration and schema bookkeeping inside `pb_data/`.

Use this directory only if you add a schema change that cannot be expressed as an
idempotent create-if-missing:

1. Add a `.sql` file named `NNNN_description.sql` here.
2. Apply it from a PocketBase migration in `internal/app` (or a `pb_hooks` migration
   file), not from a shell script, so it runs in order on every deployment.
3. Document the change in `docs/ARCHITECTURE.md` §4 — the data model table there is
   the contract reviewers read.
4. **Back up before deploying.** A migration that adds a collection is safe; one that
   drops or rewrites a column is not reversible by rolling back the binary.

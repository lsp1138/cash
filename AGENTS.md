# Repository guidance

## Project

`cash` is a Go CLI for real timekeeping and invoicing backed by SQLite. It is
both under active development and used with production data.

## Data safety

- Treat `~/.cash/cash.db` and `~/.cash/invoices/` as production data.
- Never use production data in automated tests.
- Never commit databases, invoices, customer data, credentials, or backups.
- Back up the production database before migrations, invoice changes, recurring
  billing changes, or other material mutations.
- Prefer a disposable or in-memory database for development and testing.
- Ask before destructive or irreversible changes to production data.

## Development workflow

- Preserve unrelated worktree changes.
- Format changed Go files with `gofmt`.
- Run `go test ./...` after code changes.
- Add or update tests for behavior changes.
- Keep changes small and focused.
- Record durable decisions and follow-up work in `DEVELOPMENT.md`.

## Project notes

Read `DEVELOPMENT.md` before changing database paths, backups, recurring
billing, or invoice workflows.

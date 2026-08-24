# Cash development notes

This file records decisions and follow-up work while `cash` is both under
development and being used for real timekeeping and invoicing.

## Current operating model

- Keep source code and synthetic test data in this repository.
- Keep the live database at `~/.cash/cash.db`.
- Keep generated working invoices at `~/.cash/invoices/`.
- Do not commit real databases, invoice files, customer details, or other
  production data to Git.
- Treat `~/.cash` as live production data, even when running the CLI from the
  development checkout.

## Priority development work

### 1. Separate development and production data

Add an explicit database-path setting, preferably `CASH_DB_PATH`, while keeping
`~/.cash/cash.db` as the default for normal installed use.

Suggested usage:

```sh
CASH_DB_PATH=/Users/sam/dev/cash/tmp/dev.db go run . <command>
CASH_DB_PATH=/Users/sam/.cash/cash.db cash <command>
```

Tests must continue to use temporary or in-memory databases and must never open
the production database.

Consider also adding a visible production-data warning or a `--production`
confirmation for destructive operations.

### 2. Establish automatic backups

- Use SQLite's backup mechanism rather than copying a database while it may be
  open.
- Create a timestamped backup before invoice creation, cancellation, migrations,
  and other material changes.
- Store daily backups in a second private or synced location, not only under
  `~/.cash` on the same disk.
- Retain multiple generations and periodically test restoring one.
- Keep final sent invoice PDFs in the backup location as well.

### 3. Make recurring billing safer

- Prevent two active recurring definitions with the same customer, project,
  cadence, period, currency, and amount unless explicitly intended.
- Provide CLI commands to manage recurring charges without editing SQLite
  directly, for example:

  ```sh
  cash recurring list
  cash recurring show <id>
  cash recurring update <id> --service "..." --description "..."
  cash recurring deactivate <id>
  cash recurring audit
  ```

- When a recurring definition is updated, optionally synchronize its existing
  uninvoiced materialized entries while preserving frozen lines on finalized or
  historical invoices.
- Show a confirmation summary before creating an invoice, especially when the
  amount differs from the previous period.
- Add a dry-run or preview mode that does not modify SQLite or generate final
  artifacts.

### 4. Improve invoice workflow

- Allow the tax reference to be supplied during invoice generation so the first
  PDF is complete.
- Add an explicit finalize/send workflow and preserve the exact finalized PDF.
- Consider storing a checksum for finalized invoice PDFs.
- Make invoice dates and due dates explicit options rather than relying only on
  the date the command is run.

## Working conventions

- Before changing production data, inspect the selected database path and create
  a backup.
- Develop against a disposable database populated with synthetic data.
- Run tests before using a newly changed build against production.
- Record architectural decisions and durable backlog items in this file.
- Use Git issues later if a remote repository is introduced; keep this file as
  the summary of current operating decisions.

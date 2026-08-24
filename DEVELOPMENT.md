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
- Add a `cash backup` command that creates a complete, restorable snapshot in a
  staging directory before publishing it to the configured backup destination.
- Support a configurable destination such as `CASH_BACKUP_DIR`. For Sam's local
  setup, this can point to a private Google Drive folder dedicated to Cash
  backups.
- Do not synchronize the live `~/.cash/cash.db` file directly. Google Drive may
  copy it while SQLite is open or between related writes. Synchronize completed
  timestamped snapshots instead.
- Each snapshot should contain:
  - a SQLite-consistent backup of `cash.db`;
  - finalized and sent invoice PDFs;
  - the non-secret configuration needed to understand or restore the data;
  - a manifest with creation time, schema version, file sizes, and checksums.
- Keep the live `~/.cash` directory outside Google Drive. Use a flow such as:

  ```text
  ~/.cash (live)
      -> local atomic snapshot
      -> /Users/Shared/CashBackupInbox
      -> dedicated backup service
      -> private Google Drive backup folder
  ```

- Use a separate macOS backup user for cloud delivery. The `sam` development
  user creates completed backup packages in the shared inbox but does not hold
  Google credentials.
- Run the uploader as that backup user using a macOS `launchd` LaunchDaemon, not
  an interactive Google Drive desktop session or cron job. It should start after
  reboot, retry failed uploads, and not require the other desktop user to remain
  logged in.
- Authenticate the uploader with an OAuth token readable only by the backup
  user. Prefer a dedicated Google backup account rather than Sam's normal Google
  account, because uploader tools may receive broad Drive permissions for the
  authenticated account.
- Store backups in a single `Cash Backups` folder owned by or shared with the
  dedicated backup account, and share that folder with Sam's normal email for
  convenient invoice access.
- The uploader should accept only expected regular backup files, reject symbolic
  links, never interpret file contents, verify uploaded sizes or checksums, and
  archive or mark local packages only after a confirmed upload.
- Keep OAuth tokens and uploader configuration outside the repository and
  inaccessible to the `sam` user. Codex should be able to create packages in the
  handoff inbox but should not be able to read cloud credentials.
- Encrypt backups before cloud synchronization if they contain customer,
  banking, tax, or invoice data and the destination is not otherwise considered
  sufficiently private.
- Use retention rules rather than overwriting one backup, for example seven
  daily, five weekly, and twelve monthly snapshots.
- Add `cash backup verify` and a documented restore command or procedure.
- Periodically restore the newest backup into a temporary directory and run
  integrity checks; an untested backup should not be treated as reliable.
- Later, include Cash in a broader backup policy for Sam's user data, while
  keeping this application-specific SQLite snapshot step.

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

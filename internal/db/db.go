// Package db provides SQLite persistence for the cash CLI.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/larspittman/cash/internal/models"
)

// DB wraps a SQLite connection.
type DB struct {
	conn *sql.DB
}

// DataDir returns (and creates) ~/.cash.
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".cash")
	return dir, os.MkdirAll(dir, 0o755)
}

// Open opens or creates the production database at ~/.cash/cash.db.
func Open() (*DB, error) {
	dir, err := DataDir()
	if err != nil {
		return nil, fmt.Errorf("data dir: %w", err)
	}
	return openAt(filepath.Join(dir, "cash.db"))
}

// OpenMemory opens an in-memory SQLite database (useful for tests).
func OpenMemory() (*DB, error) {
	return openAt(":memory:")
}

func openAt(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	conn.SetMaxOpenConns(1)
	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return d, nil
}

// Close releases the database connection.
func (d *DB) Close() error { return d.conn.Close() }

func (d *DB) migrate() error {
	_, err := d.conn.Exec(`
	CREATE TABLE IF NOT EXISTS customers (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT    NOT NULL UNIQUE,
		email       TEXT    NOT NULL DEFAULT '',
		address     TEXT    NOT NULL DEFAULT '',
		hourly_rate REAL    NOT NULL DEFAULT 0,
		currency    TEXT    NOT NULL DEFAULT 'USD',
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS projects (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT    NOT NULL UNIQUE,
		customer_id INTEGER REFERENCES customers(id),
		description TEXT    NOT NULL DEFAULT '',
		hourly_rate REAL,
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS time_entries (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		project_name TEXT    NOT NULL,
		hours        REAL    NOT NULL,
		message      TEXT    NOT NULL DEFAULT '',
		subservice   TEXT    NOT NULL DEFAULT '',
		start_time   DATETIME,
		end_time     DATETIME,
		committed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS timers (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		project_name TEXT    NOT NULL,
		started_at   DATETIME NOT NULL,
		message      TEXT    NOT NULL DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS invoices (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		customer_id    INTEGER  NOT NULL REFERENCES customers(id),
		invoice_number TEXT     NOT NULL UNIQUE,
		period_start   DATE     NOT NULL,
		period_end     DATE     NOT NULL,
		total_hours    REAL     NOT NULL DEFAULT 0,
		total_amount   REAL     NOT NULL DEFAULT 0,
		currency       TEXT     NOT NULL DEFAULT 'USD',
		status         TEXT     NOT NULL DEFAULT 'draft',
		pdf_path       TEXT     NOT NULL DEFAULT '',
		created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`)
	return err
}

// ─── Time Entries ─────────────────────────────────────────────────────────────

// AddTimeEntry inserts an entry and returns its new ID.
func (d *DB) AddTimeEntry(e models.TimeEntry) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO time_entries
		 (project_name, hours, message, subservice, start_time, end_time, committed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ProjectName, e.Hours, e.Message, e.Subservice,
		optTime(e.StartTime), optTime(e.EndTime), e.CommittedAt.Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetTimeEntries returns entries matching the filter, oldest first.
func (d *DB) GetTimeEntries(f models.TimeEntryFilter) ([]models.TimeEntry, error) {
	q := `SELECT id, project_name, hours, message, subservice, start_time, end_time, committed_at
	      FROM time_entries WHERE 1=1`
	var args []any

	if f.ProjectName != "" {
		q += " AND project_name = ?"
		args = append(args, f.ProjectName)
	}
	if f.From != nil {
		q += " AND committed_at >= ?"
		args = append(args, f.From.Format(time.RFC3339))
	}
	if f.To != nil {
		q += " AND committed_at < ?"
		args = append(args, f.To.Format(time.RFC3339))
	}
	q += " ORDER BY committed_at ASC"

	rows, err := d.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.TimeEntry
	for rows.Next() {
		var e models.TimeEntry
		var st, et sql.NullString
		var cat string
		if err := rows.Scan(&e.ID, &e.ProjectName, &e.Hours, &e.Message, &e.Subservice, &st, &et, &cat); err != nil {
			return nil, err
		}
		e.CommittedAt = mustParseTime(cat)
		if st.Valid {
			t := mustParseTime(st.String)
			e.StartTime = &t
		}
		if et.Valid {
			t := mustParseTime(et.String)
			e.EndTime = &t
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ─── Timers ───────────────────────────────────────────────────────────────────

// StartTimer creates a new running timer and returns its ID.
func (d *DB) StartTimer(projectName, message string) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO timers (project_name, started_at, message) VALUES (?, ?, ?)`,
		projectName, time.Now().Format(time.RFC3339), message,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetActiveTimer returns the most recent timer, or nil if none exists.
func (d *DB) GetActiveTimer() (*models.Timer, error) {
	row := d.conn.QueryRow(
		`SELECT id, project_name, started_at, message FROM timers ORDER BY id DESC LIMIT 1`,
	)
	var t models.Timer
	var sa string
	if err := row.Scan(&t.ID, &t.ProjectName, &sa, &t.Message); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	t.StartedAt = mustParseTime(sa)
	return &t, nil
}

// DeleteTimer removes a timer by ID.
func (d *DB) DeleteTimer(id int64) error {
	_, err := d.conn.Exec(`DELETE FROM timers WHERE id = ?`, id)
	return err
}

// ─── Customers ────────────────────────────────────────────────────────────────

// AddCustomer inserts a customer and returns its ID.
func (d *DB) AddCustomer(c models.Customer) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO customers (name, email, address, hourly_rate, currency) VALUES (?, ?, ?, ?, ?)`,
		c.Name, c.Email, c.Address, c.HourlyRate, c.Currency,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetCustomers returns all customers sorted by name.
func (d *DB) GetCustomers() ([]models.Customer, error) {
	rows, err := d.conn.Query(
		`SELECT id, name, email, address, hourly_rate, currency, created_at FROM customers ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Customer
	for rows.Next() {
		var c models.Customer
		var cat string
		if err := rows.Scan(&c.ID, &c.Name, &c.Email, &c.Address, &c.HourlyRate, &c.Currency, &cat); err != nil {
			return nil, err
		}
		c.CreatedAt = mustParseTime(cat)
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCustomerByName finds a customer case-insensitively, returning nil if not found.
func (d *DB) GetCustomerByName(name string) (*models.Customer, error) {
	row := d.conn.QueryRow(
		`SELECT id, name, email, address, hourly_rate, currency, created_at
		 FROM customers WHERE LOWER(name) = LOWER(?)`, name,
	)
	var c models.Customer
	var cat string
	if err := row.Scan(&c.ID, &c.Name, &c.Email, &c.Address, &c.HourlyRate, &c.Currency, &cat); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	c.CreatedAt = mustParseTime(cat)
	return &c, nil
}

// UpdateCustomer saves changed email, address, rate, and currency by ID.
func (d *DB) UpdateCustomer(c models.Customer) error {
	_, err := d.conn.Exec(
		`UPDATE customers SET email=?, address=?, hourly_rate=?, currency=? WHERE id=?`,
		c.Email, c.Address, c.HourlyRate, c.Currency, c.ID,
	)
	return err
}

// ─── Projects ─────────────────────────────────────────────────────────────────

// AddProject inserts a project and returns its ID.
func (d *DB) AddProject(p models.Project) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO projects (name, customer_id, description, hourly_rate) VALUES (?, ?, ?, ?)`,
		p.Name, optInt64(p.CustomerID), p.Description, optFloat64(p.HourlyRate),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetProjects returns all projects with their associated customer (if any).
func (d *DB) GetProjects() ([]models.Project, error) {
	rows, err := d.conn.Query(`
		SELECT p.id, p.name, p.customer_id, p.description, p.hourly_rate, p.created_at,
		       c.id, c.name, c.hourly_rate, c.currency
		FROM projects p
		LEFT JOIN customers c ON c.id = p.customer_id
		ORDER BY p.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Project
	for rows.Next() {
		var p models.Project
		var cid sql.NullInt64
		var hr sql.NullFloat64
		var cat string
		var ccid sql.NullInt64
		var cname, ccur sql.NullString
		var crate sql.NullFloat64
		if err := rows.Scan(&p.ID, &p.Name, &cid, &p.Description, &hr, &cat,
			&ccid, &cname, &crate, &ccur); err != nil {
			return nil, err
		}
		p.CreatedAt = mustParseTime(cat)
		if cid.Valid {
			p.CustomerID = &cid.Int64
		}
		if hr.Valid {
			p.HourlyRate = &hr.Float64
		}
		if ccid.Valid {
			p.Customer = &models.Customer{ID: ccid.Int64, Name: cname.String, HourlyRate: crate.Float64, Currency: ccur.String}
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProjectByName finds a project (with customer) case-insensitively.
func (d *DB) GetProjectByName(name string) (*models.Project, error) {
	row := d.conn.QueryRow(`
		SELECT p.id, p.name, p.customer_id, p.description, p.hourly_rate, p.created_at,
		       c.id, c.name, c.email, c.address, c.hourly_rate, c.currency
		FROM projects p
		LEFT JOIN customers c ON c.id = p.customer_id
		WHERE LOWER(p.name) = LOWER(?)`, name)

	var p models.Project
	var cid sql.NullInt64
	var hr sql.NullFloat64
	var cat string
	var ccid sql.NullInt64
	var cname, cemail, caddr, ccur sql.NullString
	var crate sql.NullFloat64
	if err := row.Scan(&p.ID, &p.Name, &cid, &p.Description, &hr, &cat,
		&ccid, &cname, &cemail, &caddr, &crate, &ccur); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	p.CreatedAt = mustParseTime(cat)
	if cid.Valid {
		p.CustomerID = &cid.Int64
	}
	if hr.Valid {
		p.HourlyRate = &hr.Float64
	}
	if ccid.Valid {
		p.Customer = &models.Customer{
			ID: ccid.Int64, Name: cname.String, Email: cemail.String,
			Address: caddr.String, HourlyRate: crate.Float64, Currency: ccur.String,
		}
	}
	return &p, nil
}

// GetProjectsByCustomer returns projects belonging to a customer.
func (d *DB) GetProjectsByCustomer(customerID int64) ([]models.Project, error) {
	rows, err := d.conn.Query(
		`SELECT id, name, customer_id, description, hourly_rate, created_at
		 FROM projects WHERE customer_id = ? ORDER BY name`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Project
	for rows.Next() {
		var p models.Project
		var cid sql.NullInt64
		var hr sql.NullFloat64
		var cat string
		if err := rows.Scan(&p.ID, &p.Name, &cid, &p.Description, &hr, &cat); err != nil {
			return nil, err
		}
		p.CreatedAt = mustParseTime(cat)
		if cid.Valid {
			p.CustomerID = &cid.Int64
		}
		if hr.Valid {
			p.HourlyRate = &hr.Float64
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ─── Invoices ─────────────────────────────────────────────────────────────────

// NextInvoiceNumber generates the next sequential invoice number for the year.
func (d *DB) NextInvoiceNumber(year int) (string, error) {
	var count int
	err := d.conn.QueryRow(
		`SELECT COUNT(*) FROM invoices WHERE invoice_number LIKE ?`,
		fmt.Sprintf("INV-%d-%%", year),
	).Scan(&count)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("INV-%d-%03d", year, count+1), nil
}

// AddInvoice inserts an invoice and returns its ID.
func (d *DB) AddInvoice(inv models.Invoice) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO invoices
		 (customer_id, invoice_number, period_start, period_end, total_hours, total_amount, currency, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		inv.CustomerID, inv.InvoiceNumber,
		inv.PeriodStart.Format("2006-01-02"),
		inv.PeriodEnd.Format("2006-01-02"),
		inv.TotalHours, inv.TotalAmount, inv.Currency, inv.Status,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateInvoicePDFPath sets the PDF path and marks the invoice as generated.
func (d *DB) UpdateInvoicePDFPath(id int64, path string) error {
	_, err := d.conn.Exec(`UPDATE invoices SET pdf_path=?, status='generated' WHERE id=?`, path, id)
	return err
}

// GetInvoices returns all invoices with customer name, newest first.
func (d *DB) GetInvoices() ([]models.Invoice, error) {
	rows, err := d.conn.Query(`
		SELECT i.id, i.customer_id, i.invoice_number, i.period_start, i.period_end,
		       i.total_hours, i.total_amount, i.currency, i.status, i.pdf_path, i.created_at,
		       c.name, c.email
		FROM invoices i
		JOIN customers c ON c.id = i.customer_id
		ORDER BY i.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Invoice
	for rows.Next() {
		var inv models.Invoice
		var ps, pe, cat, cname, cemail string
		if err := rows.Scan(
			&inv.ID, &inv.CustomerID, &inv.InvoiceNumber,
			&ps, &pe, &inv.TotalHours, &inv.TotalAmount,
			&inv.Currency, &inv.Status, &inv.PDFPath, &cat,
			&cname, &cemail,
		); err != nil {
			return nil, err
		}
		inv.PeriodStart = mustParseDate(ps)
		inv.PeriodEnd = mustParseDate(pe)
		inv.CreatedAt = mustParseTime(cat)
		inv.Customer = &models.Customer{Name: cname, Email: cemail}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// GetInvoiceByNumber returns a single invoice with full customer detail.
func (d *DB) GetInvoiceByNumber(number string) (*models.Invoice, error) {
	row := d.conn.QueryRow(`
		SELECT i.id, i.customer_id, i.invoice_number, i.period_start, i.period_end,
		       i.total_hours, i.total_amount, i.currency, i.status, i.pdf_path, i.created_at,
		       c.id, c.name, c.email, c.address, c.hourly_rate, c.currency
		FROM invoices i
		JOIN customers c ON c.id = i.customer_id
		WHERE i.invoice_number = ?`, number)
	var inv models.Invoice
	var ps, pe, cat string
	var cid int64
	var cname, cemail, caddr, ccur string
	var crate float64
	if err := row.Scan(
		&inv.ID, &inv.CustomerID, &inv.InvoiceNumber,
		&ps, &pe, &inv.TotalHours, &inv.TotalAmount,
		&inv.Currency, &inv.Status, &inv.PDFPath, &cat,
		&cid, &cname, &cemail, &caddr, &crate, &ccur,
	); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	inv.PeriodStart = mustParseDate(ps)
	inv.PeriodEnd = mustParseDate(pe)
	inv.CreatedAt = mustParseTime(cat)
	inv.Customer = &models.Customer{
		ID: cid, Name: cname, Email: cemail,
		Address: caddr, HourlyRate: crate, Currency: ccur,
	}
	return &inv, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func optTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func optInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func optFloat64(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

var timeFormats = []string{
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05Z",
	"2006-01-02",
}

func mustParseTime(s string) time.Time {
	s = strings.TrimSuffix(s, "+00:00")
	for _, f := range timeFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func mustParseDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

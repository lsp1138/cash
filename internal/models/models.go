package models

import "time"

const (
	InvoiceStatusDraft     = "draft"
	InvoiceStatusSent      = "sent"
	InvoiceStatusPaid      = "paid"
	InvoiceStatusCancelled = "cancelled"
)

// Customer represents a freelance client.
type Customer struct {
	ID         int64
	Name       string
	Slug       string
	Email      string
	Address    string
	HourlyRate float64
	Currency   string
	CreatedAt  time.Time
}

// Project is a billable project linked to an optional customer.
type Project struct {
	ID          int64
	Name        string
	CustomerID  *int64
	Description string
	HourlyRate  *float64 // overrides customer rate when set
	CreatedAt   time.Time
	Customer    *Customer // populated by JOIN queries
}

// EffectiveRate returns the hourly rate for this project.
func (p *Project) EffectiveRate() (float64, string) {
	if p.HourlyRate != nil && *p.HourlyRate > 0 {
		cur := "USD"
		if p.Customer != nil {
			cur = p.Customer.Currency
		}
		return *p.HourlyRate, cur
	}
	if p.Customer != nil {
		return p.Customer.HourlyRate, p.Customer.Currency
	}
	return 0, "USD"
}

// TimeEntry represents a recorded block of work.
type TimeEntry struct {
	Billable    bool   // true = billable, false = non-billable
	InvoiceID   *int64 // set when linked to an invoice
	ID          int64
	ProjectName string
	Hours       float64
	Message     string
	Subservice  string
	StartTime   *time.Time
	EndTime     *time.Time
	CommittedAt time.Time
}

// Timer represents an active running timer.
type Timer struct {
	ID          int64
	ProjectName string
	StartedAt   time.Time
	Message     string
}

// Invoice represents a billing document for a customer.
type Invoice struct {
	SentAt        *time.Time
	PaidAt        *time.Time
	CancelledAt   *time.Time
	ID            int64
	CustomerID    int64
	InvoiceNumber string
	TaxReference  string
	PeriodStart   time.Time
	PeriodEnd     time.Time
	TotalHours    float64
	TotalAmount   float64
	Currency      string
	Status        string
	PDFPath       string
	CreatedAt     time.Time
	Customer      *Customer // populated by JOIN
	Entries       []TimeEntry
	Items         []InvoiceItem
}

// InvoiceItem is a frozen snapshot line belonging to an invoice.
type InvoiceItem struct {
	TimeEntryID *int64
	ID          int64
	InvoiceID   int64
	EntryDate   time.Time
	ProjectName string
	Description string
	Subservice  string
	Hours       float64
	Rate        float64
	Amount      float64
	Currency    string
}

// RecurringChargeDefinition describes a template for fixed-fee monthly billing.
type RecurringChargeDefinition struct {
	ID          int64
	CustomerID  int64
	ProjectName string
	Subservice  string
	Description string
	Amount      float64
	Currency    string
	Billable    bool
	Cadence     string
	StartMonth  time.Time
	Active      bool
	CreatedAt   time.Time
}

// RecurringChargeEntry is a generated fixed-fee billing item for a specific month.
type RecurringChargeEntry struct {
	InvoiceID    *int64
	ID           int64
	DefinitionID int64
	CustomerID   int64
	ProjectName  string
	Subservice   string
	Description  string
	Amount       float64
	Currency     string
	Billable     bool
	PeriodStart  time.Time
	CreatedAt    time.Time
}

// TimeEntryFilter holds criteria for querying time entries.
type TimeEntryFilter struct {
	ProjectName string
	From        *time.Time
	To          *time.Time
}

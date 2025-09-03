package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/axllent/mailpit/internal/logger"

	// rqlite - https://github.com/rqlite/gorqlite | https://rqlite.io/
	_ "github.com/rqlite/gorqlite/stdlib"
)

// RQLiteDatabase implements the Database interface for RQLite
type RQLiteDatabase struct {
	db       *sql.DB
	dsn      string
	driver   string
	tenantID string
}

// NewRQLiteDatabase creates a new RQLite database instance
func NewRQLiteDatabase(config DatabaseConfig) (*RQLiteDatabase, error) {
	db, err := sql.Open("rqlite", config.DSN)
	if err != nil {
		return nil, err
	}

	// RQLite doesn't need connection pooling like PostgreSQL
	db.SetMaxOpenConns(1)

	rqliteDB := &RQLiteDatabase{
		db:       db,
		dsn:      config.DSN,
		driver:   "rqlite",
		tenantID: config.TenantID,
	}

	return rqliteDB, nil
}

// Ping implements Database.Ping
func (r *RQLiteDatabase) Ping() error {
	return r.db.Ping()
}

// Close implements Database.Close
func (r *RQLiteDatabase) Close() error {
	return r.db.Close()
}

// BeginTx implements Database.BeginTx
func (r *RQLiteDatabase) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, opts)
}

// Exec implements Database.Exec
func (r *RQLiteDatabase) Exec(query string, args ...interface{}) (sql.Result, error) {
	return r.db.Exec(query, args...)
}

// Query implements Database.Query
func (r *RQLiteDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return r.db.Query(query, args...)
}

// QueryRow implements Database.QueryRow
func (r *RQLiteDatabase) QueryRow(query string, args ...interface{}) *sql.Row {
	return r.db.QueryRow(query, args...)
}

// ExecContext implements Database.ExecContext
func (r *RQLiteDatabase) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return r.db.ExecContext(ctx, query, args...)
}

// QueryContext implements Database.QueryContext
func (r *RQLiteDatabase) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return r.db.QueryContext(ctx, query, args...)
}

// QueryRowContext implements Database.QueryRowContext
func (r *RQLiteDatabase) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return r.db.QueryRowContext(ctx, query, args...)
}

// GetDbSize implements Database.GetDbSize
func (r *RQLiteDatabase) GetDbSize() uint64 {
	// RQLite doesn't have a direct way to get database size like SQLite
	// We'll return 0 for now, as RQLite handles storage differently
	return 0
}

// Vacuum implements Database.Vacuum
func (r *RQLiteDatabase) Vacuum() error {
	// RQLite handles vacuuming automatically
	// No need to implement manual vacuuming
	return nil
}

// GetDriverName implements Database.GetDriverName
func (r *RQLiteDatabase) GetDriverName() string {
	return r.driver
}

// ApplySchemas implements Database.ApplySchemas
func (r *RQLiteDatabase) ApplySchemas() error {
	return r.dbApplyRQLiteSchemas()
}

// dbApplyRQLiteSchemas applies RQLite-specific schemas
func (r *RQLiteDatabase) dbApplyRQLiteSchemas() error {
	// Create schemas table if it doesn't exist
	schemasTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version TEXT PRIMARY KEY NOT NULL
		)
	`, r.tenant("schemas"))

	if _, err := r.db.Exec(schemasTableSQL); err != nil {
		return err
	}

	// Apply RQLite-specific schema migrations
	return r.applyRQLiteMigrations()
}

// applyRQLiteMigrations applies RQLite-specific migrations
func (r *RQLiteDatabase) applyRQLiteMigrations() error {
	// Create tables with RQLite-compatible syntax (same as SQLite)
	tables := []string{
		// Mailbox table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				created INTEGER NOT NULL,
				id TEXT NOT NULL,
				message_id TEXT NOT NULL,
				subject TEXT NOT NULL,
				metadata TEXT,
				size INTEGER NOT NULL,
				inline INTEGER NOT NULL,
				attachments INTEGER NOT NULL,
				read INTEGER,
				snippet TEXT,
				search_text TEXT
			)
		`, r.tenant("mailbox")),

		// Mailbox data table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id TEXT PRIMARY KEY NOT NULL,
				email BLOB,
				compressed INTEGER NOT NULL DEFAULT 0
			)
		`, r.tenant("mailbox_data")),

		// Tags table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
				name TEXT COLLATE NOCASE
			)
		`, r.tenant("tags")),

		// Message tags table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				key INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
				id TEXT NOT NULL,
				tag_id INTEGER NOT NULL
			)
		`, r.tenant("message_tags")),

		// Settings table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				key TEXT,
				value TEXT
			)
		`, r.tenant("settings")),
	}

	for _, tableSQL := range tables {
		if _, err := r.db.Exec(tableSQL); err != nil {
			return err
		}
	}

	// Create indexes
	indexes := []string{
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (created)`, r.tenant("idx_created"), r.tenant("mailbox")),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (id)`, r.tenant("idx_id"), r.tenant("mailbox")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (message_id)`, r.tenant("idx_message_id"), r.tenant("mailbox")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (subject)`, r.tenant("idx_subject"), r.tenant("mailbox")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (size)`, r.tenant("idx_size"), r.tenant("mailbox")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (inline)`, r.tenant("idx_inline"), r.tenant("mailbox")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (attachments)`, r.tenant("idx_attachments"), r.tenant("mailbox")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (read)`, r.tenant("idx_read"), r.tenant("mailbox")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (id)`, r.tenant("idx_message_tags_id"), r.tenant("message_tags")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (tag_id)`, r.tenant("idx_message_tags_tagid"), r.tenant("message_tags")),
	}

	for _, indexSQL := range indexes {
		if _, err := r.db.Exec(indexSQL); err != nil {
			// Ignore errors for existing indexes
			logger.Log().Debugf("[db] index creation (may already exist): %s", err.Error())
		}
	}

	// Initialize settings if empty
	_, err := r.db.Exec(fmt.Sprintf(`
		INSERT OR IGNORE INTO %s (key, value) VALUES ('DeletedSize', '0')
	`, r.tenant("settings")))

	return err
}

// tenant applies an optional prefix to the table name
func (r *RQLiteDatabase) tenant(table string) string {
	return fmt.Sprintf("%s%s", r.tenantID, table)
}

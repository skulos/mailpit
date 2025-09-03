package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/axllent/mailpit/internal/logger"

	// postgres driver
	_ "github.com/lib/pq"
)

// PostgresDatabase implements the Database interface for PostgreSQL
type PostgresDatabase struct {
	db       *sql.DB
	dsn      string
	driver   string
	tenantID string
}

// NewPostgresDatabase creates a new PostgreSQL database instance
func NewPostgresDatabase(config DatabaseConfig) (*PostgresDatabase, error) {
	db, err := sql.Open("postgres", config.DSN)
	if err != nil {
		return nil, err
	}

	// Set reasonable connection pool settings for PostgreSQL
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	postgresDB := &PostgresDatabase{
		db:       db,
		dsn:      config.DSN,
		driver:   "postgres",
		tenantID: config.TenantID,
	}

	// Configure PostgreSQL settings
	if err := postgresDB.configurePostgres(); err != nil {
		return nil, err
	}

	return postgresDB, nil
}

// configurePostgres sets up PostgreSQL-specific configurations
func (p *PostgresDatabase) configurePostgres() error {
	// Set timezone to UTC for consistency
	_, err := p.db.Exec("SET timezone = 'UTC'")
	return err
}

// Ping implements Database.Ping
func (p *PostgresDatabase) Ping() error {
	return p.db.Ping()
}

// Close implements Database.Close
func (p *PostgresDatabase) Close() error {
	return p.db.Close()
}

// BeginTx implements Database.BeginTx
func (p *PostgresDatabase) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return p.db.BeginTx(ctx, opts)
}

// Exec implements Database.Exec
func (p *PostgresDatabase) Exec(query string, args ...interface{}) (sql.Result, error) {
	return p.db.Exec(query, args...)
}

// Query implements Database.Query
func (p *PostgresDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return p.db.Query(query, args...)
}

// QueryRow implements Database.QueryRow
func (p *PostgresDatabase) QueryRow(query string, args ...interface{}) *sql.Row {
	return p.db.QueryRow(query, args...)
}

// ExecContext implements Database.ExecContext
func (p *PostgresDatabase) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.db.ExecContext(ctx, query, args...)
}

// QueryContext implements Database.QueryContext
func (p *PostgresDatabase) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return p.db.QueryContext(ctx, query, args...)
}

// QueryRowContext implements Database.QueryRowContext
func (p *PostgresDatabase) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return p.db.QueryRowContext(ctx, query, args...)
}

// GetDbSize implements Database.GetDbSize
func (p *PostgresDatabase) GetDbSize() uint64 {
	var total sql.NullFloat64

	// PostgreSQL query to get database size
	query := `
		SELECT pg_database_size(current_database()) as size
	`
	err := p.db.QueryRow(query).Scan(&total)

	if err != nil {
		logger.Log().Errorf("[db] %s", err.Error())
	}

	return uint64(total.Float64)
}

// Vacuum implements Database.Vacuum
func (p *PostgresDatabase) Vacuum() error {
	// PostgreSQL VACUUM command
	_, err := p.db.Exec("VACUUM ANALYZE")
	if err != nil {
		logger.Log().Errorf("[db] VACUUM: %s", err.Error())
		return err
	}

	return nil
}

// GetDriverName implements Database.GetDriverName
func (p *PostgresDatabase) GetDriverName() string {
	return p.driver
}

// ApplySchemas implements Database.ApplySchemas
func (p *PostgresDatabase) ApplySchemas() error {
	return p.dbApplyPostgresSchemas()
}

// dbApplyPostgresSchemas applies PostgreSQL-specific schemas
func (p *PostgresDatabase) dbApplyPostgresSchemas() error {
	// Create schemas table if it doesn't exist
	schemasTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version VARCHAR(50) PRIMARY KEY NOT NULL
		)
	`, p.tenant("schemas"))

	if _, err := p.db.Exec(schemasTableSQL); err != nil {
		return err
	}

	// Apply PostgreSQL-specific schema migrations
	return p.applyPostgresMigrations()
}

// applyPostgresMigrations applies PostgreSQL-specific migrations
func (p *PostgresDatabase) applyPostgresMigrations() error {
	// Create tables with PostgreSQL-specific syntax
	tables := []string{
		// Mailbox table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				created BIGINT NOT NULL,
				id VARCHAR(255) NOT NULL,
				message_id VARCHAR(255) NOT NULL,
				subject TEXT NOT NULL,
				metadata TEXT,
				size BIGINT NOT NULL,
				inline INTEGER NOT NULL,
				attachments INTEGER NOT NULL,
				read INTEGER,
				snippet TEXT,
				search_text TEXT
			)
		`, p.tenant("mailbox")),

		// Mailbox data table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id VARCHAR(255) PRIMARY KEY NOT NULL,
				email BYTEA,
				compressed INTEGER NOT NULL DEFAULT 0
			)
		`, p.tenant("mailbox_data")),

		// Tags table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id SERIAL PRIMARY KEY,
				name VARCHAR(255) UNIQUE NOT NULL
			)
		`, p.tenant("tags")),

		// Message tags table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				key SERIAL PRIMARY KEY,
				id VARCHAR(255) NOT NULL,
				tag_id INTEGER NOT NULL
			)
		`, p.tenant("message_tags")),

		// Settings table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				key VARCHAR(255) PRIMARY KEY,
				value TEXT
			)
		`, p.tenant("settings")),
	}

	for _, tableSQL := range tables {
		if _, err := p.db.Exec(tableSQL); err != nil {
			return err
		}
	}

	// Create indexes
	indexes := []string{
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (created)`, p.tenant("idx_created"), p.tenant("mailbox")),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (id)`, p.tenant("idx_id"), p.tenant("mailbox")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (message_id)`, p.tenant("idx_message_id"), p.tenant("mailbox")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (subject)`, p.tenant("idx_subject"), p.tenant("mailbox")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (size)`, p.tenant("idx_size"), p.tenant("mailbox")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (inline)`, p.tenant("idx_inline"), p.tenant("mailbox")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (attachments)`, p.tenant("idx_attachments"), p.tenant("mailbox")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (read)`, p.tenant("idx_read"), p.tenant("mailbox")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (id)`, p.tenant("idx_message_tags_id"), p.tenant("message_tags")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (tag_id)`, p.tenant("idx_message_tags_tagid"), p.tenant("message_tags")),
	}

	for _, indexSQL := range indexes {
		if _, err := p.db.Exec(indexSQL); err != nil {
			// Ignore errors for existing indexes
			logger.Log().Debugf("[db] index creation (may already exist): %s", err.Error())
		}
	}

	// Initialize settings if empty
	_, err := p.db.Exec(fmt.Sprintf(`
		INSERT INTO %s (key, value) 
		VALUES ('DeletedSize', '0') 
		ON CONFLICT (key) DO NOTHING
	`, p.tenant("settings")))

	return err
}

// tenant applies an optional prefix to the table name
func (p *PostgresDatabase) tenant(table string) string {
	return fmt.Sprintf("%s%s", p.tenantID, table)
}

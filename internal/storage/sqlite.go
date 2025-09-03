package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/axllent/mailpit/config"
	"github.com/axllent/mailpit/internal/logger"

	// sqlite - https://gitlab.com/cznic/sqlite
	_ "modernc.org/sqlite"
)

// SQLiteDatabase implements the Database interface for SQLite
type SQLiteDatabase struct {
	db       *sql.DB
	dsn      string
	driver   string
	tenantID string
}

// NewSQLiteDatabase creates a new SQLite database instance
func NewSQLiteDatabase(config DatabaseConfig) (*SQLiteDatabase, error) {
	// Ensure the database file can be created
	if !isFile(config.DSN) {
		// try create a file to ensure permissions
		f, err := os.Create(config.DSN)
		if err != nil {
			return nil, fmt.Errorf("[db] %s", err.Error())
		}
		_ = f.Close()
	}

	db, err := sql.Open("sqlite", config.DSN)
	if err != nil {
		return nil, err
	}

	// prevent "database locked" errors
	// @see https://github.com/mattn/go-sqlite3#faq
	db.SetMaxOpenConns(1)

	sqliteDB := &SQLiteDatabase{
		db:       db,
		dsn:      config.DSN,
		driver:   "sqlite",
		tenantID: config.TenantID,
	}

	// Configure SQLite settings
	if err := sqliteDB.configureSQLite(); err != nil {
		return nil, err
	}

	return sqliteDB, nil
}

// configureSQLite sets up SQLite-specific configurations
func (s *SQLiteDatabase) configureSQLite() error {
	if config.DisableWAL {
		// disable WAL mode for SQLite, allows NFS mounted DBs
		_, err := s.db.Exec("PRAGMA journal_mode=DELETE; PRAGMA synchronous=NORMAL;")
		return err
	} else {
		// SQLite performance tuning (https://phiresky.github.io/blog/2020/sqlite-performance-tuning/)
		_, err := s.db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL;")
		return err
	}
}

// Ping implements Database.Ping
func (s *SQLiteDatabase) Ping() error {
	return s.db.Ping()
}

// Close implements Database.Close
func (s *SQLiteDatabase) Close() error {
	return s.db.Close()
}

// BeginTx implements Database.BeginTx
func (s *SQLiteDatabase) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, opts)
}

// Exec implements Database.Exec
func (s *SQLiteDatabase) Exec(query string, args ...interface{}) (sql.Result, error) {
	return s.db.Exec(query, args...)
}

// Query implements Database.Query
func (s *SQLiteDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return s.db.Query(query, args...)
}

// QueryRow implements Database.QueryRow
func (s *SQLiteDatabase) QueryRow(query string, args ...interface{}) *sql.Row {
	return s.db.QueryRow(query, args...)
}

// ExecContext implements Database.ExecContext
func (s *SQLiteDatabase) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}

// QueryContext implements Database.QueryContext
func (s *SQLiteDatabase) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}

// QueryRowContext implements Database.QueryRowContext
func (s *SQLiteDatabase) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

// GetDbSize implements Database.GetDbSize
func (s *SQLiteDatabase) GetDbSize() uint64 {
	var total sql.NullFloat64

	err := s.db.QueryRow("SELECT page_count * page_size AS size FROM pragma_page_count(), pragma_page_size()").Scan(&total)

	if err != nil {
		logger.Log().Errorf("[db] %s", err.Error())
	}

	return uint64(total.Float64)
}

// Vacuum implements Database.Vacuum
func (s *SQLiteDatabase) Vacuum() error {
	// set WAL file checkpoint
	if _, err := s.db.Exec("PRAGMA wal_checkpoint"); err != nil {
		logger.Log().Errorf("[db] %s", err.Error())
		return err
	}

	// vacuum database
	if _, err := s.db.Exec("VACUUM"); err != nil {
		logger.Log().Errorf("[db] VACUUM: %s", err.Error())
		return err
	}

	return nil
}

// GetDriverName implements Database.GetDriverName
func (s *SQLiteDatabase) GetDriverName() string {
	return s.driver
}

// ApplySchemas implements Database.ApplySchemas
func (s *SQLiteDatabase) ApplySchemas() error {
	return dbApplySchemas()
}

// tenant applies an optional prefix to the table name
func (s *SQLiteDatabase) tenant(table string) string {
	return fmt.Sprintf("%s%s", s.tenantID, table)
}

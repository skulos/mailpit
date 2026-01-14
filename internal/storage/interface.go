// Package storage handles all database actions
package storage

import (
	"context"
	"database/sql"
)

// Database interface defines the methods that all database implementations must support
type Database interface {
	// Connection management
	Ping() error
	Close() error
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)

	// Query execution
	Exec(query string, args ...interface{}) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row

	// Database-specific operations
	GetDbSize() uint64
	Vacuum() error
	GetDriverName() string

	// Schema management
	ApplySchemas() error
}

// DatabaseConfig holds configuration for database connections
type DatabaseConfig struct {
	Driver   string
	DSN      string
	TenantID string
	Database string
}

// DatabaseFactory creates database instances based on configuration
type DatabaseFactory interface {
	CreateDatabase(config DatabaseConfig) (Database, error)
	GetSupportedDrivers() []string
}

package storage

import (
	"fmt"
	"strings"
)

// DatabaseFactoryImpl implements the DatabaseFactory interface
type DatabaseFactoryImpl struct{}

// NewDatabaseFactory creates a new database factory
func NewDatabaseFactory() *DatabaseFactoryImpl {
	return &DatabaseFactoryImpl{}
}

// CreateDatabase creates a database instance based on the configuration
func (f *DatabaseFactoryImpl) CreateDatabase(config DatabaseConfig) (Database, error) {
	switch config.Driver {
	case "sqlite":
		return NewSQLiteDatabase(config)
	case "postgres":
		return NewPostgresDatabase(config)
	case "rqlite":
		return NewRQLiteDatabase(config)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", config.Driver)
	}
}

// GetSupportedDrivers returns a list of supported database drivers
func (f *DatabaseFactoryImpl) GetSupportedDrivers() []string {
	return []string{"sqlite", "postgres", "rqlite"}
}

// DetectDriverFromDSN detects the database driver from the DSN
func (f *DatabaseFactoryImpl) DetectDriverFromDSN(dsn string) string {
	if strings.HasPrefix(dsn, "http://") || strings.HasPrefix(dsn, "https://") {
		return "rqlite"
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return "postgres"
	}
	// Detect key-value DSN for postgres socket
	if strings.Contains(dsn, "host=") && strings.Contains(dsn, "user=") && strings.Contains(dsn, "dbname=") {
		return "postgres"
	}
	// Default to SQLite for file paths or other formats
	return "sqlite"
}

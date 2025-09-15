package storage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/axllent/mailpit/config"
	"github.com/axllent/mailpit/internal/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// GormPostgresDatabase implements the Database interface using GORM for PostgreSQL
type GormPostgresDatabase struct {
	db       *gorm.DB
	sqlDB    *sql.DB
	dsn      string
	driver   string
	tenantID string
}

// NewGormPostgresDatabase creates a new GORM-based PostgreSQL database instance
func NewGormPostgresDatabase(dbConfig DatabaseConfig) (*GormPostgresDatabase, error) {
	// First, ensure the database exists
	if err := ensurePostgresDatabaseExists(dbConfig.DSN); err != nil {
		return nil, fmt.Errorf("failed to ensure database exists: %w", err)
	}

	// Configure GORM logger
	gormLogger := gormlogger.Default.LogMode(gormlogger.Silent)
	if logger.Log().Level.String() == "debug" {
		gormLogger = gormlogger.Default.LogMode(gormlogger.Info)
	}

	// Open GORM connection
	db, err := gorm.Open(postgres.Open(dbConfig.DSN), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, err
	}

	// Get underlying sql.DB for compatibility
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Set reasonable connection pool settings
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)

	gormDB := &GormPostgresDatabase{
		db:       db,
		sqlDB:    sqlDB,
		dsn:      dbConfig.DSN,
		driver:   "postgres",
		tenantID: dbConfig.TenantID,
	}

	// Configure PostgreSQL settings
	if err := gormDB.configurePostgres(); err != nil {
		return nil, err
	}

	return gormDB, nil
}

// convertSQLiteToPostgres converts SQLite-compatible queries to PostgreSQL-compatible queries
func (p *GormPostgresDatabase) convertSQLiteToPostgres(query string) string {
	// Replace ? placeholders with $1, $2, ... format
	placeholderRegex := regexp.MustCompile(`\?`)
	counter := 1

	converted := placeholderRegex.ReplaceAllStringFunc(query, func(match string) string {
		result := fmt.Sprintf("$%d", counter)
		counter++
		return result
	})

	// Fix column name case issues - PostgreSQL is case-sensitive
	converted = strings.ReplaceAll(converted, "MessageID", "message_id")
	converted = strings.ReplaceAll(converted, "SearchText", "search_text")

	// Translate SQLite JSON extraction and IFNULL to PostgreSQL equivalents
	// Pattern: IFNULL(json_extract(Metadata, '$.Key'), '{}')
	// Allow arbitrary whitespace and be case-insensitive
	jsonExtractIFNULL := regexp.MustCompile(`(?i)IFNULL\s*\(\s*json_extract\s*\(\s*Metadata\s*,\s*'\$\.(?:[A-Za-z_][A-Za-z0-9_]*)'\s*\)\s*,\s*'\{\}'\s*\)`) // generic matcher
	// Replace using a function so we can capture the key reliably with another regex
	converted = jsonExtractIFNULL.ReplaceAllStringFunc(converted, func(s string) string {
		keyRe := regexp.MustCompile(`\$\.([A-Za-z_][A-Za-z0-9_]*)`)
		m := keyRe.FindStringSubmatch(s)
		if len(m) == 2 {
			k := m[1]
			return "COALESCE((metadata::jsonb->'" + k + "'),'{}'::jsonb)::text"
		}
		return s
	})

	// Fallback explicit replacements in case the regex missed due to formatting
	pairs := map[string]string{
		"IFNULL(json_extract(Metadata, '$.To'), '{}')":      "COALESCE((metadata::jsonb->'To'),'{}'::jsonb)::text",
		"IFNULL(json_extract(Metadata, '$.From'), '{}')":    "COALESCE((metadata::jsonb->'From'),'{}'::jsonb)::text",
		"IFNULL(json_extract(Metadata, '$.Cc'), '{}')":      "COALESCE((metadata::jsonb->'Cc'),'{}'::jsonb)::text",
		"IFNULL(json_extract(Metadata, '$.Bcc'), '{}')":     "COALESCE((metadata::jsonb->'Bcc'),'{}'::jsonb)::text",
		"IFNULL(json_extract(Metadata, '$.ReplyTo'), '{}')": "COALESCE((metadata::jsonb->'ReplyTo'),'{}'::jsonb)::text",
	}
	for old, newVal := range pairs {
		if strings.Contains(converted, old) {
			converted = strings.ReplaceAll(converted, old, newVal)
		}
	}

	// If query incorrectly mixes projected columns with COUNT(*), rewrite to a pure COUNT(*)
	mixedCountRe := regexp.MustCompile(`(?is)^\s*SELECT\s+.+?,\s*COUNT\(\*\)\s+FROM\s+mailbox\s+m\s+WHERE\s+(.*?)\s+ORDER\s+BY\s+.+$`)
	if mixedCountRe.MatchString(converted) {
		converted = mixedCountRe.ReplaceAllString(converted, "SELECT COUNT(*) FROM mailbox m WHERE $1")
	}

	return converted
}

// configurePostgres sets up PostgreSQL-specific configurations
func (p *GormPostgresDatabase) configurePostgres() error {
	// Set timezone to UTC for consistency
	return p.db.Exec("SET timezone = 'UTC'").Error
}

// Ping implements Database.Ping
func (p *GormPostgresDatabase) Ping() error {
	return p.sqlDB.Ping()
}

// Close implements Database.Close
func (p *GormPostgresDatabase) Close() error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// BeginTx implements Database.BeginTx
func (p *GormPostgresDatabase) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return p.sqlDB.BeginTx(ctx, opts)
}

// Exec implements Database.Exec
func (p *GormPostgresDatabase) Exec(query string, args ...interface{}) (sql.Result, error) {
	convertedQuery := p.convertSQLiteToPostgres(query)
	logger.Log().Debugf("[gorm-postgres] original: %s", query)
	logger.Log().Debugf("[gorm-postgres] converted: %s", convertedQuery)
	return p.sqlDB.Exec(convertedQuery, args...)
}

// Query implements Database.Query
func (p *GormPostgresDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	convertedQuery := p.convertSQLiteToPostgres(query)
	logger.Log().Debugf("[gorm-postgres] original: %s", query)
	logger.Log().Debugf("[gorm-postgres] converted: %s", convertedQuery)
	return p.sqlDB.Query(convertedQuery, args...)
}

// QueryRow implements Database.QueryRow
func (p *GormPostgresDatabase) QueryRow(query string, args ...interface{}) *sql.Row {
	convertedQuery := p.convertSQLiteToPostgres(query)
	logger.Log().Debugf("[gorm-postgres] original: %s", query)
	logger.Log().Debugf("[gorm-postgres] converted: %s", convertedQuery)
	return p.sqlDB.QueryRow(convertedQuery, args...)
}

// ExecContext implements Database.ExecContext
func (p *GormPostgresDatabase) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	convertedQuery := p.convertSQLiteToPostgres(query)
	logger.Log().Debugf("[gorm-postgres] original: %s", query)
	logger.Log().Debugf("[gorm-postgres] converted: %s", convertedQuery)
	return p.sqlDB.ExecContext(ctx, convertedQuery, args...)
}

// QueryContext implements Database.QueryContext
func (p *GormPostgresDatabase) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	convertedQuery := p.convertSQLiteToPostgres(query)
	logger.Log().Debugf("[gorm-postgres] original: %s", query)
	logger.Log().Debugf("[gorm-postgres] converted: %s", convertedQuery)
	return p.sqlDB.QueryContext(ctx, convertedQuery, args...)
}

// QueryRowContext implements Database.QueryRowContext
func (p *GormPostgresDatabase) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	convertedQuery := p.convertSQLiteToPostgres(query)
	logger.Log().Debugf("[gorm-postgres] original: %s", query)
	logger.Log().Debugf("[gorm-postgres] converted: %s", convertedQuery)
	return p.sqlDB.QueryRowContext(ctx, convertedQuery, args...)
}

// GetDbSize implements Database.GetDbSize
func (p *GormPostgresDatabase) GetDbSize() uint64 {
	var total sql.NullFloat64

	// PostgreSQL query to get database size
	query := "SELECT pg_database_size(current_database()) as size"
	err := p.sqlDB.QueryRow(query).Scan(&total)

	if err != nil {
		logger.Log().Errorf("[db] %s", err.Error())
	}

	return uint64(total.Float64)
}

// Vacuum implements Database.Vacuum
func (p *GormPostgresDatabase) Vacuum() error {
	// PostgreSQL VACUUM command
	_, err := p.sqlDB.Exec("VACUUM ANALYZE")
	if err != nil {
		logger.Log().Errorf("[db] VACUUM: %s", err.Error())
		return err
	}

	return nil
}

// GetDriverName implements Database.GetDriverName
func (p *GormPostgresDatabase) GetDriverName() string {
	return p.driver
}

// ApplySchemas implements Database.ApplySchemas
func (p *GormPostgresDatabase) ApplySchemas() error {
	return p.dbApplyGormSchemas()
}

// dbApplyGormSchemas applies schemas using GORM AutoMigrate
func (p *GormPostgresDatabase) dbApplyGormSchemas() error {
	// Auto-migrate all models
	err := p.db.AutoMigrate(
		&GormMailbox{},
		&GormMailboxData{},
		&GormTag{},
		&GormMessageTag{},
		&GormSetting{},
		&GormSchema{},
	)
	if err != nil {
		return err
	}

	// Initialize settings if empty
	setting := &GormSetting{Key: "DeletedSize", Value: "0"}
	p.db.FirstOrCreate(setting, GormSetting{Key: "DeletedSize"})

	return nil
}

// ensurePostgresDatabaseExists checks if the database exists and creates it if it doesn't
func ensurePostgresDatabaseExists(dsn string) error {
	// Parse DSN to extract database name
	// dbName, err := extractDatabaseNameFromDSN(dsn)
	// logger.Log().Infof("[db] dsn: %s", dsn)
	// if err != nil {
	// 	return fmt.Errorf("failed to extract database name from DSN: %w", err)
	// }

	// // Create a connection to the default 'postgres' database
	// var defaultDSN string
	// if strings.HasPrefix(dsn, "postgres://") {
	// 	// URL format
	// 	defaultDSN = strings.Replace(dsn, "/"+dbName+"?", "/postgres?", 1)
	// 	if !strings.Contains(defaultDSN, "/postgres?") {
	// 		defaultDSN = strings.Replace(dsn, "/"+dbName, "/postgres", 1)
	// 	}
	// } else {
	// 	// Key-value format
	// 	defaultDSN = strings.Replace(dsn, "dbname="+dbName, "dbname=postgres", 1)
	// }

	defaultDSN := buildPostgresAdminDSN() // always points to dbname=postgres
	dbName := config.Database

	// Connect to postgres database
	db, err := sql.Open("postgres", defaultDSN)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer db.Close()

	// Check if database exists
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if !exists {
		// Create the database
		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
		if err != nil {
			return fmt.Errorf("failed to create database %s: %w", dbName, err)
		}
		logger.Log().Debugf("[postgres] created database: %s", dbName)
	}

	return nil
}

// extractDatabaseNameFromDSN extracts the database name from a PostgreSQL DSN
func extractDatabaseNameFromDSN(dsn string) (string, error) {
	// Handle URL format: postgres://user:password@host:port/database?sslmode=disable
	if strings.HasPrefix(dsn, "postgres://") {
		parts := strings.Split(dsn, "/")
		if len(parts) < 4 {
			return "", fmt.Errorf("invalid DSN format")
		}

		dbPart := parts[3]
		// Remove query parameters
		if strings.Contains(dbPart, "?") {
			dbPart = strings.Split(dbPart, "?")[0]
		}

		if dbPart == "" {
			return "", fmt.Errorf("no database name in DSN")
		}

		return dbPart, nil
	}

	// Handle key-value format: host=localhost port=5432 dbname=mailpit user=mailpit password=mailpit123 sslmode=disable
	// Parse key-value pairs
	pairs := strings.Fields(dsn)
	for _, pair := range pairs {
		if strings.HasPrefix(pair, "dbname=") {
			return strings.TrimPrefix(pair, "dbname="), nil
		}
	}

	return "", fmt.Errorf("no database name found in DSN")
}

func buildPostgresAdminDSN() string {
	if config.PostgresSocket != "" {
		return fmt.Sprintf("user=%s password=%s host=%s dbname=postgres sslmode=%s",
			config.PostgresUser,
			config.PostgresPassword,
			filepath.Dir(config.PostgresSocket), // socket dir, not file
			config.PostgresSSLMode,
		)
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
		config.PostgresHost,
		config.PostgresPort,
		config.PostgresUser,
		config.PostgresPassword,
		config.PostgresSSLMode,
	)
}

// tenant applies an optional prefix to the table name
func (p *GormPostgresDatabase) tenant(table string) string {
	return fmt.Sprintf("%s%s", p.tenantID, table)
}

// Package storage handles all database actions
package storage

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/axllent/mailpit/config"
	"github.com/axllent/mailpit/internal/logger"
	"github.com/klauspost/compress/zstd"
	"github.com/leporo/sqlf"

	// sqlite - https://gitlab.com/cznic/sqlite
	_ "modernc.org/sqlite"

	// rqlite - https://github.com/rqlite/gorqlite | https://rqlite.io/
	_ "github.com/rqlite/gorqlite/stdlib"
)

var (
	db           Database
	sqlDriver    string
	dbLastAction time.Time

	// zstd compression encoder & decoder
	dbEncoder    *zstd.Encoder
	dbDecoder, _ = zstd.NewReader(nil)

	temporaryFiles = []string{}
)

// InitDB will initialise the database
func InitDB() error {
	// dbEncoder
	var (
		dsn string
		err error
	)

	if config.Compression > 0 {
		var compression zstd.EncoderLevel
		switch config.Compression {
		case 1:
			compression = zstd.SpeedFastest
		case 2:
			compression = zstd.SpeedDefault
		case 3:
			compression = zstd.SpeedBestCompression
		}
		dbEncoder, err = zstd.NewWriter(nil, zstd.WithEncoderLevel(compression))
		if err != nil {
			return err
		}
		logger.Log().Debugf("[db] storing messages with compression: %s", compression.String())
	} else {
		logger.Log().Debug("[db] storing messages with no compression")
	}

	p := config.Database

	// Detect driver and create DSN
	if p == "" {
		// when no path is provided then we create a temporary file
		// which will get deleted on Close(), SIGINT or SIGTERM
		p = fmt.Sprintf("%s-%d.db", path.Join(os.TempDir(), "mailpit"), time.Now().UnixNano())
		// delete the Unix socket file on exit
		AddTempFile(p)
		sqlDriver = "sqlite"
		dsn = p
		logger.Log().Debugf("[db] using temporary database: %s", p)
	} else {
		factory := NewDatabaseFactory()
		if config.DBDriver != "" {
			sqlDriver = config.DBDriver
		} else {
			sqlDriver = factory.DetectDriverFromDSN(p)
		}
		dsn = p

		// If using PostgreSQL and individual components are provided, build DSN from components
		if sqlDriver == "postgres" && config.PostgresHost != "" {
			dsn = buildPostgresDSN()
		}

		logger.Log().Debugf("[db] opening %s database %s", sqlDriver, p)
	}

	config.Database = p

	// Create database using factory
	factory := NewDatabaseFactory()
	dbConfig := DatabaseConfig{
		Driver:   sqlDriver,
		DSN:      dsn,
		TenantID: config.TenantID,
	}

	db, err = factory.CreateDatabase(dbConfig)
	if err != nil {
		return err
	}

	for i := 1; i < 6; i++ {
		if err := Ping(); err != nil {
			logger.Log().Errorf("[db] %s", err.Error())
			logger.Log().Infof("[db] reconnecting in 5 seconds (attempt %d/5)", i)
			time.Sleep(5 * time.Second)
		} else {
			continue
		}
	}

	// create tables if necessary & apply migrations
	if err := db.ApplySchemas(); err != nil {
		return err
	}

	LoadTagFilters()

	dbLastAction = time.Now()

	sigs := make(chan os.Signal, 1)
	// catch all signals since not explicitly listing
	// Program that will listen to the SIGINT and SIGTERM
	// SIGINT will listen to CTRL-C.
	// SIGTERM will be caught if kill command executed
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	// method invoked upon seeing signal
	go func() {
		s := <-sigs
		fmt.Printf("[db] got %s signal, shutting down\n", s)
		Close()
		os.Exit(0)
	}()

	// auto-prune & delete
	go dbCron()

	go dataMigrations()

	return nil
}

// Tenant applies an optional prefix to the table name
func tenant(table string) string {
	return fmt.Sprintf("%s%s", config.TenantID, table)
}

// Close will close the database, and delete if temporary
func Close() {
	// on a fatal exit (eg: ports blocked), allow Mailpit to run migration tasks before closing the DB
	time.Sleep(200 * time.Millisecond)

	if db != nil {
		if err := db.Close(); err != nil {
			logger.Log().Warn("[db] error closing database, ignoring")
		}
	}

	// allow SQLite to finish closing DB & write WAL logs if local
	time.Sleep(100 * time.Millisecond)

	// delete all temporary files
	deleteTempFiles()
}

// Ping the database connection and return an error if unsuccessful
func Ping() error {
	return db.Ping()
}

// StatsGet returns the total/unread statistics for a mailbox
func StatsGet() MailboxStats {
	var (
		total  = CountTotal()
		unread = CountUnread()
		tags   = GetAllTags()
	)

	dbLastAction = time.Now()

	return MailboxStats{
		Total:  total,
		Unread: unread,
		Tags:   tags,
	}
}

// CountTotal returns the number of emails in the database
func CountTotal() uint64 {
	var total float64 // use float64 for rqlite compatibility

	_ = sqlf.From(tenant("mailbox")).
		Select("COUNT(*)").To(&total).
		QueryRowAndClose(context.TODO(), db)

	return uint64(total)
}

// CountUnread returns the number of emails in the database that are unread.
func CountUnread() uint64 {
	var total float64 // use float64 for rqlite compatibility

	_ = sqlf.From(tenant("mailbox")).
		Select("COUNT(*)").To(&total).
		Where("Read = ?", 0).
		QueryRowAndClose(context.TODO(), db)

	return uint64(total)
}

// CountRead returns the number of emails in the database that are read.
func CountRead() uint64 {
	var total float64 // use float64 for rqlite compatibility

	_ = sqlf.From(tenant("mailbox")).
		Select("COUNT(*)").To(&total).
		Where("Read = ?", 1).
		QueryRowAndClose(context.TODO(), db)

	return uint64(total)
}

// DbSize returns the size of the database.
func DbSize() uint64 {
	return db.GetDbSize()
}

// MessageIDExists checks whether a Message-ID exists in the DB
func MessageIDExists(id string) bool {
	var total int

	_ = sqlf.From(tenant("mailbox")).
		Select("COUNT(*)").To(&total).
		Where("MessageID = ?", id).
		QueryRowAndClose(context.TODO(), db)

	return total != 0
}

// buildPostgresDSN builds a PostgreSQL DSN from individual configuration components
func buildPostgresDSN() string {
	var dsn strings.Builder

	// Build the DSN using key=value format
	dsn.WriteString("host=" + config.PostgresHost)

	if config.PostgresPort != "" {
		dsn.WriteString(" port=" + config.PostgresPort)
	} else {
		dsn.WriteString(" port=5432") // Default PostgreSQL port
	}

	if config.PostgresDBName != "" {
		dsn.WriteString(" dbname=" + config.PostgresDBName)
	} else {
		dsn.WriteString(" dbname=mailpit") // Default database name
	}

	if config.PostgresUser != "" {
		dsn.WriteString(" user=" + config.PostgresUser)
	}

	if config.PostgresPassword != "" {
		dsn.WriteString(" password=" + config.PostgresPassword)
	}

	if config.PostgresSSLMode != "" {
		dsn.WriteString(" sslmode=" + config.PostgresSSLMode)
	} else {
		dsn.WriteString(" sslmode=prefer") // Default SSL mode
	}

	return dsn.String()
}

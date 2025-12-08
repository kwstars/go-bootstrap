package gormx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	defaultCharset              = "utf8mb4" // Default connection charset
	defaultParseTime            = true      // Whether to parse time values to time.Time
	defaultAllowNativePasswords = true      // Whether to allow native password authentication
)

// MySQLConfig contains required MySQL connection parameters.
type MySQLConfig struct {
	Username string
	Password string
	Host     string
	Port     int
	Database string
}

// Validate ensures all required fields are populated.
func (c *MySQLConfig) Validate() error {
	switch {
	case c.Username == "":
		return errors.New("username is required")
	case c.Host == "":
		return errors.New("host is required")
	case c.Port == 0:
		return errors.New("port is required")
	case c.Database == "":
		return errors.New("database is required")
	}
	return nil
}

// dsnParams holds DSN query parameters.
type dsnParams struct {
	Charset              string
	ParseTime            bool
	Loc                  *time.Location
	Timeout              time.Duration
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	TLSConfig            string
	AllowNativePasswords bool
}

// poolParams holds connection pool parameters.
type poolParams struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// Option defines the function signature for configuration options.
type Option func(*gorm.Config, *dsnParams, *poolParams) error

// WithLogger sets the GORM logger.
func WithLogger(l logger.Interface) Option {
	return func(cfg *gorm.Config, _ *dsnParams, _ *poolParams) error {
		cfg.Logger = l
		return nil
	}
}

// WithPrepareStmt enables or disables prepared statement cache.
func WithPrepareStmt(prepare bool) Option {
	return func(cfg *gorm.Config, _ *dsnParams, _ *poolParams) error {
		cfg.PrepareStmt = prepare
		return nil
	}
}

// WithCharset sets the connection charset.
func WithCharset(charset string) Option {
	return func(_ *gorm.Config, dsn *dsnParams, _ *poolParams) error {
		if charset == "" {
			return errors.New("charset cannot be empty")
		}
		dsn.Charset = charset
		return nil
	}
}

// WithParseTime sets whether to parse time values to time.Time.
func WithParseTime(parse bool) Option {
	return func(_ *gorm.Config, dsn *dsnParams, _ *poolParams) error {
		dsn.ParseTime = parse
		return nil
	}
}

// WithLocation sets the timezone location for time parsing.
func WithLocation(loc *time.Location) Option {
	return func(_ *gorm.Config, dsn *dsnParams, _ *poolParams) error {
		dsn.Loc = loc
		return nil
	}
}

// WithTimeout sets the connection timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(_ *gorm.Config, dsn *dsnParams, _ *poolParams) error {
		if timeout <= 0 {
			return errors.New("timeout must be positive")
		}
		dsn.Timeout = timeout
		return nil
	}
}

// WithReadTimeout sets the read timeout.
func WithReadTimeout(timeout time.Duration) Option {
	return func(_ *gorm.Config, dsn *dsnParams, _ *poolParams) error {
		if timeout <= 0 {
			return errors.New("read timeout must be positive")
		}
		dsn.ReadTimeout = timeout
		return nil
	}
}

// WithWriteTimeout sets the write timeout.
func WithWriteTimeout(timeout time.Duration) Option {
	return func(_ *gorm.Config, dsn *dsnParams, _ *poolParams) error {
		if timeout <= 0 {
			return errors.New("write timeout must be positive")
		}
		dsn.WriteTimeout = timeout
		return nil
	}
}

// WithTLSConfig sets the TLS configuration name.
func WithTLSConfig(tls string) Option {
	return func(_ *gorm.Config, dsn *dsnParams, _ *poolParams) error {
		dsn.TLSConfig = tls
		return nil
	}
}

// WithAllowNativePasswords sets whether to allow native password authentication.
func WithAllowNativePasswords(allow bool) Option {
	return func(_ *gorm.Config, dsn *dsnParams, _ *poolParams) error {
		dsn.AllowNativePasswords = allow
		return nil
	}
}

// WithConnectionPool sets connection pool parameters.
func WithConnectionPool(maxOpen, maxIdle int, maxLifetime, maxIdleTime time.Duration) Option {
	return func(_ *gorm.Config, _ *dsnParams, pool *poolParams) error {
		if maxOpen < 0 {
			return errors.New("maxOpen cannot be negative")
		}
		if maxIdle < 0 {
			return errors.New("maxIdle cannot be negative")
		}
		pool.MaxOpenConns = maxOpen
		pool.MaxIdleConns = maxIdle
		pool.ConnMaxLifetime = maxLifetime
		pool.ConnMaxIdleTime = maxIdleTime
		return nil
	}
}

// buildDSN constructs the MySQL DSN string.
func buildDSN(cfg *MySQLConfig, params *dsnParams) (string, error) {
	if err := cfg.Validate(); err != nil {
		return "", err
	}

	// Build base DSN: user:pass@tcp(host:port)/database
	var dsnBuilder strings.Builder
	dsnBuilder.WriteString(cfg.Username)
	if cfg.Password != "" {
		dsnBuilder.WriteString(":")
		dsnBuilder.WriteString(cfg.Password)
	}
	dsnBuilder.WriteString(fmt.Sprintf("@tcp(%s:%d)/%s",
		cfg.Host,
		cfg.Port,
		url.PathEscape(cfg.Database),
	))
	dsn := dsnBuilder.String()

	// Build query parameters
	queryParams := url.Values{
		"charset":              {params.Charset},
		"parseTime":            {fmt.Sprintf("%t", params.ParseTime)},
		"allowNativePasswords": {fmt.Sprintf("%t", params.AllowNativePasswords)},
		"multiStatements":      {"true"},
	}

	if params.Loc != nil {
		queryParams.Add("loc", url.QueryEscape(params.Loc.String()))
	}
	if params.Timeout > 0 {
		queryParams.Add("timeout", params.Timeout.String())
	}
	if params.ReadTimeout > 0 {
		queryParams.Add("readTimeout", params.ReadTimeout.String())
	}
	if params.WriteTimeout > 0 {
		queryParams.Add("writeTimeout", params.WriteTimeout.String())
	}
	if params.TLSConfig != "" {
		queryParams.Add("tls", params.TLSConfig)
	}

	return dsn + "?" + queryParams.Encode(), nil
}

// configurePool sets connection pool parameters on the underlying sql.DB.
func configurePool(sqlDB *sql.DB, params *poolParams) {
	if params.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(params.MaxOpenConns)
	}
	if params.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(params.MaxIdleConns)
	}
	if params.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(params.ConnMaxLifetime)
	}
	if params.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(params.ConnMaxIdleTime)
	}
}

// NewMySQLDB creates a new GORM database instance with MySQL connection.
func NewMySQLDB(cfg MySQLConfig, opts ...Option) (*gorm.DB, error) {
	// Initialize with library defaults
	gormCfg := &gorm.Config{}
	dsn := &dsnParams{
		Charset:              defaultCharset,
		ParseTime:            defaultParseTime,
		AllowNativePasswords: defaultAllowNativePasswords,
	}
	pool := &poolParams{}

	// Apply all options
	for _, opt := range opts {
		if err := opt(gormCfg, dsn, pool); err != nil {
			return nil, fmt.Errorf("apply option failed: %w", err)
		}
	}

	// Build DSN
	dsnString, err := buildDSN(&cfg, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to build DSN: %w", err)
	}

	// Open connection
	db, err := gorm.Open(mysql.Open(dsnString), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}
	configurePool(sqlDB, pool)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return db, nil
}

// HealthCheck verifies database connection health.
func HealthCheck(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Close gracefully closes the database connection.
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

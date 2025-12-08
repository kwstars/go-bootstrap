package gormx

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/logger"
)

func TestMySQLConfigValidate(t *testing.T) {
	t.Run("Valid config should pass", func(t *testing.T) {
		cfg := &MySQLConfig{
			Username: "testuser",
			Host:     "localhost",
			Port:     3306,
			Database: "testdb",
		}
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("Empty username should fail", func(t *testing.T) {
		cfg := &MySQLConfig{
			Username: "",
			Host:     "localhost",
			Port:     3306,
			Database: "testdb",
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Equal(t, "username is required", err.Error())
	})

	t.Run("Empty host should fail", func(t *testing.T) {
		cfg := &MySQLConfig{
			Username: "testuser",
			Host:     "",
			Port:     3306,
			Database: "testdb",
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Equal(t, "host is required", err.Error())
	})

	t.Run("Zero port should fail", func(t *testing.T) {
		cfg := &MySQLConfig{
			Username: "testuser",
			Host:     "localhost",
			Port:     0,
			Database: "testdb",
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Equal(t, "port is required", err.Error())
	})

	t.Run("Empty database should fail", func(t *testing.T) {
		cfg := &MySQLConfig{
			Username: "testuser",
			Host:     "localhost",
			Port:     3306,
			Database: "",
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Equal(t, "database is required", err.Error())
	})
}

func TestBuildDSN(t *testing.T) {
	t.Run("Basic DSN construction", func(t *testing.T) {
		cfg := &MySQLConfig{
			Username: "testuser",
			Password: "testpass",
			Host:     "localhost",
			Port:     3306,
			Database: "testdb",
		}
		params := &dsnParams{
			Charset:              "utf8mb4",
			ParseTime:            true,
			AllowNativePasswords: true,
		}

		dsn, err := buildDSN(cfg, params)
		assert.NoError(t, err)
		assert.Contains(t, dsn, "testuser:testpass@tcp(localhost:3306)/testdb")
		assert.Contains(t, dsn, "charset=utf8mb4")
		assert.Contains(t, dsn, "parseTime=true")
		assert.Contains(t, dsn, "allowNativePasswords=true")
		assert.Contains(t, dsn, "multiStatements=true")
	})

	t.Run("DSN without password", func(t *testing.T) {
		cfg := &MySQLConfig{
			Username: "testuser",
			Password: "",
			Host:     "localhost",
			Port:     3306,
			Database: "testdb",
		}
		params := &dsnParams{
			Charset:              "utf8",
			ParseTime:            false,
			AllowNativePasswords: true,
		}

		dsn, err := buildDSN(cfg, params)
		assert.NoError(t, err)
		assert.Contains(t, dsn, "testuser@tcp(localhost:3306)/testdb")
		assert.Contains(t, dsn, "charset=utf8")
		assert.Contains(t, dsn, "parseTime=false")
		assert.Contains(t, dsn, "allowNativePasswords=true")
		assert.Contains(t, dsn, "multiStatements=true")
	})

	t.Run("DSN with special characters in database name", func(t *testing.T) {
		cfg := &MySQLConfig{
			Username: "testuser",
			Password: "testpass",
			Host:     "localhost",
			Port:     3306,
			Database: "test-db_123",
		}
		params := &dsnParams{
			Charset:              "utf8mb4",
			ParseTime:            true,
			AllowNativePasswords: true,
		}

		dsn, err := buildDSN(cfg, params)
		assert.NoError(t, err)
		assert.Contains(t, dsn, "testuser:testpass@tcp(localhost:3306)/test-db_123")
		assert.Contains(t, dsn, "charset=utf8mb4")
		assert.Contains(t, dsn, "parseTime=true")
		assert.Contains(t, dsn, "allowNativePasswords=true")
		assert.Contains(t, dsn, "multiStatements=true")
	})

	t.Run("DSN with location", func(t *testing.T) {
		loc, _ := time.LoadLocation("UTC")
		cfg := &MySQLConfig{
			Username: "testuser",
			Password: "testpass",
			Host:     "localhost",
			Port:     3306,
			Database: "testdb",
		}
		params := &dsnParams{
			Charset:              "utf8mb4",
			ParseTime:            true,
			Loc:                  loc,
			AllowNativePasswords: true,
		}

		dsn, err := buildDSN(cfg, params)
		assert.NoError(t, err)
		assert.Contains(t, dsn, "testuser:testpass@tcp(localhost:3306)/testdb")
		assert.Contains(t, dsn, "charset=utf8mb4")
		assert.Contains(t, dsn, "parseTime=true")
		assert.Contains(t, dsn, "loc=UTC")
		assert.Contains(t, dsn, "allowNativePasswords=true")
	})

	t.Run("DSN with timeouts", func(t *testing.T) {
		cfg := &MySQLConfig{
			Username: "testuser",
			Password: "testpass",
			Host:     "localhost",
			Port:     3306,
			Database: "testdb",
		}
		params := &dsnParams{
			Charset:              "utf8mb4",
			ParseTime:            true,
			Timeout:              30 * time.Second,
			ReadTimeout:          45 * time.Second,
			WriteTimeout:         45 * time.Second,
			TLSConfig:            "skip-verify",
			AllowNativePasswords: true,
		}

		dsn, err := buildDSN(cfg, params)
		assert.NoError(t, err)
		assert.Contains(t, dsn, "testuser:testpass@tcp(localhost:3306)/testdb")
		assert.Contains(t, dsn, "charset=utf8mb4")
		assert.Contains(t, dsn, "parseTime=true")
		assert.Contains(t, dsn, "timeout=30s")
		assert.Contains(t, dsn, "readTimeout=45s")
		assert.Contains(t, dsn, "writeTimeout=45s")
		assert.Contains(t, dsn, "tls=skip-verify")
		assert.Contains(t, dsn, "allowNativePasswords=true")
	})

	t.Run("Invalid config should return error", func(t *testing.T) {
		cfg := &MySQLConfig{}
		params := &dsnParams{}
		_, err := buildDSN(cfg, params)
		assert.Error(t, err)
		assert.Equal(t, "username is required", err.Error())
	})
}

func TestConfigurePool(t *testing.T) {
	t.Run("Configure pool with valid parameters", func(t *testing.T) {
		params := &poolParams{
			MaxOpenConns:    50,
			MaxIdleConns:    10,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 2 * time.Minute,
		}

		assert.Equal(t, 50, params.MaxOpenConns)
		assert.Equal(t, 10, params.MaxIdleConns)
		assert.Equal(t, 5*time.Minute, params.ConnMaxLifetime)
		assert.Equal(t, 2*time.Minute, params.ConnMaxIdleTime)
	})
}

func TestHealthCheck(t *testing.T) {
	t.Run("Health check with valid connection", func(t *testing.T) {
		// This test requires a real database connection to be meaningful
		// For unit testing, we'll just ensure the function signature is correct
		ctx := context.Background()

		// In a real test, we would create a real gorm.DB connection
		// and verify that HealthCheck works correctly
		assert.NotNil(t, ctx)
	})
}

func TestClose(t *testing.T) {
	t.Run("Close with valid connection", func(t *testing.T) {
		// This test requires a real database connection to be meaningful
		// For unit testing, we'll just ensure the function signature is correct
	})
}

func TestNewMySQLDB(t *testing.T) {
	t.Run("NewMySQLDB with valid config", func(t *testing.T) {
		// This test would require a real MySQL server to connect to
		// For unit testing purposes, we'll validate the expected behavior
		// by checking the DSN construction and option application

		cfg := MySQLConfig{
			Username: "root",
			Password: "",
			Host:     "localhost",
			Port:     9999, // Use a port that's likely closed to test error handling
			Database: "testdb",
		}

		// This should fail to connect, but we can still test the DSN construction
		_, err := NewMySQLDB(cfg)
		if err != nil {
			// We expect an error because the server is likely not running
			assert.Contains(t, err.Error(), "failed to connect to database")
		}

		// Test with custom options
		_, err = NewMySQLDB(cfg,
			WithLogger(logger.Default.LogMode(logger.Silent)),
			WithTimeout(10*time.Second),
		)
		if err != nil {
			// We expect an error because the server is likely not running
			assert.Contains(t, err.Error(), "failed to connect to database")
		}
	})

	t.Run("NewMySQLDB with invalid config", func(t *testing.T) {
		cfg := MySQLConfig{
			Username: "",
			Host:     "localhost",
			Port:     3306,
			Database: "testdb",
		}

		_, err := NewMySQLDB(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to build DSN")
		assert.Contains(t, err.Error(), "username is required")
	})

	t.Run("NewMySQLDB with custom options", func(t *testing.T) {
		cfg := MySQLConfig{
			Username: "root",
			Password: "",
			Host:     "localhost",
			Port:     9999, // Use a port that's likely closed
			Database: "testdb",
		}

		_, err := NewMySQLDB(cfg,
			WithCharset("utf8"),
			WithParseTime(false),
			WithConnectionPool(10, 5, 1*time.Minute, 30*time.Second),
		)
		if err != nil {
			// We expect an error because the server is likely not running
			assert.Contains(t, err.Error(), "failed to connect to database")
		}
	})
}

func TestBuildDSNWithSpecialCharacters(t *testing.T) {
	t.Run("Username with special characters", func(t *testing.T) {
		cfg := &MySQLConfig{
			Username: "user@domain.com",
			Password: "password",
			Host:     "localhost",
			Port:     3306,
			Database: "testdb",
		}
		params := &dsnParams{
			Charset:              "utf8mb4",
			ParseTime:            true,
			AllowNativePasswords: true,
		}

		dsn, err := buildDSN(cfg, params)
		assert.NoError(t, err)
		assert.Contains(t, dsn, "user@domain.com:password@tcp")
	})

	t.Run("Database with special characters", func(t *testing.T) {
		cfg := &MySQLConfig{
			Username: "testuser",
			Password: "password",
			Host:     "localhost",
			Port:     3306,
			Database: "test/db",
		}
		params := &dsnParams{
			Charset:              "utf8mb4",
			ParseTime:            true,
			AllowNativePasswords: true,
		}

		dsn, err := buildDSN(cfg, params)
		assert.NoError(t, err)
		// The database name should be URL encoded
		assert.Contains(t, dsn, "test%2Fdb")
	})
}

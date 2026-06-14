package connector

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/Liyonglin20030201/task061402/internal/config"
)

type MySQLConnector struct {
	target config.Target
	db     *sql.DB
}

func NewMySQL(target config.Target) *MySQLConnector {
	return &MySQLConnector{target: target}
}

func (m *MySQLConnector) Connect(ctx context.Context) error {
	cfg := mysql.NewConfig()
	cfg.User = m.target.User
	cfg.Passwd = m.target.Password
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", m.target.Host, m.target.Port)
	cfg.DBName = m.target.Database
	cfg.ParseTime = true

	// 从 context 的 deadline 派生连接超时，确保不会比 context 更长
	dialTimeout := 10 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < dialTimeout {
			dialTimeout = remaining
		}
	}
	cfg.Timeout = dialTimeout

	if charset, ok := m.target.Params["charset"]; ok {
		cfg.Params = map[string]string{"charset": charset}
	}
	if timeoutStr, ok := m.target.Params["timeout"]; ok {
		if d, err := time.ParseDuration(timeoutStr); err == nil {
			cfg.Timeout = d
		}
	}

	dsn := cfg.FormatDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("connection to MySQL %s timed out after %s", m.target.Name, dialTimeout)
		}
		return fmt.Errorf("failed to ping MySQL %s: %w", m.target.Name, err)
	}

	m.db = db
	return nil
}

func (m *MySQLConnector) Close() error {
	if m.db != nil {
		// 强制清除所有空闲连接，确保连接池完全释放
		m.db.SetMaxIdleConns(0)
		m.db.SetMaxOpenConns(0)
		return m.db.Close()
	}
	return nil
}

func (m *MySQLConnector) Type() string { return "mysql" }
func (m *MySQLConnector) Name() string { return m.target.Name }

func (m *MySQLConnector) Ping(ctx context.Context) error {
	if m.db == nil {
		return fmt.Errorf("not connected")
	}
	return m.db.PingContext(ctx)
}

func (m *MySQLConnector) DB() *sql.DB { return m.db }

func (m *MySQLConnector) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if m.db == nil {
		return nil, fmt.Errorf("not connected")
	}
	return m.db.QueryContext(ctx, query, args...)
}

func (m *MySQLConnector) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return m.db.QueryRowContext(ctx, query, args...)
}

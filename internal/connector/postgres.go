package connector

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/Liyonglin20030201/task061402/internal/config"
)

type PostgresConnector struct {
	target config.Target
	db     *sql.DB
}

func NewPostgres(target config.Target) *PostgresConnector {
	return &PostgresConnector{target: target}
}

func (p *PostgresConnector) Connect(ctx context.Context) error {
	sslMode := p.target.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=10",
		p.target.Host, p.target.Port, p.target.User, p.target.Password, p.target.Database, sslMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping PostgreSQL %s: %w", p.target.Name, err)
	}

	p.db = db
	return nil
}

func (p *PostgresConnector) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

func (p *PostgresConnector) Type() string { return "postgres" }
func (p *PostgresConnector) Name() string { return p.target.Name }

func (p *PostgresConnector) Ping(ctx context.Context) error {
	if p.db == nil {
		return fmt.Errorf("not connected")
	}
	return p.db.PingContext(ctx)
}

func (p *PostgresConnector) DB() *sql.DB { return p.db }

func (p *PostgresConnector) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if p.db == nil {
		return nil, fmt.Errorf("not connected")
	}
	return p.db.QueryContext(ctx, query, args...)
}

func (p *PostgresConnector) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return p.db.QueryRowContext(ctx, query, args...)
}

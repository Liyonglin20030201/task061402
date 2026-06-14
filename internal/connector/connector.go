package connector

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Liyonglin20030201/task061402/internal/config"
)

type Connector interface {
	Connect(ctx context.Context) error
	Close() error
	Type() string
	Name() string
	Ping(ctx context.Context) error
}

type SQLConnector interface {
	Connector
	DB() *sql.DB
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row
}

type RedisConnectorInterface interface {
	Connector
	Do(ctx context.Context, args ...interface{}) (interface{}, error)
	Info(ctx context.Context, section string) (string, error)
	SlowLogGet(ctx context.Context, count int64) ([]SlowLogEntry, error)
}

type SlowLogEntry struct {
	ID            int64
	Timestamp     int64
	Duration      int64
	Command       []string
	ClientAddr    string
	ClientName    string
}

func NewFromTarget(target config.Target) (Connector, error) {
	switch target.Type {
	case "mysql":
		return NewMySQL(target), nil
	case "postgres":
		return NewPostgres(target), nil
	case "redis":
		return NewRedis(target), nil
	default:
		return nil, fmt.Errorf("unsupported database type %q for target %q: valid types are mysql, postgres, redis",
			target.Type, target.Name)
	}
}

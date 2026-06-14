package connector

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
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

func (m *MySQLConnector) GetReplicationStatus(ctx context.Context) (*ReplicationStatus, error) {
	if m.db == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Try MySQL 8.0+ syntax first, fall back to legacy
	useNewSyntax := true
	rows, err := m.db.QueryContext(ctx, "SHOW REPLICA STATUS")
	if err != nil {
		useNewSyntax = false
		rows, err = m.db.QueryContext(ctx, "SHOW SLAVE STATUS")
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	if !rows.Next() {
		return &ReplicationStatus{
			Role:          "primary",
			IsReplicating: false,
			Details:       map[string]interface{}{"role": "primary"},
		}, nil
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}
	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, fmt.Errorf("failed to scan replication status: %w", err)
	}

	colMap := make(map[string]interface{})
	for i, col := range columns {
		colMap[col] = values[i]
	}

	status := &ReplicationStatus{
		Role:    "replica",
		Details: make(map[string]interface{}),
	}

	// Use the correct column names based on which query succeeded
	var ioRunningKey, sqlRunningKey, lagKey, ioErrKey, sqlErrKey, masterHostKey, masterPortKey string
	if useNewSyntax {
		ioRunningKey = "Replica_IO_Running"
		sqlRunningKey = "Replica_SQL_Running"
		lagKey = "Seconds_Behind_Source"
		ioErrKey = "Last_IO_Error"
		sqlErrKey = "Last_SQL_Error"
		masterHostKey = "Source_Host"
		masterPortKey = "Source_Port"
	} else {
		ioRunningKey = "Slave_IO_Running"
		sqlRunningKey = "Slave_SQL_Running"
		lagKey = "Seconds_Behind_Master"
		ioErrKey = "Last_IO_Error"
		sqlErrKey = "Last_SQL_Error"
		masterHostKey = "Master_Host"
		masterPortKey = "Master_Port"
	}

	ioRunning := colValToString(colMap[ioRunningKey])
	sqlRunning := colValToString(colMap[sqlRunningKey])
	status.IORunning = strings.EqualFold(ioRunning, "Yes")
	status.SQLRunning = strings.EqualFold(sqlRunning, "Yes")
	status.IsReplicating = status.IORunning && status.SQLRunning
	status.LastIOError = colValToString(colMap[ioErrKey])
	status.LastSQLError = colValToString(colMap[sqlErrKey])
	status.MasterHost = colValToString(colMap[masterHostKey])
	status.MasterPort = colValToString(colMap[masterPortKey])

	// Lag: NULL means the SQL thread cannot determine lag (broken state)
	lagRaw := colMap[lagKey]
	if lagRaw == nil || isNullBytes(lagRaw) {
		status.LagIsNull = true
		status.LagSeconds = -1
	} else {
		status.LagIsNull = false
		status.LagSeconds = colValToInt(lagRaw)
	}

	status.Details["io_running"] = ioRunning
	status.Details["sql_running"] = sqlRunning
	status.Details["last_io_error"] = status.LastIOError
	status.Details["last_sql_error"] = status.LastSQLError

	return status, nil
}

func colValToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case []byte:
		return string(val)
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

func colValToInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return int(val)
	case []byte:
		n, _ := strconv.Atoi(string(val))
		return n
	case string:
		n, _ := strconv.Atoi(val)
		return n
	default:
		return 0
	}
}

func isNullBytes(v interface{}) bool {
	if b, ok := v.([]byte); ok {
		return b == nil || strings.EqualFold(string(b), "null")
	}
	return false
}

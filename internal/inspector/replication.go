package inspector

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Liyonglin20030201/task061402/internal/config"
	"github.com/Liyonglin20030201/task061402/internal/connector"
)

type ReplicationInspector struct{}

func NewReplicationInspector() *ReplicationInspector { return &ReplicationInspector{} }

func (r *ReplicationInspector) Name() string { return "replication" }

func (r *ReplicationInspector) Run(ctx context.Context, conn connector.Connector, cfg *config.Config) (*Result, error) {
	result := NewResult("replication")

	if !cfg.Checks.Replication.Enabled {
		return result.Finish(StatusSkipped, "replication check disabled"), nil
	}

	switch conn.Type() {
	case "mysql":
		return r.runMySQL(ctx, conn, cfg, result)
	case "postgres":
		return r.runPostgres(ctx, conn, cfg, result)
	case "redis":
		return r.runRedis(ctx, conn, cfg, result)
	default:
		return result.Finish(StatusSkipped, fmt.Sprintf("replication check not supported for %s", conn.Type())), nil
	}
}

func (r *ReplicationInspector) runMySQL(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	sqlConn, ok := conn.(connector.SQLConnector)
	if !ok {
		return result.Finish(StatusSkipped, "connector does not support SQL queries"), nil
	}

	rows, err := sqlConn.Query(ctx, "SHOW REPLICA STATUS")
	if err != nil {
		rows, err = sqlConn.Query(ctx, "SHOW SLAVE STATUS")
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "access denied") ||
				strings.Contains(strings.ToLower(err.Error()), "permission") {
				result.RiskScore = 20
				result.Details["error"] = err.Error()
				result.Details["required_privilege"] = "REPLICATION CLIENT"
				return result.Finish(StatusWarning, "insufficient privilege to check replication status"), nil
			}
			return nil, fmt.Errorf("failed to query replication status: %w", err)
		}
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	if !rows.Next() {
		result.Details["role"] = "primary"
		return result.Finish(StatusSuccess, "server is a primary (no replication configured)"), nil
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

	result.Details["role"] = "replica"

	ioRunning := byteToString(colMap["Slave_IO_Running"])
	if ioRunning == "" {
		ioRunning = byteToString(colMap["Replica_IO_Running"])
	}
	sqlRunning := byteToString(colMap["Slave_SQL_Running"])
	if sqlRunning == "" {
		sqlRunning = byteToString(colMap["Replica_SQL_Running"])
	}

	result.Details["io_running"] = ioRunning
	result.Details["sql_running"] = sqlRunning

	if !strings.EqualFold(ioRunning, "Yes") {
		result.RiskScore = 90
		result.Details["suggestion"] = "IO thread stopped. Check network connectivity to primary, verify MASTER_HOST/MASTER_PORT settings, ensure primary's binlog is accessible. Run SHOW REPLICA STATUS for Last_IO_Error."
		return result.Finish(StatusError, "replication IO thread is not running"), nil
	}
	if !strings.EqualFold(sqlRunning, "Yes") {
		result.RiskScore = 90
		result.Details["suggestion"] = "SQL thread stopped. Check Last_SQL_Error for details. May need STOP REPLICA; SET GLOBAL SQL_SLAVE_SKIP_COUNTER=1; START REPLICA; for non-critical errors, or restore from backup for data corruption."
		return result.Finish(StatusError, "replication SQL thread is not running"), nil
	}

	lagRaw := colMap["Seconds_Behind_Master"]
	if lagRaw == nil {
		lagRaw = colMap["Seconds_Behind_Source"]
	}

	lag := parseLag(lagRaw)
	result.Details["seconds_behind"] = lag

	maxLag := cfg.Checks.Replication.MaxLagSeconds
	criticalLag := cfg.Checks.Replication.CriticalLagSeconds

	if lag >= criticalLag {
		result.RiskScore = 85
		result.Details["suggestion"] = fmt.Sprintf("Replication lag is %ds (critical threshold: %ds). Check for long-running transactions on primary, increase replica parallel workers (replica_parallel_workers), or check replica IO/disk bottlenecks.", lag, criticalLag)
		return result.Finish(StatusError, fmt.Sprintf("critical replication lag: %d seconds", lag)), nil
	}
	if lag >= maxLag {
		result.RiskScore = 50
		result.Details["suggestion"] = fmt.Sprintf("Replication lag is %ds (warn threshold: %ds). Monitor trend; consider tuning replica_parallel_workers or reducing write load on primary.", lag, maxLag)
		return result.Finish(StatusWarning, fmt.Sprintf("replication lag: %d seconds (threshold: %ds)", lag, maxLag)), nil
	}

	return result.Finish(StatusSuccess, fmt.Sprintf("replication healthy, lag: %d seconds", lag)), nil
}

func (r *ReplicationInspector) runPostgres(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	sqlConn, ok := conn.(connector.SQLConnector)
	if !ok {
		return result.Finish(StatusSkipped, "connector does not support SQL queries"), nil
	}

	var isInRecovery bool
	row := sqlConn.QueryRow(ctx, "SELECT pg_is_in_recovery()")
	if err := row.Scan(&isInRecovery); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "permission") {
			result.RiskScore = 20
			result.Details["error"] = err.Error()
			result.Details["required_privilege"] = "pg_monitor"
			return result.Finish(StatusWarning, "insufficient privilege to check replication status"), nil
		}
		return nil, fmt.Errorf("failed to check recovery state: %w", err)
	}

	if isInRecovery {
		return r.runPostgresReplica(ctx, sqlConn, cfg, result)
	}
	return r.runPostgresPrimary(ctx, sqlConn, cfg, result)
}

func (r *ReplicationInspector) runPostgresPrimary(ctx context.Context, conn connector.SQLConnector, cfg *config.Config, result *Result) (*Result, error) {
	result.Details["role"] = "primary"

	rows, err := conn.Query(ctx, `SELECT client_addr, state,
		COALESCE(EXTRACT(EPOCH FROM write_lag)::int, 0) as write_lag_s,
		COALESCE(EXTRACT(EPOCH FROM replay_lag)::int, 0) as replay_lag_s
		FROM pg_stat_replication`)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "permission") {
			result.RiskScore = 20
			result.Details["required_privilege"] = "pg_monitor"
			return result.Finish(StatusWarning, "insufficient privilege to query pg_stat_replication"), nil
		}
		return nil, fmt.Errorf("failed to query pg_stat_replication: %w", err)
	}
	defer rows.Close()

	type replicaInfo struct {
		ClientAddr string `json:"client_addr"`
		State      string `json:"state"`
		WriteLag   int    `json:"write_lag_s"`
		ReplayLag  int    `json:"replay_lag_s"`
	}
	var replicas []replicaInfo
	maxLag := 0

	for rows.Next() {
		var r replicaInfo
		if err := rows.Scan(&r.ClientAddr, &r.State, &r.WriteLag, &r.ReplayLag); err != nil {
			continue
		}
		replicas = append(replicas, r)
		if r.ReplayLag > maxLag {
			maxLag = r.ReplayLag
		}
	}

	result.Details["connected_replicas"] = len(replicas)
	result.Details["replicas"] = replicas
	result.Details["max_replay_lag_s"] = maxLag

	if len(replicas) == 0 {
		result.Details["suggestion"] = "No replicas connected. If replicas are expected, check their pg_hba.conf, connection strings, and network."
		return result.Finish(StatusSuccess, "primary server with no connected replicas"), nil
	}

	criticalLag := cfg.Checks.Replication.CriticalLagSeconds
	maxLagThreshold := cfg.Checks.Replication.MaxLagSeconds

	if maxLag >= criticalLag {
		result.RiskScore = 85
		result.Details["suggestion"] = fmt.Sprintf("Replay lag %ds exceeds critical threshold. Check replica performance (CPU/IO), WAL sender activity, and network bandwidth between primary and replica.", maxLag)
		return result.Finish(StatusError, fmt.Sprintf("critical replay lag on replica: %ds", maxLag)), nil
	}
	if maxLag >= maxLagThreshold {
		result.RiskScore = 50
		result.Details["suggestion"] = fmt.Sprintf("Replay lag %ds exceeds warn threshold. Monitor replica apply rate.", maxLag)
		return result.Finish(StatusWarning, fmt.Sprintf("replay lag on replica: %ds", maxLag)), nil
	}

	return result.Finish(StatusSuccess, fmt.Sprintf("primary with %d healthy replica(s), max lag: %ds", len(replicas), maxLag)), nil
}

func (r *ReplicationInspector) runPostgresReplica(ctx context.Context, conn connector.SQLConnector, cfg *config.Config, result *Result) (*Result, error) {
	result.Details["role"] = "replica"

	var status, senderHost string
	var lastMsgReceive interface{}
	row := conn.QueryRow(ctx, `SELECT status, sender_host, last_msg_receipt_time FROM pg_stat_wal_receiver LIMIT 1`)
	err := row.Scan(&status, &senderHost, &lastMsgReceive)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") || strings.Contains(err.Error(), "does not exist") {
			return result.Finish(StatusSuccess, "replica with no WAL receiver info (possibly using file-based replication)"), nil
		}
		if strings.Contains(strings.ToLower(err.Error()), "permission") {
			result.RiskScore = 20
			result.Details["required_privilege"] = "pg_monitor"
			return result.Finish(StatusWarning, "insufficient privilege to query pg_stat_wal_receiver"), nil
		}
		return nil, fmt.Errorf("failed to query pg_stat_wal_receiver: %w", err)
	}

	result.Details["wal_receiver_status"] = status
	result.Details["sender_host"] = senderHost

	if status != "streaming" {
		result.RiskScore = 85
		result.Details["suggestion"] = "WAL receiver is not streaming. Check network to primary, verify primary's pg_hba.conf, and confirm replication slot exists."
		return result.Finish(StatusError, fmt.Sprintf("WAL receiver status: %s (expected: streaming)", status)), nil
	}

	return result.Finish(StatusSuccess, fmt.Sprintf("replica streaming from %s", senderHost)), nil
}

func (r *ReplicationInspector) runRedis(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	redisConn, ok := conn.(connector.RedisConnectorInterface)
	if !ok {
		return result.Finish(StatusSkipped, "connector does not support Redis commands"), nil
	}

	info, err := redisConn.Info(ctx, "replication")
	if err != nil {
		return nil, fmt.Errorf("failed to get replication info: %w", err)
	}

	infoMap := parseRedisInfo(info)
	role := infoMap["role"]
	result.Details["role"] = role

	if role == "master" {
		connectedSlaves := 0
		if v, ok := infoMap["connected_slaves"]; ok {
			connectedSlaves, _ = strconv.Atoi(v)
		}
		result.Details["connected_slaves"] = connectedSlaves

		maxLag := 0
		for i := 0; i < connectedSlaves; i++ {
			slaveKey := fmt.Sprintf("slave%d", i)
			if slaveInfo, ok := infoMap[slaveKey]; ok {
				lag := parseRedisSlaveOffset(slaveInfo, infoMap["master_repl_offset"])
				if lag > maxLag {
					maxLag = lag
				}
			}
		}
		result.Details["max_offset_lag"] = maxLag

		criticalLag := cfg.Checks.Replication.CriticalLagSeconds
		maxLagThreshold := cfg.Checks.Replication.MaxLagSeconds

		if connectedSlaves == 0 {
			return result.Finish(StatusSuccess, "Redis master with no connected replicas"), nil
		}

		if maxLag >= criticalLag*1000 {
			result.RiskScore = 85
			result.Details["suggestion"] = "Large offset lag detected. Check replica network connection, memory usage, and RDB/AOF disk IO on replicas."
			return result.Finish(StatusError, fmt.Sprintf("critical replication offset lag: %d bytes", maxLag)), nil
		}
		if maxLag >= maxLagThreshold*1000 {
			result.RiskScore = 50
			return result.Finish(StatusWarning, fmt.Sprintf("replication offset lag: %d bytes", maxLag)), nil
		}

		return result.Finish(StatusSuccess, fmt.Sprintf("Redis master with %d replica(s), max offset lag: %d bytes", connectedSlaves, maxLag)), nil
	}

	masterLinkStatus := infoMap["master_link_status"]
	result.Details["master_link_status"] = masterLinkStatus
	result.Details["master_host"] = infoMap["master_host"]
	result.Details["master_port"] = infoMap["master_port"]

	if masterLinkStatus != "up" {
		result.RiskScore = 90
		result.Details["suggestion"] = "Master link is down. Check network connectivity to master, verify master is running, and check requirepass/masterauth configuration."
		return result.Finish(StatusError, "Redis replica: master link is down"), nil
	}

	secondsBehind, _ := strconv.Atoi(infoMap["master_last_io_seconds_ago"])
	result.Details["master_last_io_seconds_ago"] = secondsBehind

	if secondsBehind > cfg.Checks.Replication.CriticalLagSeconds {
		result.RiskScore = 85
		result.Details["suggestion"] = "No recent communication from master. Check master health and network."
		return result.Finish(StatusError, fmt.Sprintf("no master communication for %ds", secondsBehind)), nil
	}

	return result.Finish(StatusSuccess, fmt.Sprintf("Redis replica connected to %s:%s", infoMap["master_host"], infoMap["master_port"])), nil
}

func parseRedisInfo(info string) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}

func parseRedisSlaveOffset(slaveInfo string, masterOffset string) int {
	masterOff, _ := strconv.Atoi(masterOffset)
	fields := strings.Split(slaveInfo, ",")
	for _, f := range fields {
		kv := strings.SplitN(f, "=", 2)
		if len(kv) == 2 && kv[0] == "offset" {
			slaveOff, _ := strconv.Atoi(kv[1])
			lag := masterOff - slaveOff
			if lag < 0 {
				lag = 0
			}
			return lag
		}
	}
	return 0
}

func parseLag(raw interface{}) int {
	if raw == nil {
		return 0
	}
	switch v := raw.(type) {
	case int64:
		return int(v)
	case []byte:
		n, _ := strconv.Atoi(string(v))
		return n
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
}

func byteToString(v interface{}) string {
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

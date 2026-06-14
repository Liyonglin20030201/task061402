package inspector

import (
	"context"
	"fmt"
	"strings"

	"github.com/Liyonglin20030201/task061402/internal/config"
	"github.com/Liyonglin20030201/task061402/internal/connector"
)

type PermissionInspector struct{}

func NewPermissionInspector() *PermissionInspector { return &PermissionInspector{} }

func (p *PermissionInspector) Name() string { return "permission" }

func (p *PermissionInspector) Run(ctx context.Context, conn connector.Connector, cfg *config.Config) (*Result, error) {
	result := NewResult("permission")

	if !cfg.Checks.Permission.Enabled {
		return result.Finish(StatusSkipped, "permission check disabled"), nil
	}

	switch conn.Type() {
	case "mysql":
		return p.runMySQL(ctx, conn, cfg, result)
	case "postgres":
		return p.runPostgres(ctx, conn, cfg, result)
	case "redis":
		return p.runRedis(ctx, conn, cfg, result)
	default:
		return result.Finish(StatusSkipped, fmt.Sprintf("unsupported type: %s", conn.Type())), nil
	}
}

func (p *PermissionInspector) runMySQL(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	sqlConn, ok := conn.(connector.SQLConnector)
	if !ok {
		return result.Finish(StatusError, "invalid connector type"), nil
	}

	query := `SELECT user, host, Super_priv, Grant_priv, File_priv, Shutdown_priv, Process_priv
		FROM mysql.user WHERE user NOT IN ('mysql.sys', 'mysql.session', 'mysql.infoschema')`

	rows, err := sqlConn.Query(ctx, query)
	if err != nil {
		result.RiskScore = 20
		return result.Finish(StatusWarning, fmt.Sprintf("insufficient privilege to scan permissions: %v", err)), nil
	}
	defer rows.Close()

	var users []map[string]interface{}
	var risks []string
	for rows.Next() {
		var user, host, superPriv, grantPriv, filePriv, shutdownPriv, processPriv string
		if err := rows.Scan(&user, &host, &superPriv, &grantPriv, &filePriv, &shutdownPriv, &processPriv); err != nil {
			continue
		}

		entry := map[string]interface{}{
			"user":          user,
			"host":          host,
			"super":         superPriv,
			"grant":         grantPriv,
			"file":          filePriv,
			"shutdown":      shutdownPriv,
			"process":       processPriv,
		}
		users = append(users, entry)

		if superPriv == "Y" {
			for _, pattern := range cfg.Checks.Permission.DenyPatterns {
				if strings.Contains(strings.ToUpper(pattern), "SUPER") {
					risks = append(risks, fmt.Sprintf("user %s@%s has SUPER privilege", user, host))
					break
				}
			}
		}
		if grantPriv == "Y" {
			for _, pattern := range cfg.Checks.Permission.DenyPatterns {
				if strings.Contains(strings.ToUpper(pattern), "GRANT ALL") {
					risks = append(risks, fmt.Sprintf("user %s@%s has GRANT privilege", user, host))
					break
				}
			}
		}
	}

	result.Details["users"] = users
	result.Details["risks"] = risks

	if len(risks) == 0 {
		result.RiskScore = 0
		return result.Finish(StatusSuccess, fmt.Sprintf("scanned %d users, no permission risks", len(users))), nil
	}

	result.RiskScore = min(len(risks)*20, 80)
	return result.Finish(StatusWarning, fmt.Sprintf("%d permission risks found", len(risks))), nil
}

func (p *PermissionInspector) runPostgres(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	sqlConn, ok := conn.(connector.SQLConnector)
	if !ok {
		return result.Finish(StatusError, "invalid connector type"), nil
	}

	query := `SELECT rolname, rolsuper, rolcreaterole, rolcreatedb, rolcanlogin, rolreplication
		FROM pg_roles WHERE rolname NOT LIKE 'pg_%'`

	rows, err := sqlConn.Query(ctx, query)
	if err != nil {
		result.RiskScore = 20
		return result.Finish(StatusWarning, fmt.Sprintf("insufficient privilege to scan roles: %v", err)), nil
	}
	defer rows.Close()

	var roles []map[string]interface{}
	var risks []string
	for rows.Next() {
		var name string
		var isSuper, createRole, createDB, canLogin, replication bool
		if err := rows.Scan(&name, &isSuper, &createRole, &createDB, &canLogin, &replication); err != nil {
			continue
		}

		entry := map[string]interface{}{
			"name":        name,
			"superuser":   isSuper,
			"createrole":  createRole,
			"createdb":    createDB,
			"canlogin":    canLogin,
			"replication": replication,
		}
		roles = append(roles, entry)

		if isSuper && canLogin {
			risks = append(risks, fmt.Sprintf("role %s is a superuser with login", name))
		}
	}

	result.Details["roles"] = roles
	result.Details["risks"] = risks

	if len(risks) == 0 {
		result.RiskScore = 0
		return result.Finish(StatusSuccess, fmt.Sprintf("scanned %d roles, no permission risks", len(roles))), nil
	}

	result.RiskScore = min(len(risks)*25, 90)
	return result.Finish(StatusWarning, fmt.Sprintf("%d permission risks found", len(risks))), nil
}

func (p *PermissionInspector) runRedis(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	redisConn, ok := conn.(*connector.RedisConnector)
	if !ok {
		return result.Finish(StatusError, "invalid connector type"), nil
	}

	info, err := redisConn.Info(ctx, "server")
	if err != nil {
		return result.Finish(StatusWarning, fmt.Sprintf("failed to get Redis info: %v", err)), nil
	}

	var risks []string
	for _, line := range strings.Split(info, "\r\n") {
		if strings.HasPrefix(line, "requirepass:") {
			pass := strings.TrimPrefix(line, "requirepass:")
			if pass == "" {
				risks = append(risks, "Redis has no password set")
			}
		}
	}

	configInfo, err := redisConn.Info(ctx, "all")
	if err == nil {
		if strings.Contains(configInfo, "protected-mode:no") {
			risks = append(risks, "Redis protected-mode is disabled")
		}
	}

	result.Details["risks"] = risks

	if len(risks) == 0 {
		result.RiskScore = 0
		return result.Finish(StatusSuccess, "Redis permission check passed"), nil
	}

	result.RiskScore = min(len(risks)*30, 80)
	return result.Finish(StatusWarning, fmt.Sprintf("%d Redis security risks found", len(risks))), nil
}

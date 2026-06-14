# dbinspect

数据库巡检命令行工具，支持 MySQL、PostgreSQL 和 Redis 的全面健康检查。

## 功能特性

| 功能 | 命令 | 说明 |
|------|------|------|
| 连接检测 | `dbinspect ping` | 测试数据库连通性与响应延迟 |
| 慢查询分析 | `dbinspect slowquery` | 分析 performance_schema / pg_stat_statements / Redis slowlog |
| 容量统计 | `dbinspect capacity` | 数据库/表容量、内存使用统计 |
| 索引建议 | `dbinspect index` | 检测重复索引、未使用索引、缺失主键 |
| 备份校验 | `dbinspect backup` | 验证备份文件存在性、时效性 |
| 权限扫描 | `dbinspect permission` | 审计用户权限，识别高危授权 |
| 风险评分 | `dbinspect risk` | 综合评估各项检查结果，输出加权风险分 |
| 报告导出 | `dbinspect report` | 导出 HTML/JSON/CSV 格式报告 |
| 完整巡检 | `dbinspect inspect` | 执行全部检查项并汇总 |
| 插件扩展 | `dbinspect plugin` | 管理和运行自定义检查插件 |

## 技术栈

- **语言**: Go 1.22+
- **CLI框架**: [Cobra](https://github.com/spf13/cobra)
- **配置管理**: YAML (gopkg.in/yaml.v3)
- **本地存储**: SQLite (modernc.org/sqlite, 纯Go实现)
- **数据库驱动**: go-sql-driver/mysql, lib/pq, go-redis/v9

## 快速开始

### 安装

```bash
# 从源码构建
git clone https://github.com/Liyonglin20030201/task061402.git
cd task061402
go mod tidy
make build

# 或直接安装
go install github.com/Liyonglin20030201/task061402@latest
```

### 配置

```bash
# 复制示例配置
cp configs/example.yaml dbinspect.yaml

# 编辑配置文件，填入实际的数据库连接信息
# 支持环境变量引用: ${MYSQL_PASSWORD}
```

### 使用

```bash
# 测试连接
./bin/dbinspect ping --target prod-mysql

# 运行完整巡检
./bin/dbinspect inspect --target prod-mysql --report

# 仅分析慢查询
./bin/dbinspect slowquery --target prod-mysql --top 10

# 生成报告
./bin/dbinspect report --format html --output ./reports/

# 查看风险评分
./bin/dbinspect risk --target prod-mysql -v
```

## 命令设计

```
dbinspect
├── ping         [--target name]                    # 连接检测
├── slowquery    [--target name] [--top N]          # 慢查询分析
├── capacity     [--target name]                    # 容量统计
├── index        [--target name]                    # 索引建议
├── backup       [--target name]                    # 备份校验
├── permission   [--target name]                    # 权限扫描
├── risk         [--target name]                    # 风险评分
├── report       [--format html|json|csv] [--output path]  # 报告导出
├── inspect      [--target name] [--report]         # 完整巡检
├── plugin       list|run <name>                    # 插件管理
└── version                                         # 版本信息

全局参数:
  --config, -c    配置文件路径 (默认: ./dbinspect.yaml)
  --target, -t    目标数据库名称
  --timeout       全局超时 (默认: 30s)
  --verbose, -v   详细输出
  --log-level     日志级别: debug|info|warn|error
```

## 配置文件

详见 [configs/example.yaml](configs/example.yaml)，支持：

- 多目标数据库配置
- 环境变量引用 (`${VAR_NAME}`)
- 每项检查独立开关与阈值
- 风险权重自定义
- 插件目录配置

### 配置校验规则

- `targets[].type` 必须为 mysql/postgres/redis
- `targets[].port` 范围 1-65535
- `risk.weights` 各项权重之和必须为 100
- `report.format` 必须为 html/json/csv
- 容量阈值: warn < critical

## 数据表定义

巡检结果存储在本地 SQLite 数据库 (`data/dbinspect.db`)：

```sql
-- 巡检结果
CREATE TABLE inspections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL,
    target_name TEXT NOT NULL,
    target_type TEXT NOT NULL,
    check_type TEXT NOT NULL,
    status TEXT NOT NULL,        -- success|warning|error|skipped
    risk_score INTEGER DEFAULT 0,
    summary TEXT,
    details TEXT,               -- JSON
    started_at DATETIME NOT NULL,
    finished_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 操作日志
CREATE TABLE operation_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL,
    level TEXT NOT NULL,
    component TEXT NOT NULL,
    message TEXT NOT NULL,
    metadata TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 报告记录
CREATE TABLE reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL,
    format TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## 巡检执行流程

```
1. CLI 入口 → Cobra 解析命令与参数
2. PersistentPreRun:
   ├── 加载 YAML 配置 → 校验 → 解析环境变量
   ├── 初始化 SQLite 存储 (自动迁移)
   ├── 初始化日志器 (stdout + SQLite)
   └── 生成 run_id (UUID)
3. 命令处理:
   ├── 解析目标 (--target 或全部)
   ├── 对每个目标:
   │   ├── 创建带超时的连接器
   │   ├── 运行检查器 (支持 context 取消)
   │   ├── 捕获结果 → 存入 SQLite
   │   └── 记录每步日志
   └── 计算风险评分
4. 报告生成 (如需要):
   ├── 从 SQLite 加载结果
   └── 写入临时文件 → 原子重命名
5. 输出摘要 → 退出码: 0=健康, 1=警告, 2=严重
```

## 异常处理

| 异常场景 | 处理策略 |
|----------|----------|
| 连接超时 | 可配置超时 + 重试机制 + 逐次记录 |
| 权限不足 | 捕获特定错误码，标记为 skipped，继续其他检查 |
| SQL执行失败 | panic 恢复 + 错误上下文包装 + 脱敏日志 |
| 大库扫描超时 | 基于 context 的取消机制 + 部分结果保存 |
| 报告生成中断 | 临时文件写入 + 原子重命名 + 失败清理 |
| 配置错误 | 全量校验(不逐项失败) + 清晰错误信息 |

## 插件系统

插件放置在配置指定的目录（默认 `./plugins/`），支持：

- Shell 脚本 (.sh)
- Python 脚本 (.py)
- 可执行文件

插件通过环境变量接收目标信息，输出 JSON 格式结果：

```json
{
  "status": "warning",
  "risk_score": 45,
  "summary": "Custom check found issues",
  "details": {"key": "value"}
}
```

## 测试

```bash
# 单元测试
make test

# 带覆盖率
make test-cover

# 静态分析
make vet
```

## 项目结构

```
├── main.go                 # 入口
├── cmd/                    # Cobra 命令
├── internal/
│   ├── config/             # 配置加载与校验
│   ├── connector/          # 数据库连接管理
│   ├── inspector/          # 巡检逻辑
│   ├── store/              # SQLite 持久化
│   ├── report/             # 报告生成
│   ├── plugin/             # 插件系统
│   └── logger/             # 结构化日志
├── configs/                # 示例配置
├── plugins/                # 插件目录
└── tests/                  # 测试数据
```

## 部署

### 交叉编译

```bash
make build-linux    # Linux amd64
make build-windows  # Windows amd64
make build-darwin   # macOS amd64
```

### 退出码

- `0`: 所有检查通过，数据库健康
- `1`: 存在警告，需关注
- `2`: 存在严重问题，需立即处理

## License

MIT

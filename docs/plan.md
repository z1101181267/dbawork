# DBAWORK 数据源配置 + 驱动管理实现方案

## Context

对齐开源工具 DBCheck (github.com/fiyo/DBCheck) 的数据源配置菜单和数据库驱动管理菜单，采用 Go + Python 组合构建数据库综合管理平台。Go 负责 API 网关与元数据 CRUD，Python 负责数据库适配器与驱动运行时，Vue 3 + Element Plus 负责前端界面。Go-Python 间通过 gRPC 通信，元数据存 SQLite。服务器隧道同时支持 Linux SSH 和 Windows WinRM 两种方式。

## 整体架构

```
Vue 3 + Element Plus (前端)
    │ HTTPS / JSON / WebSocket
Go 后端 (Gin) — API 网关 + SQLite 元数据 CRUD + 凭据 AES-GCM 加密
    │ gRPC (protobuf)
Python 后端 (FastAPI + gRPC server) — 数据库适配器 + 驱动运行时 + 服务器隧道(SSH/WinRM)
```

**职责切分：**
- Go：datasources/drivers/groups 表 CRUD、凭据加密存储、文件落盘路径分配、HTTP 路由/认证
- Python：实际连库、pip 包检测/安装、连接测试、SSH/WinRM 隧道建立、巡检逻辑
- 驱动元数据由 Go 写入 SQLite；"是否可用"状态由 Python 检测后回写 Go 同步 `installed` 字段

## 项目目录结构

```
DBAWORK/
├── go-backend/
│   ├── cmd/dbawork/main.go
│   ├── internal/
│   │   ├── config/
│   │   ├── db/                        # SQLite 连接 + 迁移
│   │   │   ├── sqlite.go
│   │   │   └── migrations/001_init.sql
│   │   ├── model/                     # datasource/group/driver 模型
│   │   ├── repository/               # CRUD
│   │   ├── service/                  # 业务编排 (repo + gRPC client)
│   │   ├── handler/                  # Gin handler
│   │   ├── middleware/               # JWT auth / CORS / recover
│   │   ├── router/
│   │   ├── crypto/                   # AES-GCM
│   │   └── grpcclient/              # Python gRPC 客户端封装
│   ├── pkg/csvimport/                # CSV 批量导入解析
│   └── go.mod
├── py-backend/
│   ├── adapters/                     # 数据库适配器 (对标 DBCheck main_*.py)
│   │   ├── base.py
│   │   ├── mysql_adapter.py           # pymysql
│   │   ├── oracle_adapter.py          # cx_Oracle
│   │   ├── postgres_adapter.py        # psycopg2
│   │   ├── mssql_adapter.py           # pymssql
│   │   └── registry.py               # db_type -> adapter 映射
│   ├── drivers/
│   │   ├── catalog.py                # 25 类 DB_TYPE_CATALOG
│   │   ├── pip_manager.py            # pip 检测/安装/卸载
│   │   ├── jdbc_loader.py            # jpype 加载 jar (扩展位)
│   │   ├── ssh_tunnel.py            # sshtunnel 封装 (Linux)
│   │   └── winrm_tunnel.py          # pywinrm 封装 (Windows)
│   ├── rpc/
│   │   ├── server.py
│   │   └── servicers/
│   │       ├── driver_servicer.py
│   │       └── connection_servicer.py
│   ├── config/db_type_catalog.json
│   ├── app.py                        # FastAPI 入口
│   └── requirements.txt
├── proto/dbawork.proto               # gRPC 契约
├── frontend/
│   ├── src/
│   │   ├── views/datasource/         # 数据源配置页
│   │   │   ├── index.vue             # 列表 + 分组树
│   │   │   ├── Form.vue              # 新建/编辑表单
│   │   │   ├── ImportCSV.vue
│   │   │   └── ServerTunnel.vue       # 服务器隧道(SSH/WinRM)子表单
│   │   ├── views/driver/             # 驱动管理页
│   │   │   ├── index.vue
│   │   │   ├── DetectButton.vue
│   │   │   └── InstallDialog.vue
│   │   ├── api/                      # datasource.js/driver.js/group.js
│   │   ├── components/               # DbTypeTag/StatusBadge
│   │   ├── router/
│   │   └── App.vue
│   ├── package.json
│   └── vite.config.js
├── data/                             # SQLite + 驱动文件
│   ├── metadata.db
│   └── drivers/
└── deploy/docker-compose.yml
```

## SQLite 数据表设计

### groups (数据源分组)
```sql
CREATE TABLE groups (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        VARCHAR(128) NOT NULL,
    parent_id   INTEGER DEFAULT 0,    -- 0=顶级, 支持树形
    sort_order  INTEGER DEFAULT 0,
    description VARCHAR(256) DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### datasources (数据源主表)
```sql
CREATE TABLE datasources (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            VARCHAR(128) NOT NULL,
    group_id        INTEGER DEFAULT 0,
    db_type         VARCHAR(32)  NOT NULL,    -- mysql/oracle/postgresql/mssql/...
    host            VARCHAR(255) NOT NULL,
    port            INTEGER      NOT NULL,
    db_name         VARCHAR(128) DEFAULT '',  -- Oracle=SID/service_name
    username        VARCHAR(128) NOT NULL,
    password_enc    BLOB,                     -- AES-GCM 密文
    password_iv     BLOB,                     -- nonce
    extra_params    TEXT DEFAULT '',          -- JSON: charset/ssl/service_name
    -- 服务器隧道 (SSH for Linux / WinRM for Windows)
    tunnel_type     VARCHAR(8)  DEFAULT 'none', -- none/ssh/winrm
    tunnel_host     VARCHAR(255) DEFAULT '',
    tunnel_port     INTEGER DEFAULT 0,        -- ssh=22, winrm=5985(http)/5986(https)
    tunnel_user     VARCHAR(128) DEFAULT '',
    tunnel_password_enc BLOB,                -- 隧道密码密文
    tunnel_key_enc      BLOB,                 -- SSH 私钥密文
    tunnel_key_pass_enc BLOB,                -- SSH 私钥口令密文
    tunnel_auth_scheme VARCHAR(16) DEFAULT '', -- WinRM: basic/ntlm/kerberos
    tunnel_transport   VARCHAR(8)  DEFAULT '', -- WinRM: http/https
    tunnel_use_ssl     INTEGER DEFAULT 0,     -- WinRM: 是否启用 SSL
    status          TINYINT DEFAULT 0,        -- 0=未测 1=正常 2=异常
    last_test_at    DATETIME,
    last_error      TEXT DEFAULT '',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### drivers (驱动元数据, 对齐 DBCheck jdbc_driver_registry)
```sql
CREATE TABLE drivers (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    db_type         VARCHAR(32)  NOT NULL,
    driver_kind     VARCHAR(8)   NOT NULL,     -- 'python' | 'jdbc'
    package_name    VARCHAR(128) DEFAULT '',   -- pymysql/cx_Oracle/psycopg2/pymssql
    version         VARCHAR(32)  DEFAULT '',
    driver_class    VARCHAR(255) DEFAULT '',
    jar_filename    VARCHAR(255) DEFAULT '',
    jar_path        TEXT DEFAULT '',
    file_size       INTEGER DEFAULT 0,
    is_active       INTEGER DEFAULT 0,        -- 每 db_type 仅 1 条=1
    installed       INTEGER DEFAULT 0,        -- Python 检测后回写
    installed_version VARCHAR(64) DEFAULT '',
    note            TEXT DEFAULT '',
    uploaded_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(db_type, driver_kind, version, package_name)
);
```

### db_type_hidden / db_type_custom (软删 + 自定义类型)
```sql
CREATE TABLE db_type_hidden (
    db_type   VARCHAR(32) PRIMARY KEY NOT NULL,
    hidden_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE db_type_custom (
    db_type           VARCHAR(32) PRIMARY KEY NOT NULL,
    name_zh           VARCHAR(64)  NOT NULL,
    name_en           VARCHAR(64)  NOT NULL,
    driver_class_hint VARCHAR(255) DEFAULT '',
    is_jdbc           INTEGER DEFAULT 1,
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## gRPC proto 定义 (proto/dbawork.proto)

两个 service：ConnectionService（连接测试）、DriverService（驱动管理）。数据源/分组/驱动元数据 CRUD 全在 Go HTTP 层。

```proto
syntax = "proto3";
package dbawork.v1;
option go_package = "dbawork/proto/dbaworkv1";

message Empty {}
message Result { bool ok = 1; string message = 2; string detail = 3; }

// 服务器隧道 (SSH for Linux / WinRM for Windows)
message ServerTunnel {
  string tunnel_type = 1;             // none/ssh/winrm
  string host = 2;
  int32  port = 3;                    // ssh=22, winrm=5985/5986
  string user = 4;
  string password = 5;
  // SSH 专用
  string private_key = 6;
  string key_passphrase = 7;
  // WinRM 专用
  string auth_scheme = 8;            // basic/ntlm/kerberos
  string transport = 9;              // http/https
  bool   use_ssl = 10;
}
message DataSourceConn {
  int32 id = 1; string db_type = 2; string host = 3; int32 port = 4;
  string db_name = 5; string username = 6; string password = 7;
  map<string,string> extra = 8; ServerTunnel tunnel = 9;
}
message TestResult {
  bool ok = 1; string error = 2; string db_version = 3; int32 latency_ms = 4; string tested_at = 5;
}
message DsIdRequest { int32 id = 1; }

service ConnectionService {
  rpc TestConnection(DataSourceConn) returns (TestResult);
  rpc TestConnectionById(DsIdRequest) returns (TestResult);
}

// 驱动管理
message DbTypeInfo {
  string key = 1; string name_zh = 2; string name_en = 3;
  string driver_class_hint = 4; bool is_jdbc = 5; int32 order = 6; bool is_custom = 7;
}
message ListDbTypesRequest { bool include_hidden = 1; }
message ListDbTypesReply { repeated DbTypeInfo types = 1; }
message DriverStatus {
  string db_type = 1; string driver_kind = 2; string package_name = 3;
  bool installed = 4; string installed_version = 5; string note = 6;
}
message DetectAllReply { repeated DriverStatus statuses = 1; }
message DetectOneRequest { string db_type = 1; }
message InstallDriverRequest {
  string db_type = 1; string driver_kind = 2; string package_name = 3; string version = 4;
}
message UninstallDriverRequest { string db_type = 1; string driver_kind = 2; }
message UploadJarRequest {
  string db_type = 1; string version = 2; string driver_class = 3;
  string jar_filename = 4; bytes jar_bytes = 5;
}
message UploadJarReply { bool ok = 1; string jar_path = 2; int64 file_size = 3; string message = 4; }
message DeleteJarRequest { string jar_path = 1; }
message ActivateDriverRequest { string db_type = 1; int32 driver_id = 2; }

service DriverService {
  rpc ListDbTypes(ListDbTypesRequest) returns (ListDbTypesReply);
  rpc DetectAll(Empty) returns (DetectAllReply);
  rpc DetectOne(DetectOneRequest) returns (DriverStatus);
  rpc InstallDriver(InstallDriverRequest) returns (Result);
  rpc UninstallDriver(UninstallDriverRequest) returns (Result);
  rpc UploadJar(UploadJarRequest) returns (UploadJarReply);
  rpc DeleteJar(DeleteJarRequest) returns (Result);
  rpc ActivateDriver(ActivateDriverRequest) returns (Result);
}
```

## Go REST API 路由 (全部 /api/v1 前缀)

| 方法 | 路径 | 说明 | 跨语言 |
|---|---|---|---|
| GET | `/health` | 健康检查 | 否 |
| GET/POST/PUT/DELETE | `/groups[/:id]` | 分组 CRUD | 否 |
| GET | `/datasources` | 列表 `?group_id=&db_type=&status=&keyword=` | 否 |
| GET/POST/PUT/DELETE | `/datasources[/:id]` | 数据源 CRUD (凭据加密) | 否 |
| POST | `/datasources/:id/test` | 测试连接 | gRPC TestConnectionById |
| POST | `/datasources/test-adhoc` | 未保存前临时测试 | gRPC TestConnection |
| POST | `/datasources/import` | CSV 批量导入 | 否 |
| GET | `/datasources/export.csv` | 导出(不含密码) | 否 |
| GET/POST/DELETE | `/db-types[/:key]` | 类型目录 CRUD | 否 |
| POST | `/db-types/:key/restore` | 恢复隐藏 | 否 |
| GET | `/drivers` | 驱动列表 `?db_type=` | 否 |
| POST | `/drivers/jdbc` | 上传 jar | gRPC UploadJar |
| DELETE | `/drivers/:id` | 删驱动 | gRPC DeleteJar |
| POST | `/drivers/:id/activate` | 设默认 | gRPC ActivateDriver |
| POST | `/drivers/detect` | 全量检测 | gRPC DetectAll |
| POST | `/drivers/detect/:db_type` | 单类型检测 | gRPC DetectOne |
| POST | `/drivers/install` | 安装 python 驱动 | gRPC InstallDriver |
| POST | `/drivers/uninstall` | 卸载 | gRPC UninstallDriver |

## Python 适配器架构

base.py 定义统一接口 `connect()/test()/close()`，registry.py 维护 db_type→adapter 映射。

| db_type | 适配器文件 | pip 包 | test SQL |
|---|---|---|---|
| mysql | mysql_adapter.py | pymysql | `SELECT VERSION()` |
| oracle | oracle_adapter.py | cx_Oracle | `SELECT banner FROM v$version WHERE rownum=1` |
| postgresql | postgres_adapter.py | psycopg2 | `SELECT version()` |
| mssql | mssql_adapter.py | pymssql | `SELECT @@VERSION` |

drivers/catalog.py 镜像 DBCheck 25 类 catalog，pip_manager.py 用 importlib 检测、subprocess pip 安装/卸载。

**服务器隧道架构**：`ssh_tunnel.py` 用 sshtunnel 建立 SSH 端口转发（Linux），`winrm_tunnel.py` 用 pywinrm 建立 WinRM 会话端口转发（Windows）。连接前按 `tunnel_type` 实例化对应隧道，把远端数据库端口映射到本地端口，再把本地端口传给数据库驱动。

## 前端页面设计

**数据源配置页** `views/datasource/index.vue`：左侧 el-tree 分组 + 右侧 el-table 列表，工具栏(新建/批量导入/导出CSV/批量删除)，状态 el-tag(绿=正常/红=异常/灰=未测)

**服务器隧道子表单** `views/datasource/ServerTunnel.vue`：el-select 选隧道类型(none/SSH/WinRM)，选 SSH 展示(host/port/user/password/私钥/口令)，选 WinRM 展示(host/port/user/password/认证方式 basic/ntlm/kerberos/传输协议 http/https/SSL 开关)

**驱动管理页** `views/driver/index.vue`：表格列(db_type/logo/驱动种类/包名/版本/installed状态/is_active/操作)，操作按钮(检测/安装/卸载/上传JAR/设默认/删除)

## 实现阶段

### 阶段 0：骨架
1. 建目录结构，初始化 go.mod / requirements.txt / package.json
2. 写 proto/dbawork.proto，protoc 生成 Go + Python stub
3. docker-compose.yml 三服务

### 阶段 1：元数据层 (Go)
4. internal/db/sqlite.go 接 modernc.org/sqlite (纯 Go 免 CGO) + 迁移加载
5. internal/crypto/ AES-GCM 封装
6. repository/ 三类 repo (group/datasource/driver) + 单测
7. model/ 结构体与 DTO (详情 DTO 不含 password)

### 阶段 2：驱动管理 (Python + Go)
8. py-backend/drivers/catalog.py 镜像 25 类 catalog
9. pip_manager.py detect/install/uninstall + 单测
10. rpc/servicers/driver_servicer.py 实现 8 个 RPC
11. Go grpcclient/driver_client.go + service/driver_service.go 编排
12. Go handler/driver_handler.go + 路由
13. 前端 views/driver/* + api/driver.js 联调

### 阶段 3：数据源管理 (Go + Python)
14. py-backend/adapters/ 四个适配器 + registry.py + ssh_tunnel.py + winrm_tunnel.py + 单测
15. rpc/servicers/connection_servicer.py 实现 TestConnection
16. Go service/datasource_service.go (CRUD + 测试编排: 解密→gRPC→写status)
17. pkg/csvimport/ CSV 解析
18. Go handler/datasource_handler.go + group_handler.go + 路由
19. 前端 views/datasource/* + api 联调

### 阶段 4：收尾
20. JWT 中间件 + 登录占位
21. 全局错误处理 + 前端错误提示

## 关键设计决策

- **modernc.org/sqlite 而非 mattn/go-sqlite3**：纯 Go 免 CGO，简化 Windows 构建链
- **凭据 AES-GCM + gRPC 内网明文**：Go 加密落库，调 Python 时解密后明文走 gRPC
- **CSV 列对齐 DBCheck connectInfo**：`db_type,name,host,port,db_name,user,password,...`
- **db_types 软删 + 自定义分离两表**：复刻 DBCheck driver_type_hidden/driver_type_custom
- **Python pip 驱动优先、JAR 留扩展位**：drivers.driver_kind 字段保两种共存
- **WinRM 隧道**：Windows 服务器通过 pywinrm 建立 5985(HTTP)/5986(HTTPS) 端口转发，与 SSH 隧道共用 tunnel_type 字段切换，支持 basic/ntlm/kerberos 三种认证

## 验证方式

1. **Go 后端**：`go test ./internal/repository/... -v` 验证 CRUD 单测
2. **Python 后端**：`pytest py-backend/tests/` 验证适配器和 pip_manager 单测
3. **gRPC 联调**：Go handler 调 `/drivers/detect`，验证 Python 返回 installed 状态并回写 SQLite
4. **前端 E2E**：浏览器打开数据源配置页 → 新建 MySQL 数据源 → 测试连接 → 验证 status 刷新
5. **驱动管理**：打开驱动管理页 → 点"检测" → 验证 pymysql 已安装、cx_Oracle 未安装状态正确显示
6. **WinRM 隧道**：新建 SQL Server 数据源 → 选隧道类型 WinRM → 填 Windows 服务器信息 → 测试连接 → 验证通过 WinRM 端口转发连到远端 MSSQL

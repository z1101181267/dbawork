# DBAWORK 数据库综合管理平台

数据源配置 + 驱动管理（对标 [DBCheck](https://github.com/fiyo/DBCheck)），三端组合架构：

- **Go 网关**（Gin + SQLite）：数据源 / 分组 / 驱动元数据 CRUD、凭据 AES-GCM 加密、HTTP API、JWT 鉴权（登录占位）
- **Python 运行时**（FastAPI + gRPC）：数据库适配器（MySQL/Oracle/PostgreSQL/SQL Server/SQLite…）、pip 驱动检测/安装/卸载、JAR 管理、服务器隧道（SSH / WinRM）
- **Vue 3 前端**（Element Plus + Vite）：数据源配置页、驱动管理页、登录占位页

## 架构

```
Vue 3 + Element Plus (前端 :5173 dev / :80 容器)
    │ /api/v1  HTTPS/JSON
Go 后端 (Gin :8080) — API 网关 + SQLite 元数据 CRUD + 凭据 AES-GCM 加密
    │ gRPC :50051 (protobuf, 64MB 消息上限)
Python 后端 (FastAPI :8000 + gRPC server) — 数据库适配器 + 驱动运行时 + 服务器隧道(SSH/WinRM)
```

**职责切分**：Go 管元数据与凭据（datasources/drivers/groups 表、加密存储、路由鉴权）；Python 管实际连库、pip 包检测/安装、连接测试、SSH/WinRM 隧道；驱动"是否可用"由 Python 检测后回写 Go 侧 `installed` 字段。

## 目录结构

```
DBAWORK/
├── go-backend/            # Go 网关（module dbawork）
│   ├── cmd/dbawork/       # 入口
│   ├── internal/{config,db,model,repository,service,handler,middleware,router,crypto,grpcclient,catalog}
│   ├── pkg/csvimport/     # CSV 批量导入解析
│   └── proto/dbaworkv1/   # protoc 生成的 Go stub
├── py-backend/            # Python 运行时
│   ├── adapters/          # 数据库适配器 + registry
│   ├── drivers/           # catalog / pip_manager / ssh_tunnel / winrm_tunnel
│   ├── rpc/               # gRPC server + servicers（+ gen/ 生成的 stub）
│   ├── config/            # db_type_catalog.json（25 类收敛目录）
│   ├── tests/             # pytest
│   └── app.py             # FastAPI 入口
├── frontend/              # Vue 3 + Element Plus
├── proto/dbawork.proto    # gRPC 契约（单一事实源）
├── data/                   # 运行时数据：metadata.db / secret.key / drivers/（不入库）
├── deploy/docker-compose.yml
└── scripts/               # gen_proto 等工具脚本
```

## 快速开始（Windows 本地开发）

> 前置：Go ≥ 1.24；Python 3.13（**需完整安装版**，不要使用缺少 `venv` 模块的嵌入式分发包）；Node.js ≥ 20。

### 1. Python 运行时

```powershell
cd py-backend
# 用完整版 Python 创建虚拟环境（本机示例：D:\Python\Python313\python.exe）
& "D:\Python\Python313\python.exe" -m venv .venv
.venv\Scripts\python.exe -m pip install -r requirements.txt
# 可选驱动（装不上的可跳过，不影响主流程）
.venv\Scripts\python.exe -m pip install -r requirements-optional.txt
# 生成 gRPC stub
.venv\Scripts\python.exe tools\gen_proto.py
# 启动（两个终端）
.venv\Scripts\python.exe -m rpc.server      # gRPC :50051
.venv\Scripts\python.exe app.py             # FastAPI :8000
```

### 2. Go 网关

```powershell
cd go-backend
go build -o bin/dbawork.exe ./cmd/dbawork
.\bin\dbawork.exe                           # HTTP :8080（数据目录默认 ..\data）
```

### 3. 前端

```powershell
cd frontend
npm install
npm run dev                                 # http://localhost:5173（/api 代理到 :8080）
```

生产构建：`npm run build` 产出 `dist/`；容器部署见下一节。

## Docker 部署

```powershell
cd deploy
docker compose up -d --build
# 前端 http://localhost:8081 ｜ Go API http://localhost:8080 ｜ Python gRPC :50051 / FastAPI :8000
```

## 测试与验证

```powershell
# Go：crypto / repository / service / csvimport 单元测试
cd go-backend; go test ./...

# Python：catalog / pip_manager / adapters / registry / tunnels / stubs
cd py-backend; .venv\Scripts\python.exe -m pytest tests -q
```

### 本机验证记录（2026-09-14）

- `go test ./...`：crypto / repository / service / csvimport 全部通过
- `pytest tests -q`：**39 passed**
- 端到端：SQLite 数据源经 Go→gRPC→Python 测试连接返回 `ok:true / db_version=3.50.4`，状态回写「正常」
- 端到端：驱动全量检测 **21/22 已就绪**（仅 trino 未装，属可选；Oracle 驱动为官方 python-oracledb）
- 端到端：测试 JAR 经 gRPC 上传落盘 `data/drivers/db2/11.5/` 并自动设为默认
- 前端 `npm run build` 成功；页面经 headless 浏览器实测截图

端到端验收建议：
1. 启动三端后打开前端 → 数据源配置页新建数据源 → "测试连接" → 状态列变绿（正常）并显示版本与耗时；
2. 驱动管理页点"全量检测" → 查看 pymysql 等已安装状态是否正确回写；
3. 新建 MSSQL 数据源并选择 WinRM 隧道 → 填写 Windows 服务器信息 → 测试连接。

## 配置（环境变量）

| 变量 | 默认值 | 适用 | 说明 |
|---|---|---|---|
| `DBAWORK_HTTP_ADDR` | `:8080` | Go | HTTP 监听地址 |
| `DBAWORK_DATA_DIR` | `../data` | Go/Py | 数据目录（metadata.db / secret.key / drivers/） |
| `DBAWORK_DB_PATH` | `<data>/metadata.db` | Go/Py | SQLite 路径 |
| `DBAWORK_SECRET_KEY_PATH` | `<data>/secret.key` | Go/Py | AES-GCM 主密钥（首次启动自动生成 32 字节） |
| `DBAWORK_PY_GRPC` | `127.0.0.1:50051` | Go | Python gRPC 地址 |
| `DBAWORK_AUTH_ENABLED` | `false` | Go | 是否启用 JWT 鉴权 |
| `DBAWORK_AUTH_USER` / `DBAWORK_AUTH_PASS` | `admin` / `admin` | Go | 登录占位账号 |
| `DBAWORK_JWT_SECRET` | dev 默认值 | Go | JWT 签名密钥（生产必须修改） |
| `GRPC_PORT` | `50051` | Py | gRPC 端口 |
| `API_PORT` | `8000` | Py | FastAPI 端口 |

## REST API 一览（前缀 /api/v1）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/health` | 健康检查 |
| POST | `/auth/login` | 登录（占位，签发 JWT） |
| GET/POST | `/groups`；PUT/DELETE `/groups/:id` | 分组 CRUD（GET 返回嵌套树） |
| GET/POST | `/datasources` | 列表（`?group_id=&db_type=&status=&keyword=`）/ 新建 |
| GET/PUT/DELETE | `/datasources/:id` | 详情 / 更新 / 删除 |
| POST | `/datasources/:id/test` | 测试连接（解密→gRPC→回写状态） |
| POST | `/datasources/test-adhoc` | 未保存前临时测试 |
| POST | `/datasources/import` | CSV 批量导入（multipart） |
| GET | `/datasources/export.csv` | 导出（不含密码） |
| GET/POST | `/db-types`；DELETE `/db-types/:key`；POST `/db-types/:key/restore` | 类型目录（内置+自定义-隐藏） |
| GET | `/drivers` | 驱动列表（`?db_type=`） |
| POST | `/drivers/jdbc` | 上传 JAR（multipart） |
| DELETE | `/drivers/:id` / POST `/drivers/:id/activate` | 删除 / 设默认 |
| POST | `/drivers/detect` / `/drivers/detect/:db_type` | 全量 / 单类型检测 |
| POST | `/drivers/install` / `/drivers/uninstall` | pip 安装 / 卸载 |

统一响应：`{ "code": 0, "message": "ok", "data": … }`；错误时 `code=1` 且 `message` 为可读中文原因。

## 关键设计说明

- **SQLite 驱动选型**：`modernc.org/sqlite`（纯 Go 免 CGO，Windows 构建链简单）。
- **凭据加密**：AES-256-GCM，密文与 nonce 分列存储；主密钥 `data/secret.key`（裸 32 字节）。Go 调 Python 前解密为明文经内网 gRPC 传输。
- **时间戳**：统一 RFC3339 UTC 文本（`strftime('%Y-%m-%dT%H:%M:%fZ','now')`）。
- **WinRM 隧道**：Windows 服务器通过 `netsh interface portproxy` 建立端口转发（pywinrm 执行），客户端连接 `<隧道主机>:<转发端口>`；需远程管理员权限。SSH 隧道经 sshtunnel 本地端口转发。
- **驱动双轨**：python（pip 管理）与 jdbc（JAR 上传，jpype 扩展位）共存于 drivers 表；每 db_type 仅一条 `is_active=1`。
- **TestConnectionById**：Python 直读 `metadata.db`（只读）+ `secret.key` 完成独立测试通道；主链路（Go handler）仍走"Go 解密 → TestConnection"。
- **gRPC 消息上限**：两端均放宽至 64MB（JAR 上传需要）。

## 常见问题（FAQ）

- **`No module named venv`**：使用了嵌入式 Python。改用完整安装版（如 `D:\Python\Python313\python.exe`）创建虚拟环境。
- **PowerShell 禁止运行脚本**：用 `scripts\gen_proto.cmd`，或 `powershell -ExecutionPolicy Bypass -File scripts\gen_proto.ps1`。
- **Windows 下日志中文乱码**：日志文件为 UTF-8；控制台代码页导致显示异常，属正常现象。
- **Oracle 驱动**：使用官方 python-oracledb（thin 模式纯 Python，无需安装 Oracle 客户端；旧 cx_Oracle 已被其接替）。
- **个别可选驱动安装失败**（如 pymssql、trino）：属可选依赖，跳过即可；对应数据源在装好驱动前测试会提示。
- **pip 下载慢或超时**：可为虚拟环境配置国内镜像：`.venv\Scripts\python.exe -m pip config set global.index-url https://pypi.tuna.tsinghua.edu.cn/simple`

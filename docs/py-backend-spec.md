# py-backend 施工细则（补充 docs/plan.md，冲突以本文件为准）

> 目标：按 docs/plan.md 完成 Python 后端（数据库适配器 + 驱动运行时 + 服务器隧道 + gRPC 服务 + FastAPI 入口），并通过测试验收。

## 0. 环境约定

- 位置：`DBAWORK/py-backend`（Windows PowerShell 环境）
- 虚拟环境：`python -m venv .venv`，之后一律使用 `.venv\Scripts\python.exe`
- 依赖：
  - `requirements.txt`（核心，必须装好）：`grpcio grpcio-tools protobuf fastapi uvicorn[standard] sshtunnel pywinrm pymysql psycopg2-binary requests pytest`
  - `requirements-optional.txt`（尽力安装，失败不阻塞）：`oracledb pymssql clickhouse-driver pymongo redis dmPython ksycopg2`（Oracle 使用官方 oracledb）
- 若 pip 默认源慢或失败：追加 `-i https://pypi.tuna.tsinghua.edu.cn/simple`

## 1. proto 生成（第一步）

- 写 `tools/gen_proto.py`：用 `grpc_tools.protoc` 把 `../proto/dbawork.proto` 生成到 `rpc/gen/`
  - `--python_out` + `--grpc_python_out`，`-I ../proto`
  - 生成后：写 `rpc/gen/__init__.py`；把 `dbawork_pb2_grpc.py` 里的 `import dbawork_pb2 as ...` 修补为 `from . import dbawork_pb2 as ...`
- 在 py-backend 目录运行：`.venv\Scripts\python.exe tools\gen_proto.py`
- 验证：`.venv\Scripts\python.exe -c "from rpc.gen import dbawork_pb2, dbawork_pb2_grpc; print('ok')"`

## 2. 文件清单（见 docs/plan.md 目录树，最终以此为准）

```
py-backend/
├── adapters/{__init__.py, base.py, mysql_adapter.py, oracle_adapter.py, postgres_adapter.py, mssql_adapter.py, sqlite_adapter.py, registry.py}
├── drivers/{__init__.py, catalog.py, pip_manager.py, jdbc_loader.py, ssh_tunnel.py, winrm_tunnel.py}
├── rpc/{__init__.py, server.py, gen/…, servicers/{__init__.py, driver_servicer.py, connection_servicer.py}}
├── config/db_type_catalog.json   ← 已提供，不要改写
├── tools/gen_proto.py
├── tests/{__init__.py, test_catalog.py, test_pip_manager.py, test_registry.py, test_adapters.py, test_tunnels.py, test_stubs.py}
├── app.py
├── requirements.txt
└── requirements-optional.txt
```

## 3. drivers/catalog.py

- 加载 `config/db_type_catalog.json`（路径相对 py-backend 根目录解析，用 `pathlib`）
- 提供：`CATALOG`（list[dict]）、`by_key(key)`、`python_types()`、`jdbc_types()`、`IMPORT_NAME_MAP`（pip 包名→import 名：`{"clickhouse-driver": "clickhouse_driver"}`，其余默认 `replace("-", "_")`；`sqlite3` 为标准库白名单）
- 校验：25 条、key 唯一

## 4. drivers/pip_manager.py

- `detect_one(db_type) -> dict`、`detect_all() -> list[dict]`，返回结构对齐 proto `DriverStatus` 字段：`{db_type, driver_kind, package_name, installed, installed_version, note}`
  - python kind：`importlib.util.find_spec(import_name)` 判 installed；`importlib.metadata.version(dist_name等)` 取版本（异常返回空串）
  - 标准库白名单（sqlite3）：恒 `installed=True`，版本用 `sqlite3.sqlite_version`
  - jdbc kind：检查 `../data/drivers/<db_type>/` 下是否存在 .jar → installed；note 显示 jar 文件数
- `install(db_type, package_name, version="") -> (ok, message)`：仅 python kind；`subprocess [sys.executable, "-m", "pip", "install", spec, "--disable-pip-version-check", "-q]`，timeout=300s，message 带输出尾部；标准库 → `(True, "标准库模块，无需安装")`
- `uninstall(db_type) -> (ok, message)`：`pip uninstall -y`；标准库 → 拒绝
- 安装/卸载后返回最新 detect 结果

## 5. adapters/

- `base.py`：`BaseAdapter.__init__(conn: dict)`，`conn` 结构：
  `{db_type, host, port, db_name, username, password, extra: dict, connect_host, connect_port}`
  （`connect_host/connect_port` 为隧道建立后的实际连接端点；无隧道时等于原值）
  - 统一方法：`connect()`、`test() -> (ok: bool, error: str, db_version: str)`、`close()`
- `mysql_adapter.py`（pymysql）：charset 取 `extra.charset` 默认 utf8mb4；`db_name` 为空传 None；connect_timeout=10；test SQL：`SELECT VERSION()`
- `oracle_adapter.py`（python-oracledb，兼容旧驱动 cx_Oracle）：`extra.service_name` 优先 → `makedsn(service_name=…)`；否则用 db_name 作 service_name；test SQL：`SELECT banner FROM v$version WHERE rownum=1`
- `postgres_adapter.py`（psycopg2）：connect_timeout=10；sslmode 取 `extra.sslmode`（无则不传）；test SQL：`SELECT version()`
- `mssql_adapter.py`（pymssql）：login_timeout=10；test SQL：`SELECT @@VERSION`
- `sqlite_adapter.py`（标准库 sqlite3，用于本地演示与测试）：db_name 为文件路径或 `:memory:`；test SQL：`SELECT sqlite_version()`
- `registry.py`：
  - `REGISTRY = {mysql, sqlite, oracle, postgresql, mssql → 适配器类}`
  - `ALIASES = {mariadb/tidb/doris/starrocks/oceanbase/polardb/tdsql/gbase8a → mysql; gaussdb/opengauss/kingbase → postgresql}`
  - `get_adapter(db_type) -> class | None`；None 时上层错误信息：`"{db_type} 适配器尚未实现"`

## 6. drivers/ssh_tunnel.py 与 winrm_tunnel.py

统一接口：`start() -> (host, port)` 返回数据库驱动应连接的端点；`stop()`。

- `SSHTunnel`（sshtunnel 库）：
  - `sshtunnel.SSHTunnelForwarder((host, ssh_port), ssh_username=…, ssh_password=… 或 ssh_pkey=…, ssh_private_key_password=…, remote_bind_address=(db_host, db_port), allow_agent=False, host_pkey_direct_keys=False)`
  - 私钥支持两种输入：内容（`-----BEGIN` 开头）→ 写临时文件使用；或已是路径 → 直接用
  - `start()` 返回 `("127.0.0.1", local_bind_port)`
- `WinRMTunnel`（pywinrm，Windows 服务器端口转发）：
  - `pywinrm.Session(endpoint=f"{transport}://{host}:{port}/wsman", auth=(user, password), transport=auth_scheme, server_cert_validation="ignore")`
  - 选远程空闲端口：随机 31000-39999，通过 WinRM 执行 `netstat -ano` 检查占用，重试 ≤5 次
  - 建立转发：`netsh interface portproxy add v4tov4 listenaddress=0.0.0.0 listenport=<lp> connectaddress=<db_host> connectport=<db_port>`
  - 防火墙放行尽力而为（失败仅记录 warning）：`netsh advfirewall firewall add rule name="dbawork-<lp>" dir=in action=allow protocol=TCP localport=<lp>`
  - `start()` 返回 `(tunnel_host, listen_port)`（客户端直接连隧道主机上的转发端口）
  - 失败信息需提示：需远程管理员权限 / 检查 WinRM 是否开启
  - `stop()`：删除 portproxy 与防火墙规则（尽力）

## 7. rpc/

- `rpc/servicers/connection_servicer.py`：
  - `TestConnection(DataSourceConn) -> TestResult`：按 `tunnel` 字段建隧道（ssh/winrm）→ `registry.get_adapter(db_type)` → `test()` → close → 停隧道；计时 `latency_ms`；`tested_at` 用 UTC isoformat；异常 → `ok=False, error=…`
  - `TestConnectionById(DsIdRequest) -> TestResult`：只读打开 `../data/metadata.db`（env `DBAWORK_DB_PATH` 可覆盖）取该行；密码解密：key 文件 env `DBAWORK_SECRET_KEY_PATH`（默认 `../data/secret.key`，32 字节裸 key），AES-GCM（nonce = `password_iv`，无 AAD）→ 组装 DataSourceConn → 复用 TestConnection 逻辑；缺行/缺 key → 明确错误信息
- `rpc/servicers/driver_servicer.py`：实现 plan 中 8 个 RPC
  - `ListDbTypes`：返回 catalog（`is_custom=False`；hidden 逻辑由 Go 侧处理）
  - `UploadJar`：写 `../data/drivers/<db_type>/<version>/<filename>`（创建目录），返回绝对路径 + 文件大小
  - `DeleteJar`：删文件；不存在 → ok=False
  - `ActivateDriver`：记录进程内 active 映射，返回 ok（持久化由 Go 负责）
- `rpc/server.py`：`grpc.server(ThreadPoolExecutor(max_workers=8))`，`GRPC_PORT` env 默认 50051，监听 `0.0.0.0`；注册两个 servicer；提供 `serve()` / `stop()`；`python -m rpc.server` 可独立启动

## 8. app.py

- FastAPI：`GET /health` → `{"status": "ok", "service": "py-backend", "grpc_port": …}`
- `python app.py` 启动 uvicorn，端口 env `API_PORT` 默认 8000

## 9. tests/（pytest，全绿）

- `test_catalog.py`：25 条、key 唯一、必填字段齐全
- `test_pip_manager.py`：detect 假包（未安装）、sqlite3 白名单、pymysql 已安装（venv 内）
- `test_registry.py`：核心 5 类映射 + 别名映射 + 未实现报错
- `test_adapters.py`：sqlite 适配器真连 `:memory:` 通过；mysql 适配器用 `unittest.mock` patch `pymysql.connect` 验证 DSN 参数与 test SQL 调用
- `test_tunnels.py`：构造与参数校验（mock sshtunnel/pywinrm，不真连）
- `test_stubs.py`：`rpc.gen` 可导入、两个 servicer 类存在
- 运行：`.venv\Scripts\python.exe -m pytest tests -q`

## 10. 验收与纪律

- 验收：pytest 全绿；`tools/gen_proto.py` 可重复执行；`rpc.server`、`app` 模块可导入
- 只允许写 `DBAWORK/py-backend/**`，不得改动 `go-backend/`、`frontend/`、`proto/`、`config/db_type_catalog.json`、`docs/`
- 不做 git 初始化/提交；不启动长期驻留进程（测试用临时进程需自行清理）
- 网络：如遇下载缓慢可用国内镜像（pip 清华源；npm 不需要）

## 11. 变更记录

- 2026-09-14：Oracle 驱动由 cx_Oracle 切换为官方 python-oracledb（Python 3.13 无 cx_Oracle wheel，源码构建在 setuptools ≥ 81 环境因缺少 pkg_resources 而失败）。`oracle_adapter.py` 优先导入 oracledb、兼容旧 cx_Oracle；`config/db_type_catalog.json` 中 `oracle.package_name=oracledb`；Go 侧 `EnsurePythonDriver` 会随目录自动迁移历史包名行。

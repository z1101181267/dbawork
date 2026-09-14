# frontend 施工细则（补充 docs/plan.md，冲突以本文件为准）

> 目标：按 docs/plan.md 完成 Vue 3 + Element Plus 前端（数据源配置页 + 驱动管理页 + 登录占位），`npm run build` 成功。

## 0. 环境约定

- 位置：`DBAWORK/frontend`（Windows PowerShell 环境，Node 22 / npm 10 已装）
- 依赖安装：`npm install`；如慢：`npm config set registry https://registry.npmmirror.com`
- 技术栈：Vite + Vue 3（`<script setup>`）+ Vue Router 4 + Pinia（可选，不用也行）+ Element Plus + @element-plus/icons-vue + axios
- 验收命令：`npm run build` 必须成功；不要启动 dev server
- 所有界面文案用中文

## 1. 结构与路由

```
frontend/
├── index.html
├── package.json           # scripts: dev / build / preview
├── vite.config.js         # server.proxy: "/api" → http://127.0.0.1:8080
├── src/
│   ├── main.js            # 挂载 ElementPlus(zh-cn locale) + router
│   ├── App.vue            # 只放 <router-view/>
│   ├── layout/AppLayout.vue
│   ├── router/index.js    # / → redirect /datasources；/datasources；/drivers；/login
│   ├── api/{http.js, datasource.js, group.js, driver.js, auth.js}
│   ├── components/{DbTypeTag.vue, StatusBadge.vue}
│   └── views/
│       ├── datasource/{index.vue, Form.vue, ImportCSV.vue, ServerTunnel.vue}
│       ├── driver/{index.vue, DetectButton.vue, InstallDialog.vue}
│       └── login/index.vue
```

## 2. 布局 AppLayout.vue

- 左侧 el-menu（router 模式）：数据源配置（/datasources，图标 Coin）、驱动管理（/drivers，图标 Cpu）；顶部标题「DBAWORK 数据库管理平台」；右侧登录状态占位（读 localStorage token 显示"已登录/未登录"，提供"退出"清 token）
- el-main 内 `<router-view/>`

## 3. api 层

- `http.js`：axios 实例 `baseURL: "/api/v1"`、`timeout: 90000`
  - 请求拦截：附加 `Authorization: Bearer <token>`
  - 响应拦截：成功返回 `resp.data`；错误 → `ElMessage.error(resp.data?.message || resp.message || "请求失败")` 并 reject；401 → 清 token 跳 `/login`
  - 提供 `downloadFile(url, filename)` 辅助（blob 下载）
- `datasource.js`：`list(params)`、`get(id)`、`create(data)`、`update(id, data)`、`remove(id)`、`test(id)`、`testAdhoc(data)`、`importCsv(formData)`、`exportUrl`（`/api/v1/datasources/export.csv`）
- `group.js`：`list()`、`create(data)`、`update(id, data)`、`remove(id)`
- `driver.js`：`list(params)`、`detectAll()`、`detectOne(dbType)`、`install(data)`、`uninstall(data)`、`uploadJar(formData)`、`remove(id)`、`activate(id)`、`dbTypes()`（GET `/db-types`）
- `auth.js`：`login(data)`（POST `/auth/login`）

### 后端接口响应约定（联调基准）

- 统一响应：`{ code: 0, message: "ok", data: … }`，`code != 0` 表示业务错误（http 状态码 200 或 4xx/5xx，都以 message 提示为准）
- `/db-types` 返回：`[{ key, name_zh, name_en, driver_class_hint, is_jdbc, order, is_custom, default_port }]`
- `/datasources` 列表项字段：`id, name, group_id, db_type, host, port, db_name, username, extra_params, tunnel_type, tunnel_host, tunnel_port, tunnel_user, tunnel_auth_scheme, tunnel_transport, tunnel_use_ssl, status(0/1/2), last_test_at, last_error, created_at, updated_at`（**无密码字段**）
- `/datasources/:id/test` 返回 TestResult：`{ ok, error, db_version, latency_ms, tested_at }`
- `/groups` 返回**嵌套树**：`[{ id, name, parent_id, sort_order, description, children: [] }]`
- `/drivers` 列表项：`id, db_type, driver_kind(python|jdbc), package_name, version, driver_class, jar_filename, jar_path, file_size, is_active(0/1), installed(0/1), installed_version, note`
- `/drivers/detect` 返回统计 `{ total, installed }`；`/drivers/detect/:db_type` 返回单个驱动对象
- CSV 导入返回：`{ success_count, fail_count, errors: [{ line, message }] }`

## 4. 组件

- `DbTypeTag.vue`：props `value`(db_type)、`label`；按 key 映射颜色（mysql 蓝 / oracle 红 / postgresql 深蓝 / mssql 紫 / 其余灰蓝），显示 `label || value`
- `StatusBadge.vue`：props `status`、`lastError`；0=灰「未测」、1=绿「正常」、2=红「异常」（异常时 tooltip 显示 lastError）

## 5. 数据源配置页 views/datasource/index.vue

- 左栏（260px）分组树：
  - 顶部「新建分组」按钮；`el-tree` 显示"全部"虚拟根 + 分组树（数据来自 `/groups`）
  - 节点右键/悬浮下拉：新建子组、重命名、删除（confirm）
  - 点击节点 → 按 `group_id` 过滤右侧列表（"全部"不过滤）
- 右侧：
  - 筛选栏：db_type（取自 /db-types）、status（未测/正常/异常）、关键字（名称/主机）、「查询」「重置」
  - 工具栏：「新建数据源」(primary)、「批量导入」、「导出 CSV」、「批量删除」(勾选后可用, danger)
  - `el-table`：多选列 / ID / 名称 / 类型(DbTypeTag) / 主机 / 端口 / 库名 / 隧道（ssh、winrm、—）/ 状态(StatusBadge) / 最近测试 / 操作
  - 操作列：「测试」（loading，调 test(id)，成功 ElMessage 展示 db_version + latency_ms，然后刷新该行）、「编辑」、「删除」（confirm）
- 空状态 el-empty；加载 loading

## 6. Form.vue（新建/编辑，el-drawer 或 el-dialog，宽 720）

- 字段：名称*、分组（el-tree-select，数据同分组树）、类型*（/db-types，显示 name_zh；选择后自动带 `default_port`）、主机*、端口*、库名、用户名*、密码（编辑态留空=不修改，占位提示）
- 扩展参数（可折叠）：charset / service_name / sslmode 三个常用输入 + 「附加 JSON」textarea（如 `{"connect_timeout": 10}`），提交时合并进 `extra_params`（JSON 字符串）
- 嵌入 `ServerTunnel.vue`
- 底部：「测试连接」→ POST `/datasources/test-adhoc`（提交当前表单值）→ 展示结果；「取消」；「保存」→ create/update，成功后 emit 刷新

## 7. ServerTunnel.vue

- props：`modelValue`（`{tunnel_type, tunnel_host, tunnel_port, tunnel_user, tunnel_password, tunnel_private_key, tunnel_key_passphrase, tunnel_auth_scheme, tunnel_transport, tunnel_use_ssl}`）
- 隧道类型 el-radio-group：不使用 / SSH（Linux）/ WinRM（Windows）
- SSH 表单：主机、端口(默认 22)、用户名、认证方式（密码 / 私钥）→ 密码 或（私钥 textarea 粘贴 + 私钥口令）；私钥留空编辑态=不修改
- WinRM 表单：主机、端口（默认随传输：http=5985 / https=5986）、用户名、密码、认证方式（basic/ntlm/kerberos）、传输协议（http/https）、SSL 开关

## 8. ImportCSV.vue

- 说明区展示列头格式：`db_type,name,host,port,db_name,username,password,group_name`（后 4 列隧道字段可选：`tunnel_type,tunnel_host,tunnel_port,tunnel_user,tunnel_password`）
- 「下载模板」：前端生成 CSV blob 下载
- el-upload（accept=".csv"，auto-upload=false，限制 1 个文件）→「开始导入」→ FormData POST `/datasources/import` → 结果面板：成功 n / 失败 m + 失败明细表格（行号 + 原因）

## 9. 驱动管理页 views/driver/index.vue

- 顶部：db_type 筛选（可选）+「全量检测」按钮（loading，POST /drivers/detect，完成后刷新 + 提示 installed/total）
- 表格列：DB 类型（DbTypeTag）/ 驱动种类（python=绿 tag、jdbc=紫 tag）/ 包名或 JAR 文件 / 版本 / 可用状态（已安装=`installed_version` 绿 tag；未安装=灰 tag）/ 默认（is_active=1 显示红"默认"tag，否则「设为默认」文字按钮）/ 备注 / 操作
- 操作：
  - python 行：「检测」「安装」（打开 InstallDialog）「卸载」（confirm；sqlite3 标准库禁用）「设为默认」（未安装禁用）「删除」
  - jdbc 行：「上传 JAR」（打开上传 dialog）「设为默认」「删除」
- `DetectButton.vue`：props `dbType` → 点击 POST `/drivers/detect/:db_type`，emit 更新
- `InstallDialog.vue`：props `row`；字段：包名（默认 row.package_name）、版本（可选）→ POST `/drivers/install` {db_type, driver_kind:"python", package_name, version} → 成功刷新
- 「上传 JAR」dialog（可内联在 index.vue）：db_type（el-select 全类型，默认当前行）、版本、driver_class、.jar 文件 → FormData POST `/drivers/jdbc` → 刷新
- 「设为默认」→ POST `/drivers/:id/activate`；「删除」→ DELETE `/drivers/:id`（jdbc 后端会连带删 jar 文件）

## 10. 登录占位 views/login/index.vue

- 居中卡片：用户名 / 密码 / 登录按钮；提示默认 `admin / admin`
- 成功后存 `localStorage.token` 跳 `/datasources`；失败 ElMessage 错误
- 路由不做强制守卫（后端默认关闭鉴权）；仅 401 拦截跳转

## 11. 验收与纪律

- 验收：`npm run build` 成功、无编译错误；页面结构与本细则一致
- 只允许写 `DBAWORK/frontend/**`，不得改动 `go-backend/`、`py-backend/`、`proto/`、`docs/`
- 不做 git 初始化/提交；不启动开发服务器；不引入额外重型依赖（除 Element Plus 生态）

# PTC 交接文档（WebView + 独立服务端）

## 当前状态

- 项目已从 MyGo 原生 GPU UI 迁移到 **MyGo web 前端模式**：Go 后端 + React 19 +
  Vite 8 + TypeScript，页面运行在系统 WebKitGTK 视图里。原实现归档在
  `.legacy/native-ui/`，不参与编译。
- 业务已拆成独立 HTTP 服务端（`cmd/server` + `internal/host`）。桌面端默认在进程内
  内嵌同一套服务；在 `client.json` 填服务端地址后变成纯客户端，多终端共享一份数据。
- 云机 / 短信 / 登录已从 Mock 切换为可配置 **HTTP 网关**（`internal/driver/gateway.go`），
  内置演示网关（`internal/provider/demo`，也可 `cmd/provider` 单独运行）。设置里的
  `provider_base_url` / `provider_token` 为空时使用内置演示网关。
- 新增 **物理 Android 设备网关**（`cmd/device-gateway` + `internal/provider/adbgateway`）：
  通过 adb 发现设备、唤醒/熄屏/重启、真实截图，并支持 `GET /v1/phones` 同步到平台。
  登录返回 `manual:true`，操作员用外部 scrcpy/Escrcpy 完成 QQ 登录后回控制台确认；
  本阶段不读取短信、不做 QQ 自动化、不部署 Redroid。
- 控制台已显示绑定云机的**真实画面**（截图接口 + 3 秒轮询，页面隐藏时暂停）。
- 高级设置已持久化到服务端 SQLite，所有终端共享；桌面端的服务连接配置保存在本机
  `client.json`（0600）。
- 已修复的已知缺陷：确认失败后后台登录“复活”账号、失败后旧验证码仍显示、删除账号
  留下云机/卡位孤儿引用、两套绑定字段不一致、钱包读改写非原子、崩溃后账号永久卡在
  等待验证码。
- 后端：`go build ./...`、`go vet ./...`、`go test ./...`、`go test -race ./...` 全绿。
- 前端：`bun run typecheck`、`bun run build:web` 通过。
- 真实窗口验收：工作台、账号、短信、云机、卡位、钱包、用户、设置、控制台均正常；
  登扣 → 收码 → 确认成功/失败全链路通过；控制台画面随时间变化；远程模式双客户端
  SSE 自动刷新；存号（含绑定云机）、批量导入、删除确认（云机释放）、筛选、排序、
  多选、充值、用户开关均验证过。

源码没有 Git 仓库；改造前的完整备份在 `../cloud-phone.backup-20261112/`。

## 一分钟启动

```bash
export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH GOTOOLCHAIN=local
cd /path/to/cloud-phone
bun install
go tool mygo generate          # 只在 Go API 变化后需要
go tool mygo dev               # 内嵌模式开发窗口
```

生产构建：`go tool mygo build`（产物在 `build/`，前端已嵌入）。

独立服务端 + 远程客户端：

```bash
go build -o build/cloud-phone-server ./cmd/server
./build/cloud-phone-server --listen 127.0.0.1:8787 --token dev

# 另一终端：桌面端 client.json 指向服务端后启动
# ~/.config/存号网络/client.json
# {"server_url":"http://127.0.0.1:8787","token":"dev"}
go tool mygo dev
```

物理 Android 设备网关：

```bash
go build -o build/cloud-phone-device-gateway ./cmd/device-gateway
./build/cloud-phone-device-gateway --listen 127.0.0.1:8791 \
  --token dev-gateway --adb adb --config configs/device-gateway.example.json
# 平台设置填 http://127.0.0.1:8791 / dev-gateway，云机页「同步设备」
```

Windows on ARM：`go tool mygo build -platform windows/arm64`，产物在
`build/windows-arm64/`（便携 exe + NSIS 安装包，Linux 上需 `makensis`）。
设备网关可另用 `bun run build:gateway:windows-arm64` 交叉编译，不依赖 SQLite DLL。

## 架构与数据流

```
React 页面 ──call──> internal/api (mygo.Bind)
    ▲                    │
    │   stateChanged     │ Snapshot() 一次读全部
    └────────────────────┘
                           internal/backend.Backend
                             ├── local  → internal/service → internal/store (sqlite)
                             └── remote → HTTP + SSE → internal/server
                                                         ├── service / store
                                                         └── gateway driver → 演示/厂商网关
                                                                               └── device-gateway → adb → 物理设备
                                                                                                     ▲
                                                                             scrcpy/Escrcpy（人工）────┘
```

- **服务端**（`internal/server`）：Bearer 鉴权，`GET /api/events` SSE，`GET /api/snapshot`
  一次返回全部状态；错误统一为 `{"error":"中文","code":"..."}`。
- **组合根**（`internal/host`）：打开 SQLite → 按需 Seed（`--seed` / `CLOUDPHONE_SEED=1`）
  → 启动演示网关 → gateway → service（`RecoverStuckLogins` + `RepairBindings`）→
  HTTP server。`cmd/server` 与桌面内嵌模式共用。
- **桌面端**（`main.go`）：读 `client.json`（env 可覆盖）→ 空地址则内嵌启动
  （127.0.0.1 随机端口 + 随机令牌），否则 `backend.NewRemote`；失败时窗口照常打开，
  错误页可进入「连接设置」。
- **事件**：服务端变更 → SSE → 远程客户端 `api.EmitChange` → `stateChanged` →
  前端 120ms 防抖后重新拉快照；另有 15s 可见时兜底刷新。
- `internal/api/api.go` 是唯一前端入口；导出方法会被 `go tool mygo generate` 变成
  `src/mygo.ts` 的类型化函数（camelCase）。修改任何导出方法签名后必须重新 generate
  + typecheck。

### 登录与数据一致性

- `StartLogin` 有 attempt / generation 守卫：重复点击不会并发多个登录；`ready` 状态
  表示验证码已到、等待人工确认；确认失败会取消后台 attempt 并清空验证码，不会复活。
- 登录完成写库前比对 `attempts[id]` 与 generation，过期结果直接丢弃；关闭期间不写库，
  下次启动由 `RecoverStuckLogins` 把 `logging_in` / `waiting` 恢复为异常，但保留
  `ready` 供用户确认。
- 绑定唯一来源是 `phones.account_id`；`accounts.bound_phone_id` 是反查键，
  `accounts.bound_phone` 只是显示名。`SetAccountBinding` 事务维护三方一致，启动时
  `RepairBindings` 修复脏链；删除账号会释放云机并清空卡位占用。
- 钱包在 `walletMu` 下串行“查余额 → 校验 → 算新余额”，再由事务写流水与余额。
- 设置保存在单行 JSON 的 `settings` 表；`provider_base_url` 校验 http/https 并去掉
  尾部 `/`。

### HTTP 接口

`GET /api/health`、`GET /api/snapshot`、`GET /api/events`；
账号：`POST /api/accounts`、`PATCH/DELETE /api/accounts/{id}`、
`POST /api/accounts/batch-delete`、`POST /api/accounts/import`、
`POST /api/accounts/export.csv`、`POST /api/accounts/{id}/login`、
`POST /api/accounts/{id}/login/confirm`、`GET /api/accounts/{id}/screenshot`；
云机 / 用户 / 钱包：`POST /api/phones/sync`、`POST /api/phones/{id}/state`、
`POST /api/users/{id}/enabled`、`POST /api/wallet/topup|charge`；设置：
`GET/PUT /api/settings`。

### 驱动网关协议

| Method | Path | Body / response |
|---|---|---|
| GET | `/v1/phones` | `{"phones":[{external_id,name,region,image,state,ip}]}`，可选设备发现 |
| POST | `/v1/phones/{externalID}/power` | `{"action":"start\|stop\|restart"}` → `{"state":"running"}` |
| GET | `/v1/phones/{externalID}/screenshot` | `image/png` |
| GET | `/v1/sms/wait?phone=...` | `{"code":"..."}`（长轮询；物理网关 501） |
| POST | `/v1/login/execute` | `{rule, account_id, number, display, verify_phone, phone, options}` → `{"ok":bool,"code":"","manual":false,"message":""}` |

`externalID` 为空回退云机名称。配置了 `provider_base_url` 就用它 + token，否则用
内置演示网关；读取设置失败直接报错，不静默切换。换真实厂商只需让服务端指向实现该
协议的网关。`manual:true` 且 `code==""` 表示人工登录就绪，平台置 `ready` 且不写短信。

## 容易踩的坑

1. **不能有 `mygo.json`**：它与 `mygo.config.ts` 冲突。配置只在 `mygo.config.ts`。
2. **`src/mygo.ts` 是生成文件**，不要手改；改 `internal/api` 后跑
   `go tool mygo generate` + `bun run typecheck`。
3. **字体**：系统没有 CJK 字体，`src/assets/NotoSansSC-Regular.otf` 随前端打包。
   `--mono` 字体栈末尾必须有 `"Noto Sans SC"`，否则等宽区域里的中文会变成方框。
4. **数据库路径**：默认 `<用户配置目录>/存号网络/cloudphone.db`（Linux 通常
   `~/.config/存号网络/cloudphone.db`）。内嵌模式才打开本地库；远程模式必须只由服务端
   打开，**同一个 SQLite 文件只能有一个写者**。旧原生版 `./userData/cloudphone.db`
   会在内嵌模式首次启动时自动复制一次。
5. **数据默认是空的**：首次运行只建表，不插入演示数据。需要演示数据时，服务端加
   `--seed`，内嵌模式设置 `CLOUDPHONE_SEED=1`。想重新来一遍：停服删除
   `cloudphone.db` 后再启动。
6. **连接设置重启生效**：`client.json` 由 `SaveConnection` 写入；令牌留空表示保持
   已保存的令牌（页面不回显令牌），避免误清空锁死。环境变量
   `CLOUDPHONE_EMBEDDED=1`、`CLOUDPHONE_SERVER_URL`、`CLOUDPHONE_TOKEN` 优先级高于文件。
7. **SSE 不能设 `WriteTimeout`**：`internal/server` 的 `Serve` 故意不设；否则事件流会被
   定时掐断。关闭顺序是先关 SSE hub 再 `Shutdown`。
8. **WSL 渲染**：`main.go` 自动设置 `WEBKIT_DISABLE_DMABUF_RENDERER=1`；若仍异常，
   按 README 依次试 `WEBKIT_DISABLE_COMPOSITING_MODE=1`、`LIBGL_ALWAYS_SOFTWARE=1`、
   `GDK_BACKEND=x11`。
9. **E2E 构建**：`e2e.go` 带 `//go:build e2e`，需要先 `bun run build:web` 生成
   `dist/` 再 `go build -tags e2e`；发布构建不包含自动化插件。
10. **标题栏**：窗口是 `TitleBarHiddenInset`，没有系统标题栏。顶栏与侧栏品牌区就是
    拖动区域（`--app-region: drag`）；新增顶栏里的可交互元素时，若它不是
    button/input/a 等，需要给它 `--app-region: no-drag`，否则点击会变成拖窗。
    右侧控件要避开 `--mygo-titlebar-inset-right` 预留区（原生窗口按钮在那里）。
11. **SQLite 原生库**：运行 / 测试需要 `libmygo-sqlite3.so`；离线时用
    `MYGO_SQLITE_LIBRARY` 指向 `.natives/libmygo-sqlite3.so`。独立服务端同样需要。
12. **安全**：令牌与供应商 token 都是明文存储；非回环部署必须加 TLS 反向代理。
    `Snapshot` 会向已认证客户端返回 `provider_token`，当前没有操作员级鉴权。
13. **物理设备网关**：`device-gateway` 默认 `127.0.0.1:8791`，令牌与平台服务端令牌
    独立。它只执行固定的 adb 命令；设备访问通过配置或 `adb devices` 解析，不能把
    任意 HTTP 路径直接传给 adb。`serial` 是稳定 `external_id`，`address` 只是 TCP
    选择器。`stop` 是熄屏不是断电，断电设备无法远程开机。
14. **ADB / scrcpy 共享**：网关与 Escrcpy 必须使用同一个 adb server 和版本；不要在
    网关运行期间执行 `adb kill-server`。Windows ARM64 的 USB 驱动可能不可用，优先
    无线调试或 Linux 网关主机；USB 设备只能由连接它的主机上的 scrcpy 操作。
15. **设备同步不删除云机**：只新增/更新 `external_id`，不改 `account_id`；网关暂时
    离线不会清空库存。需要清理旧设备时目前只能删库或后续加删除接口。

## 关键文件

| 文件 | 说明 |
|---|---|
| `main.go` | 读配置、内嵌/远程后端、WSL 兜底、创建窗口、退出清理 |
| `cmd/server/main.go` | 独立服务端入口与参数 |
| `cmd/provider/main.go` | 独立演示网关入口 |
| `cmd/device-gateway/main.go` | 物理 Android 设备网关入口 |
| `internal/host/host.go` | 服务端组合根（DB、Seed、demo、gateway、service、server） |
| `internal/server/server.go` | HTTP 路由、Bearer 鉴权、错误映射、SSE |
| `internal/backend/remote.go` | HTTP 客户端 + SSE 重连（指数退避） |
| `internal/api/api.go` | 前端 API、`stateChanged`、连接设置、CSV 导出 |
| `internal/service/` | 登录编排、绑定、钱包、设置、快照、CSV、设备同步、错误定义 |
| `internal/store/` | 仓储接口 + SQLite、幂等迁移、种子数据 |
| `internal/driver/gateway.go` | 网关适配器（power / screenshot / SMS / login / devices） |
| `internal/provider/demo/demo.go` | 演示网关与 480×800 PNG 渲染 |
| `internal/provider/adbgateway/` | ADB 执行器、物理设备发现、power/screenshot/manual 协议 |
| `configs/device-gateway.example.json` | 设备配置样例 |
| `internal/clientconfig/` | `client.json` 读写（0600） |
| `src/App.tsx` | Shell、导航、搜索、全局状态、弹窗编排、兜底刷新 |
| `src/pages/Accounts.tsx` | 筛选/排序/多选/批量操作/行内动作 |
| `src/modals/ConsoleModal.tsx` | 控制台：登录全状态 + 实时画面 + 人工登录卡片 |
| `src/components/DevicePreview.tsx` | 云机画面（错误保留最后一帧） |
| `src/components/ConnectionPanel.tsx` | 服务连接设置（错误页也可用） |
| `src/style.css` | 设计 token 与全部样式 |

## 剩余工作

1. **三个占位模块**：账号库存管理、提号管理、商家管理（目前是统一的建设中页面）。
2. **操作员登录与权限**：当前只有服务端 Bearer 令牌，没有每用户鉴权；用户启用开关
   只存状态。若要多操作员，需要登录态与权限校验。
3. **SSE 断线重放**：目前断线重连后触发一次全量 `sync`，没有 `Last-Event-ID` 增量重放。
4. **支付渠道**：钱包充值是直接写余额，没有接真实支付。
5. **可选**：前端组件测试（vitest）；控制台「快速扫码 / 快速查询 / 提号」页签仍是
   占位。
6. **设备网关后续**：云机删除/下架接口（当前同步只增不删）；短信验证码读取；
   Redroid 容器电源适配；应用内嵌实时投屏/触控（当前明确使用外部 scrcpy）；
   Windows ARM64 USB 驱动与 Escrcpy 真机验证。

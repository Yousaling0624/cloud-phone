# cloud-phone · 存号网络 云手机运营平台

一个「存号网络」式**云手机运营平台**后台。由三部分组成：

- **桌面客户端**：MyGo web 前端模式，React 页面运行在系统 WebKitGTK 视图中；
- **HTTP 服务端**：Go 业务 + SQLite + SSE 事件推送，可独立运行，也可由桌面端内嵌；
- **驱动网关**：云机 / 短信 / 登录统一走可配置的 HTTP 协议，内置演示网关用于联调；
  `cmd/device-gateway` 可用 ADB 管理物理 Android 设备，交互式操作交给外部
  scrcpy / Escrcpy。

原来的原生 GPU UI 版本已完整归档在 `.legacy/native-ui/`，不参与编译。迁移原因见
下文「黑屏修复」。

## 运行模式

```
React 页面 (WebKitGTK)
  │ mygo.Bind（API 方法签名由 go tool mygo generate 生成）
  ▼
internal/api ──> internal/backend.Backend
                   ├── local   直接调 service（测试 / 回退）
                   └── remote  HTTP + SSE ──────────────┐
                                                        ▼
                                           internal/server（独立进程或 GUI 内嵌）
                                             ├── service（业务编排 / 登录 attempt）
                                             ├── store（SQLite）
                                             └── gateway driver ── HTTP ──> 演示网关 / 厂商网关
```

- **内嵌模式**（默认）：`client.json` 的 `server_url` 为空时，桌面端在进程内启动同一套
  HTTP 服务（`127.0.0.1` 随机端口 + 随机令牌），数据在
  `<用户配置目录>/存号网络/cloudphone.db`，开箱即用。
- **远程模式**：`client.json` 填服务端地址与令牌，桌面端只当客户端，不打开本地数据库；
  多个终端 / 操作员共享同一份数据。服务端地址也可用环境变量覆盖。

保存连接设置后需要**重启应用**生效。

## 功能

- **工作台**：账号 / 在线 / 云机 / 待用验证码 KPI、快捷入口、最近短信与最近账单。
- **账号管理**（核心页）：状态 / 云机 / 搜索筛选；存号（可同时绑定空闲云机）、批量导入、
  导出（原生保存对话框，UTF-8 BOM CSV）；备注与账号排序；多选与批量导出 / 删除；云机
  开机 / 重启；行内打开控制台、编辑、删除；底部「进行中任务」条。
- **账号控制台**：账号与安全信息、登扣 / 快速扫码 / 快速查询 / 提号页签、登录方式、
  可折叠高级设置、验证码等待 / 展示 / 复制、登录成功 / 失败确认；自动网关显示验证码，
  物理设备网关显示「人工登录已就绪」与可复制的 scrcpy 命令；右侧实时显示绑定云机
  画面（3 秒轮询，页面隐藏时暂停）。
- **短信管理**：全部 / 未使用 / 已使用筛选与计数、复制验证码；登录确认后验证码自动
  标记为已使用。
- **云手机管理**：云机列表与开机 / 关机 / 重启，操作经网关下发；可「同步设备」把
  ADB 设备导入为云机，并显示设备号 / ADB 地址与可复制的 scrcpy 命令。
- **卡位管理**：SIM 卡位、运营商、绑定账号与状态。
- **系统钱包**：余额卡片、快捷金额、充值 / 扣费、账单流水（服务端事务写入）。
- **用户信息**：操作员卡片与持久化的启用开关。
- **系统设置**：云机网关地址 / 令牌、登录默认设置、服务连接（模式、地址、配置文件路径、
  测试 / 保存）。设置保存在服务端数据库，所有终端共享。
- **账号库存 / 提号管理 / 商家管理**：统一的「建设中」占位页。

## 快速启动

### 1. 桌面端（内嵌模式）

```bash
export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH GOTOOLCHAIN=local
cd /path/to/cloud-phone
bun install
go tool mygo generate     # 只在 Go API 变化后需要
go tool mygo dev          # 开发窗口
```

生产构建：`go tool mygo build`（产物在 `build/`，前端已嵌入）。

### 2. 独立服务端 + 桌面客户端（多终端共享）

```bash
go build -o build/cloud-phone-server ./cmd/server
./build/cloud-phone-server --listen 127.0.0.1:8787 --token dev
```

另开终端启动桌面端，把 `client.json` 指向服务端。配置文件位于
`<用户配置目录>/存号网络/client.json`（Linux 通常为
`~/.config/存号网络/client.json`，权限 0600）：

```json
{
  "server_url": "http://127.0.0.1:8787",
  "token": "dev"
}
```

也可以先启动桌面端，在「系统设置 → 服务连接」里填写地址与令牌、测试连接并保存，
然后重启。令牌留空表示保持已保存的令牌；已保存的令牌不会回显到页面。

### 3. 单独运行演示网关

服务端默认 `--demo` 会在进程内启动演示网关。若要把网关拆成独立进程：

```bash
go build -o build/cloud-phone-provider ./cmd/provider
./build/cloud-phone-provider --listen 127.0.0.1:8790 --token demo
```

然后在「系统设置 → 云机网关」填写 `http://127.0.0.1:8790` 与 `demo`，保存后立即生效。
换真实厂商时，只需把网关地址指向实现同一协议的厂商服务，客户端与业务层无需改动。

### 4. 物理 Android 设备网关（ADB + 外部 scrcpy）

`device-gateway` 用 ADB 管理真实 Android 手机，提供与演示网关相同的云机协议；它不做
QQ 自动化，也不读取短信。操作员用外部 scrcpy / Escrcpy 完成登录，再回控制台确认。

前置条件：

1. 安装 Android platform-tools，确认 `adb version` 可运行；
2. 手机开启开发者选项与 USB 调试，连接后运行 `adb devices -l`，状态必须是 `device`
   （首次连接需要在手机上点「允许」）；
3. 网关和服务端能访问同一个 ADB 环境。WSL 下可通过 usbipd 或无线调试接入。

```bash
go build -o build/cloud-phone-device-gateway ./cmd/device-gateway
./build/cloud-phone-device-gateway \
  --listen 127.0.0.1:8791 \
  --token dev-gateway \
  --adb adb \
  --config configs/device-gateway.example.json
```

`--config` 可省略，此时自动发现 `adb devices -l` 里的设备。配置里 `serial` 是稳定的
设备号（USB 序列号）；`address` 是可选 WiFi ADB 地址，存在时网关优先用它连接，但
`external_id` 仍使用 `serial`，换传输方式不会丢失账号绑定。无线调试需要先在手机上
配对，并保证地址稳定。

启动服务端（建议关闭不用的演示网关）：

```bash
./build/cloud-phone-server --listen 0.0.0.0:8787 --token dev --demo=false
```

在「系统设置 → 云机网关」填写 `http://127.0.0.1:8791` 与 `dev-gateway`，保存后到
「云手机管理」点「同步设备」；在线的设备会以「运行中」云机出现，离线设备显示异常。
把账号绑定到云机后，控制台开始登扣会显示「人工登录已就绪」，复制命令到能访问设备
ADB 的机器执行：

```bash
scrcpy -s <设备号>
# 无线设备通常是：adb connect 192.168.1.50:5555 && scrcpy -s 192.168.1.50:5555
```

手动完成 QQ 登录后点「登录成功」；失败点「登录失败」。手机上的锁屏密码无法绕过，
需要人工在 scrcpy 中解锁。

电源语义（物理设备没有真正的远程开关机）：

| 平台按钮 | 实际动作 |
|---|---|
| 开机 | `KEYCODE_WAKEUP` 唤醒屏幕，并尽力执行 `wm dismiss-keyguard` |
| 关机 | `KEYCODE_SLEEP` 熄屏；ADB 仍在线，不是断电 |
| 重启 | `adb reboot`，立即返回，不等待重启完成 |

**已知边界**：断电设备无法远程开机；未授权的 ADB 设备显示异常；不提供短信验证码
读取；不在应用内做实时投屏/触控；不自动执行 QQ 登录。Redroid 暂未部署，后续可把
容器暴露成 ADB TCP 地址并复用同一网关。

## 技术栈

| 层 | 选择 |
|---|---|
| 桌面框架 | MyGo v0.8.3，`WindowOptions.URL: "/"` 加载前端 |
| 前端 | React 19.3、TypeScript 7、Vite 8 |
| 样式 | 手写 CSS 设计系统（深色侧栏 + 浅色内容，青绿主色） |
| 状态 | 单一 `Snapshot` + Go `stateChanged` 事件刷新（120ms 防抖，15s 兜底轮询） |
| 服务端 | Go 1.27 `net/http`，Bearer 鉴权，SSE 事件流 |
| 数据 | MyGo sqlite 插件（内嵌 Zig SQLite，无 cgo），`internal/store` 仓储接口 |
| 字体 | 随前端打包 Noto Sans SC（系统零 CJK 字体） |

窗口使用 `TitleBarStyle: mygo.TitleBarHiddenInset`：系统不再绘制原生标题栏，页面顶栏
（`.topbar`）与侧栏品牌区（`.brand`）即标题栏，标记为 `--app-region: drag` 可拖动、
双击最大化；右上角原生窗口按钮覆盖在顶栏上，`--mygo-titlebar-inset-*` 自动为它们
留出空间。

## 目录结构

```
cloud-phone/
├── main.go                    # 读 client.json → 内嵌/远程后端 → 创建 webview 窗口
├── mygo.config.ts             # MyGo 应用配置（不使用 mygo.json）
├── package.json / vite.config.ts / tsconfig.json / index.html
├── e2e.go                     # 仅 -tags e2e：embed dist + 自动化插件
├── cmd/
│   ├── server/main.go         # 独立 HTTP 服务端
│   ├── provider/main.go       # 独立演示网关
│   └── device-gateway/main.go # 物理 Android 设备网关（ADB）
├── configs/
│   └── device-gateway.example.json
├── src/
│   ├── mygo.ts                # `go tool mygo generate` 生成的类型化客户端
│   ├── main.tsx / App.tsx     # 入口、Shell、路由、全局状态与弹窗编排
│   ├── style.css              # 全部设计 token 与组件样式
│   ├── assets/NotoSansSC-Regular.otf
│   ├── lib/                   # 上下文、格式化、类型、截图轮询、登录方式、scrcpy 命令
│   ├── components/            # Icon、Modal、Toast、基础控件、设备预览、连接面板
│   ├── pages/                 # 工作台、账号、短信、云机、卡位、钱包、用户、设置
│   └── modals/                # 存号/编辑/导入/删除确认/控制台/连接设置
├── internal/
│   ├── api/                   # 唯一绑定给前端的 API + stateChanged 事件
│   ├── backend/               # Backend 接口；local / remote(HTTP+SSE) / health
│   ├── server/                # HTTP 路由、鉴权、SSE hub
│   ├── host/                  # 组合根：DB + 演示网关 + service + server
│   ├── service/               # 登录编排、绑定、钱包、设置、快照、CSV、设备同步
│   ├── store/                 # SQLite 仓储、幂等迁移、种子数据
│   ├── model/                 # 领域模型与 wire 类型
│   ├── driver/                # 驱动接口、Mock、HTTP 网关适配器、设备发现
│   ├── provider/demo/         # 演示网关（含 480×800 PNG 渲染）
│   ├── provider/adbgateway/   # 物理设备网关：ADB 执行器、设备发现、HTTP 协议
│   └── clientconfig/          # client.json 读写（0600）
├── resources/icon.png
└── .legacy/native-ui/         # 归档的原生 UI 实现
```

## 开发命令

每个 shell 先设置 Go 环境：

```bash
export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH GOTOOLCHAIN=local
cd /path/to/cloud-phone
```

| 命令 | 作用 |
|---|---|
| `bun install` | 安装前端依赖 |
| `go tool mygo generate` | 由 Go API 重新生成 `src/mygo.ts`（改 API 后必跑） |
| `bun run typecheck` | `tsc --noEmit` |
| `bun run build:web` | 生产构建前端到 `dist/` |
| `go tool mygo dev` | 启动 Vite + 应用窗口（需要图形环境） |
| `go tool mygo build` | 生产构建，前端嵌入二进制 |
| `bun run build:server` | 构建独立服务端到 `build/cloud-phone-server` |
| `bun run build:provider` | 构建演示网关到 `build/cloud-phone-provider` |
| `bun run build:gateway` | 构建物理设备网关到 `build/cloud-phone-device-gateway` |
| `bun run build:gateway:windows-arm64` | 交叉编译 Windows ARM64 设备网关 |
| `go build ./...` / `go test ./...` | 编译 / 测试 Go 后端 |

### 改 Go API 之后

`src/mygo.ts` 是生成文件。新增/修改 `internal/api` 的导出方法后：

```bash
go tool mygo generate   # 重新生成 src/mygo.ts
bun run typecheck       # 前端类型必须通过
```

事件（`mygo.NewEvent`）也会生成到 `events` 对象；当前只有
`events.stateChanged`，页面收到后重新拉取 `Snapshot`。服务端通过 SSE
（`GET /api/events`）把变更推给桌面客户端，再由客户端广播成该事件。

## 配置

### 桌面端

- `client.json`：`server_url`（空 = 内嵌服务）、`token`。
- 环境变量（优先级高于文件，便于测试）：
  - `CLOUDPHONE_EMBEDDED=1`：强制内嵌模式；
  - `CLOUDPHONE_SERVER_URL`：覆盖服务端地址；
  - `CLOUDPHONE_TOKEN`：覆盖访问令牌；
  - `CLOUDPHONE_SEED=1`：内嵌模式空库时插入演示数据（默认不插入）。

启动连接失败时窗口仍会打开，错误页提供「连接设置」入口，可修改并测试连接。

### 服务端

```bash
./build/cloud-phone-server \
  --listen 127.0.0.1:8787 \
  --token <访问令牌> \
  --db /path/to/cloudphone.db \
  --demo=true \
  --demo-fail-rate 0
```

| 参数 | 默认 | 说明 |
|---|---|---|
| `--listen` | `127.0.0.1:8787` | HTTP 监听地址；跨机访问用 `0.0.0.0:8787` |
| `--token` | `CLOUDPHONE_TOKEN` | 必填，客户端 Bearer 令牌；不会打印到日志 |
| `--db` | `<用户配置目录>/存号网络/cloudphone.db` | SQLite 文件 |
| `--seed` | `false` | 空库时插入演示数据（仅用于演示 / 测试） |
| `--demo` | `true` | 进程内运行演示网关 |
| `--demo-fail-rate` | `0` | 演示登录失败率（0–1），用于验证失败路径 |

需要演示数据时显式加 `--seed`：

```bash
./build/cloud-phone-server --listen 127.0.0.1:8787 --token dev --seed
```

演示网关也可单独运行：

```bash
./build/cloud-phone-provider --listen 127.0.0.1:8790 --token demo --fail-rate 0
```

### 数据与迁移

- 首次运行默认创建**空数据库**，不会自动插入演示账号。需要演示数据时，服务端加
  `--seed`，或内嵌模式设置 `CLOUDPHONE_SEED=1`；演示数据包含 5 个账号、5 台云机、
  短信、卡位、用户与钱包记录。
- 内嵌模式启动前，`adoptLegacyDatabase` 会把旧原生版留在 `./userData/cloudphone.db`
  的数据复制到新位置（仅当新库不存在时）。
- 想恢复演示数据：停掉服务端，删除对应 `cloudphone.db`，再用 `--seed` 启动。
- **同一个 SQLite 文件只能有一个写者**：独立服务端运行时，桌面客户端必须走远程模式，
  不要再让另一个进程直接打开同一个数据库。

## 驱动网关协议

`internal/driver/gateway.go` 实现 `PhoneDriver` / `SmsDriver` / `LoginDriver` /
`DeviceLister`，每次调用读取服务端设置：配置了 `provider_base_url` 就用它 +
`provider_token`，否则回退到内置演示网关；读取设置失败会直接报错，不会静默切换。
所有请求带 `Authorization: Bearer <token>`，非 2xx 响应会带上状态码与响应片段。

| Method | Path | Body / response |
|---|---|---|
| GET | `/v1/phones` | `{"phones":[{external_id,name,region,image,state,ip}]}`，用于设备同步 |
| POST | `/v1/phones/{externalID}/power` | `{"action":"start\|stop\|restart"}` → `{"state":"running"}` |
| GET | `/v1/phones/{externalID}/screenshot` | `image/png` |
| GET | `/v1/sms/wait?phone=...` | `{"code":"..."}`（长轮询，物理网关返回 501） |
| POST | `/v1/login/execute` | `{rule, account_id, number, display, verify_phone, phone, options}` → `{"ok":bool,"code":"","manual":false,"message":""}` |

`externalID` 为空时回退到云机名称。超时：设备发现 20s、开关机 30s、截图 20s、
登录由服务端 10 分钟登录上下文约束。`GET /v1/phones` 是可选扩展；不支持它的网关会让
「同步设备」返回 502，不影响原有开关机 / 截图 / 登录。

`manual:true` 表示网关没有自动执行登录，平台把账号置为 `ready`（验证码为空）；操作员
在外部 scrcpy/Escrcpy 中完成登录后，在控制台点「登录成功」或「登录失败」。
`manual` 为空码时不会写入短信收件箱。

演示网关的开机 / 关机 / 重启分别为 600 / 300 / 800ms，验证码 3–8s 到达，截图是
480×800、随时间变化的 PNG。它用于联调，不是真实云机；配置真实网关后自动切换。

## 安全说明

- 服务端使用 Bearer 令牌鉴权（常量时间比较）；令牌通过 `--token` 或环境变量传入，
  平台日志不打印令牌。
- 非回环地址部署时，令牌与业务数据都在明文 HTTP 上传输，**应在前面加 TLS 反向代理**。
- `client.json` 以 0600 权限保存；令牌不会回显到页面，留空保存表示保持原令牌。
- `Snapshot` 会向已认证客户端返回 `provider_token`（当前没有操作员级鉴权）。只把服务端
  开放给可信网络。
- `device-gateway` 默认只监听 `127.0.0.1:8791`，令牌与平台服务端令牌相互独立；它执行
  adb 命令，权限很高，只应在可信机器上运行。不要把 ADB 端口或网关直接暴露到公网；
  远程部署请用私有网络 + TLS 反向代理。ADB 无线调试也会让同网段的人拥有设备控制权。
- 独立服务端与内嵌模式共用 SQLite 单写者约束，见「数据与迁移」。

## SQLite 原生库

MyGo 的 sqlite 插件运行 / 测试时需要原生库 `libmygo-sqlite3.so`。未打包的程序会
自动下载到 `os.UserCacheDir()/mygo/natives/<sha>/`。离线时可用：

1. `MYGO_SQLITE_LIBRARY=/path/to/libmygo-sqlite3.so`；
2. 应用资源目录（`mygo.App.Path(PathResources)`）；
3. 可执行文件目录。

仓库内的 `.natives/libmygo-sqlite3.so` 可用于本地开发。独立服务端与桌面端一样需要该库。

## Windows（含 Windows on ARM）

MyGo 无 cgo，可从 Linux/WSL 交叉编译桌面端：

```bash
go tool mygo build -platform windows/arm64
go tool mygo build -platform windows/arm64,windows/amd64   # 同时出 x64
```

产物在 `build/windows-arm64/`：

- `存号网络.exe`：便携版，必须与同目录的 `mygo-sqlite3.dll` 一起分发；
- `cloud-phone-setup-0.1.0-arm64.exe`：NSIS 安装包（内含 DLL 与卸载器）。

在 Linux 上生成安装包需要 `makensis`（Fedora：`sudo dnf install -y mingw32-nsis`），
缺少时 MyGo 会跳过安装包、只保留便携 exe。应用使用系统 WebView2 运行时（Windows 11
自带 ARM64 版）；exe 未签名，首次运行会有 SmartScreen 提示。

需要独立服务端 / 演示网关时，可以一起交叉编译到同一目录（共用 `mygo-sqlite3.dll`）：

```bash
GOOS=windows GOARCH=arm64 go build -o build/windows-arm64/cloud-phone-server.exe ./cmd/server
GOOS=windows GOARCH=arm64 go build -o build/windows-arm64/cloud-phone-provider.exe ./cmd/provider
```

设备网关不依赖 SQLite，可单独交叉编译，不需要 `mygo-sqlite3.dll`：

```bash
bun run build:gateway:windows-arm64
# 或
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build \
  -o build/windows-arm64/cloud-phone-device-gateway.exe ./cmd/device-gateway
```

Windows on ARM 上运行 adb 才是主要风险：先确认 `adb devices -l` 能看到 `device`；
USB 驱动不兼容时优先使用无线调试，或把网关放到 Linux 主机。网关必须和 scrcpy/
Escrcpy 使用同一个 adb server 和版本，不要执行 `adb kill-server`，否则会中断正在
运行的 scrcpy 会话。桌面包默认使用内嵌服务，无需额外启动服务端；`build/windows-arm64/`
中的 `cloud-phone-setup-0.1.0-arm64.exe` 是安装包，`存号网络.exe` + `mygo-sqlite3.dll`
是可便携分发的组合。打包目录同时包含 `cloud-phone-device-gateway.exe`、
`device-gateway.example.json` 和 `设备网关使用说明.txt`；设备网关没有 SQLite 依赖。

## 黑屏修复（WSLg）

原原生 GPU UI 在 WSLg 下窗口全黑、只有鼠标划过或焦点位置局部重绘，是原生渲染路径
兼容性问题。现在页面由 WebKitGTK 绘制，不再受该渲染器影响；`main.go` 还会在检测到
WSL 时（且用户未显式设置时）设置：

```go
WEBKIT_DISABLE_DMABUF_RENDERER=1
```

若某台 WSLg / 驱动组合仍然黑屏，依次尝试：

```bash
WEBKIT_DISABLE_COMPOSITING_MODE=1 \
LIBGL_ALWAYS_SOFTWARE=1 \
GDK_BACKEND=x11 \
go tool mygo dev
```

窗口还设置了 `BackgroundColor: "#f4f6fb"`（与页面 `--canvas` 一致），首帧不会黑闪。

## 字体

系统没有任何中日韩字体，因此 Noto Sans SC 随前端打包（`src/assets/`），由
`style.css` 的 `@font-face` 加载。等宽字体栈末尾也加入了 Noto Sans SC，避免
登录规则、CSV 文本等 `monospace` 区域里的中文显示成方框。

## 测试与验收

```bash
go build ./... && go vet ./... && go test ./... && go test -race ./...
go tool mygo generate && bun run typecheck && bun run build:web
go build -o build/cloud-phone-server ./cmd/server
go build -o build/cloud-phone-provider ./cmd/provider
bun run build:gateway
go tool mygo doctor       # go/bun/gtk/webkit 环境
```

测试覆盖：

- `internal/service`：导入去重、CSV、钱包、云机状态、绑定生命周期、删除级联、登录确认
  失败、重启恢复、绑定修复、设备同步、人工登录空码确认；
- `internal/driver`：网关设备发现、鉴权、`manual` 解析、协议错误；
- `internal/provider/adbgateway`：fake ADB 下的设备发现、开机/熄屏/重启命令、截图、
  人工登录、短信 501、未知设备、配置身份（serial/address）；
- `internal/api`：快照、账号生命周期、CSV、用户开关、钱包、登录全流程、失败路径、
  截图、设置持久化与校验；
- `internal/host`：完整 HTTP + 远程客户端集成（鉴权、导入绑定、设置指向演示网关、
  480×800 截图、CSV BOM、SSE 跨客户端刷新、错误令牌拒绝、设备同步与人工登录）；
- `internal/clientconfig`：读写、缺省文件、0600 权限。

服务端手工验收：

```bash
./build/cloud-phone-server --listen 127.0.0.1:8787 --token dev --db /tmp/cp-test.db &
curl -s -H 'Authorization: Bearer dev' http://127.0.0.1:8787/api/health
curl -s -H 'Authorization: Bearer dev' http://127.0.0.1:8787/api/snapshot
curl -s -H 'Authorization: Bearer dev' http://127.0.0.1:8787/api/accounts/1/screenshot -o /tmp/shot.png
file /tmp/shot.png   # PNG 480x800
curl -N -H 'Authorization: Bearer dev' http://127.0.0.1:8787/api/events
```

设备网关手工验收：

```bash
./build/cloud-phone-device-gateway --listen 127.0.0.1:8791 --token dev-gateway --adb adb &
curl -s -H 'Authorization: Bearer dev-gateway' http://127.0.0.1:8791/v1/health
curl -s -H 'Authorization: Bearer dev-gateway' http://127.0.0.1:8791/v1/phones
curl -s -o /tmp/device.png -w '%{http_code}\n' \
  -H 'Authorization: Bearer dev-gateway' \
  http://127.0.0.1:8791/v1/phones/<设备号>/screenshot
curl -s -o /dev/null -w '%{http_code}\n' \
  -H 'Authorization: Bearer dev-gateway' \
  http://127.0.0.1:8791/v1/sms/wait?phone=x   # 501
```

然后到平台设置相同地址与令牌，云机页点「同步设备」，绑定账号并在控制台走人工登录。

### 自动化 E2E

`e2e.go`（`//go:build e2e`）内嵌 `dist/` 并链接 MyGo automation 插件：

```bash
go install github.com/egoist/mygo/plugins/automation/cmd/mygo-automation
bun run build:web
go build -tags e2e -o build/cloud-phone-e2e .
MYGO_AUTOMATION=/tmp/cloudphone.sock ./build/cloud-phone-e2e &

mygo-automation open --socket /tmp/cloudphone.sock
mygo-automation screenshot /tmp/dashboard.png
mygo-automation eval "document.body.innerText"
```

页面是 web 窗口，用 `eval` 读取 / 点击 DOM、`screenshot` 截图；发布构建
（`go tool mygo build`）不包含 automation 插件。

前端单元测试未引入（无 vitest）；关键交互通过 automation 在真实窗口里验证。

## 开源协议

本项目以 [The Unlicense](LICENSE) 发布，贡献至公共领域（public domain）。
可自由复制、修改、发布、商用与分发，无需保留版权声明。

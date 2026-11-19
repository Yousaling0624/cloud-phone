# 云手机平台（cloud-phone）— PTC 模式迁移交接提示词

> 把下面「提示词」整段复制到 PTC 模式的新会话即可无缝续做。当前工作目录：`/home/yousa/project`，项目在 `/home/yousa/project/cloud-phone`。

---

## 提示词（复制以下全部）

你在 `/home/yousa/project/cloud-phone` 继续开发一个「存号网络」式**云手机运营平台**。这是一个 **Go 项目**，前端用 **MyGo 原生 GPU UI**（[github.com/egoist/mygo](https://github.com/egoist/mygo) 的 `ui` 包，纯 Go 绘制，无 HTML/JS/webview），后端 Go。遵循「**尽量复用 MyGo 现成组件/插件，不重复造轮子**」的原则。

### 一、环境（已装好，直接用）
- Go：`/home/yousa/.local/go/bin/go`（go1.27.4 linux/arm64）。每个 shell 先执行：
  ```bash
  export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH GOTOOLCHAIN=local
  ```
- 系统：Fedora 44 on WSL2 aarch64；GTK3 dev 已装（`pkg-config --modversion gtk+-3.0` → 3.24.52）。
- MyGo 版本：`v0.8.3`（`go.mod` 已 pin）。运行 `mygo` CLI：`go tool mygo ...`（`go.mod` 里有 `tool`）。
- 构建/校验：
  ```bash
  cd /home/yousa/project/cloud-phone
  go build ./...          # 必须通过
  go test ./...           # 必须通过
  go tool mygo vet .      # 检查视图里误存的 build 值
  go tool mygo dev        # 开发启动（需要 WSLg/图形显示；无显示时仅能编译不能看窗口）
  ```
- ⚠️ 注意：`go tool mygo doctor` 会报 webkit 缺失，这是**网页前端模式**才需要的，本项目是原生 UI（只要 GTK3），可忽略。

### 二、目标与视觉
复刻一张截图里的云手机运营后台（深色左侧导航 + 顶部页签 + 账号列表大表 + 「云手机控制台」弹窗）。主要界面：
- 左侧深色侧栏：工作台 / 短信管理 / **账号管理**（含子项：账号管理、账号库存管理、提号管理）/ 卡位管理 / 云手机管理 / 商家管理 / 系统钱包 / 系统管理。顶部显示「存号网络 账号 运营」。
- 顶部页签：账号管理 / 用户信息 / 收信箱 / 云手机管理。
- 账号管理页：筛选条（状态下拉+账号搜索框+搜索按钮(青绿)+清空）、操作条（存号/批量导入账号/导出我的账号/更多操作）、统计条（异常预警/未绑云机/未绑定账号/在线账号）、账号 `ui.Table`（列：勾选、备注、账号、绑定云机、状态说明、云机操作、操作[打开/编辑/删除]）、底部「进行中任务」条。
- 云手机控制台弹窗（`ui.Modal`）：左栏「账号与安全」（头像、账号名、添加/注册账号、关联QQ/QID/手机号/账号绑定与授权管理、修改密码/登录设备管理/手势解锁）；右栏 `ui.Tabs`（登扣/快速扫码/快速查询/提号）+ 登录方式 + 「登录设置」`ui.Collapsible` + 「登录验证码」卡片（验证码占位、复制、等待短信提示、「执行规则失败: qq_account_management_login_rule」+`loading` 转圈）+「登录结果确认」（登录成功并关闭/登录失败并关闭）；底部工具条（放大操作/获取截图/切换IP/切真机网络/验证码/等待接收/详情/结束本次操作）。
- 主色青绿（accent 约 `#12a594` 系），深色侧栏。

### 三、复用清单（务必用内置组件，别自绘）
| 界面元素 | 复用 MyGo 组件/插件 |
|---|---|
| 左侧导航+分组 | `ui.Sidebar` + `ui.SidebarSection` + `ui.SidebarItem` |
| 顶部页签 | `ui.Tabs(c, &sel, "账号管理","用户信息","收信箱","云手机管理")` |
| 账号大表 | `ui.Table(c, &listState, []ui.TableColumn{...}, n, cell)` + `ui.ListState`(支持排序/多选/列宽持久化/右键菜单) |
| 状态徽章 | `ui.Badge` |
| 头像 | `ui.Avatar(c, name, *ui.Bitmap)`（bitmap 传 nil 显示首字） |
| 搜索框/下拉/金额 | `ui.SearchField(c, &q)` / `ui.Select(c, &sel, []string{...})` / `ui.DecimalInput(c, &v, lo, hi, step)` |
| 按钮 | `ui.PrimaryButton`(主) / `ui.Button`(次) |
| 可折叠 | `ui.Collapsible(c, label, &openBool, fn)` |
| 弹窗 | `ui.Modal(c, &openBool, fn)`；左右分栏 `ui.Split(c, &size, first, second)` |
| 加载转圈 | **`plugins/loading`**：`loading.Arc(c)` / `loading.Ring(c)` 等 |
| 提示条 | `c.Toast("...")`（Context 方法，自动 toast viewport） |
| 图标 | `ui.Icon(c, *ui.SVG)`（`ui.MustParseSVG([]byte(...))` 内嵌 SVG；不要用 sfsymbols，Linux 无） |

Element 链式方法（都在 `Element` 上）：`Fill/Grow/Width/Height/Size/Padding/PaddingY/PaddingX/Gap/Border/Background/Radius/FontSize/Bold/TextColor/AlignItems/Justify/Children/Label/MinWidth/Shrink/Cache` 等。
主题：`c.SetTheme(*ui.Theme)`，`ui.Theme` 字段见下（`Background/Surface/SurfaceHover/SurfacePressed/Border/Text/TextMuted/Accent/AccentHover/AccentPressed/AccentText/Danger/Warning/Success/Selection/Focus/Radius/Spacing/FontSize`）。用 `ui.DarkTheme()` 或 `ui.LightTheme()` 复制后改字段；`WithAccent(ui.Hex("#12a594"))` 快速设主色。
配色工具：`ui.RGB/RGBA/Hex/Oklch/Mix/Alpha`。

**官方插件决定**：sqlite ✅（持久化，`sqlite.Open` 直接 Go 用、无需注册）;loading ✅（纯 Go，无需注册）;glass 可选（质感）;**不要引入 fetch/websocket**（那是网页前端模式用的，原生 UI 用 Go `net/http`）;**不要引入 sfsymbols**（仅 macOS）。

### 四、已完成（可直接用，勿重写）
目录结构：
```
cloud-phone/
├── main.go                    # 仍是脚手架 counter 示例，需替换为真实 App 启动（见下）
├── main_test.go               # 仍是脚手架测试，需替换
├── go.mod / go.sum / mygo.json / resources/icon.png
└── internal/
    ├── model/model.go         # ✅ 全部领域模型：Account, CloudPhone, Sms, SimSlot, User, Wallet, LedgerEntry, Task + 各状态常量
    ├── store/store.go         # ✅ Repository 接口 + SQLite 实现（基于 plugins/sqlite），含全部 CRUD、钱包、SetBalance、DefaultPath()
    ├── store/seed.go          # ✅ Seed()：首次运行灌入演示数据（账号 2441864785 祈 / 云机 24418647 / 短信 / 卡位 / 用户 / 钱包余额 56.40）
    ├── service/service.go     # ✅ Service：Accounts/ImportAccount/ImportCSV/DeleteAccount/SaveAccount/StartLogin/ConfirmLogin/Phones/SetPhoneState/Wallet/TopUp/Charge/Tasks 全套业务
    └── driver/driver.go       # ✅ 接口 PhoneDriver/SmsDriver/LoginDriver + Mock 实现（MockPhone/MockSms/MockLogin，3-8s 出验证码、可配置失败率）+ placeholderPNG
```
**校验状态**：`go build ./internal/...` 已通过 ✅。

关键 API 签名（照抄即可）：
- `service.New(repo store.Repository, phone driver.PhoneDriver, sms driver.SmsDriver, login driver.LoginDriver) *Service`
- `store.Open(path string) (*store.SQLite, error)`；`store.OpenMemory()` 用于测试；`store.DefaultPath() string`
- `svc.StartLogin(ctx, accountID int64, onUpdate func(model.Account, error))` —— 内部起 goroutine，回调在**工作线程**，UI 里要用 `win.Update(func(){...})` 包住再改状态。
- `svc.ConfirmLogin(ctx, id, ok bool) (model.Account, error)`
- `svc.SetPhoneState(ctx, id, action string)`，action ∈ start/stop/restart。
- `driver.MockLogin{Sms: driver.MockSms{}, FailRate: 0.2}`

### 五、待完成（按顺序）
1. **theme.go**：定制主题（深色侧栏区用 `t.Surface` 或更深色、accent 青绿）。放在 `internal/app/theme.go` 或 main。
2. **App 状态 + shell**：新建 `internal/app/` 包，`type App struct` 持有 `svc *service.Service`、`win *mygo.Window`、当前侧栏页 `page string`、当前页签 `tab int`、各页的 `ui.ListState`、表格数据切片、控制台弹窗状态 `consoleOpen bool` + 当前账号 `consoleAccount model.Account` 等。用 `ui.Row` → 左 `ui.Sidebar` + 右 `ui.Column`(`ui.Tabs` + 内容 switch)。
3. **账号管理页**：筛选/操作/统计/`ui.Table`/任务条；行「打开」→ 打开控制台弹窗（`consoleOpen=true`，载入该账号）。
4. **控制台弹窗**：`ui.Modal` + `ui.Split` 左右栏 + 右栏 `ui.Tabs` + `ui.Collapsible` + 验证码卡（等待时 `loading.Arc`）+ 结果按钮。点「登扣」→ `svc.StartLogin`（回调里 `win.Update` 刷新账号并触发重绘）；验证码到达显示；点「登录成功/失败」→ `svc.ConfirmLogin`。
5. **云手机/短信/卡位/用户钱包页**：同构，用 `ui.Table`/`ui.List`/`ui.Form` 拼。钱包页用 `ui.DecimalInput` + `TopUp/Charge`。
6. **main.go 接线**：打开 sqlite（`store.Open(store.DefaultPath())`，WSL 下 `mygo.PathUserData` 目录需存在，必要时 `os.MkdirAll`）→ `Seed` → 组 `service.New(...)` → `mygo.App.WhenReady` 里 `NewWindow(mygo.WindowOptions{Title:"存号网络", Width:1280, Height:800, MinWidth:1024, MinHeight:640, StateKey:"main", Content: ui.View(app.view)})` → `mygo.App.Run()`。维护 `app.win` 供异步 `Update`。
7. **测试**：`main_test.go` 用 `ui.NewTester(app.view, w, h)` 无窗口测试：断言侧栏/表格存在；`tt.Click("打开")` 后弹窗出现；点「登录成功并关闭」后账号状态变更（可注入 `driver.MockLogin{FailRate:0}` 保证成功）。另加 service/store 单元测试（内存库）验证 CRUD、ImportCSV 去重、钱包扣费。
8. **README**：写清如何 `go tool mygo dev` / `build`、driver 如何替换真实厂商。

### 六、约束与坑（重要）
- **视图函数每帧重建**：不要把 `ui.Element` 存进 App 状态；只存数据、`ui.ListState`、`ui.Handle`。用 `c.Key(...)`/`.ItemKey` 给控件稳定身份。
- **异步改 UI 必须 `win.Update(func(){...})`**（异步，回主线程重建视图）。不要在 goroutine 里直接改 UI 状态。
- **长任务勿阻塞 UI 线程**：driver 调用放 goroutine（service 已封装）。
- 该 skill 在 `.agents/skills/mygo-maintenance/SKILL.md`，遇到 API 拿不准时读 **实际源码**：`go list -m -f '{{.Dir}}' github.com/egoist/mygo`，再看 `ui/*.go` 和 `ui/api_gen.go` 里的函数签名（别只信在线文档，版本要对应 v0.8.3）。
- `ui.Table` 的 cell 回调里用 `ui.Text(c, ...)`；选中/多选/排序都走 `ui.ListState`（`.Selected`/`.Selection`/`.Sort`）；列结构体是 `ui.TableColumn{Title,ID,Width,MinWidth,MaxWidth,Align,Sortable,Fixed,RowHeader}`。
- `ui.Split(c, &size *float32, first, second func())`；`ui.Avatar(c, name string, image *ui.Bitmap)`；`ui.Select(c, &sel string, options []string)`。
- 每个新 shell 开头先 `export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH GOTOOLCHAIN=local`，否则找不到 go。
- 完成后必须 `go build ./... && go test ./... && go tool mygo vet .` 全绿；有图形显示时用 `go tool mygo dev` 目视核对截图。

---

## 附：当前文件校验快照（交接时）
- `go build ./internal/...` ✅ 通过
- go1.27.4，GTK 3.24.52，MyGo v0.8.3
- `main.go` / `main_test.go` 仍是 `mygo init` 自带的 counter 脚手架，**尚未替换**。

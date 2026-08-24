# SailBoard 开发进度

最后更新：2026-08-24

SailBoard 是 Windows + macOS 双平台项目，不考虑 Linux。

## 项目状态：功能已冻结，v1.0 已发布

所有计划内功能均已实现，Windows/macOS 两端功能对等，**不再新增功能**——尤其是不会做联网同步/云端/账号体系类功能，这是本项目从设计之初就有的边界（design doc §45 已列为范围之外），现在功能收尾阶段再次明确一遍：SailBoard 是纯本地单机工具，往后的工作只是 bug 修复、细节打磨、以及尚未做的平台补完（见"已知限制"），不会往"云同步剪贴板"之类的方向发展。

v1.0 已在 GitHub Releases 发布两个预编译产物：

| 平台 | 文件 |
| --- | --- |
| Windows (x86-64) | `SailBoard_v1.0_Windows_x86.exe` |
| macOS (Apple Silicon) | `SailBoard_v1.0_Mac_arm64.dmg` |

发布页：https://github.com/SciSail/sailboard/releases/tag/v1.0 。两个产物都是 ad-hoc/未做正式代码签名的构建（"已知限制"里有说明），下载后需要各自平台的一次性放行操作，`README.md` 的"下载使用"一节有写明。

## 当前状态

Windows 版本功能闭环完成：全局快捷键唤出、窗口定位、选择粘贴（含自动按键注入）、图片剪贴板（含 alpha 通道）、文件/文件夹剪贴板、颜色值识别、富文本（Office 系列带格式复制）、URL 预览（标题/favicon/描述/预览图）、来源应用识别与合并、系统托盘、暂停记录、开机启动均已实现并通过 `wails build` 产物的多次真实启动验证。UI 为 iOS 风格亮色毛玻璃、全屏宽度底部弹出面板，唤出/关闭动效已从纯 CSS 方案改为原生窗口物理滑动。

macOS 版本功能闭环完成，与 Windows 功能对等：`internal/platform.Controller` 接口的全部 27 个方法都已是真实原生实现（Cocoa/Carbon via cgo），没有一个还停留在 stub。窗口定位/滑动动效/失焦自动隐藏、全局热键（默认 `Cmd+Shift+V`）、菜单栏图标（应用不出现在程序坞）、剪贴板图片/富文本/文件读写、来源应用图标、文件缩略图、自动粘贴注入（需辅助功能权限，权限提示与其"授权后不用重启"均已挪到设置窗口，见下方"本轮完成"）、开机启动、单实例互斥、设置窗口跨进程通知（含"捕获快捷键时暂停旧热键"）、空格键 Quick Look 预览均已实现并经真机验证——本机开发环境（无真实屏幕/TCC 沙箱化）做不到的那部分（截图观感、辅助功能未授权分支、鼠标+快捷键交互路径）均已由用户在自己机器上确认通过。打包出的 DMG 带拖拽安装背景图提示（`build/darwin/package-dmg.sh`）。

## 本轮完成（2026-08-24 深夜：去掉数字键 1-9 快速选择/粘贴）

用户要求去掉主面板"按数字键 1-9 直接选中/粘贴第 N 张卡片"这个快捷键，数字键改成和其他普通字符一样，被当作搜索输入捕获。

- [x] `frontend/src/App.tsx`：删掉 keydown 处理里单独判断 `event.key >= "1" && event.key <= "9"` 直接 `paste(items[index].id)` 的分支——删除后，数字键自然落入紧接着的"任意可打印字符转发到搜索框"兜底分支（这个分支本来就是按 `event.key.length === 1` 判断的，数字键天然满足，不需要额外改动），行为上和输入字母完全一致：聚焦搜索框、字符正常输入。同步更新了上面一处提到"1-9 快捷键"的注释。
- [x] `README.md`"快速操作"一条里去掉"或数字键 1-9 选择/粘贴"的描述；`CLAUDE.md` 里 `App.tsx` 那条也去掉了"1-9"并补了一句说明这个快捷键已经在这轮按用户要求去掉。屏幕上的提示条（`.hints`，"← → 选择 Enter 粘贴..."）本来就没提过数字键，不用改。
- [x] `cd frontend && npm run build`（`tsc` + `vite build`）、`wails build` 全部通过，产物 `build/bin/SailBoard.exe` 编译成功。

## 本轮完成（2026-08-24 深夜：设置窗口提示横幅留白 + 关闭按钮）

上一轮加的"辅助功能"权限提示文案较长，容易在小窗口里跟第一行控件（保留历史/空间限制）重叠；用户要求把 `.settings-page` 顶部留白加大一点给提示横幅腾地方，并给提示横幅右侧加一个关闭 × 按钮。

- [x] `frontend/src/Settings.css`：`.settings-page` 的 `padding` 从 `22px 24px`（四边相等）改成 `46px 24px 22px`（只加顶部，其它三边不变）——`.notice` 本来就是 `position: absolute` 覆盖在页面内容之上而不占据文档流（这个设计是刻意的，见 `.settings-page` 注释：避免提示出现/消失时页面高度跟着变、进而在原生 WebView 里冒出滚动条），所以这个留白是无条件加的，不是"有提示时才加"，避免内容随提示的显示/隐藏跳动。`.notice` 本身从纯文字块改成 `display: flex`（`.notice-text` 占余下空间 + `.notice-close` 固定 20×20px 在右侧），新增 `.notice-close` 的 hover/active 态。
- [x] `frontend/src/Settings.tsx`：`{notice && <p className="notice">{notice}</p>}` 改成 `<div className="notice">` 包一个 `.notice-text` span 和一个 `.notice-close` 按钮，点击就是 `setNotice("")`，跟其它清空 notice 的地方（进入捕获态、Esc、有效按键）用的是同一个状态。
- [x] `cd frontend && npm run build`、`go build ./...`、`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` 全部通过；重新 `wails build` 出新的 `build/bin/SailBoard.app`。
- [x] **真机验证完成（用户确认）**：本机没有真实屏幕/屏幕录制权限，没法截图自行确认；用户在自己机器上打开设置窗口实测，顶部留白和关闭按钮观感确认没问题，`46px` 不用再调。

## 本轮完成（2026-08-24 深夜：macOS "辅助功能"权限提示挪到设置窗口）

用户反馈：macOS 上"已复制到剪贴板：accessibility permission not granted — enable SailBoard under System Settings > Privacy & Security > Accessibility, then try pasting again"这条提示只在粘贴失败后、在主面板里冒出来，体验上比较被动；还追加了一个相关联的问题：授权之后这条提示在应用里不会消失，只有重启 app 才会消失。

- [x] **需求**：把这条提示挪到设置窗口里，复用 Settings.tsx 已有的下滑式 toast（`.notice`/`noticeIn` 动画，之前给 Alt+Space/保留组合键用的那一套），不新增面板；打开设置窗口时主动检查一次，而不是等粘贴失败才被动提示。
- [x] **顺带的根因分析（权限已授予但提示不消失）**：`AXIsProcessTrustedWithOptions` 在同一个长期运行的进程里似乎会缓存查询结果——这是很多依赖辅助功能权限的 Mac 应用（不只是本项目）都记录过的已知怪癖：在系统设置里勾选权限后，已经在跑的进程不一定能实时感知到，通常需要重启该进程。之前这条检查是在**主进程**里做的（`SendPaste` 每次粘贴时查一次），主进程从启动到用户手动重启一直是同一个进程，天然会撞上这个缓存问题。设置窗口不一样：它是每次打开都全新拉起的独立进程（`main.go` 的 `runSettingsWindow`），所以只要把检查挪到设置窗口里做，"进程刚起来、还没查过一次"这件事本身就让它不会被这个缓存问题绊住——不需要额外的强制刷新逻辑，架构上顺带解决了。
- [x] **实现**：
  - `internal/platform/foreground_darwin.go` 新增 `AccessibilityTrustedDirect() bool`（`sb_accessibility_trusted(0)`，不弹系统对话框，纯查询），`SendPaste` 里原来那条又长又重复的英文提示精简成 `"accessibility permission not granted"`（详细的操作指引现在只在设置窗口里出现一次，不用在主面板和设置窗口各写一遍）。`controller_stub.go`/`settingswindow_windows.go` 加对应的 `return true`（Windows/其它平台没有这个权限概念，检查永远"已授权"，提示永远不出现）。
  - `settings_app.go` 新增 `SettingsApp.IsAccessibilityTrusted()`，直接透传 `platform.AccessibilityTrustedDirect()`。`settingsBindings.ts` 补对应的手写绑定（这个文件本来就是手写的，见文件顶部注释——`wails generate module` 只会给实际 `Bind()` 的 `App` 生成绑定，`SettingsApp` 从来没被自动生成过）。
  - `Settings.tsx` 新增一个纯 mount-time 的 `useEffect`：调 `IsAccessibilityTrusted()`，`false` 时 `setNotice(...)` 一段翻译过的中文提示（系统设置路径改成 macOS 实际菜单命名"系统设置 › 隐私与安全性 › 辅助功能"）。和其它几个 mount-time effect（`GetSettings`/`GetCacheDir`/`refreshUsage`）并列，互不冲突；不会被后续的快捷键捕获相关 effect 意外清空（那些只在用户主动点"设置快捷键"或按键时才会 `setNotice("")`）。
- [x] `cd frontend && npm run build`（`tsc` + `vite build`）、`go build ./...`（原生 macOS）、`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` 交叉编译、`go vet ./...`、`go test ./...`（同上，除那一个无关的既有失败用例）全部通过。
- [x] 重新 `wails build` 出新的 `build/bin/SailBoard.app`。
- [x] **真机验证完成（用户确认）**：本机开发环境下 `AXIsProcessTrustedWithOptions` 对任意进程都直接返回"已授权"（连 `tccutil reset` 都清不掉，猜测是沙箱本身没启用逐应用 TCC 限制），没法自行触发"未授权"分支肉眼确认；用户在自己机器上实测，未授权时设置窗口正确弹出提示，且确认了这次修复真正要解决的"授权后不用重启主 app、下次打开设置窗口就不再提示"这一点。

## 本轮完成（2026-08-24 深夜：macOS 补上"捕获快捷键时暂停旧热键"机制，功能对等收尾）

"已知限制"里记的最后一个功能差距：Windows 那轮加的"设置窗口捕获新快捷键期间暂停主进程当前热键"（`OnHotkeySuspendRequested`/`OnHotkeyResumeRequested`）当时明确写了"本机是 Windows 开发机，`.m` 改不了也测不了，留到有 Mac 环境时补"——这次直接在 Mac 上补上。

- [x] 照抄 `ipc_darwin.go`/`.m`/`.h` 里 `NotifySettingsChanged`/`OnSettingsChanged` 那一套 `NSDistributedNotificationCenter` 模式，原样复制一份给 suspend/resume：`ipc_darwin.h` 新增 `sb_notify_suspend_hotkey`/`sb_notify_resume_hotkey` 声明；`ipc_darwin.m` 新增两个通知名常量、两个 `addObserverForName` 注册（挂在已有的 `sb_watch_distributed_notifications` 里）、两个 `postNotificationName` 发送函数；`ipc_darwin.go` 里 `darwinController` 新增 `OnHotkeySuspendRequested`/`OnHotkeyResumeRequested` 方法（不再是继承自 `stubController` 的空实现）、新增 `SuspendHotkeyDirect`/`ResumeHotkeyDirect`（替换掉 `settingswindow_darwin.go` 里原来的空函数）、新增两个 `//export` 回调 `sbSuspendHotkeyNotification`/`sbResumeHotkeyNotification`，同样经 `darwinMainThreadCallbacks` 转一手（避免在 `NSDistributedNotificationCenter` 回调栈里直接起 goroutine，踩 `hotkey_darwin.go` 那个 SIGSEGV 的坑）。`app.go`/`settings_app.go`/`Settings.tsx` 三处的调用点这次修复前就已经是平台无关的通用代码（`8c80b9c` 那轮加的），不用改。
- [x] `go build ./...`（原生 macOS）、`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` 交叉编译、`go vet ./...`、`go test ./...`（同上，除那一个无关的既有失败用例）全部通过。
- [x] **真机验证完成（程序坞图标部分复用同一次构建）**：`CGO_LDFLAGS="-framework UniformTypeIdentifiers" wails build -platform darwin/arm64 -clean` 编译出 `build/bin/SailBoard.app`，`build/darwin/package-dmg.sh` 打出 `build/bin/SailBoard-1.0.0-arm64.dmg`；启动主进程确认 `lsappinfo` 报 `type="UIElement"`（没有回归）。
- [x] **真机验证完成（用户确认）**：设置窗口"点击设置快捷键"捕获态期间按当前生效的全局热键，主面板确认不会弹出。

## 本轮完成（2026-08-24 深夜：macOS 设置窗口在程序坞里出现多个图标 + 打开设置后主面板/设置窗口互相"消失"）

用户反馈两个 macOS 问题：1）程序坞里出现了好几个 SailBoard 图标；2）召出主面板后点"设置"，主面板和设置窗口会一起自动隐藏。一开始按 Windows 场景排查了一轮（focuswatch_windows.go 加了个 debounce），后来用户澄清是 macOS/程序坞，Windows 那版改动已撤销（`git checkout` 还原），未提交、未使用。

- [x] **根因（两个现象是同一个机制）**：设置窗口（`main.go` 的 `runSettingsWindow`）是重新拉起同一个可执行文件的**第二个独立进程**（Wails v2 没有单进程多窗口 API，这是设计文档里就定好的取舍），不是主进程的第二个窗口。这带来两个连锁问题：
  1. **程序坞图标**：Wails 的 `AppDelegate.m` 在 `applicationWillFinishLaunching` 里无条件把 activation policy 设成 `Regular`，主进程靠 `tray_darwin.go` 的 `ShowTray` 调用 `darwinHideDockIcon()` 切回 `Accessory` 才不出现在程序坞；但 `settings_app.go` 从来没调用过这个函数——每次打开设置，这第二个进程都会顶着 `Regular` policy 在程序坞露一个图标，如果窗口被直接用标题栏关掉而不是真正退出进程，图标还会常驻，多开几次就堆出好几个。
  2. **主面板/设置窗口互相"隐藏"**：`controller_darwin.go` 的 `WatchFocusLoss` 靠轮询 `[NSApp isActive]`（`controller_darwin.m` 的 `sb_app_is_active`）判断"是否切到了别的 app"，注释里当初的假设是"切到 SailBoard 自己的设置窗口时,本进程仍是 active"——这个假设只在主面板/设置窗口是**同一个进程**的两个窗口时成立；而实际架构是两个进程，切到设置窗口时主进程的 `isActive` 会真的变 false，触发和"切到 Chrome"完全一样的处理：`app.go` 的 `HideWindowAnimated` 不仅把主面板滑走，还会 `RestoreForeground` 把焦点还给"主面板最初被唤出前"活跃的那个 app——这一下就把刚打开的设置窗口的焦点又抢走了，看起来像两个窗口都消失了。
- [x] **修复**：
  - `internal/platform/settingswindow_darwin.go` 新增 `HideDockIconDirect()`（直接复用已有的 `darwinHideDockIcon()`），`settings_app.go` 的 `startup` 里在窗口显示前调用——设置窗口不再在程序坞露出图标（Windows/其他平台对应加了空实现：`settingswindow_windows.go`、`controller_stub.go`）。
  - `controller_darwin.m` 的 `sb_app_is_active`：不再只看 `[NSApp isActive]`，改成比较系统级最前台 app 的 bundle identifier 和自己的 bundle identifier 是否一致——主面板和设置窗口是同一个 .app 启动的两个进程，bundle id 天然相同，这样切换到设置窗口时仍会正确判定为"还是我们自己"，只有真的切到别的 app 时才会判定为失焦；bundle id 拿不到时（未打包的 `wails dev` 裸二进制）回退到旧的 `[NSApp isActive]` 逻辑，不影响开发模式。
- [x] `go build ./...`（原生 macOS）与 `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` 交叉编译均通过；`go test ./...` 除一个与本次改动无关、修改前就存在的失败用例（`TestParseFilesHashIsOrderIndependent`，Windows 路径格式测试在 macOS 上跑）外全部通过。
- [x] **真机验证完成**：程序坞图标这一半——`wails build` 出 `build/bin/SailBoard.app` 后，主进程 + 手动拉起的 `--settings` 子进程同时跑（`lsappinfo list` 确认两个进程都是 `type="UIElement"`），再直接查 Dock 本身（`osascript` 问 `System Events` 的 Dock 进程列表要 `SailBoard` 相关的 UI 元素）——一个都没有，图标问题确认修好。焦点互相"隐藏"那一半——**用户确认**：热键唤出主面板、鼠标点"设置"，两个窗口都不再互相隐藏。

## 本轮完成（2026-08-24 深夜：macOS 设置界面里 Cmd/Win 修饰键标签打磨）

"已知限制"里记的一条收尾项：macOS 上手动捕获快捷键时，Cmd 键在 UI 上一直显示成 Windows 的叫法"Win"。

- [x] `frontend/src/Settings.tsx`：仿照 `App.tsx` 已有的 `isMac`（`navigator.platform`/`navigator.userAgent` 里找 "Mac"）判断，`formatCapturedKey` 里 `event.metaKey` 对应的标签按平台选 `"Cmd"`（macOS）或 `"Win"`（其他平台），未支持按键的提示文案里列举的修饰键名称同步跟着平台变。`internal/platform/hotkey.go` 的 `ParseHotkeySpec` 本来就把 `cmd`/`command`/`super`/`win`/`windows`/`meta` 当同一个修饰键的别名处理（大小写不敏感），纯展示层面的改动，不影响已保存的旧快捷键解析，也不需要动 Go 侧代码。
- [x] `cd frontend && npm run build`（`tsc` + `vite build`）通过；`go build ./...` 重新嵌入前端产物后仍编译通过。
- [x] **真机验证完成（用户确认）**：捕获快捷键时标签正确显示为"Cmd"。

## 本轮完成（2026-08-24 更晚：设置窗口 Alt 键捕捉失效 + 200% 缩放下面板过小的修复）

用户反馈两个 Windows 设置窗口的问题：1）自定义快捷键时按下 Alt 会叫出 Windows 自带的系统菜单，导致 Alt 组合快捷键捕捉不到；2）200% 缩放设备上设置面板出现滚动条，窗口大小没有跟着缩放适配。均只影响独立的设置窗口进程（`main.go` 的 `runSettingsWindow`/`settings_app.go`），不影响主面板。

- [x] **根因（Alt 键）**：设置窗口是 Wails 默认创建的普通标题栏窗口（`WS_OVERLAPPEDWINDOW`，含 `WS_SYSMENU`），本身没有调用过 `SetMenu` 挂任何菜单项，但 Windows 仍会为这类窗口提供一份默认系统菜单（还原/移动/大小/最小化/最大化/关闭）。单独按下 Alt（或 F10）时，`DefWindowProc` 处理未被消费的 `WM_SYSKEYUP` 会合成一条 `WM_SYSCOMMAND`/`SC_KEYMENU` 消息弹出这份默认菜单并把键盘焦点切换进"菜单模式"——这正是用户说的"叫出 Windows 自带的一个面板"，且发生在原生消息层，前端 `Settings.tsx` 对 `keydown` 事件调用 `preventDefault()`/`stopPropagation()` 拦不住（那是 DOM 层，`WM_SYSCOMMAND` 是更早的 Win32 消息层）。
- [x] **修复（Alt 键）**：`internal/platform/settingswindow_windows.go`（新文件）的 `suppressSystemMenuKey`：用 `SetWindowLongPtrW(hwnd, GWLP_WNDPROC, ...)` 给设置窗口挂一个自定义窗口过程（`syscall.NewCallback`，模式与 `winmsg_windows.go` 的隐藏消息窗口一致，只是这次是给 Wails 已创建的窗口做子类化而非新建窗口），拦截 `WM_SYSCOMMAND` 且 `wParam & 0xFFF0 == SC_KEYMENU` 时直接返回 0（吞掉，不转发给原窗口过程），其余所有消息原样 `CallWindowProcW` 转发——关闭/最小化/最大化/移动/缩放走的是别的 `SC_*` 常量，标题栏按钮不受影响，只是"单独按 Alt/F10 弹出无内容的系统菜单"这一个行为被关掉。
- [x] **根因（200% 缩放面板过小，未100%确认）**：读 Wails v2.10.2 源码确认标准窗口创建路径（`winc.Form.SetSize` 内部的 `scaleWithWindowDPI`，以及 `WM_DPICHANGED` 处理）理论上会正确按 `GetDpiForWindow` 缩放物理像素，但本项目已经三次在这一层踩到 Wails 自己的坐标/尺寸计算不可靠（见 CLAUDE.md 记录的 `MonitorFromPoint`/`WindowSetPosition`/上一轮 `panelHeight` 三个坑）；实测 100% 缩放下截图确认 `Settings.css` 内容已经把 560 逻辑像素的窗口高度填得很满、余量很小（见截图，"关于 SailBoard" 链接和底部按钮之间几乎无空隙），缩放到 200% 时哪怕有一点点缩放误差就会立刻表现为出滚动条——与用户描述吻合，但本机当前是 100% 缩放（`Graphics.DpiX=96`），且这是共享桌面、不便为了测试擅自修改用户的系统显示缩放，所以**这个根因推断没有真机 200% 环境复现确认**。
- [x] **修复（200% 缩放）**：同一新文件的 `resizeToActualMonitorScale`：不信任 Wails 内部缩放，改为拿到设置窗口 HWND 后自己用 `MonitorFromWindow` + `GetDpiForMonitor`（与 `screen_windows.go` 的 `workAreaNearCursor` 同一套调用）算出窗口实际所在显示器的缩放比例，再用 `SetWindowPos` 直接把窗口设成 `420×560`（`main.go` 新增的 `settingsWindowWidth`/`settingsWindowHeight` 常量，与 `wails.Run` 的 `options.App.Width/Height` 共用同一份值）按该比例换算后的物理像素尺寸——如果 Wails 自己的缩放本来就是对的，这一步只是把窗口设成同样的尺寸（无副作用的空操作）；如果不对，则直接纠正。`settings_app.go` 的 `startup()` 开头调用 `platform.FixSettingsWindowDirect(settingsWindowTitle, settingsWindowWidth, settingsWindowHeight)`——Wails 是"先创建原生窗口、再调用 OnStartup、最后才 Show()"的顺序（读源码确认），所以这个调整发生在窗口首次显示之前，不会有可见的尺寸跳变。
- [x] 新增 `internal/platform/settingswindow_darwin.go`（no-op：macOS 用点坐标系，没有物理/逻辑像素换算问题，也没有 Alt 键系统菜单这回事）和 `controller_stub.go` 里的 no-op（非 Windows 非 macOS 平台），保持 `platform.FixSettingsWindowDirect` 在三个平台分支都有实现，`settings_app.go` 可以无条件调用不用加 `GOOS` 判断。
- [x] `go build ./...`、`go vet ./...`（仍是 CLAUDE.md 记录的那 8 处已知 unsafe.Pointer 误报，未新增）、`go test ./...` 全部通过；`GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build` 依旧因为 darwin 代码本身依赖 cgo 而报 `undefined: darwinController`——用 `git stash` 确认这个报错在本轮改动之前就存在，与本轮无关（这不是本项目惯用的交叉编译检查方向：CLAUDE.md 记录的标准做法是反过来，在 macOS 上跑 `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build` 检查有没有碰到 Windows 编译路径，本项目目前在 Windows 机器上开发，没有反向等价手段）。跑通一次 `wails build`，产物 `build/bin/SailBoard.exe` 编译成功。
- [x] 真机验证了尺寸修复的"无回归"部分：在本机 100% 缩放下启动设置窗口，`GetWindowRect` 确认窗口仍是精确的 420×560（与修复前一致），截图确认内容渲染正常，说明新加的 `resizeToActualMonitorScale` 在 100% 缩放下确实是无副作用的空操作，没有引入尺寸错误。
- [x] **真机验证完成**：用户在真实环境里确认了 200% 缩放下设置面板正常显示、不再出现滚动条，以及 Alt 键修复本身生效（单独按 Alt/F10 不再弹出系统菜单）——本机当时受限于共享桌面环境无法自测的两项，均已由用户在实机上确认通过。

## 本轮完成（2026-08-24 再晚：设置窗口四项反馈——Alt+Space 仍无法捕捉、提示框撑出滚动条、主面板遮挡设置窗口、捕获时旧快捷键仍会触发）

用户真机试用了上一轮的 Alt 键修复后反馈：系统菜单确实不再弹出了，但 Alt+Space 这个组合本身依然捕捉不到；另外发现三个新问题——新增的提示 `<p className="notice">` 出现时会把设置面板撑高到出现滚动条（明确要求不能有滚动条）；点击设置面板后主面板会盖住一部分设置窗口（主面板 `AlwaysOnTop` 导致）；配置快捷键时如果误按到当前生效的旧快捷键，仍会弹出主面板，体验不优雅。四项都只涉及设置窗口/主面板交互，不影响 macOS（除下面第 4 点的部分基础设施外）。

- [x] **根因（Alt+Space 仍捕捉不到）**：不是上一轮修复没生效——系统菜单确实被吞掉了（`WM_SYSCOMMAND`/`SC_KEYMENU` 层面成功拦截）。但 `Alt+Space`（以及裸 Alt、`Alt+F10`）是 Windows/Chromium 更早一层就特殊处理的经典系统保留键：WebView2 把这类组合识别为"系统加速键"，压根不会把它们当作普通按键分发给页面的 `keydown` 事件——`Settings.tsx` 的键盘监听从一开始就收不到这个按键，不是拦截/`preventDefault` 能解决的问题（那是 DOM 层，问题出在 DOM 事件根本不触发）。而且就算 JS 层能拼出 "Alt+SPACE" 这个 spec 字符串，它作为**全局**热键在语义上也有问题：Alt+Space/Alt+F10 是系统级保留给"任意窗口的系统菜单"用的，把它注册成 SailBoard 的全局热键会和系统里几乎每个窗口的默认行为冲突，本来就不是一个好选择。
- [x] **修复思路**：不强行让这类组合可捕捉（需要更底层的全局键盘钩子，风险和复杂度都明显更高，不是这几个反馈的诉求），改为在原生层检测到这类"被系统保留键吞掉"的按键时，主动告知前端"这个组合不能用"，而不是像现在这样静默卡在捕获状态。`internal/platform/settingswindow_windows.go` 新增 `settingsOnReservedKey` 回调 + `SetOnSystemMenuKeyDirect`：`settingsWndProcDispatch` 每次吞掉一次 `SC_KEYMENU` 时都触发它（不区分是裸 Alt 还是 Alt+Space/F10——两者走的是同一条系统消息路径，且裸 Alt 单独触发只会发生在"没有同时按下其他键就松开"的场景，不会和正常的 `Alt+字母` 捕获误触发混淆，因为只要 Alt 按住期间另一个键被按下过，系统就不会把这次 Alt 释放当成"孤立按键"处理）。`settings_app.go` 的 `startup()` 里把这个回调接到 `runtime.EventsEmit(ctx, "hotkey:reserved")`；`Settings.tsx` 新增对应的 `EventsOn("hotkey:reserved", ...)` 监听，仅在当前正处于 `capturing` 状态时才生效（用 `setCapturing` 的函数式更新读最新值，避免闭包拿到过期的 `capturing`），弹出提示"Alt+Space、Alt+F10 等属于 Windows 系统保留组合键，无法用作快捷键，请换一个组合"并退出捕获状态。macOS/其他平台在 `settingswindow_darwin.go`/`controller_stub.go` 里是空实现（macOS 的 Cmd 组合键捕获不存在这个系统保留键问题）。
- [x] **修复（提示框撑出滚动条）**：`Settings.css`：`.settings-page` 从 `min-height: 100vh` 改成 `height: 100%; overflow: hidden`（`html, body` 也加 `overflow: hidden`），配合 `.notice` 从"参与文档流、把后面内容往下挤"改成 `position: absolute`（页面本身设 `position: relative`）浮在页面顶部的一条 toast——出现/消失都不再改变页面内容总高度，物理上不可能再出现滚动条（不管是这版还是以后任何内容更长的场景）。视觉上从原来浅蓝底色改成更实一点的蓝底白字 + 阴影，读起来更像"浮在上面的提示"而不是"页面的一部分"，加了个 `noticeIn` 淡入位移的小动效。
- [x] **修复（主面板遮挡设置窗口）**：根因是 `main.go` 里主面板窗口开了 `AlwaysOnTop: true`（Windows 上即 `WS_EX_TOPMOST`），topmost 窗口无论谁在前台聚焦，都会盖在所有非 topmost 窗口之上——设置窗口原来不是 topmost，天然被压在下面。修复：给 `runSettingsWindow` 的 `options.App` 也加上 `AlwaysOnTop: true`，让两者同属"最上层"分组，组内仍按正常的聚焦/激活顺序排序，点击设置窗口就能正常盖过主面板；不影响设置窗口和其它无关应用窗口之间的相对关系。
- [x] **修复（捕获快捷键时旧快捷键仍会触发主面板）**：新增一对跨进程 IPC，与已有的 `NotifySettingsChanged`/`OnSettingsChanged` 走同一套"设置窗口独立进程 → 消息专用隐藏窗口"机制（`winmsg_windows.go` 新增 `wmSuspendHotkey`/`wmResumeHotkey` 两个自定义消息、`msgWindow` 新增对应回调字段；`ipc_windows.go` 新增 `SuspendHotkeyDirect`/`ResumeHotkeyDirect` 独立函数；`types.go` 的 `Controller` 接口新增 `OnHotkeySuspendRequested`/`OnHotkeyResumeRequested`，`controller_windows.go` 接到 `msgWindow` 的新回调；`controller_defaults.go` 的 `stubController` 给这两个方法一个共享空实现，macOS 直接继承，没有单独覆盖——见下面的取舍说明）。`app.go` 新增 `App.suspendHotkey`/`resumeHotkey`：前者解注册当前热键且不重新注册（复用 `applyShortcut` 已有的 `unregisterHotkey` 字段），后者重新读一遍已保存的设置并 `applyShortcut` 回去——和 `OnSettingsChanged` 回调调用的是同一个 `applyShortcut`,只是不需要等一次真正的保存。`settings_app.go` 新增 `BeginShortcutCapture`/`EndShortcutCapture` 两个绑定方法，`Settings.tsx` 用一个监听 `capturing` 变化的 `useEffect` 在进入/退出捕获状态时调用（所有退出路径——捕获成功、Esc 取消、按了不支持的键、上面提到的 `hotkey:reserved` 事件——都统一收敛到 `setCapturing(false)`,这一个 effect 就能覆盖全部场景）；另外在 `SettingsApp.shutdown()` 里无条件调用一次 `ResumeHotkeyDirect` 兜底（`EndShortcutCapture` 是幂等的重新注册,不会因为"其实没在捕获中就关闭了窗口"这种路径产生副作用），防止用户在捕获过程中直接关闭设置窗口（Alt+F4、标题栏 X）导致热键永久停摆。
  - **macOS 取舍（当时）**：`OnHotkeySuspendRequested`/`OnHotkeyResumeRequested` 在 macOS 上只是继承 `stubController` 的空实现，`SuspendHotkeyDirect`/`ResumeHotkeyDirect` 在 `settingswindow_darwin.go` 里也是空函数——即暂停/恢复这套机制当时只在 Windows 上真正生效。没有当场补齐是因为：1）这个问题的紧迫性主要来自 Windows 这次修复的 Alt/系统菜单场景，macOS 的 Cmd 组合键捕获没有同等的系统保留键冲突；2）真正实现需要照 `ipc_darwin.go`/`.m`/`.h` 里 `NotifySettingsChanged` 那一套再加一对 `NSDistributedNotificationCenter` 通知，属于 cgo 代码，本机是 Windows 开发机，既编译不了也没法验证，贸然改 `.m`/`.h` 属于盲写，风险比价值高；留作已知缺口。**已在后续的 Mac 开发机会话里补上**——见上方"macOS 补上'捕获快捷键时暂停旧热键'机制"一节，照抄的正是这里说的同一套模式。
- [x] `go build ./...`、`go vet ./...`（仍是那 8 处已知 unsafe.Pointer 误报，未新增）、`go test ./...`、`cd frontend && npm run build`（`tsc` + `vite build`）全部通过；`wails build` 产物 `build/bin/SailBoard.exe` 编译成功。
- [x] 启动过一次新的设置窗口确认 `GetWindowRect` 仍精确是 420×560（尺寸修复没有被这轮改动破坏）。
- [x] **真机验证完成**：上面四点用户均已在自己环境里确认通过——提示框不再撑出滚动条、设置窗口能正常盖过主面板、捕获时旧快捷键不会再触发主面板；Alt+Space 的提示/识别行为在下一轮进一步升级为可直接用作快捷键（见下）。

## 本轮完成（2026-08-24 又晚：主面板在另一台设备上右键后变深灰色）

用户反馈：新编译的版本在当前开发机上没问题，但在另一台 Windows 设备上，唤出主面板后按右键，界面会变成深灰色。

- [x] **根因（推断，未能在本机复现——本机右键正常，问题只在另一台设备上出现）**：本项目从未在任何地方主动处理过右键/`contextmenu`，右键点击会触发 WebView2 自带的默认右键菜单。`main.go` 原本没有显式设置 `options.App.EnableDefaultContextMenu`（Go 零值 `false`），Wails 内部逻辑是 `AreDefaultContextMenusEnabled = f.debug || EnableDefaultContextMenu`——`wails build`（非 `-debug`）产物 `f.debug` 为 false，理论上默认菜单已经是关闭的。但主面板窗口比较特殊：无边框、`AlwaysOnTop`（`WS_EX_TOPMOST`）、`WebviewIsTransparent` + `WindowIsTranslucent` + `BackdropType: Acrylic`（`WS_EX_NOREDIRECTIONBITMAP` + DWM 亚克力毛玻璃背景）——这几个特性叠加在一起本来就是比较少见的窗口组合，怀疑是另一台设备的 WebView2 Runtime 版本/显卡驱动没有完全遵守 `AreDefaultContextMenusEnabled(false)` 这个设置（不同 WebView2 Runtime 版本对这个 API 的支持程度不完全一致，是有先例的已知问题），右键仍然尝试起一个原生右键菜单弹层——新建一个弹层窗口和这种"亚克力+透明+置顶"的窗口叠在一起，在某些 GPU/驱动下会让 DWM 没能正确重新合成亚克力材质,退化成纯色深灰色兜底（这是 Acrylic/Mica 类毛玻璃背景在合成失败时的典型表现，Windows 生态里对这几个特性组合本来就没有很稳的兼容性保证）。这个解释目前只是"读代码 + 已知 Windows/WebView2 生态问题"推出来的最合理机制，**没能在本机复现来实锤**（本机右键完全正常）。
- [x] **修复**：双保险，不依赖单一层面：
  1. `main.go`：把 `EnableDefaultContextMenu` 从"没写、隐式零值 false"改成显式写出 `false`，行为不变，只是让这个决定看得见。
  2. `frontend/src/main.tsx`、`frontend/src/settings-main.tsx`：各自新增一行 `document.addEventListener('contextmenu', event => event.preventDefault())`——在 DOM 层再拦一次，不管 WebView2 那边的设置在某台设备上是否真的生效，右键都不会有任何默认行为（包括不会去尝试起一个原生菜单弹层）。两个入口文件都加，覆盖主面板和设置窗口。
- [x] `go build ./...`、`cd frontend && npm run build`（`tsc` + `vite build`）、`wails build` 全部通过，产物 `build/bin/SailBoard.exe` 编译成功。
- **未能完成的验证**：这是这次修复清单里最没底的一项——问题只在"另一台设备"上出现，本机（开发机）完全无法复现，根因分析停留在"读代码 + 已知同类 Windows/Acrylic/WebView2 生态问题"的合理推测层面，没有实锤到具体是哪一层（WebView2 Runtime 版本？显卡驱动？两者都有可能）导致默认菜单设置没生效。修的两处都是"不管根因具体是什么，都能把触发条件（右键起菜单弹层）直接掐掉"的防御性动作，理论上应该能解决，但没有在问题设备上验证过。需要用户在那台出问题的设备上装上这次的新构建，重复"唤出主面板 → 右键"确认深灰色问题不再出现；如果修复后仍然复现，说明根因不是右键菜单本身，需要用户描述更多细节（比如是不是只有右键才会触发，长按/其他操作会不会也触发，那台设备的显卡型号/是否是虚拟机或 RDP 环境等）以便继续排查。

## 本轮完成（2026-08-24 又又晚：主面板深灰色问题——修正触发条件，改走 CSS 隔离方案）

用户纠正了上一轮的诊断：触发条件不是右键点击，而是方向键→或者点击其他卡片，用户自己的直觉是"感觉有点像失焦"。上一轮的 `contextmenu` 拦截修复因此打错了目标（保留下来无害，但不是真正的修复）。

- [x] **根因（比上一轮更精确，仍未能在本机复现确认）**：读 `App.tsx` 发现方向键→和"点击尚未选中的卡片"这两个触发条件都会调用 `setSelected`，进而命中同一个 `useEffect`（第 119-121 行）：`cardRefs.current[selected]?.scrollIntoView({ behavior: "smooth", ... })`——两个触发条件殊途同归，走的是同一处 smooth-scroll 动画。而 `App.css` 的 `.shell`（覆盖整个窗口的最外层）本身叠了 `backdrop-filter: blur(34px) saturate(180%)`，且是叠在原生 Acrylic 背景（`main.go` 的 `windows.Acrylic`）之上的"双重模糊"——这本来就是一个偏重的合成负担。卡片栏 smooth-scroll 动画期间会持续触发重绘，怀疑在某些显卡/驱动组合下，这种"透明窗口 + 原生 Acrylic 背景 + 上面再叠一层 CSS 高斯模糊"的合成在滚动引发的持续失效重绘下会跟不上，退化成纯色兜底——这和用户说的"感觉像失焦"也能对上：Windows 原生 Acrylic 背景在窗口失焦时确实会主动变得更不透明/更平，一个合成失败的场景在视觉上可能和"看起来像失焦变灰"混在一起,不容易用肉眼区分开。
- [x] **修复（用户在三个选项——轻量隔离 / 直接去掉 CSS 模糊层 / 取消 smooth 滚动——里选择了"先试轻量隔离"，不改变现有视觉效果）**：`App.css` 的 `.card-rail` 新增 `contain: paint; will-change: transform; transform: translateZ(0);`——给浏览器一个更强的提示，让这个横向滚动容器尽量被提升到独立的 GPU 合成图层，滚动时只挪动这个图层本身，不强迫上层 `.shell` 的 `backdrop-filter` 跟着每一帧重新采样合成。没有动 `.shell` 的模糊设置本身，纯粹是"隔离滚动动画、别牵连模糊层"的防御性提示。
- [x] `cd frontend && npm run build`（`tsc` + `vite build`）、`wails build` 全部通过；本机（问题不会复现的这台机器）启动主面板做了一次纯视觉抽查（截图确认卡片栏渲染正常、毛玻璃背景无异常），排除这次改动在正常设备上引入明显视觉回归。
- **未能完成的验证**：这依然是"读代码 + 通用 GPU 合成知识"推出的最合理机制,不是实锤——本机完全无法复现，没有办法验证这个 CSS 隔离提示是否真的解决了问题设备上的合成失败（`contain`/`will-change` 只是给浏览器的提示，不保证在有驱动级 bug 的 GPU 上一定管用）。需要用户在会复现问题的那台设备上装这次构建，重复"方向键→"和"点击其他卡片"确认深灰色不再出现。**如果这次隔离没能解决**：用户在选项里还预留了另外两个更有把握、但代价不同的后备方案——直接去掉 `.shell` 的 CSS 模糊层只保留原生 Acrylic（从机制上彻底消除双重模糊合成负担，但会损失 CSS 那层额外加的饱和度/模糊调整，视觉会有细微变化）,或者把 `scrollIntoView` 的 `behavior` 从 `"smooth"` 改成 `"auto"`（瞬间跳转，从根源上消除持续重绘的触发源，但键盘/点击切换卡片时会丢失现有的平滑滑动手感）——先反馈这次的结果，再决定要不要往这两个方向继续。

## 本轮完成（2026-08-24 三度深夜：主面板深灰色——定位到精确触发源，去掉 CSS 模糊层）

用户进一步纠正：不需要点击，**鼠标悬浮到别的卡片上**就会触发灰色，比之前汇报的"方向键→/点击"范围更小、更精确。

- [x] **根因（这轮才真正定位精确，此前两轮的"方向键/点击"只是这个更小触发源的两个间接路径）**：卡片唯一的悬浮效果是 `.card:hover { transform: translateY(-3px); ... }`——纯鼠标移动，不涉及点击、不涉及 `selected` 变化、不涉及上一轮怀疑的 smooth-scroll。这就把范围收窄到"仅仅是给一个元素施加 `transform`"本身。而 `transform` 是触发浏览器新建独立 GPU 合成图层的经典场景——这强烈指向：`.shell` 的 CSS `backdrop-filter: blur(34px)`（叠在原生 Acrylic 背景之上的第二层模糊）在旁边任何地方新建 GPU 合成图层时都需要重新采样合成，而这台问题设备的显卡/驱动在这个重新采样过程中会失败,退化成纯色兜底。上一轮"只隔离卡片栏滚动"的轻量方案之所以没用，是因为它压根没对上真正的触发源（滚动只是两条间接路径之一，真正的触发源是任意卡片的 hover transform，此前一直被"方向键/点击也会触发"这个更宽泛但不精确的现象掩盖）。
- [x] **修复**：用户在"去掉 CSS 模糊层"/"只去掉悬浮位移"/"再试一次常驻图层提示"三个选项里选择了最彻底的一个——`App.css` 的 `.shell` 规则整个删掉 `backdrop-filter: blur(34px) saturate(180%)` 和 `-webkit-backdrop-filter`（保留 `background: var(--glass)` 半透明白色调不变）。面板的毛玻璃效果从此完全由 `main.go` 已有的原生 Acrylic 背景（`windows.Acrylic`）提供，不再叠加第二层 CSS 自己的模糊——从机制上整个消除"双重模糊在新图层出现时合成失败"这一类问题，而不是继续找哪个动画/效果会触发它（`.card` 的入场/退场动画、`.card-actions` 的 opacity 过渡等，理论上都可能是下一个触发点，逐个堵不如直接去掉双重模糊的根）。顺带撤销了上一轮加在 `.card-rail` 上的 `contain`/`will-change`/`translateZ` 隔离提示——那是针对"滚动会牵连模糊层"这个已经不成立的假设加的,模糊层本身都没了,这层防御自然也没有意义,按项目一贯风格删掉而不是留着当死代码。
- [x] `cd frontend && npm run build`（`tsc` + `vite build`）、`wails build` 全部通过；本机再次启动主面板截图抽查，界面渲染正常，和去掉模糊层之前肉眼对比无明显差异（本机桌面背景本身偏纯色，模糊与否在截图里不容易看出区别，但至少确认没有布局/渲染错误）。
- **未能完成的验证**：和前两轮一样，本机无法复现原问题，这次的"去根"修复没能在问题设备上实测。但这次去掉的是双重模糊合成本身，而不是像前两轮那样试图用 CSS 提示绕过某一个具体触发点——从原理上讲，只要问题设备上的合成失败确实和"CSS backdrop-filter 叠加原生 Acrylic 背景"这个组合有关（目前证据链——精确定位到"仅需新建一个 GPU 图层就触发"——支持这个机制,但仍然是推断,没有实锤到具体是哪一层驱动/API 出的问题）,这个修复应该能连带解决掉所有类似触发点(不只是 hover,理论上入场动画等也会一并规避)。需要用户在问题设备上装这次构建，重点验证：1）悬浮别的卡片；2）方向键切换；3）点击切换卡片——确认三者都不再出现灰色。如果这次仍然复现，说明合成失败的根因不在 CSS backdrop-filter 叠加，而可能在原生 Acrylic 背景本身（`windows.Acrylic`）在该设备上就不稳定，需要往"换用更简单的背景类型（比如 `windows.Mica` 或纯 CSS 背景色，放弃原生模糊）"这个方向继续排查——那将是一个更大的设计取舍，需要另外讨论。

## 本轮完成（2026-08-24 四度深夜：让 Alt+Space 真的能用作快捷键）

用户问"有什么办法兼容 Alt+Space 作为快捷键吗"——追问的是能不能真的**用上** Alt+Space，不只是"解释为什么捕捉不到"。

- [x] **关键验证（在本机独立确认，不依赖问题设备，是纯 Win32 API 行为，和显卡/Acrylic 无关）**：写了个独立的最小 Go 程序单独调用 `RegisterHotKey(0, id, MOD_ALT, VK_SPACE)`，在本机确认**调用成功**——Windows 并不在 API 层面禁止把 Alt+Space 注册成全局热键。也就是说，之前几轮定位到的"捕捉不到"完全是设置界面**实时监听 `keydown` 事件**这一种交互方式的局限（WebView2/Chromium 把 Alt+Space 当系统保留加速键，压根不派发到页面），跟 Alt+Space 能不能被用作真正的全局热键是两回事——只要换一种方式把这个组合"喂"给设置界面，而不是指望用户按下时页面能收到 `keydown`，就可以支持它。
- [x] **修复**：复用上一轮已经在做的"原生层检测到吞掉 SC_KEYMENU 时通知前端"机制，把它从"只报告失败"升级成"能识别出具体是哪个组合就直接报告出来"：
  - `internal/platform/settingswindow_windows.go`：`settingsWndProcDispatch` 吞掉 `SC_KEYMENU` 时新增 `resolveReservedKeySpec()`——用 `GetAsyncKeyState(VK_SPACE)` 判断此刻 Space 键是否还按着（Alt+Space 触发 `WM_SYSCOMMAND` 发生在 Space 刚按下、`TranslateMessage` 几乎立即把它翻译成 `WM_SYSCHAR(' ')` 的那一刻，此时用户物理上大概率还按着 Space；裸 Alt 单独松开触发的 `SC_KEYMENU` 则没有 Space 参与，这个判断足以把两种情形区分开）。能确定是 Alt+Space 就返回 `"Alt+SPACE"`，识别不出来（比如裸 Alt 单独一下）就返回空字符串，交给回调。
  - `SetOnSystemMenuKeyDirect`/`settings_app.go` 的回调签名从 `func()` 改成 `func(resolvedSpec string)`，把这个识别结果透传到 `runtime.EventsEmit(ctx, "hotkey:reserved", resolvedSpec)`；macOS/其他平台的空实现同步改签名（没有实际行为变化，只是类型对齐，反正在那些平台上从来不会被调用）。
  - `Settings.tsx` 的 `hotkey:reserved` 监听：`resolvedSpec` 非空时，当成一次**成功**的捕获处理——直接把 `settings.shortcut` 设成这个 spec（和正常键盘捕获走的是同一份 `settings.shortcut` 状态，后续保存逻辑完全不用改），并弹一条提示"已设置为 Alt+SPACE。提示：系统里所有窗口的 Alt+Space 系统菜单快捷键都会改由 SailBoard 接管"——明确把"抢了这个系统级默认行为"这个代价告诉用户，而不是悄悄让它生效；`resolvedSpec` 为空时（裸 Alt/F10）维持上一轮"不支持，换一个组合"的提示不变。
- [x] `go build ./...`、`go vet ./...`（仍是那 8 处已知误报，未新增）、`go test ./...`、`cd frontend && npm run build`（`tsc` + `vite build`）、`wails build` 全部通过。
- [x] **真机验证完成**：用户已确认端到端链路正常——设置界面能正确捕获并填入 Alt+Space、保存后作为全局快捷键真的能唤出面板。

## 本轮完成（2026-08-24 五度深夜：拦截 Ctrl+C / Alt+Tab 等常用快捷键，不允许占用）

用户要求：像 Alt+Tab、Ctrl+C、Ctrl+V 这类常用按键，不要让用户能把它们设成 SailBoard 的全局快捷键——和之前处理系统保留键（Alt+Space 那批）一样弹提示建议换一个，而不是真的注册上去。

- [x] **和"系统保留键"是两个不同的问题，分开处理**：Alt+Space/裸 Alt 那批是"Windows 根本不会把这个按键当普通按键分发给页面"，技术上没法通过正常捕获拿到；而 Ctrl+C/Alt+Tab 这批**技术上完全捕获得到**（是普通的 `keydown` 事件，`RegisterHotKey` 也不会拒绝注册它们）——问题在于就算捕获、注册成功了，也会在系统里把这个组合从"复制"/"切换窗口"变成"唤出 SailBoard"，静默破坏用户在所有其他软件里的这个快捷键。这是产品层面该主动拦的，不是能力上办不到。
- [x] **实现**：`frontend/src/Settings.tsx` 新增 `RESERVED_COMBOS`（一份精选、非穷举的常见系统/软件快捷键集合：`Ctrl+C/V/X/Z/Y/A/S/F/N/O/P/W/Tab`、`Alt+Tab`、`Alt+F4`），在两处捕获出口都过一遍：
  1. 正常键盘捕获路径（`formatCapturedKey` 拼出合法 spec 之后）——命中就弹"是系统或软件的常用快捷键，占用它会导致其失效，建议换一个组合"，退出捕获状态，不写入 `settings.shortcut`。
  2. 上一轮新加的 Alt+Space 原生识别路径（`hotkey:reserved` 事件的 `resolvedSpec`）——目前这条路径只会解析出 `"Alt+SPACE"`（不在黑名单里，不受影响），但还是过一遍同一份检查,保证以后如果原生层识别范围扩大了（比如哪天也去识别 Alt+F4——不过目前 Alt+F4 走的是 `SC_CLOSE` 而不是 `SC_KEYMENU`，本来就不会进这条路径，属于已知的、这轮之前就存在的独立小事项，未在本轮处理），也不会绕开这份拦截。
  - 判断只在前端（TypeScript）做一份，没有在 Go 侧（`internal/platform/hotkey.go`）重复一份黑名单：这个 spec 字符串目前只有这一个写入入口（这个捕获 UI），没有第二条路径能绕开前端校验直接落库，所以不需要跨语言重复维护同一份列表。
- [x] `cd frontend && npm run build`（`tsc` + `vite build`）、`wails build` 全部通过。
- [x] **真机验证完成**：用户已确认捕获时按 Ctrl+C/Alt+Tab 等常用快捷键会正确弹出"常用快捷键"提示，且不会被设置上。

## 本轮完成（2026-08-24 六度深夜：残留的卡片悬浮深色闪烁——常驻图层提示）

用户反馈去掉 `.shell` CSS 模糊层后，整个界面变灰的问题确实不再出现了，但残留一个更小范围的偶发问题：召出面板后偶尔悬浮卡片时，那张卡片的背景会闪一下深色（带"左右展开"的动画感），偶发、不稳定；鼠标划过多张卡片时可以同时出现在多张卡片上。

- [x] **根因（合理推断，与上一轮同一类机制，范围更小，同样未能在本机复现）**：`.card` 本身没有 `backdrop-filter`，但 `.card:hover` 会触发 `transform: translateY(-3px)` + `box-shadow` 的过渡动画（`transition: transform .16s ...`）——`transform` 过渡是浏览器新建独立 GPU 合成图层的经典触发点，和上一轮"整个面板变灰"的根因（新建合成图层时在这台设备上合成失败，退化成纯色）是同一类问题，只是这次触发对象是单张卡片而不是覆盖全窗口的 `.shell`，所以表现为"只有悬浮到的卡片变深色"而不是整个界面；"偶发"也能对上——合成失败本身就不是每次都发生，是显卡/驱动在特定时序下才会踩到的问题。之前"去掉 CSS 模糊层"只解决了 `.shell` 这一处，没有解决"卡片自己 hover 时新建图层"这个独立的触发点。
- [x] **修复**：`App.css` 的 `.card` 基础规则新增 `will-change: transform;`——让浏览器把这张卡片**长期**保持在独立的 GPU 合成图层里，而不是只在 `transform` 真正发生变化（悬浮/按下/入场/退场动画）的那一刻才临时新建、动画结束后又可能被回收。这样悬浮时只是"挪动一个已经存在的图层"，不再是"新建一个图层"，从机制上避开推测中的那个触发点，不需要等哪个具体动画去逐个堵。
- [x] `cd frontend && npm run build`（`tsc` + `vite build`）、`wails build` 全部通过。
- **未能完成的验证**：仍然是本机无法复现（这两轮的问题都只在用户那台设备上出现），`will-change` 也仍然只是给浏览器一个提示、不是强制保证，不能确保在有驱动级 bug 的 GPU 上一定管用。需要用户在问题设备上装这次构建，重点验证：面板召出后反复把鼠标划过多张卡片，确认不再出现深色闪烁。如果这次还是有残留，说明"常驻图层提示"这条路线本身就顶不住那台设备的驱动问题，可能需要考虑更激进的方案（比如整体放弃 `windows.Acrylic` 原生背景、换成纯 CSS 静态颜色，从根上不再依赖任何"透明窗口叠加原生合成"的组合）——但那是一个明显更大的视觉设计取舍，需要另外权衡再做决定。

## 本轮完成（2026-08-24 七度深夜：定位到关键新线索——问题和"唤出流程"绑定，不是纯 CSS 问题）

用户给了迄今为止最关键的一条线索：`will-change` 那版仍然复现，但补充说——**只在快捷键唤出之后**才会出现；点击切到设置面板一下，主面板就恢复正常；但下一次快捷键唤出，问题又回来。这条线索把此前几轮"哪个 CSS 属性触发了合成失败"的猜测方向直接推翻了，指向了一个完全不同、更具体的机制。

- [x] **根因（这轮改的是 Go 代码而不是 CSS，推理链更具体，仍未能在问题设备上实测确认，但读代码找到了一个真实存在、此前没意识到的竞态）**：`app.go` 的 `ShowWindow` 里，`SlideReveal`（`slide_windows.go`：每 10ms 调一次 `SetWindowPos` 物理挪动整个原生窗口，持续 `panelAnimationMs`=220ms）是丢到一个独立 goroutine 里异步跑的，跑的同时，主 goroutine 立刻（几乎同一时刻）调用了 `a.platform.FocusSelf`——这个函数内部会 `SetForegroundWindow`/`BringWindowToTop`/`SetFocus`，真实触发窗口的激活（`WM_ACTIVATE`）。也就是说，"窗口正在被每 10ms 挪动一次"和"窗口被抢焦点激活"这两件事目前是**并发**发生的,不是先后顺序。WebView2 的合成器在窗口还在快速移动的同时收到激活事件,懐疑会让它的合成状态在那一小段时间内进入一个不一致/过期的状态——如果这段时间内又有卡片 hover 触发新的 `transform` 图层，就可能读到这个坏状态，渲染成纯色深色。"点击设置面板再切回来"能自愈,是因为那是一次干净的、窗口早已静止时发生的激活/失焦循环,没有并发的窗口移动,合成器能正常刷新;下一次快捷键唤出又会重新触发同样的并发场景。这条推理链能同时解释"只在快捷键唤出后出现""偶发""切到设置面板后自愈""下次唤出又复现"这四个现象,是目前几轮里唯一一个能完整对上全部症状的机制,不再是"某个 CSS 属性单独导致"这种更零散的猜测。
- [x] **修复（低风险、纯增量，不改变现有已验证过的键盘抢焦点行为）**：在 `SlideReveal` 那个 goroutine 里,滑动动画结束之后再补一次 `a.platform.FocusSelf(appWindowTitle)` 调用——不是把原来"立即调用"的那次挪走或延后(那次是特意为了输入响应速度立即触发的,继续保留、不动它),而是在窗口已经完全静止之后,再触发一轮干净的激活/聚焦循环,人为复现"切到设置面板再切回来"那种能让它自愈的效果,让每次唤出都自动走一遍这个"愈合"步骤,不需要用户手动切到设置面板才能恢复。因为是纯追加（多调一次已经验证很久很安全的函数),不删除、不延迟任何现有逻辑,不应该产生除了"该函数被多调一次"之外的其他行为变化。
- [x] `go build ./...`、`go vet ./...`（仍是那 8 处已知误报）、`go test ./...`、`wails build` 全部通过；本机启动主面板做了一次视觉抽查（截图确认渲染正常,没有键盘聚焦相关的明显问题,不过本轮没有用合成按键去实际验证输入响应速度有没有受影响——这个改动理论上不会,因为原来那次立即调用完全没动)。
- **未能完成的验证**：这是目前几轮里推理链最完整、也最有信心的一版，但仍然只是推理，没能在问题设备上实测。需要用户装上这次构建，重点验证两件事：1）快捷键唤出后立刻悬浮多张卡片，反复试几次，确认深色闪烁是否真的消失了；2）唤出后立刻打字（不等它完全静止），确认搜索框依旧能立刻接收键盘输入、方向键选卡片依旧正常——这个改动没有动"立即聚焦"那次调用，理论上不受影响，但既然是新加的一次额外聚焦调用，还是需要实测确认没有引入新的、比如"刚打的头几个字被吞掉"这类边缘情况。如果深色闪烁这次真的解决了，说明根因确实和"窗口挪动中被激活"的并发有关；如果没解决，说明这个并发假说也不成立，需要往其他方向（比如彻底放弃原生 Acrylic 背景，见上一轮末尾提到的更大取舍）继续排查。

## 本轮完成（2026-08-24 八度深夜：卡片悬浮深色闪烁——查明是 WebView2 上游未解决问题，决定暂不动代码）

前几轮（"整个面板变灰"→"only 卡片变深色"→"only 唤出后偶发"→"和其他软件的悬浮高亮互斥"）的每一版推测性修复用户在问题设备上试了都没能解决，用户建议不要再靠猜代码，改成检索有没有类似案例。这轮改用网络检索代替继续猜 CSS/时序,查到了实质性的证据,但最终决定**暂不改代码**。

- [x] **检索到的关键证据（均为公开、可复核的 issue，非本项目内部推测）**：
  - Wails 官方仓库 issue #2340《Windows option WebviewIsTransparent has undesired effects on backdrop-filter blur CSS rule》——`WebviewIsTransparent=true` 时 `backdrop-filter` 元素渲染错误，Wails 维护者标记为 **upstream issue**（问题在 WebView2 本身，不是 Wails 代码可以修的）。
  - Microsoft 官方 WebView2Feedback 仓库 issue #2419《WebView2 cannot actually be transparent.》——窗口被移动/缩放后，WebView2 会把移动前的旧画面当成一张**静态图片**展示,而不是保持真正透明（原话："the webview2 use the old background as an image after parent window resized"）。
  - 这条和 SailBoard 的场景吻合度很高：`slide_windows.go` 的 `SlideReveal` 每次唤出都要用 `SetWindowPos` 每 10ms 挪一次整个窗口做滑入动效——"窗口被频繁移动"正是触发上面这个"卡在旧画面"问题的确切条件；卡片 hover 的 `transform` 很可能只是恰好第一个去"读取"这个可能已经错乱的透明合成结果的动作，真正的病灶在窗口刚滑动完、WebView2 的透明渲染表面没能正确刷新。
  - 一并排查过另一个可能方向——Wails 仓库 issue #5705《WebView2 controller permanently breaks on cross-GPU monitor switch (dual-GPU laptop)》，双显卡笔记本切换 GPU 导致 WebView2 渲染表面失效——**已向用户确认问题设备只有一块显卡，排除这条**。
- [x] **决定：暂不改代码**。前几轮"整个面板变灰"到"卡片偶发变深"这一路问题，本质上很可能就是 `WebviewIsTransparent`（配合原生 Acrylic 背景实现真·透明毛玻璃）这个组合在 WebView2 上游层面本来就不稳，且这个不稳定性和"窗口被物理移动"（SlideReveal 的滑动动效）绑定——目前没有已知的、能在应用层可靠修复的办法。三条可能的方向都要动到已经反复打磨、之前专门讨论过取舍的核心设计（唤出动效 / 真透明毛玻璃观感），用户明确要求"不要做过激的修复,导致原本正常的设备出现问题"——这几轮的教训是,在没有问题设备可以实测的情况下,每一次"看起来合理"的修复都有实际引入新问题、或者对现在完全正常的大多数设备产生副作用的风险,不值得为了一台设备去冒这个险。当前决定：**保留现状**（不撤销之前几轮已经落地的改动——去掉 `.shell` CSS 模糊层、`.card` 的 `will-change`、`ShowWindow` 里滑动结束后多补的一次 `FocusSelf`——这几处都是无害的小改动，没有证据表明它们导致任何回归，只是没能证实解决了这个问题；也不再新增改动），记录为已知限制。
- [x] **已知限制新增一条**（见下方"已知限制"小节）：极少数 Windows 设备上，快捷键唤出面板后偶发卡片悬浮背景变深色（带展开动画），切换到设置窗口再切回来会暂时恢复；开发机和目前测试过的大多数设备上无法复现；根因大概率是 WebView2 "`WebviewIsTransparent` + 窗口被移动"这个组合在上游未修复的渲染稳定性问题（详见上方 issue 链接），不是本项目代码层面能可靠修复的问题；不建议为了这个低频问题牺牲核心的原生毛玻璃透明效果或唤出滑动动效。

## 本轮完成（2026-08-24 晚：macOS 空格键 Quick Look 预览 + 卡片点击交互调整）

用户明确要求在"功能已冻结"（见顶部）之后新增这一项，仅限 macOS 版本，不影响 Windows。选中卡片后按空格，调用系统原生 Quick Look 面板预览图片/文件——而不是自绘一个预览 UI。

- [x] `internal/platform/types.go`：`Controller` 接口新增 `PreviewFile(paths []string) bool`——传入待预览的文件路径（多文件一起传入，面板内部自带左右切换），已可见则关闭（镜像系统级空格键的开/关行为），返回是否处于展示状态。
- [x] `internal/platform/quicklook_darwin.{go,h,m}`（新文件，随 darwin 全部方法一样单独成文件，模式抄 `clipboard_files_darwin.go`/`file_thumbnail_darwin.go`）：用 `QLPreviewPanel`（`#import <Quartz/Quartz.h>`，链接 `-framework Quartz`）实现真正的系统 Quick Look 面板；`SBQuickLookDataSource`（`QLPreviewPanelDataSource`）持有一个 `static` 的 `NSArray<NSURL *>`，`alloc/init` 出来后同样存成 `static` 复用（这个是 `alloc`/`init` 已经隐式 retain，不属于 `tray_darwin.m` 那条"便利构造器必须显式 retain"的坑，但复用同一个实例避免每次 toggle 都重新绑定 dataSource 身份）。**踩了一个新坑**：`main.go`/`AppDelegate.m` 把主窗口固定在 `NSFloatingWindowLevel`（`AlwaysOnTop`），Quick Look 面板默认层级比这个低，不特殊处理会被主窗口挡住看不见——`sb_quicklook_toggle` 里显式把 `panel.level` 设到 `NSFloatingWindowLevel + 1`。
- [x] `internal/platform/controller_defaults.go`（`stubController`，非 darwin 通用）、`controller_windows.go`（`windowsController`，Windows 不走 stub，需要单独补一个方法）都加了恒返回 `false` 的空实现——Windows 侧没有 Quick Look 这类系统级预览面板，前端调用这个方法在 Windows 上就是纯粹的 no-op。
- [x] `app.go` 新增 `App.PreviewSelection(id string) (bool, error)`：按 `item.Type` 取待预览路径。`ContentImage`/`ContentFile` 直接用已有的磁盘文件（前者单个 PNG 路径，后者按 `"\n"` 拆成多个）。**用户追加需求**："文本、链接等类型，也接入空格预览"——`ContentText`/`ContentColor`/`ContentURL` 这三类本身没有对应的磁盘文件，新增 `quicklookTextFile`/`quicklookWeblocFile` 两个helper，把 `item.Text` 现写一份临时文件再喂给 Quick Look：文本/颜色写成 `.txt`（走系统内置纯文本预览器），链接写成 `.webloc`（macOS 书签文件的标准 plist 格式，系统自带预览器会显示 favicon+标题，比纯文本更贴近"链接预览"）。两者都以 `item.Hash`（capture 时已经算好的去重哈希）作文件名，写到 `dataDir/quicklook/` 下，天然内容寻址、重复预览同一项不会重复写盘——抄的是 `diskImageStore` 已有的思路。
- [x] `frontend/src/App.tsx`：keydown 里新增空格分支（放在 input 判断之后、"任意可打印字符转发到搜索框"兜底分支之前——否则空格会先被当成"输入搜索关键字"截走），调用新增的 `previewItem`（catch 静默忽略，覆盖"该类型不可预览"和"Windows 上恒 false"两种正常场景，现在有了上面的 webloc/txt 兜底，实际上五种卡片类型全部可预览）；hints 提示条新增"Space 预览"，用 `navigator.platform`/`userAgent` 判断 `isMac` 做门控，避免 Windows 用户看到一条实际不生效的提示。
- [x] **用户追加需求**："左键点击的时候，应该先选中，而不是直接粘贴，选中的再次点击，才是粘贴"：卡片原来的 `onClick` 是点一下就直接粘贴，改成 `onCardClick`——第一版加了独立的 `armedId` state（因为卡片当时还有 `onMouseMove` 悬停联动 `selected`，不能直接拿 `selected === index` 当"已确认选中"用，否则悬停就等于选中，第一次点击会立刻命中并粘贴）。
- [x] **用户又追加需求**："现在的点击策略和选中跟随鼠标悬浮冲突了。改成取消跟随鼠标悬浮，只有当鼠标点击了，才会跳到对应的选择，保留左右选择优先"：直接删掉卡片的 `onMouseMove={() => setSelected(index)}`（连带清掉 `App.tsx` 里解释这个 hover 行为的两段旧注释），selection 现在只由 ←/→、数字键、和点击驱动。既然 hover 不再动 `selected` 了，上一条的 `armedId` 独立状态也跟着一起删掉、简化回 `selected === index ? 粘贴 : 选中`——两步点击的语义没变，只是判断条件从"armed 标记"退回成"就是当前选中项"，因为现在这两者已经等价了。键盘的 Enter/数字键 1-9 直接粘贴、←/→ 移动选中，全程没碰。
- [x] `frontend/wailsjs/go/main/App.{d.ts,js}`：一开始手工补了 `PreviewSelection` 绑定（当时开发环境没有 `wails` CLI 在 `PATH` 里），后来发现 `~/go/bin/wails` 其实是装了的，跑了一次真正的 `wails build` 让它自动重新生成——生成结果和手工补的那版完全一致，验证了手工补得没错；这两个文件本来就不纳入版本控制，以后每次 `wails build`/`wails generate module` 都会正常覆盖，不需要再手动维护。
- [x] `go build ./...`（darwin 原生）、`go vet ./...`（darwin 干净）、`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build/vet ./...`（Windows 交叉编译通过，`go vet` 仍是 CLAUDE.md 记录的那 8 处已知 unsafe.Pointer 误报，数量没变）、`npm run build`（前端 tsc + vite）全部通过；`go test ./...` 里 `TestParseFilesHashIsOrderIndependent` 失败——确认是改动前就存在的问题（测试硬编码了 Windows 路径 `C:\...`，在 macOS 上跑必然失败），跟这次改动无关。另外用 `export PATH="$PATH:$(go env GOPATH)/bin"` 找到本机已装的 `wails` CLI，跑通了两次完整的 `CGO_LDFLAGS="-framework UniformTypeIdentifiers" wails build -platform darwin/arm64 -clean`（一次在加文本/链接预览、点击改两步之前，一次在之后），产物 `build/bin/SailBoard.app` 都编译打包成功。
- [x] `build/darwin/package-dmg.sh` 用最新一次 `wails build` 产物打了一份 DMG（`build/bin/SailBoard-1.0.0-arm64.dmg`，约 6.7MB），脚本本身跑完没报错——但这只证明打包流程没坏，不代表 DMG 里的功能是好的。
- [x] **真机验证完成**：用户已在真实 Mac 上确认 Quick Look 面板正常弹出且层级盖住主面板、各类型卡片预览内容正确、两步点击确认手感正常。

## 本轮完成（2026-08-24：Windows 高 DPI 缩放下面板显示不全的 bug 修复）

功能已冻结（见顶部），这是 v1.0 发布后第一个用户反馈的 bug 修复：Windows 系统显示缩放设为 200% 时，面板唤出后纵向只显示一半。

- [x] **根因**：`app.go` 的 `panelHeight = 330` 是按 100% 缩放设计的 CSS/逻辑像素尺寸，但一直被原样传给 `platform.Controller.PositionSelf`/`SlideReveal`（底层是 `SetWindowPos`）。`build/windows/wails.exe.manifest` 声明了 Per-Monitor-V2 DPI awareness，`SetWindowPos`/`GetMonitorInfo` 收发的都是**物理像素**，WebView2 内部却仍按 CSS px 布局——200% 缩放下，330 物理像素的窗口只给 WebView2 留了 165 CSS px 的高度，App.css 按 330 CSS px 设计的内容因此被砍掉一半。之前完全没有对 `panelHeight` 做任何 DPI 换算，是本项目 Windows 侧一直没测过高分屏缩放场景暴露出的一个真实缺口，不是这次改动引入的新问题。
- [x] **修复**：`internal/platform/screen_windows.go` 的 `workAreaNearCursor()` 新增调用 `GetDpiForMonitor`（`shcore.dll`，`MDT_EFFECTIVE_DPI`），把鼠标所在显示器的 DPI 缩放比例（如 1.0/1.5/2.0）随工作区矩形一起返回；查询失败时兜底按 100% 处理，不影响原有区域查询这条主路径。`platform.Controller.WorkAreaNearCursor()` 签名相应改为 `(Rect, float64, bool)`（Windows/macOS/stub 三处实现同步更新——macOS 恒返回 `scale=1.0`，因为 AppKit 用点坐标系，天生与物理像素分辨率无关，不存在这个 bug）。`app.go` 的 `ShowWindow` 用换算后的 `h := int(math.Round(float64(panelHeight) * scale))` 计算窗口位置和尺寸，再传给 `SlideReveal`/`PositionSelf`——`panelHeight` 本身仍是 330 这个设计值不变，只是使用前多了一次按目标显示器缩放比例的换算。
- [x] `go build ./...`、`go vet ./...`（仍只有 CLAUDE.md 记录的那 8 处已知 unsafe.Pointer 误报）、`go test ./...` 全部通过；macOS 侧因为改动的 darwin 文件用了 cgo，Windows 机器交叉编译不了（`GOOS=darwin CGO_ENABLED=0` 这条惯用的检查手段只对不含 cgo 的 windows 侧代码有效），只能靠改动量小、签名与其余方法一致来把风险降到最低，等有 Mac 机器时建议按惯例跑一次 `wails build -platform darwin/arm64` 确认没有破坏 macOS 编译路径。
- [x] `wails build` 产物验证：`build/bin/SailBoard.exe` 编译通过。
- [x] **真机验证完成**：用户已在真实 200% 缩放的 Windows 设备上确认主面板唤出后完整显示，不再半高。

## 本轮完成（2026-08-23 更深夜：v1.0 发布收尾）

- [x] 项目状态明确为"功能已冻结"：在文档顶部新增"项目状态"一节，写明不再新增功能（尤其是联网同步类），往后只做 bug 修复/细节/平台补完
- [x] 记录 v1.0 GitHub Release 的两个下载产物（Windows exe / macOS dmg）及发布页链接
- [x] `README.md` 新增"📥 下载使用"一节（放在"功能一览"之后、构建说明之前），给出两个平台的直接下载链接 + macOS 首次运行放行/辅助功能授权提示；原"🛠️ 安装与使用"改名"🛠️ 从源码构建"并加一句导语，明确这一节是给开发者和"Releases 里没有对应平台包"的场景用的，不再是所有用户的默认路径
- [x] "后续建议"里关于"要不要签名再对外发布 macOS 版"的悬而未决项收口：明确当前选择是 ad-hoc 签名直接发，不等 Developer ID，把原本的"如果要正式分发"改写成"这是已经做出的选择，不是待办"

## 本轮完成（2026-08-23 深夜：macOS 补完到功能对等）

在上一轮"macOS 首次编译 + 窗口原生行为"基础上，把 `internal/platform.Controller` 剩下的方法全部实现完，macOS 从此和 Windows 功能对等。

- [x] **托盘图标改用透明背景版本**：`main.go` 新增 `//go:embed logo_tb.png`，`trayIconPNG` 按 `goruntime.GOOS` 在两个已 embed 的图之间选择——Windows 继续用 `logo.png`（不透明背景，任务栏托盘本身不透明，无所谓），macOS 用 `logo_tb.png`（透明背景），因为菜单栏其它图标都是透明的，方块背景在菜单栏里会很突兀
- [x] **macOS 版 Office 复制被误判成图片的 bug**（和 Windows 那次一模一样的问题）：`clipboard_richtext_darwin.go` + `.m`/`.h` 新增 `ReadClipboardRichText`/`WriteClipboardRichText`，读 `NSPasteboardTypeHTML`/`NSPasteboardTypeRTF` + `NSPasteboardTypeString`，用和 Windows 版一模一样的"非空纯文本 + 至少一种富文本格式同时存在才算真的格式化拷贝"判定逻辑。watcher.go 里 `ReadRichText` 本来就排在 `ReadImage` 前面检查（这个优先级顺序是跨平台共享代码，Windows 那次已经定下来了），macOS 只是终于把 `ReadClipboardRichText` 从 stub 填成真实现——不需要改 watcher 本身
- [x] **剪贴板文件/文件夹读写**（`clipboard_files_darwin.go` + `.m`/`.h`）：`ReadClipboardFiles` 读 `NSPasteboardURLReadingFileURLsOnlyKey` 限定的 `file://` URL（不用更宽松的 URL 读取选项，避免把网页链接当成文件）；`WriteClipboardFiles` 用 `NSURL fileURLWithPath:` + `[pb writeObjects:]`。实现后 `ReadClipboardImage` 之前担心的边界情况（Finder 复制一个图片文件被误判成内联图片）自然解决，因为 watcher 里 `ReadFiles` 排在 `ReadImage` 前面检查
- [x] **文件缩略图**（`file_thumbnail_darwin.go` + `.m`/`.h`）：可解码的图片格式（png/jpg/gif）用 Go 标准库解码+最近邻缩放出真实预览图，其余文件/文件夹用 `NSWorkspace.iconForFile:` 取系统图标——纯 Go 的解码/缩放逻辑是把 `file_thumbnail_windows.go` 里同名逻辑照抄了一份，没有抽成两平台共享代码（这次统一按之前的约定：宁可重复几十行，也不碰 `_windows.go` 文件）
- [x] **开机启动**（`autolaunch_darwin.go`，纯 Go 无需 cgo）：写/删 `~/Library/LaunchAgents/com.wails.SailBoard.plist`，`RunAtLoad=true`；`AutoLaunchEnabled` 和 Windows 版一样做"这个记录指向的可执行文件是否还存在"检查，而不是只看文件在不在
- [x] **单实例互斥**（`singleinstance_darwin.go`，纯 Go）：`flock()` 一个固定路径的锁文件（`~/Library/Application Support/SailBoard/singleinstance.lock`），非阻塞排他锁。**踩到一个纯 Go 层面的真 bug，值得记一笔**：第一版代码里，拿到锁之后那个 `*os.File` 只是函数局部变量，函数返回后就没有任何地方再引用它——`os.File` 会注册一个 runtime finalizer，在被 GC 回收时自动关闭底层 fd，而关闭 fd 就等于释放了 `flock`。用一个几秒钟就退出的独立小程序测试完全测不出来（活得不够久，GC 根本没机会跑），但真实的 SailBoard 主程序跑起来之后（`wails.Run()` 期间持续分配对象，必然触发多次真实 GC），锁在某次 GC 后就被悄悄释放了——现象是"明明第一个实例还活着，第二个实例却总能成功启动，变成两个进程同时运行"。定位过程：先怀疑 `flock` 本身在 macOS 上不可靠（写了个独立复现程序验证，结果证明 `flock` 完全正常），再对比"独立小程序里工作正常，真实 app 里不工作"，才想到 GC 时机的差异。修法是把拿到锁的 `*os.File` 存进一个包级变量长期持有，问题一次性解决，不再复发
- [x] **设置窗口跨进程通知**（`ipc_darwin.go` + `.m`/`.h`）：Windows 用隐藏消息窗口 + `PostMessage` 做跨进程通知，macOS 没有对应的窗口/消息循环可用（设置窗口是完全独立的进程），改用 `NSDistributedNotificationCenter`（系统级、按名字广播的进程间通知，标准 mac 方案）。`OnSettingsChanged`/`OnShowRequested` 注册监听，`NotifySettingsChanged`/`RequestShowMainWindow`（独立函数，非 Controller 方法，因为设置窗口进程没有 Controller）发广播。监听回调是在主线程上同步跑的原生回调，和热键/菜单栏点击是完全一样的性质，一样会踩中"回调栈里再起 goroutine 调 cgo 就崩"的坑，所以复用了同一条 `darwinMainThreadCallbacks` channel 解决，没有重新踩坑
- [x] **`FocusIfExists`**（`ipc_darwin.go` 的 `sb_focus_if_exists`）：避免重复打开第二个设置窗口。Windows 版能直接用系统级 `FindWindow` 按标题找任意进程的窗口，macOS 的 `[NSApp windows]`（`controller_darwin.m` 已有的 `sb_find_window`）只能看到*自己进程*的窗口——设置窗口是独立进程，看不见。改用 `CGWindowListCopyWindowInfo`（Quartz 系统级窗口列表，含其他进程的窗口）按标题找到窗口后取其 `kCGWindowOwnerPID`，再用 `NSRunningApplication activateWithOptions:` 激活那个进程
- [x] **DMG 加拖拽安装提示**：`build/darwin/package-dmg.sh` + `build/darwin/dmg-background.png`，用临时读写 DMG + AppleScript 摆放 Finder 图标视图（背景图、图标位置、隐藏工具栏/状态栏）再转成压缩只读 DMG，不依赖装不上的 `create-dmg`。踩了一个坑：背景图第一版做成 1200×800（按"@2x"想的），结果图标位置和背景图对不上——Finder 是把背景图缩放去适配窗口，不是当成 retina 资源按比例摆放，图片必须和窗口内容区（600×400pt）做成 1:1 尺寸，图标坐标才能跟背景图上画的位置对上
- [x] 全程用 `go vet ./...`、`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build`、真实 `wails build -platform darwin/arm64` 三件套在每一步之后复查；`go test ./...`/`gofmt -l .` 全过（同上一轮，只剩一个已知的、与本轮无关的 Windows 专属测试在非 Windows GOOS 下必然失败）

## 本轮完成（2026-08-23 晚间：macOS 首次编译 + 窗口原生行为）

这是本项目第一次实际在 macOS 机器（Apple Silicon）上编译运行，之前所有 macOS 相关内容都只是规划。

- [x] **`internal/platform` 拆分出 `controller_defaults.go`**（`!windows` 标签）：把原来 `controller_stub.go` 里的 `stubController` 类型和它的全部平面方法实现，从"只给非 Windows 平台用的 `New()`"里剥离出来，单独成一个不含 `New()` 的文件；`controller_stub.go` 收窄成 `!windows && !darwin`，只剩 `New()` 返回纯 stub。这样 macOS 的 `darwinController` 可以 `embed stubController` 继承全部还没实现的方法，只覆盖已实现的几个，不用把 30 个方法的空实现抄一遍——**这个拆分本身不改变任何行为，纯粹是为了让 macOS 能复用 stub 代码**，Windows 编译路径完全没碰
- [x] **macOS 窗口定位 + 滑动动效**（`controller_darwin.go` + `.m`/`.h`，cgo + Cocoa）：`WorkAreaNearCursor`（`NSScreen.visibleFrame` + `NSEvent.mouseLocation`，含 Cocoa 底左原点 → 本项目统一的左上原点坐标系换算）、`PositionSelf`（`NSWindow setFrame:display:`）、`FocusSelf`（`activateIgnoringOtherApps` + `makeKeyAndOrderFront`）、`SlideReveal`/`SlideDismiss`（Go 侧沿用与 `slide_windows.go` 完全相同的 ease-out 三次贝塞尔手工循环，只是每帧改调 Cocoa 而不是 `SetWindowPos`，未抽成两平台共享代码——明确是为了不去碰 Windows 那份文件，宁可留一点重复）。AppKit 调用必须在主线程执行，`sb_main_sync` 用 `[NSThread isMainThread]` 判断后按需 `dispatch_sync` 到主队列。真机验证：面板正确贴底、全宽度显示，不再是之前悬浮居中的问题面板
- [x] **失焦自动隐藏**（`WatchFocusLoss`）：没有用 Windows 那种 WinEventHook 式的按窗口过滤方案，而是轮询 `NSApplication.isActive`（200ms 一次，和剪贴板 watcher 的轮询粒度一个量级）——因为 macOS 上"切到 SailBoard 自己的设置窗口"和"切到别的 app"天然就能用 app 级 active 状态区分（前者应用本身仍是 active，后者才会变 inactive），不需要像 Windows 那样显式传入标题列表过滤，`titles` 参数在 macOS 实现里未使用。真机验证：点击别处后面板正确自动收起
- [x] **macOS 全局热键**（`hotkey_darwin.go` + `.m`/`.h`）：用 Carbon 的 `RegisterEventHotKey`（不需要辅助功能/输入监控权限，是第三方 mac 启动器类工具的标准做法），键位表（字母/数字/F1-F20/常用控制键）手工映射到 HIToolbox 的 `kVK_*` 虚拟键码（这些码和 ASCII 无算术关系，不像 Windows 能直接用字符值，只能查表）。修复默认快捷键：`internal/storage.DefaultSettings()` 保持平台无关（仍返回 `"Ctrl+Shift+V"`），只在 `app.go` 启动逻辑里加了一处 `goruntime.GOOS == "darwin"` 判断，**仅对全新安装**（`HasSettings()` 为 false 时）把默认值换成 `"Cmd+Shift+V"`，已保存过设置的老用户不受影响；这个判断在 Windows 上恒为 false，不改变 Windows 行为
  - **真机验证中蹦出一个真实 SIGSEGV 崩溃**，值得记一笔：全局热键触发 `a.ShowWindow()` 后必现闪退，但完全相同的 `ShowWindow()`/`SlideReveal` 代码从 app 启动 300ms 后的延时调用触发时从来没崩过。定位下来是**从 cgo 回调（Carbon 的热键事件处理函数，运行在主线程，同步调回 Go 的 `sbHotkeyFired`）内部再 `go func(){ ... SlideReveal ... }()` 起一个新 goroutine，这个新 goroutine 第一次发起的 cgo 调用（进 Cocoa/GCD）必现段错误**，而从普通（非 cgo 回调内部）的 goroutine 起同样的新 goroutine 调同样的代码完全正常——嵌套在 cgo 回调栈里再起 goroutine 调 cgo，是这次踩到的坑，不是 dispatch_sync 本身的问题。修法：把"从原生回调收到事件"和"跑 Go handler"解耦成 `darwinMainThreadCallbacks` 一个 buffered channel + 一个进程启动时就常驻、用普通 `go` 语句起的 goroutine 消费——这样 handler 永远从"正常"的 goroutine 出发，不再嵌套在原生回调栈里。后来实现菜单栏图标（AppKit 菜单项的 target-action 回调，同样是原生回调）时命中了完全一样的坑，复用了这同一条 channel 解决。真机验证：连续多次"隐藏 → 按热键 → 弹回"、"隐藏 → 点菜单栏图标 → 弹回"均不再崩溃
- [x] **菜单栏图标（替代程序坞）**：`tray_darwin.go` + `.m`/`.h`，`NSStatusBar` + `NSMenu`（"显示 SailBoard"/"暂停记录"/"退出"，文案与 Windows 托盘一致）；`NSMenuItem` 的 target-action 只能挂一个真正的 Objective-C 对象+selector，不能直接塞 Go 闭包，所以加了个极薄的 `SBTrayTarget` 类转发三个 selector 到 cgo-exported 的 Go 函数。踩了两个坑，都是真机验证才发现的：
  - **`LSUIElement=true` 光加进 `build/darwin/Info.plist` 没用**：Wails 自己的 `AppDelegate.m` 在 `applicationWillFinishLaunching` 里无条件 `[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular]`，比我们自己的启动代码跑得早，把 Info.plist 的设置直接覆盖掉——这是 Wails v2.10.2 的固定行为，`pkg/options/mac.Options` 里其实有个 `ActivationPolicy` 字段但整个被注释掉没启用，没有官方配置入口。只能自己在原生代码里再调一次 `setActivationPolicy:NSApplicationActivationPolicyAccessory` 把它改回去（`sb_set_activation_policy_accessory`，`internal/platform/controller_darwin.m`）。用 `lsappinfo list` 能直接看到 app 的 `type` 字段从 `Foreground` 变成 `UIElement`，这是比"肉眼看程序坞"更可靠的验证方式。且这个调用必须放在**创建菜单栏图标之后**（`ShowTray` 方法末尾，见 `darwinHideDockIcon`）——先切到 Accessory 策略再建图标，图标不会正确渲染，猜测是 AppKit 在 Accessory 状态下对新建状态栏项的处理路径不同，没有深挖，只是通过对照实验确认了这个顺序依赖
  - **图标建了但从来没渲染出来，跟上面那条其实是两个独立的 bug**：本项目的 Objective-C 代码全部没开 ARC（`-fobjc-arc`），而 `[NSStatusBar systemStatusBar] statusItemWithLength:]` 是"便利构造器"（不是 `alloc`/`new`/`copy` 前缀），按 Cocoa 手动引用计数的约定返回的是一个 autorelease 对象——直接赋值给 `static NSStatusItem *sStatusItem` 不会持有它，函数返回、`@autoreleasepool` 一退出这个对象就被销毁了，状态栏图标跟着消失。两个坑叠加在一起，一开始很容易误判成"是不是激活策略的顺序问题"（真机反复对照测试排除掉了），实际后来发现是这个更底层的引用计数 bug：给 `sStatusItem` 手动加一次 `retain` 就彻底解决了，跟激活策略顺序无关。这是"未开 ARC 的 Objective-C + 需要长期存活的对象存到 C 静态变量里"这一模式本身的通用陷阱，以后再往这几个 `_darwin.m` 文件里加类似"建一个对象、存起来复用"的代码时要留意
- 同时给 `build/darwin/Info.plist` 加了 `LSUIElement=true`（虽然被 Wails 覆盖，但保留着作为文档/以防未来 Wails 版本修复这个问题）：应用从此完全不出现在程序坞和 Cmd+Tab 里，只能通过热键或菜单栏图标唤出/操作
- [x] **中文剪贴板文本乱码修复**：Wails 自己在 macOS 上的剪贴板读写（`runtime.ClipboardGetText`/`ClipboardSetText`）是 shell 出 `pbpaste`/`pbcopy` 子进程实现的；而 `pbpaste` 的输出编码依赖 `LANG`/`LC_ALL`，一个从 Finder/程序坞正常启动的 GUI `.app` 不会像 Terminal 里的进程那样继承一份 shell 配置好的 locale 环境变量——实测在没有 `LANG` 的环境下 `pbpaste` 把中文剪贴板内容吐成了 GBK 而不是 UTF-8。修法是 `main.go` 的 `fixDarwinLocale()`：进程启动最早时机 `os.Setenv("LANG"/"LC_ALL", "en_US.UTF-8")`，`os/exec` 生成的子进程默认继承父进程环境，一次设置对 `pbpaste`/`pbcopy` 全程生效；仅在 `darwin` 上执行，Windows 不受影响
- [x] **剪贴板图片读写**（`clipboard_darwin.go` + `.m`/`.h`）：`ReadClipboardImage` 只读 `NSPasteboardTypeTIFF` 这个经典表示（几乎所有会把图片放上剪贴板的 mac 应用都会提供），特意不用更宽松的"从剪贴板读 NSImage"接口——后者连一个指向图片文件的 `file://` URL 也能读出图片来，会把 Finder 复制一个图片*文件*（该走文件引用逻辑）误判成内联图片字节；`WriteClipboardImage` 反过来把 PNG 转 TIFF 写回，选中图片卡片粘贴时不再报"未实现"错误（此前这个报错还有个连带 bug：`PasteItem` 遇到 `CopyItem` 报错会直接 return，导致面板压根不隐藏）。顺带实现了 `ClipboardSequence`（`NSPasteboard.changeCount`），watcher 现在能跳过没有变化的轮询 tick
- [x] **来源应用图标**（`activeapp_darwin.go` + `.m`/`.h`）：`NSWorkspace.frontmostApplication` 拿名称/bundle 路径，图标用 `NSImage drawInRect:` 手工画进一张 64×64 的画布再转 PNG（直接改 `.size` 属性只是改显示提示，不会真的重采样像素，取图标必须真画一遍）
- [x] **自动粘贴注入**（`foreground_darwin.go` + `.m`/`.h`）：`CaptureForeground`/`RestoreForeground` 用 `NSRunningApplication`（记住 pid，隐藏面板后 `activateWithOptions:` 拉回前台），`SendPaste` 用 `CGEventCreateKeyboardEvent` 模拟 Cmd+V 发到 HID 事件层。这条路径需要"辅助功能"系统权限，`AXIsProcessTrustedWithOptions` 检测；没有权限时第一次粘贴触发一次系统权限弹窗（`sync.Once` 只弹一次，不会每次粘贴失败都弹一遍去骚扰用户），同时返回 error 走既有的"已复制到剪贴板，请手动 Cmd+V"兜底提示（这条兜底路径本来就是给"SendPaste 失败"设计的，macOS 加入后完全复用，没改任何前端代码）
- [x] **"打开文件夹"按钮**（`folder_darwin.go`）：`exec.Command("open", path)`，纯子进程调用，和 Windows 的 `explorer path` 是同一个模式（自然也是从 `controller_defaults.go` 的共享 stub 里挪出来的，写法上跟 `New()` 当初的拆分一致）
- [x] **确认一个误会**：用户观察到系统弹出"本地网络权限"提示，怀疑是 SailBoard 请求了不该要的局域网访问。用 `lsof -p <pid> -i` 实测排查：那条到局域网设备的 TCP 连接实际属于 Claude Code CLI 自身进程，与 SailBoard 完全无关，SailBoard 的 `Info.plist`/`wails.json` 都没有任何网络监听/mDNS 相关配置——排除，不是本项目问题
- [x] 全程用 `go vet ./...` 和 `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build`（无需真实 Windows 机器，纯交叉编译类型检查）在每一次 macOS 侧改动后复查，确认 Windows 编译路径完全不受影响；`go test ./...` 除一个已知的、与本轮改动无关的 Windows 路径专属测试失败（`TestParseFilesHashIsOrderIndependent` 断言 `filepath.Base` 处理反斜杠路径，在非 Windows GOOS 下 `path/filepath` 不认反斜杠分隔符，是该测试本身只能在 Windows GOOS 下过，不是回归）外全部通过

## 本轮完成（2026-08-23）

- [x] 新增内容类型"颜色"：识别 `#hex`（3/4/6/8 位）与 `(R,G,B[,A])` 元组，兼容中英文括号/逗号、无括号纯数字逗号连接、前后空白/制表符；无法识别时正确回落为纯文本；卡片预览按实际色值渲染背景
- [x] 剪贴板内容合并逻辑修复：不同来源应用复制的相同内容合并为同一条历史时，同时刷新来源应用图标/名称与时间（此前只刷新时间，来源信息保持陈旧）；来源解析失败（空 `AppInfo`）时不清空已知的正确来源
- [x] 文件剪贴板锁审查：`writeClipboardFiles` 改为先准备好全部内存块（DROPFILES + Preferred DropEffect）再 `OpenClipboard`，缩短临界区
- [x] 唤出/关闭动效重做（三轮迭代，详见 git log）：
  1. CSS `clip-path` + 原生点击穿透实验——真机测试中先后遇到"窗口卡死无响应"和"背景不再消失"两个问题，判断是给一个已用 `SetWindowCompositionAttribute` 做 Acrylic、且内嵌 WebView2 的窗口动态挂载 `WS_EX_LAYERED` 导致的合成异常，放弃
  2. `AnimateWindow(AW_BLEND)` 原生渐隐——真机测试证实 `AW_BLEND` 淡出的是内容快照而非实时画面，`.sheet` 自身的 CSS 位移在快照期间被冻结、完全不可见，效果退化成纯渐隐，放弃
  3. **最终方案**：`SetWindowPos` 连续调用，让整个原生窗口（背景+内容）物理滑动，不涉及 alpha/layered 状态，DWM 按普通窗口拖动路径合成，天然流畅；`.sheet` 不再需要自己的 CSS transform。已确认真机效果符合预期
- [x] 修复方向键选择被"滚动到边界时鼠标悬停"劫持的 bug：卡片悬浮选中改用 `onMouseMove` 而非 `onMouseEnter`——键盘导航触发的 `scrollIntoView` 会让卡片在静止的鼠标下方滑过，Chromium 据此合成 `mouseenter`/`mouseleave` 以保持 `:hover` 状态一致，但 `mousemove` 只在指针真实移动时才触发，天然规避了这个问题
- [x] 面板顶部内边距按反馈缩小（7px → 4px）
- [x] README 品牌化重做：仿 SciSail/scholarShip 的居中 logo/标题/标语头部 + 截图区块排版；`screenshot.png` 换成实际面板截图（替换旧的整屏截图）
- [x] URL 预览补全 description 与预览图：`internal/webpreview` 新增 `ExtractDescription`（优先 `og:description`，回退 `<meta name="description">`）与 `ExtractImageURL`（`og:image`，相对路径按页面 URL 解析），纯字符串逻辑，单元测试覆盖属性顺序无关性与缺失场景；前端 `UrlCardContent` 在有预览图时改为图片封面 + 底部标题浮层布局（类似聊天软件的链接卡片），否则维持原有大 favicon + 标题布局，并在有 description 时于标题下方显示两行摘要。真机验证：GitHub 链接的封面图与标题浮层渲染正确
- [x] 图片粘贴写回补上 alpha 通道：`writeClipboardImage` 新增写入 `CF_DIBV5`（32bpp，显式 `bV5AlphaMask`），与原有 24bpp `CF_DIB` 一并写入剪贴板——纯 32bpp `CF_DIB` 没有字段能告诉接收方"第 4 字节是真 alpha 还是未用的 padding"，这正是 `CF_DIBV5` 存在的原因；顺带修正了两个 DIB 写入函数此前经 `.At(x,y).RGBA()`（返回预乘 alpha）读取像素、导致半透明像素 RGB 被错误变暗的问题，改为对 `image.NRGBA.Pix` 直接读取非预乘（straight）分量。已用临时手动测试直连剪贴板验证 `BitCount`/`AlphaMask`/逐像素 alpha 值均正确，随后按项目惯例删除该测试文件
- [x] Office 系列（Word/Excel/PowerPoint）带格式复制被误判为图片的问题修复：这些应用复制文字时会同时把纯文本、`HTML Format`/`Rich Text Format` 富文本标记、以及一张预览位图一起放上剪贴板，此前 watcher 按"文件 > 图片 > 文本"的优先级检查，图片检查在文本之前，于是每次都被当成图片捕获，实际的格式化文字反而丢了。新增 `platform.Controller.ReadClipboardRichText`：在同一次 `OpenClipboard` 内同时读取 `HTML Format`/`Rich Text Format`/`CF_UNICODETEXT`，只有"确有非空纯文本 + 至少一种富文本格式同时存在"才判定为真正的格式化文字拷贝（用来排除"浏览器复制图片时附带一个只包一个 `<img>` 标签的 HTML Format，但没有正文文本"这类假阳性）；watcher 里排在读文件之后、读图片之前检查。`clipboard.Item`/`RawContent` 新增 `HTML`/`RTF` 字段（仅附加在文本/URL/颜色类型上），数据库新增 `html_content`/`rtf_content` 列（`migrateRichText`，schema v3）。粘贴时（`CopyItem`）新增 `WriteClipboardRichText`：把 html/rtf 和纯文本一起写回剪贴板，具体粘贴到哪个应用、该应用用哪种格式，交给 Windows 剪贴板本身的多格式机制和接收方自己决定——不需要 SailBoard 检测"当前是不是 Office"。真机验证：测试过程中意外从真实 PowerPoint 复制到的内容（HTML 里带 `ProgId=PowerPoint.Slide`/`Generator=Microsoft PowerPoint 15` 标记，确认是真货而非构造数据）被正确识别为 `content_type=text` 而非 `image`，直接查库确认；写回路径用临时手动测试验证了 HTML/RTF/`CF_UNICODETEXT` 三种格式都能正确原样写回剪贴板，且"仅纯文本、无富文本格式"时能正确回落到 `ok=false`（不误判），随后按惯例删除测试文件。**真机验证完成**：用户已确认粘贴进 Word 后确实显示为加粗/带格式，写回路径的视觉效果得到人工确认

## 已完成（历史）

- [x] 初始化 Go + Wails + React + TypeScript 项目
- [x] 配置 Wails 窗口与 Windows 打包输出（`StartHidden`，由快捷键/托盘唤出）
- [x] SQLite 历史数据库与 schema 初始化，含版本化迁移（`user_version`）
- [x] 文本、HTTP(S) 链接、图片、文件/文件夹、颜色值的内容解析（优先级：文件 > 图片 > URL/颜色 > 文本）
- [x] SHA-256 内容哈希与 CRLF/LF 去重；文件按排序后路径集合哈希，与顺序无关
- [x] 重复内容更新使用时间、来源应用并重新置顶（跨来源应用合并）
- [x] 剪贴板轮询监听（250ms），基于序列号减少无效读取
- [x] 历史列表、实时搜索、删除与清空非收藏记录
- [x] 收藏/取消收藏；自动清理会保留收藏内容
- [x] 按保留天数和最大存储空间进行清理（文件类型历史项 `ByteSize` 恒为 0，不占用空间预算，只受保留天数约束；不触碰磁盘上的原文件）
- [x] 底部横向卡片 UI、搜索、键盘左右选择（含滚动到视口内）、鼠标悬浮选择（`onMouseMove`，不被程序化滚动劫持）、数字键 1-9 快速粘贴、Enter、两段式 Esc、鼠标滚轮映射为横向滚动
- [x] 设置界面（保留时间、空间限制、快捷键、开机启动、GitHub 链接、清空历史/恢复默认）
- [x] `internal/platform`：跨平台 Controller 接口 + Windows/macOS 完整原生实现 + 其他平台 stub
- [x] Windows 全局快捷键注册（`RegisterHotKey`，可在设置中自定义组合）
- [x] 窗口按鼠标所在屏幕的可用区域定位到底部（`MonitorFromPoint` + `GetMonitorInfo`）
- [x] 记录/恢复前一个前台窗口，选中后自动模拟 `Ctrl+V`（`SendInput`），失败时回退为"已复制到剪贴板"提示
- [x] Windows 图片剪贴板读取（CF_DIB → PNG）、图片落盘（按 hash 去重）、卡片内图片预览
- [x] 图片历史项粘贴时写回系统剪贴板（`CF_DIBV5` 32bpp 含 alpha + `CF_DIB` 24bpp 兜底，双格式同时写入）
- [x] Windows 文件/文件夹剪贴板：`CF_HDROP` 读取（`DragQueryFileW`）、写回时附带 `Preferred DropEffect=Copy`，粘贴产生真实复制而非移动；仅记录路径引用，不复制文件字节
- [x] 来源应用识别（进程名、路径）与小图标提取，缓存后内联展示在卡片上；跨应用复制相同内容会合并并刷新来源
- [x] 系统托盘图标（显示 / 暂停/恢复 / 退出），Windows: Shell_NotifyIcon 原生实现；macOS: NSStatusBar 菜单栏图标
- [x] 暂停记录（运行时开关，托盘与工具栏均可切换）
- [x] 开机启动（Windows：`HKCU\...\Run` 注册表项；macOS：`~/Library/LaunchAgents` LaunchAgent plist），跟随设置实时同步
- [x] URL 异步预览（标题 + favicon + description + 预览图，3s 超时，按需懒加载，不阻塞剪贴板监听）
- [x] Go 单元测试：哈希规范化、URL/颜色值判断、去重与置顶（含跨来源合并）、迁移幂等性、Capture 的图片/文件落盘与来源标记、DIB 解码往返、热键组合解析、URL 预览的纯文本解析（标题/favicon/description/预览图）
- [x] `go test ./...`、`go vet ./...`（仅剩四处已知安全的 unsafe.Pointer 误报，均在 Windows 代码里）、`gofmt -l .` 均通过
- [x] 前端生产构建通过
- [x] `wails build -clean` 通过，Windows 产物 `build/bin/SailBoard.exe`、macOS 产物 `build/bin/SailBoard.app`/DMG 均经真实进程启动验证
- [x] UI 视觉重做：iOS 毛玻璃风格、亮色系、半透明（Windows 原生 Acrylic backdrop + CSS 双层玻璃），全屏宽度、从屏幕底部滑出的面板（而非悬浮窗）
- [x] 动效打磨：面板从底部物理滑入/滑出（原生窗口位置调用，非 CSS）、卡片入场交错淡入、删除时的收缩淡出动画、分段选项卡滑动指示器、卡片悬浮/按下的轻量反馈
- [x] 修复多个真实原生层 bug（均通过真机截图/日志 + 模拟按键回归发现，而非代码审查）：
  - Windows：`MonitorFromPoint` 按值传递 `POINT` 结构体时错误地拆成两个独立参数，导致显示器定位在多显示器/非主屏场景下计算错误
  - Windows：隐藏窗口在后台线程中被 Wails 唤起时无法真正抢到键盘焦点；已通过 `AttachThreadInput` workaround（`platform.Controller.FocusSelf`）修复
  - Windows：键盘处理逻辑中 `Esc` 被输入框早退分支意外吞掉；已重构为 `Esc` 始终优先处理
  - Windows：`runtime.WindowSetPosition` 在 Wails Windows 后端里是"相对当前所在屏幕工作区原点的偏移"而非绝对屏幕坐标；已改为 `platform.Controller.PositionSelf` 直接调用 `SetWindowPos`
  - Windows：唤出/关闭动效的两轮失败实验及最终物理滑动方案
  - Windows：方向键选择被程序化滚动下的合成 `mouseenter` 劫持
  - macOS：全局热键/菜单栏点击触发的原生回调里直接跑 Go handler 导致 SIGSEGV（见上方"本轮完成"）
  - macOS：`pbpaste` 在没有 `LANG`/`LC_ALL` 的 GUI 进程环境下把中文剪贴板内容吐成 GBK 而非 UTF-8

## macOS 实现状态

`internal/platform.Controller` 的全部方法（窗口定位/滑动动效/失焦隐藏/全局热键/菜单栏图标/剪贴板文本-图片-富文本-文件读写/来源应用图标/文件缩略图/自动粘贴注入/开机启动/单实例互斥/设置窗口跨进程通知）在 macOS 上都已是真实原生实现，不再有任何 stub 方法——见上方两轮"本轮完成"。

每一项都是在真实 Mac 机器上用 `wails build` 产物实际启动验证过的——这是本项目一贯的验证标准（见 `CLAUDE.md`），不能靠"语法看起来对"就判定完成；这两轮里踩到过一个只有真机才能复现的 SIGSEGV、一个只有真机 GUI 启动方式才能复现的编码 bug（`pbpaste`/`LANG`）、一个只有真实长时间运行才会触发 GC 从而暴露的单实例锁失效 bug——全部是静态看代码完全看不出问题、必须真跑起来才会暴露的那类。

## 已知限制

- 来源应用图标：Windows 用 `GetDIBits` 读取，部分旧版无 alpha 通道的图标按不透明处理；macOS 用 `NSWorkspace.frontmostApplication.icon` 手工画 64×64 PNG。
- 图片写回剪贴板：Windows 同时写 `CF_DIBV5`（含真实 alpha）与 `CF_DIB`（24bpp 兜底），能否看到透明效果取决于接收方是否检查 `CF_DIBV5`，这是兜底机制本身的设计取舍，不是 bug；macOS 写 `NSPasteboardTypeTIFF`（TIFF 本身支持 alpha，未见类似兜底需求）。
- 全局快捷键、托盘、来源应用识别、图片/文件剪贴板等原生代码（Windows 的 Win32 部分和 macOS 的 Cocoa/Carbon 部分）都未做自动化测试（需要真实桌面/主线程 run loop 会话），仅有纯逻辑部分（热键解析、DIB/图标编解码、URL 预览解析）被单元测试覆盖；均已通过 `wails build` 产物的多次真实进程启动验证没有崩溃或运行时报错。
- **未签名的本地 dev 构建每次重新编译都要重新授予一次"辅助功能"权限**：`wails build` 产物是 ad-hoc 签名（没有 Apple Developer 证书做正式代码签名），系统按签名身份记录权限授予，二进制一变签名就变，TCC 会把新构建当成"新 app"——上一次构建授予的辅助功能权限对新构建不生效，需要在系统设置里重新勾选。等有正式 Developer ID 签名 + 公证后这个问题会消失（同一份证书签出来的版本身份稳定）。
- **RDP 会话下跨应用切换焦点不可靠**（Windows）：`AppActivate`/`SendKeys` 之类的自动化窗口切换在测试中未能稳定把键盘输入送到目标应用，推测是 Windows 前台窗口锁的限制（SailBoard 自己的 `FocusSelf` 也遇到过同类问题，用 `AttachThreadInput` 绕过）；这是自动化工具在这类远程会话下的已知局限，不代表被测代码有问题。
- **极少数 Windows 设备上，快捷键唤出主面板后偶发卡片悬浮背景变深色**（带展开动画），切换到设置窗口再切回来会暂时恢复，下次唤出又会复现；开发机及目前测试过的大多数设备上无法复现。查过多轮（详见上方几轮"本轮完成"记录）：先后怀疑过 CSS `backdrop-filter` 与原生 Acrylic 背景叠加、卡片 hover 触发的 GPU 合成图层新建、唤出滑动动效与抢焦点的并发时序，均在问题设备上验证无效；改用检索代替继续猜测后，查到 Wails 官方 issue [#2340](https://github.com/wailsapp/wails/issues/2340) 和 Microsoft WebView2Feedback [#2419](https://github.com/MicrosoftEdge/WebView2Feedback/issues/2419) 两个已被标记为 **upstream（问题出在 WebView2 本身）** 的公开报告：`WebviewIsTransparent=true` 时窗口被移动/缩放后，WebView2 会把移动前的旧画面当静态图片展示而非保持真正透明——`slide_windows.go` 的 `SlideReveal` 每次唤出都用 `SetWindowPos` 高频挪动整个窗口做滑入动效，与触发条件吻合。已排除双显卡切换（Wails issue [#5705](https://github.com/wailsapp/wails/issues/5705)）这一相邻方向——问题设备确认只有一块显卡。三条可能的修复方向（放弃滑入动效改硬切换、放弃真透明毛玻璃改纯 CSS 模拟、继续在应用层打补丁绕过一个未解决的上游 bug）都要动到已经反复打磨过的核心设计，用户明确要求不要为了少数设备的低频问题冒着影响大多数正常设备的风险去"过激修复"，因此决定暂不改动，记录为已知限制，留待以后 WebView2/Wails 上游修复，或有更多问题设备样本、更有把握的方案时再处理。

## 关键文件

- `app.go`：Wails 生命周期、前端 API、原生能力编排（热键/托盘/暂停/图片/文件/URL 预览/唤出关闭动效）
- `main.go`：Wails 窗口配置、`fixDarwinLocale`（macOS 剪贴板中文乱码修复）、`trayIconPNG`（Windows/macOS 各自的托盘图标选择）
- `internal/clipboard/`：内容解析（文本/URL/图片/文件/颜色）、监听、哈希、去重、图片落盘服务
- `internal/storage/`：SQLite 仓储、版本化迁移、设置和清理
- `internal/platform/`：跨平台 Controller 接口，Windows/macOS 均已完整原生实现
  - `*_windows.go`：Win32 实现（含 `slide_windows.go` 的窗口滑动动效、`clipboard_richtext_windows.go` 的 Office 富文本读写、`singleinstance_windows.go`/`ipc_windows.go` 的单实例与跨进程通知）
  - `*_darwin.go`/`.m`/`.h`：macOS 实现（Cocoa/Carbon via cgo）——`controller_darwin.*`（窗口定位/滑动动效/失焦隐藏/`darwinMainThreadCallbacks` 回调调度/Dock 图标隐藏）、`hotkey_darwin.*`（Carbon 全局热键）、`tray_darwin.*`（菜单栏图标）、`clipboard_darwin.*`（图片读写/序列号）、`clipboard_richtext_darwin.*`（HTML/RTF 富文本读写）、`clipboard_files_darwin.*`（文件/文件夹读写）、`file_thumbnail_darwin.*`（文件缩略图）、`activeapp_darwin.*`（来源应用图标）、`foreground_darwin.*`（自动粘贴注入）、`folder_darwin.go`（打开文件夹）、`autolaunch_darwin.go`（LaunchAgent 开机启动）、`singleinstance_darwin.go`（flock 单实例互斥）、`ipc_darwin.*`（`NSDistributedNotificationCenter` 跨进程通知 + `FocusIfExists`）
  - `controller_defaults.go`（`stubController` 类型本体）+ `controller_stub.go`（`!windows && !darwin` 的 `New()`）：Windows/macOS 之外其他平台的占位
- `internal/webpreview/`：URL 标题/favicon/description/预览图抓取（纯逻辑可测试部分与网络抓取分离）
- `frontend/src/`：卡片界面（文本/URL/图片/文件/颜色五种卡片组件）、设置界面
- `build/darwin/package-dmg.sh` + `dmg-background.png`：macOS DMG 打包（拖拽安装背景图提示）
- `README.md`：品牌化后的项目说明，含 logo 与截图、下载使用（v1.0 Release 直接下载）、Windows/macOS 从源码构建步骤
- `SailBoard_DESIGN.md`：完整设计规格

## 后续建议

功能已冻结（见顶部"项目状态"），v1.0 已用 ad-hoc 签名的形式在 GitHub Releases 发布——即先不做正式 Apple Developer ID 签名/公证，用户下载后手动放行一次即可，接受 Gatekeeper 首次运行提示这个小摩擦，换来不用等代付费证书；这是当前阶段的既定选择，不是待办。之后如果这个摩擦真的成为问题（比如用户反馈多了）再考虑补签名，不主动排期。

剩下这条与"新功能"无关，不改变功能范围，且不是代码能解决的问题，暂不安排：

1. 每次重新构建（未签名）都要求 macOS 用户在系统设置里重新授予一次"辅助功能"权限——ad-hoc 签名的代价（签名身份随每次编译变化，TCC 把新构建当成"新 app"），等真需要正式 Developer ID 签名时一并解决（见上）。

（原先这里的第二条——macOS 设置界面 `Cmd`/`Win` 修饰键标签文案——已在上方"本轮完成"里修好。）

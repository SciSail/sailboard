# SailBoard 开发进度

最后更新：2026-08-30

SailBoard 是 Windows + macOS 双平台项目，不考虑 Linux。

## 项目状态：功能已冻结，v1.1 修复版

所有计划内功能均已实现，Windows/macOS 两端功能对等，**不再新增功能**——尤其是不会做联网同步/云端/账号体系类功能，这是设计之初就有的边界（design doc §45）。SailBoard 是纯本地单机工具，往后的工作只是 bug 修复、细节打磨，不会往"云同步剪贴板"之类的方向发展。

v1.1 在 v1.0 基础上修复剪贴板监听阻塞、图片读取兼容性、富文本本地图片资源和历史内容配额统计问题。

v1.1 预编译产物：

| 平台 | 文件 |
| --- | --- |
| Windows (x86-64) | `SailBoard_v1.1_Windows_x86.exe` |
| macOS (Apple Silicon) | `SailBoard_v1.1_Mac_arm64.dmg` |

发布页：https://github.com/SciSail/sailboard/releases/tag/v1.1 。两个产物都是 ad-hoc/未做正式代码签名的构建（见"已知限制"），下载后需要各自平台的一次性放行操作，`README.md` 的"下载使用"一节有写明。

## 当前状态

**Windows**：全局快捷键唤出（含 Alt+Space 支持——原生层能识别出用户按下的是 Alt+Space 并直接作为快捷键提交，同时提示"会接管系统所有窗口的 Alt+Space 系统菜单"这个代价；Ctrl+C/V/X/Z/Y/A/S/F/N/O/P/W/Tab、Alt+Tab、Alt+F4 等常用快捷键会被拒绝设置）、窗口按 DPI 缩放正确定位与显示、选择粘贴（含自动按键注入）、图片剪贴板（含 alpha 通道）、文件/文件夹剪贴板、颜色值识别、富文本（Office 系列带格式复制）、URL 预览（标题/favicon/描述/预览图）、来源应用识别与合并、系统托盘、暂停记录、开机启动、单实例互斥。独立的设置窗口进程已修复：Alt/F10 单独按下不再弹出系统菜单打断快捷键捕获、200% 等非 100% 缩放下窗口正确适配不出滚动条、`AlwaysOnTop` 不会被主面板遮挡、捕获新快捷键期间旧快捷键会临时暂停（不会误触发主面板）。UI 为 iOS 风格亮色毛玻璃、全屏宽度底部弹出面板，唤出/关闭动效是原生窗口物理滑动（非 CSS）。

**macOS**：`internal/platform.Controller` 接口全部方法都是真实原生实现（Cocoa/Carbon via cgo），与 Windows 功能对等，含窗口定位/滑动动效/失焦自动隐藏、全局热键（默认 `Cmd+Shift+V`）、菜单栏图标（不出现在程序坞，含多开设置窗口时图标去重）、剪贴板图片/富文本/文件读写、来源应用图标、文件缩略图、自动粘贴注入（辅助功能权限提示挪到设置窗口主动检查，不必等粘贴失败；授权后无需重启即生效）、开机启动、单实例互斥、设置窗口跨进程通知（含捕获快捷键时暂停旧热键，与 Windows 对等）、空格键 Quick Look 预览（图片/文件预览原文件，文本/链接/颜色临时生成 `.txt`/`.webloc` 交给系统预览器）。设置界面里 Cmd 键正确显示为"Cmd"标签而非 Windows 的"Win"。主面板与设置窗口互相切换时不会再彼此"消失"（`WatchFocusLoss` 改用 bundle identifier 比较，而非单纯的 app-active 状态）。

卡片交互：←/→ 选择、Enter 粘贴当前选中项，鼠标点击先选中、再次点击已选中的卡片才粘贴（避免误触，selection 不再跟随鼠标悬浮），Delete 删除，两段式 Esc；数字键不再是快捷选择键，和普通字符一样落入搜索输入。

## 已完成功能

- Go + Wails v2 + React/TypeScript，SQLite 历史数据库（版本化 `user_version` migration）
- 内容解析：文本、HTTP(S) 链接、图片、文件/文件夹、颜色值（`#hex`/`rgb(a)`），优先级 文件 > 图片 > URL/颜色 > 文本
- SHA-256 内容哈希去重（文本 CRLF/LF 规范化，文件按排序路径集合哈希）；重复内容合并时刷新来源应用与时间并重新置顶
- 剪贴板监听：Windows 使用 `WM_CLIPBOARDUPDATE` 事件通知，连续 600ms 无变化后才读取，忙碌时从 600ms 开始低频退避；`CF_HDROP` 只在锁内复制原始块并在关闭剪贴板后解析，后台快照不接管 Ctrl+C/V，2s 安全轮询也经过相同静默窗口，macOS 保留轮询回退
- 历史列表、实时搜索、收藏/取消收藏（收藏内容自动清理时豁免）、删除与清空非收藏记录
- 按保留天数和最大存储空间自动清理（空间统计只计算历史内容及其唯一引用资源，不把整个缓存目录计入）
- 底部横向卡片 UI、iOS 毛玻璃风格、全屏宽度底部滑出面板、卡片入场/退场动画
- 设置界面（保留时间、空间限制、快捷键自定义、开机启动、缓存目录、GitHub 链接、清空历史/恢复默认）
- Windows 全局热键（`RegisterHotKey`）、窗口按鼠标所在屏幕定位、自动模拟 `Ctrl+V` 粘贴（失败回退为"已复制到剪贴板"提示）
- 图片剪贴板读写（Windows `CF_DIB`/`CF_DIBV5` 含 alpha；macOS `NSPasteboardTypeTIFF`）
- 文件/文件夹剪贴板（Windows `CF_HDROP`；macOS `file://` URL），仅记录路径引用不复制字节
- 富文本剪贴板（Office 系列带格式复制），HTML/RTF 随文本/URL/颜色类型一起落库；仅将 `file://` 与 `data:` 本地图片外置为去重资源，远程图片不下载，粘贴时按需还原
- 来源应用识别与图标提取、系统托盘/菜单栏图标、暂停记录、开机启动
- URL 异步预览（标题 + favicon + description + 预览图，3s 超时，懒加载不阻塞剪贴板监听）
- macOS 空格键 Quick Look 预览
- Go 单元测试覆盖：哈希/去重、监听通知与忙碌重试、富文本图片外置/还原、资源引用与配额统计、迁移幂等性、DIB 解码、热键解析、URL 预览解析
- `wails build` 通过，Windows/macOS 产物均经真实进程启动验证

## 已知限制

- 来源应用图标：Windows 用 `GetDIBits` 读取，部分旧版无 alpha 通道的图标按不透明处理；macOS 用 `NSWorkspace.frontmostApplication.icon` 手工画 64×64 PNG。
- 富文本中的远程 `http(s)` 图片不会被 SailBoard 下载；如果来源应用只提供远程地址，粘贴时仍依赖接收方自行加载。
- 富文本只处理通用 HTML/RTF 剪贴板格式，不解析 Office/PPT 私有二进制格式；这属于有意保持的边界。
- 图片写回剪贴板：Windows 同时写 `CF_DIBV5`（含真实 alpha）与 `CF_DIB`（24bpp 兜底），能否看到透明效果取决于接收方是否检查 `CF_DIBV5`；macOS 写 `NSPasteboardTypeTIFF`。
- 全局快捷键、托盘、来源应用识别、真实系统剪贴板事件回调等原生代码未做自动化测试（需要真实桌面/主线程 run loop 会话）；监听器的通知/重试逻辑、DIB/图标编解码、富文本资源处理、热键解析、URL 预览解析等纯逻辑已有单元测试。Windows 已通过 `wails build` 产物的真实进程启动验证。
- **未签名的本地 dev 构建每次重新编译都要重新授予一次 macOS "辅助功能"权限**：ad-hoc 签名身份随每次编译变化，TCC 把新构建当成"新 app"；等有正式 Developer ID 签名 + 公证后会消失。
- **极少数 Windows 设备上，快捷键唤出主面板后偶发卡片悬浮背景闪一下深色**（带展开动画），切到设置窗口再切回来会暂时恢复，下次唤出又会复现；开发机及大多数测试过的设备上无法复现。根因追查到 Wails 官方 issue [#2340](https://github.com/wailsapp/wails/issues/2340) 和 Microsoft WebView2Feedback [#2419](https://github.com/MicrosoftEdge/WebView2Feedback/issues/2419) 两个已标记为 **upstream**（问题在 WebView2 本身）的公开报告：`WebviewIsTransparent=true` 时窗口被移动后，WebView2 会把移动前的旧画面当静态图片展示——`slide_windows.go` 的 `SlideReveal` 每次唤出都高频挪动窗口做滑入动效，与触发条件吻合。已排除双显卡切换（问题设备只有一块显卡）。可能的修复方向（放弃滑入动效改硬切换、放弃真透明毛玻璃改纯 CSS 模拟）都要动到已反复打磨过的核心设计，为少数设备的低频问题冒着影响大多数正常设备的风险不划算，暂不处理。

## 关键文件

- `app.go`：Wails 生命周期、前端 API、原生能力编排（热键/托盘/暂停/图片/文件/URL 预览/唤出关闭动效）
- `main.go`：Wails 窗口配置、`fixDarwinLocale`（macOS 剪贴板中文乱码修复）、`trayIconPNG`
- `settings_app.go`：独立设置窗口进程的 Wails 绑定 API
- `internal/clipboard/`：内容解析（文本/URL/图片/文件/颜色）、监听、哈希、去重、图片落盘服务
- `internal/storage/`：SQLite 仓储、版本化迁移、设置和清理
- `internal/platform/`：跨平台 Controller 接口，Windows/macOS 均已完整原生实现
  - `*_windows.go`：Win32 实现（`slide_windows.go` 窗口滑动动效、`clipboard_richtext_windows.go` Office 富文本、`settingswindow_windows.go` 设置窗口 Alt 键/DPI 修复、`singleinstance_windows.go`/`ipc_windows.go` 单实例与跨进程通知）
  - `*_darwin.go`/`.m`/`.h`：macOS 实现（Cocoa/Carbon via cgo）——`controller_darwin.*`（窗口定位/滑动/失焦隐藏/回调调度/Dock 图标隐藏）、`hotkey_darwin.*`（Carbon 全局热键）、`tray_darwin.*`（菜单栏图标）、`clipboard_darwin.*`/`clipboard_richtext_darwin.*`/`clipboard_files_darwin.*`（剪贴板读写）、`file_thumbnail_darwin.*`、`activeapp_darwin.*`、`foreground_darwin.*`（自动粘贴注入）、`quicklook_darwin.*`（Quick Look 预览）、`folder_darwin.go`、`autolaunch_darwin.go`、`singleinstance_darwin.go`、`ipc_darwin.*`（跨进程通知 + `FocusIfExists`）、`settingswindow_darwin.go`
  - `controller_defaults.go`/`controller_stub.go`：非 Windows/macOS 平台的占位
- `internal/webpreview/`：URL 标题/favicon/description/预览图抓取
- `frontend/src/`：卡片界面（文本/URL/图片/文件/颜色五种卡片组件）、设置界面
- `build/darwin/package-dmg.sh` + `dmg-background.png`：macOS DMG 打包
- `README.md`：项目说明，含下载使用、从源码构建步骤
- `CLAUDE.md`：架构与开发约定

## 后续建议

功能已冻结，v1.1 已用 ad-hoc 签名的形式在 GitHub Releases 发布——不等正式 Apple Developer ID 签名/公证，用户下载后手动放行一次即可；这是既定选择，不是待办，摩擦真的成为问题再考虑补签名。macOS 每次重新构建都要求用户重新授予一次"辅助功能"权限也是同一个签名取舍的代价，等真需要正式签名时一并解决。

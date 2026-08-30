<p align="center">
  <a href="https://github.com/SciSail/sailboard">
    <img src="logo.png" alt="SailBoard Logo" width="128">
  </a>
</p>

<h1 align="center">SailBoard</h1>

<p align="center">
  轻盈、iOS 风格的跨平台剪贴板历史工具。
  <br />
  无需注册、无需登录、没有多余的功能。
  <br />
  <strong>当前版本：v1.1（问题修复版）</strong>
  <br />
  <a href="https://github.com/SciSail/sailboard"><strong>查看 SailBoard 仓库 »</strong></a>
</p>

---

## 🚀 功能一览

- **全屏宽度毛玻璃面板**：iOS 风格亮色毛玻璃底部横向卡片栏，从屏幕底部滑出，而非悬浮小窗
- **全局快捷键唤出**：默认 `Ctrl+Shift+V`（macOS 上为 `Cmd+Shift+V`，可自定义），窗口自动定位到鼠标所在屏幕底部
- **多种内容类型**：文本、HTTP(S) 链接（自动抓取标题 + favicon）、图片、文件/文件夹、颜色值（`#hex`、`rgb`/`rgba`，预览按实际色值渲染）
- **智能去重**：基于内容 SHA-256 去重，重复项目自动置顶并刷新来源应用与时间，不产生重复历史
- **来源应用识别**：卡片内联展示复制来源应用的图标
- **不打断系统剪贴板**：Windows 使用系统剪贴板变更事件监听，不模拟 `Ctrl+C`，捕获读取、图片解码和落盘在后台完成
- **富文本与图片修复**：支持通用 HTML/RTF；本地 `file://` / `data:` 图片按内容去重保存，粘贴时还原，远程图片不会被下载
- **快速操作**：←/→ 选择、Enter 粘贴当前选中项，鼠标点击先选中、再次点击已选中的卡片才粘贴（避免误触），Delete 删除，两段式 Esc（先清空搜索再关闭）
- **Quick Look 预览（仅 macOS）**：选中卡片后按空格，调用系统原生 Quick Look 面板预览——图片/文件直接预览原文件，文本/链接/颜色值会临时生成一份 `.txt`/`.webloc` 交给系统预览器，链接因此能看到 favicon + 标题而不是纯文本；Windows 上这个按键不做任何事
- **收藏与清理**：支持收藏、按保留天数与最大占用空间自动清理，收藏内容永不过期
- **系统集成**：托盘/菜单栏图标（macOS 上应用不出现在程序坞，纯菜单栏驻留）、选中后自动写回剪贴板并模拟粘贴到之前的前台应用（macOS 上需要在系统设置里授予一次"辅助功能"权限）、开机启动、只允许单实例运行

## 🆕 v1.1 更新

- 修复 Windows 监听剪贴板时可能阻塞系统复制/粘贴的问题：改为 `WM_CLIPBOARDUPDATE` 事件通知，读取失败自动重试，并保留低频安全轮询。
- 修复图片剪贴板读取兼容性：支持 PNG、`CF_DIBV5`、`CF_DIB`，改善透明通道、调色板和多种位深图片的处理。
- 富文本内容只保留通用 HTML/RTF，不解析 Office/PPT 私有二进制格式；本地图片资源去重存储，远程图片保持原地址且不触发网络下载。
- 存储空间统计改为历史内容及其引用资源，删除历史时同步清理孤儿资源；历史列表不再默认加载完整富文本载荷。

## 📥 下载使用

Windows 和 macOS（Apple Silicon）已有编译好的安装包，从 [Releases 页面](https://github.com/SciSail/sailboard/releases/latest) 下载即可，无需自行编译：

| 平台 | 下载 |
| --- | --- |
| Windows (x86-64) | [SailBoard_v1.1_Windows_x86.exe](https://github.com/SciSail/sailboard/releases/download/v1.1/SailBoard_v1.1_Windows_x86.exe) |
| macOS (Apple Silicon) | [SailBoard_v1.1_Mac_arm64.dmg](https://github.com/SciSail/sailboard/releases/download/v1.1/SailBoard_v1.1_Mac_arm64.dmg) |

macOS 首次打开需要在"系统设置 > 隐私与安全性"里允许运行（未签名 App 的标准提示），并在弹窗后授予一次"辅助功能"权限（自动粘贴注入需要）。

安装后按下 `Ctrl+Shift+V`（macOS 上 `Cmd+Shift+V`）唤出面板，选中卡片即可粘贴；快捷键可在设置里自定义。

其他平台（Windows x86 以外的架构、macOS Intel、Linux 等）目前没有现成安装包，需要按下方"从源码构建"自行编译。

## 🛠️ 从源码构建

面向开发者，或 Releases 里没有对应平台安装包的情况。需要 Go 1.23+、Node.js 18+、[Wails v2](https://wails.io/docs/gettingstarted/installation/)。

```bash
git clone https://github.com/SciSail/sailboard.git
cd sailboard
go test ./...
wails dev      # 桌面应用 + 前端热更新
wails build    # 打包，Windows 产物在 build/bin/SailBoard.exe
```

macOS 上（需先装 Xcode Command Line Tools）：

```bash
CGO_LDFLAGS="-framework UniformTypeIdentifiers" wails build -platform darwin/arm64 -clean
build/darwin/package-dmg.sh   # 产物 .app 打成 DMG，默认输出 build/bin/SailBoard-1.1.0-arm64.dmg
```

自动粘贴注入在 macOS 上需要在"系统设置 > 隐私与安全性 > 辅助功能"里手动授权一次（首次触发时应用会自动弹出对应的系统设置面板）。架构与实现细节见 `CLAUDE.md`。

---

## 📸 应用截图

<p align="center">
  <img src="screenshot.png" alt="SailBoard 剪贴板历史面板截图" width="90%">
</p>

---

## 项目结构

```text
app.go                          Wails 暴露的应用 API 与生命周期
internal/clipboard/             内容解析、哈希、事件/轮询监听、去重、富文本资源与图片落盘服务
internal/storage/                SQLite migration、仓储、设置与清理
internal/platform/               跨平台原生能力接口，Windows / macOS 双平台完整原生实现
internal/webpreview/             URL 标题与 favicon 抓取
frontend/src/                   React 底部卡片栏与设置界面
```

Wails API 已隔离 UI、剪贴板服务、SQLite 仓储层与 `internal/platform` 原生能力层，并含单元测试。

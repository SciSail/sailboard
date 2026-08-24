# SailBoard — 跨平台剪贴板历史工具设计与开发说明

> 项目名：**SailBoard**  
> 定位：一个轻量、快速、跨平台的剪贴板历史与快速粘贴工具。  
> 首发平台：**macOS + Windows**。  
> 技术原则：**Go 为核心、UI 尽量轻、优先完成完整闭环，再逐步优化细节。**  
> 参考交互：macOS Paste 类软件的底部横向卡片栏，但视觉与实现保持 SailBoard 自己的风格。

---

## 0. 给 Coding Agent 的总指令

请按本文档完成 SailBoard 的第一版实现。

开发时遵循以下优先级：

1. **先完成整个可运行框架，不要因为某个细节阻塞主流程。**
2. 首先完成：
   - 监听剪贴板
   - 保存历史
   - 去重
   - 快捷键唤出
   - 底部横向卡片 UI
   - 左右选择
   - 回车/点击后自动粘贴
   - 搜索
   - 收藏
   - 清理策略
3. 对于以下功能，如果跨平台实现存在困难，可以先用接口 + stub 占位：
   - 精确识别复制来源应用
   - URL 高级网页预览
   - Linux
   - 多设备同步
   - 云同步
4. **任何阶段都必须保证项目可以编译和运行。**
5. 每完成一个模块，都补充对应测试。
6. 不要过早引入复杂架构、插件系统、账号体系、云后端。
7. 第一版目标不是“功能最多”，而是：
   - 快
   - 稳
   - 搜索响应快
   - 快捷键唤出快
   - 自动粘贴可靠
   - 数据不会无限增长
8. 首发只考虑 macOS 与 Windows，但所有系统相关能力必须通过抽象接口隔离，方便以后加入 Linux。

---

# 1. 产品定义

## 1.1 核心目标

SailBoard 解决一个问题：

> 用户复制过的内容不应该因为下一次复制而消失。

用户通过全局快捷键唤出 SailBoard，在屏幕底部看到最近复制的内容，并可以快速查找、选择和再次粘贴。

默认快捷键：

- macOS：`Command + Shift + V`
- Windows：`Ctrl + Shift + V`

核心体验：

```text
复制 → SailBoard 自动记录
            ↓
快捷键唤出
            ↓
左右选择 / 滚动 / 搜索
            ↓
Enter / 点击
            ↓
写回系统剪贴板
            ↓
自动粘贴到刚才正在使用的应用
            ↓
SailBoard 自动隐藏
```

---

# 2. 第一版功能范围

## 2.1 必须实现

### A. 剪贴板监听

支持以下三类：

- 文本
- 链接
- 图片

每条记录至少保存：

- 内容
- 类型
- 创建时间
- 最近复制时间
- 来源应用
- 字符数 / 图片尺寸
- hash
- 是否收藏

---

### B. 剪贴板历史去重

不能因为用户重复复制同一内容而产生多个重复卡片。

规则：

```text
新内容复制
  ↓
计算标准化后的 hash
  ↓
数据库中不存在
  → 创建新记录
  → 放到最前面

数据库中已存在
  → 不新建
  → 更新 last_used_at
  → 将该卡片移到最前面
```

排序依据：

```text
ORDER BY last_used_at DESC
```

收藏不改变去重规则。

---

### C. 全局快捷键唤出

默认：

```text
macOS   Command + Shift + V
Windows Ctrl + Shift + V
```

要求：

- 无论当前正在使用什么应用，都能唤出 SailBoard。
- 唤出时 SailBoard 位于当前屏幕底部。
- 尽量不抢占过多视觉空间。
- 窗口置顶。
- 按 `Esc` 隐藏。
- 成功粘贴后自动隐藏。

---

### D. 横向卡片栏

主体 UI 位于屏幕底部。

参考：

```text
┌──────────────────────────────────────────────────────────────┐
│  🔍 搜索                                      剪贴板 | 收藏  │
│                                                              │
│ [card] [card] [card] [card] [card] [card] [card] →          │
└──────────────────────────────────────────────────────────────┘
```

卡片横向排列。

默认选中第一张，也就是最近一条。

---

### E. 选择与粘贴

唤出后支持：

```text
← / →         切换卡片
鼠标滚轮       左右切换
鼠标点击       选择并粘贴
Enter          粘贴选中内容
Esc            关闭
```

选中后：

1. 将内容重新写入系统剪贴板。
2. SailBoard 隐藏。
3. 焦点返回用户刚才使用的应用。
4. 模拟：
   - macOS：`Command + V`
   - Windows：`Ctrl + V`

---

### F. 搜索

唤出 SailBoard 后，直接输入文字即可进入搜索。

例如：

```text
Command + Shift + V
github
```

此时：

```text
搜索框：github
下方卡片：只显示包含 github 的历史记录
```

要求：

- 实时搜索
- 无需点击搜索框
- `Backspace` 删除
- `Esc`
  - 如果搜索框有内容：先清空搜索
  - 如果搜索为空：关闭 SailBoard

首版搜索范围：

- 文本内容
- URL
- 来源应用名称

---

### G. 收藏

界面顶部两个 tab：

```text
剪贴板
收藏
```

任意卡片可点击：

```text
☆
```

变成：

```text
★
```

收藏内容：

- 永不过期
- 不参与普通历史清理
- 仍然与普通剪贴板共用同一条记录
- 只是 `favorite = true`

---

### H. 设置

第一版设置页只需要：

#### 历史保留时间

```text
1 天
7 天
30 天
90 天
永久
```

默认建议：

```text
30 天
```

#### 历史最大空间

```text
100 MB
500 MB
1 GB
5 GB
无限制
```

默认建议：

```text
1 GB
```

#### 快捷键

允许用户修改全局快捷键。

#### 开机启动

```text
[ ] 登录时启动 SailBoard
```

---

# 3. 推荐技术方案

## 3.1 主技术栈

推荐：

```text
Core        Go
Desktop     Wails
Frontend    TypeScript + React
UI          Tailwind CSS
Database    SQLite
Search      SQLite FTS5 / LIKE
Storage     Local filesystem
```

推荐使用：

> **Go + Wails**

而不是 Electron。

主要原因：

- Go 非常适合系统级常驻应用。
- 二进制体积明显小于 Electron。
- 内存占用更低。
- 后端系统调用实现方便。
- Windows / macOS 均可编译。
- 前端仍然可以使用 React 快速实现 Paste 风格 UI。
- 可以清楚地把“系统能力”和“UI”分开。

---

## 3.2 不建议第一版使用纯 Go GUI

例如：

- Fyne
- Gio

不是不能实现，而是 SailBoard 的核心界面包含：

- 动画
- 横向卡片
- 搜索
- 卡片预览
- favicon
- hover
- tab
- 弹出窗口

Web UI 的开发效率明显更高。

因此推荐：

```text
Go
  +
Wails
  +
React
```

---

# 4. 总体架构

建议分为五层：

```text
┌─────────────────────────────────────┐
│             React UI                │
├─────────────────────────────────────┤
│          Wails App Bridge           │
├─────────────────────────────────────┤
│          Application Layer          │
├─────────────────────────────────────┤
│       Clipboard / Storage Core      │
├─────────────────────────────────────┤
│ macOS Adapter     Windows Adapter   │
└─────────────────────────────────────┘
```

核心原则：

> React 不直接接触任何系统 API。

所有系统能力统一通过 Go 接口实现。

---

# 5. 推荐项目目录

```text
sailboard/
├── app/
│   ├── app.go
│   ├── lifecycle.go
│   └── events.go
│
├── cmd/
│   └── sailboard/
│       └── main.go
│
├── internal/
│   ├── clipboard/
│   │   ├── service.go
│   │   ├── watcher.go
│   │   ├── parser.go
│   │   ├── hash.go
│   │   └── types.go
│   │
│   ├── platform/
│   │   ├── platform.go
│   │   ├── darwin/
│   │   │   ├── clipboard.go
│   │   │   ├── hotkey.go
│   │   │   ├── paste.go
│   │   │   ├── app_source.go
│   │   │   └── window.go
│   │   │
│   │   └── windows/
│   │       ├── clipboard.go
│   │       ├── hotkey.go
│   │       ├── paste.go
│   │       ├── app_source.go
│   │       └── window.go
│   │
│   ├── storage/
│   │   ├── db.go
│   │   ├── repository.go
│   │   ├── migrations.go
│   │   ├── cleanup.go
│   │   └── files.go
│   │
│   ├── search/
│   │   └── search.go
│   │
│   ├── preview/
│   │   ├── url.go
│   │   └── favicon.go
│   │
│   ├── config/
│   │   └── config.go
│   │
│   └── sync/
│       ├── provider.go
│       └── noop.go
│
├── frontend/
│   ├── src/
│   │   ├── App.tsx
│   │   ├── components/
│   │   │   ├── ClipboardBar.tsx
│   │   │   ├── ClipboardCard.tsx
│   │   │   ├── SearchBar.tsx
│   │   │   ├── Tabs.tsx
│   │   │   └── Settings.tsx
│   │   │
│   │   ├── hooks/
│   │   ├── stores/
│   │   ├── styles/
│   │   └── types/
│   └── package.json
│
├── migrations/
├── scripts/
├── tests/
├── wails.json
├── go.mod
├── README.md
└── DESIGN.md
```

---

# 6. 数据模型

SQLite。

主表：

```sql
CREATE TABLE clipboard_items (
    id TEXT PRIMARY KEY,

    content_type TEXT NOT NULL,

    text_content TEXT,
    file_path TEXT,

    content_hash TEXT NOT NULL UNIQUE,

    source_app_name TEXT,
    source_app_bundle_id TEXT,
    source_app_icon_path TEXT,

    char_count INTEGER,
    image_width INTEGER,
    image_height INTEGER,

    url TEXT,
    url_title TEXT,
    url_description TEXT,
    url_preview_image TEXT,
    favicon_path TEXT,

    is_favorite INTEGER NOT NULL DEFAULT 0,

    created_at INTEGER NOT NULL,
    last_used_at INTEGER NOT NULL,

    byte_size INTEGER NOT NULL DEFAULT 0
);
```

配置：

```sql
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

---

# 7. ClipboardItem Go 模型

```go
type ContentType string

const (
    ContentText  ContentType = "text"
    ContentURL   ContentType = "url"
    ContentImage ContentType = "image"
)

type ClipboardItem struct {
    ID          string
    Type        ContentType

    Text        string
    FilePath    string

    Hash        string

    SourceApp   AppInfo

    CharCount   int

    ImageWidth  int
    ImageHeight int

    URL         *URLPreview

    Favorite    bool

    CreatedAt   time.Time
    LastUsedAt  time.Time

    ByteSize    int64
}
```

---

# 8. Clipboard Watcher

定义统一接口：

```go
type ClipboardWatcher interface {
    Start(ctx context.Context, onChange func(RawClipboardContent)) error
    Stop() error
}
```

系统实现：

```text
darwinClipboardWatcher
windowsClipboardWatcher
```

---

## 8.1 macOS

可以基于：

```text
NSPasteboard.general.changeCount
```

通过 Go + Objective-C bridge / cgo 读取。

第一版允许使用轮询：

```text
100~300 ms
```

推荐：

```text
200 ms
```

如果：

```text
changeCount != lastChangeCount
```

则读取内容。

不要为了追求事件驱动而阻塞 MVP。

---

## 8.2 Windows

优先使用：

```text
AddClipboardFormatListener
WM_CLIPBOARDUPDATE
```

如果 Wails 窗口句柄集成复杂，可以第一版也先使用轮询。

后续再优化为原生事件监听。

---

# 9. 内容解析优先级

读取到系统剪贴板后：

```text
image
  ↓
URL
  ↓
text
```

具体逻辑：

```text
if clipboard contains image:
    ContentImage

else if clipboard contains text:
    if text is valid HTTP/HTTPS URL:
        ContentURL
    else:
        ContentText
```

首版不要同时保存复杂富文本格式。

以后可扩展：

```text
HTML
RTF
file
color
code
```

---

# 10. Hash 与去重

这是核心逻辑之一。

定义：

```go
func ComputeContentHash(content NormalizedContent) string
```

推荐：

```text
SHA-256
```

---

## 10.1 文本标准化

建议：

```text
保留实际文本
hash 时做有限标准化：
- \r\n → \n
```

不要：

- trim 掉首尾空格
- 合并多个空格
- lowercase

因为这些可能是用户真正复制的内容。

---

## 10.2 URL

URL 可以首先：

```text
TrimSpace
```

然后 hash。

第一版不要做激进 URL canonicalization。

不要自动删除：

```text
utm_*
fragment
query
```

因为不同 URL 可能实际上有意义。

---

## 10.3 图片

图片统一转换成：

```text
PNG bytes
```

然后：

```text
SHA256(PNG bytes)
```

注意：

同一视觉图片如果因为编码不同导致 hash 不同，第一版可以接受。

以后可以加入 perceptual hash。

---

# 11. 图片存储

不要把完整图片 BLOB 放进 SQLite。

文件存储：

```text
AppData/
└── SailBoard/
    ├── sailboard.db
    ├── clipboard/
    │   ├── ab/
    │   │   └── abcdef....png
    │   └── ...
    ├── previews/
    ├── icons/
    └── cache/
```

根据 hash 前两位分目录：

```text
clipboard/{hash[0:2]}/{hash}.png
```

优点：

- 避免一个目录里出现几万文件
- 文件名天然去重
- 数据库只记录路径

---

# 12. 来源应用

这是一个跨平台复杂点，因此通过接口实现：

```go
type AppSourceProvider interface {
    GetForegroundApp() (AppInfo, error)
}
```

```go
type AppInfo struct {
    Name       string
    Identifier string
    IconPath   string
}
```

---

## 12.1 工作方式

由于系统剪贴板本身通常不会可靠提供“是谁复制的”，第一版采用：

> 检测到剪贴板发生变化时，记录当时前台应用。

例如：

```text
用户在 Chrome 按 Command+C
          ↓
200 ms 内检测 clipboard change
          ↓
frontmostApplication = Chrome
          ↓
记录 Chrome
```

这不是 100% 准确，但对于第一版足够。

---

## 12.2 macOS

获取：

```text
NSWorkspace.shared.frontmostApplication
```

可获得：

- localizedName
- bundleIdentifier
- icon

---

## 12.3 Windows

获取：

```text
GetForegroundWindow
GetWindowThreadProcessId
OpenProcess
QueryFullProcessImageName
```

再提取：

- 应用名
- exe 路径
- icon

---

# 13. 防止 SailBoard 自己重复捕获

这是必须解决的问题。

用户选择历史卡片后：

```text
SailBoard → SetClipboard(item)
```

这会触发 clipboard watcher。

如果不处理，会把它再次当作新复制事件。

解决：

```go
type ClipboardService struct {
    ignoreNextHash string
}
```

粘贴前：

```text
ignoreNextHash = item.Hash
SetClipboard(item)
```

watcher 收到：

```text
if hash == ignoreNextHash:
    ignore
    ignoreNextHash = ""
```

但是：

需要同步更新该记录：

```text
last_used_at = now
```

---

# 14. 清理策略

设置：

```text
RetentionDays
MaxStorageBytes
```

收藏：

```text
is_favorite = true
```

永远不自动删除。

---

## 14.1 时间清理

例如：

```text
30 天
```

执行：

```sql
DELETE
FROM clipboard_items
WHERE is_favorite = 0
AND last_used_at < cutoff;
```

---

## 14.2 空间清理

如果：

```text
current_size > max_size
```

则：

```text
按 last_used_at ASC
从最旧的非收藏项目开始删除
直到 current_size <= max_size
```

删除数据库记录时：

如果对应文件不再被引用，则同步删除文件。

---

## 14.3 清理触发时机

不要每次复制都全表扫描。

建议：

```text
程序启动时
+
每 30 分钟
+
新增大图片后
```

---

# 15. UI 设计

## 15.1 主窗口

窗口：

```text
frameless
transparent / semi-transparent
always-on-top
bottom aligned
```

宽度：

```text
屏幕宽度
```

高度建议：

```text
260 ~ 340 px
```

首版默认：

```text
300 px
```

---

## 15.2 布局

```text
┌───────────────────────────────────────────────────────────────┐
│  Clipboard    Favorites                         🔍 Search      │
│                                                               │
│ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐       │
│ │ Text   │ │ Link   │ │ Image  │ │ Text   │ │ Text   │       │
│ │        │ │        │ │        │ │        │ │        │       │
│ │ body   │ │ body   │ │ image  │ │ body   │ │ body   │       │
│ │        │ │        │ │        │ │        │ │        │       │
│ │ 86 ch  │ │ chrome │ │800×600 │ │ 24 ch  │ │ 11 ch  │       │
│ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘       │
└───────────────────────────────────────────────────────────────┘
```

---

# 16. 卡片设计

卡片分三部分：

```text
Header
Body
Footer
```

---

## 16.1 Header

左上：

```text
TEXT
LINK
IMAGE
```

右上：

```text
App Icon
```

旁边允许：

```text
☆
```

---

## 16.2 文本卡片

主体：

```text
最多显示前 N 行
```

例如：

```text
4~8 行
```

超出：

```text
…
```

footer：

```text
128 chars
```

---

## 16.3 图片卡片

主体：

```text
object-fit: cover
```

或者：

```text
contain
```

建议：

```text
contain
```

避免截图被裁切。

footer：

```text
1920 × 1080
```

---

## 16.4 链接卡片

第一版：

```text
favicon
title
domain
URL
```

例如：

```text
GitHub
github.com
https://github.com/...
```

第二版再做：

```text
OpenGraph preview image
description
```

---

# 17. URL 预览策略

链接复制后：

```text
立即保存 URL
```

不要等待网络请求。

异步：

```text
FetchPreview(url)
```

成功后补充：

```text
title
description
favicon
preview image
```

失败：

```text
只显示 URL
```

必须有：

```text
timeout 3s
```

不能因为网页请求阻塞 clipboard watcher。

---

# 18. 搜索实现

首版数据量通常不会非常大。

可以先：

```sql
SELECT *
FROM clipboard_items
WHERE text_content LIKE ?
   OR url LIKE ?
   OR source_app_name LIKE ?
ORDER BY last_used_at DESC
LIMIT 100;
```

如果数据量变大，再切换 FTS5。

建议直接预留：

```go
type SearchService interface {
    Search(query string, mode SearchMode) ([]ClipboardItem, error)
}
```

---

# 19. 键盘状态机

这是 UX 的关键。

状态：

```text
Hidden
VisibleBrowse
VisibleSearch
```

---

## 19.1 唤出

```text
Hidden
  ↓ hotkey
VisibleBrowse
```

初始化：

```text
query = ""
selectedIndex = 0
```

---

## 19.2 左右

```text
ArrowLeft
ArrowRight
```

只改变：

```text
selectedIndex
```

---

## 19.3 输入字符

例如按：

```text
g
```

状态：

```text
VisibleBrowse
  ↓
VisibleSearch
```

query：

```text
g
```

继续按：

```text
ithub
```

query：

```text
github
```

---

## 19.4 Backspace

如果 query 非空：

```text
删除一个字符
```

---

## 19.5 Esc

规则：

```text
if query != "":
    query = ""
    return browse mode

else:
    hide window
```

---

## 19.6 Enter

```text
PasteSelected()
```

---

# 20. 鼠标滚轮

不要把滚轮直接作为网页的纵向滚动。

在卡片区域捕获：

```text
wheel
```

转换为：

```text
selectedIndex + 1
selectedIndex - 1
```

建议加：

```text
80~120 ms debounce
```

防止触控板滚动过快。

---

# 21. 自动粘贴

最重要的完整流程：

```text
用户正在 VS Code
        ↓
Command+Shift+V
        ↓
记录 previousActiveApp = VS Code
        ↓
显示 SailBoard
        ↓
用户选择 item
        ↓
Go 写入 clipboard
        ↓
SailBoard hide
        ↓
恢复 VS Code focus
        ↓
等待 30~100ms
        ↓
模拟 Command+V
```

Windows：

```text
Ctrl+V
```

---

# 22. Platform Adapter

定义：

```go
type Platform interface {
    Clipboard() ClipboardProvider
    Hotkey() HotkeyProvider
    ActiveApp() ActiveAppProvider
    Paste() PasteProvider
    Window() WindowProvider
}
```

---

## 22.1 ClipboardProvider

```go
type ClipboardProvider interface {
    Read() (RawClipboardContent, error)
    Write(item ClipboardItem) error
}
```

---

## 22.2 HotkeyProvider

```go
type HotkeyProvider interface {
    Register(binding Hotkey, handler func()) error
    Unregister(binding Hotkey) error
}
```

---

## 22.3 PasteProvider

```go
type PasteProvider interface {
    PasteToPreviousApp() error
}
```

---

# 23. macOS 权限

自动粘贴时，如果通过系统事件模拟键盘：

```text
Command + V
```

通常需要：

> Accessibility 权限

因此第一次使用自动粘贴时：

```text
SailBoard needs Accessibility permission to paste automatically.
```

需要提供按钮：

```text
Open System Settings
```

如果权限没有开启：

退化为：

```text
把内容写入 clipboard
+
提示用户手动 Command+V
```

不能因为权限问题让整个软件无法使用。

---

# 24. Windows 权限

通常：

```text
SendInput
```

即可模拟：

```text
Ctrl+V
```

但如果目标程序以管理员权限运行，而 SailBoard 不是管理员，输入注入可能失败。

首版：

- 正常应用直接支持。
- 如果失败，仍保持内容已经写入 clipboard。
- UI 提示：

```text
Copied. Press Ctrl+V to paste.
```

---

# 25. 窗口行为

唤出时记录：

```text
previousForegroundWindow
```

然后显示 SailBoard。

关闭时：

```text
RestoreForegroundWindow(previousForegroundWindow)
```

必须在下一次唤出前刷新。

不要保存跨 session 的 window handle。

---

# 26. 收藏逻辑

点击星标：

```go
SetFavorite(id string, favorite bool)
```

数据库：

```sql
UPDATE clipboard_items
SET is_favorite = ?
WHERE id = ?;
```

Favorites tab：

```sql
SELECT *
FROM clipboard_items
WHERE is_favorite = 1
ORDER BY last_used_at DESC;
```

收藏不会复制出第二份记录。

---

# 27. 配置文件

建议用户可读配置：

```text
config.json
```

或者全部存在 SQLite settings 表。

建议第一版：

```json
{
  "retention_days": 30,
  "max_storage_mb": 1024,
  "launch_at_login": false,
  "hotkey_mac": "Cmd+Shift+V",
  "hotkey_windows": "Ctrl+Shift+V"
}
```

---

# 28. 多设备同步预留

第一版不实现同步。

但是保留接口：

```go
type SyncProvider interface {
    Push(ctx context.Context, item ClipboardItem) error
    Pull(ctx context.Context, since time.Time) ([]ClipboardItem, error)
    Delete(ctx context.Context, id string) error
}
```

默认：

```go
type NoopSyncProvider struct{}
```

不要：

- 建服务器
- 做账号
- 做 WebSocket
- 做登录

只留接口。

---

# 29. 数据可同步字段

未来如果实现同步，建议同步：

```text
id
type
content
hash
created_at
last_used_at
favorite
metadata
```

不要同步：

```text
source_app_icon_path
local file path
OS window handle
```

图片应该通过：

```text
content hash
+
blob storage
```

另行同步。

---

# 30. 隐私设计

剪贴板是高敏感数据。

即使第一版不做复杂安全功能，也要从结构上预留。

后续支持：

```text
应用黑名单
隐私模式
暂停记录
清空历史
自动清理
敏感内容识别
```

至少第一版必须：

```text
设置页 → Clear History
```

行为：

- 删除非收藏历史 / 或让用户选择全部删除。
- 同步删除图片文件。
- 不留下孤儿文件。

---

# 31. 建议增加的第一版小功能

这些成本很低，但体验提升明显。

### 1. 暂停记录

托盘菜单：

```text
Pause Clipboard History
```

### 2. 清空历史

```text
Clear Clipboard History
```

### 3. 托盘图标

macOS Menu Bar / Windows System Tray。

菜单：

```text
Open SailBoard
Pause History
Settings
Quit
```

---

# 32. UI 动画

不要一开始做复杂动画。

只需要：

```text
窗口出现：150ms
卡片选中：100ms
卡片位移：150ms
```

选中卡片：

```text
scale: 1.03
```

并有明显 outline / shadow。

重点是反馈明确，而不是炫酷。

---

# 33. 性能指标

目标：

### Clipboard 捕获

```text
复制发生后 < 500 ms 写入历史
```

### 快捷键

```text
热键 → UI 可见 < 150 ms
```

### 搜索

10,000 条文本历史：

```text
输入 → 结果更新 < 50 ms
```

### 内存

空闲常驻尽量：

```text
< 150 MB
```

如果 Wails 能做到更低更好。

---

# 34. 异常处理

任何 clipboard parsing error：

```text
log
+
skip
```

不能导致 watcher 崩溃。

任何 URL preview error：

```text
ignore
```

任何 icon load error：

```text
fallback default icon
```

任何 paste injection error：

```text
clipboard write 保留成功
+
显示 copied 状态
```

---

# 35. 日志

建议：

```text
logs/sailboard.log
```

使用：

```text
slog
```

不要记录完整剪贴板文本。

日志只能记录：

```text
item id
type
byte size
hash prefix
source app
error
```

例如：

```text
clipboard captured type=text size=218 hash=0f3ab2 source=Chrome
```

不要：

```text
clipboard captured "my password is ..."
```

---

# 36. 开发阶段

原则：

> 每个阶段结束都必须能运行。

---

# Phase 0 — 项目骨架

目标：

```text
Wails app 能启动
React UI 能显示
Go ↔ React 调用正常
```

完成：

- 初始化 Go module
- 初始化 Wails
- React + TypeScript
- 基础窗口
- SQLite 初始化
- logger

验收：

```text
go test ./...
wails dev
```

均能正常运行。

---

# Phase 1 — Clipboard MVP

实现：

- clipboard watcher
- 文本捕获
- 图片捕获
- URL 判断
- SQLite 存储
- hash 去重
- history API

暂时允许：

```text
source app = Unknown
```

验收：

复制：

```text
A
B
A
```

最终历史：

```text
A
B
```

并且：

```text
A 位于最前
```

---

# Phase 2 — 主 UI

实现：

- 底部窗口
- 横向 cards
- 文本卡
- 图片卡
- URL 卡
- 选中状态

先提供普通按钮打开窗口。

暂时不要依赖全局热键。

这样 UI 开发不会被系统权限阻塞。

---

# Phase 3 — 全局快捷键

实现：

```text
macOS Cmd+Shift+V
Windows Ctrl+Shift+V
```

唤出：

```text
bottom panel
```

Esc：

```text
hide
```

---

# Phase 4 — 选择与粘贴

实现：

- 左右键
- 鼠标滚轮
- 点击
- Enter
- SetClipboard
- Restore focus
- Paste key simulation
- 自动隐藏

必须做：

```text
避免 watcher 捕获自身写入
```

---

# Phase 5 — 搜索

实现：

- 键盘输入直接搜索
- 搜索框
- 实时筛选
- Backspace
- Esc 清空
- query change debounce

建议：

```text
50 ms debounce
```

---

# Phase 6 — 收藏

实现：

- 星标
- Favorites tab
- 收藏不清理

---

# Phase 7 — 设置与清理

实现：

- retention
- max size
- cleanup
- clear history
- shortcut setting
- launch at login

---

# Phase 8 — 来源应用

实现：

macOS：

```text
frontmostApplication
bundle id
icon
```

Windows：

```text
foreground process
exe name
icon
```

如果这个阶段有系统细节问题：

> 不允许阻塞前面的核心功能。

---

# Phase 9 — URL Preview

实现：

- title
- favicon
- domain
- optional OG image

必须异步。

---

# Phase 10 — Release

实现：

macOS：

```text
.app
.dmg
```

Windows：

```text
.exe
installer
```

---

# 37. Release 目录

建议：

```text
dist/
├── macos/
│   ├── SailBoard.app
│   └── SailBoard.dmg
│
└── windows/
    ├── SailBoard.exe
    └── SailBoard-Setup.exe
```

---

# 38. CI

GitHub Actions：

```text
macos-latest
windows-latest
```

流程：

```text
checkout
setup-go
setup-node
npm ci
go test ./...
frontend test
wails build
package
upload artifact
```

Release tag：

```text
v0.1.0
```

自动生成：

```text
macOS artifact
Windows artifact
```

---

# 39. 测试

## 39.1 Hash

测试：

```text
相同文本 → 相同 hash
不同文本 → 不同 hash
\r\n 与 \n → 相同 hash
```

---

## 39.2 Dedup

输入：

```text
A
B
A
```

期望：

```text
count = 2
order = A, B
```

---

## 39.3 收藏

```text
A favorite
cleanup
```

A 仍然存在。

---

## 39.4 Retention

创建：

```text
old normal item
old favorite item
new normal item
```

cleanup 后：

```text
old normal deleted
old favorite retained
new normal retained
```

---

## 39.5 Size Cleanup

构造：

```text
10 MB
20 MB
30 MB
```

max：

```text
40 MB
```

必须从最旧的非收藏内容开始删除。

---

## 39.6 搜索

数据：

```text
github issue
openai api
google.com
```

搜索：

```text
git
```

只返回：

```text
github issue
```

---

## 39.7 自身写入防重复

执行：

```text
PasteExistingItem(A)
```

watcher 不应创建新 A。

---

# 40. UI E2E 测试

至少覆盖：

```text
打开
左右切换
搜索
清空搜索
收藏
切换 tab
Enter paste
Esc close
```

---

# 41. 手工测试矩阵

## macOS

测试目标：

```text
Finder
Safari
Chrome
VS Code
Terminal
Word
微信
```

验证：

- 能记录
- 能显示来源
- 能粘贴
- 焦点能返回

---

## Windows

测试目标：

```text
Explorer
Chrome
Edge
VS Code
PowerShell
Word
微信
```

---

# 42. 图片专项测试

复制：

```text
截图
网页图片
Photos 图片
Word 图片
```

验证：

- hash
- PNG 落盘
- 卡片预览
- 尺寸显示
- 再次粘贴

---

# 43. 链接专项测试

复制：

```text
https://github.com
https://example.com?a=1
普通包含 URL 的句子
```

只有“整个文本本身是 URL”时才识别为：

```text
LINK
```

例如：

```text
see https://github.com
```

仍然是：

```text
TEXT
```

---

# 44. Acceptance Criteria

SailBoard v0.1 必须达到：

- [ ] macOS 可以运行
- [ ] Windows 可以运行
- [ ] 文本复制能自动记录
- [ ] 图片复制能自动记录
- [ ] URL 能识别为链接
- [ ] 重复内容不会产生重复卡片
- [ ] 重复复制会把历史卡片移动到最前
- [ ] 可以通过全局快捷键唤出
- [ ] UI 位于屏幕底部
- [ ] 卡片横向排列
- [ ] 左右键可以选择
- [ ] 鼠标滚轮可以选择
- [ ] Enter 可以粘贴
- [ ] 点击可以粘贴
- [ ] 粘贴后窗口自动隐藏
- [ ] 搜索可以实时过滤
- [ ] 收藏可以永久保留
- [ ] 可以配置过期时间
- [ ] 可以配置最大空间
- [ ] 清理不会删除收藏
- [ ] 可以清空历史
- [ ] 主流程没有明显崩溃
- [ ] macOS / Windows 代码通过 platform adapter 隔离
- [ ] 已保留 sync provider 接口
- [ ] 已包含单元测试
- [ ] README 能说明如何开发和打包

---

# 45. 非目标

v0.1 不做：

- OCR
- AI 搜索
- 内容总结
- 自动分类
- 多设备同步
- iCloud
- 账号
- 云端数据库
- 团队共享
- Linux 正式支持
- 富文本编辑器
- 剪贴板工作流
- 脚本自动化
- 浏览器插件

---

# 46. 后续路线

## v0.2

可以增加：

```text
HTML / RTF
文件复制
拖拽
更丰富 URL preview
应用黑名单
自定义快捷键
pin
```

---

## v0.3

可以增加：

```text
OCR
全文索引
标签
固定分组
历史时间轴
```

---

## v0.4

同步：

```text
SailBoard SyncProvider
```

可以适配：

```text
iCloud
WebDAV
S3
自建 SailBoard Server
```

---

# 47. 未来同步接口设计原则

同步不要和核心数据库强绑定。

正确关系：

```text
ClipboardService
      ↓
Repository
      ↓
EventBus
      ↓
SyncProvider
```

而不是：

```text
ClipboardService
      ↓
Cloud API
```

这样第一版没有网络时仍然是完整软件。

---

# 48. 关键事件

建议内部事件：

```text
clipboard:item-added
clipboard:item-updated
clipboard:item-deleted

ui:show
ui:hide

history:changed
favorite:changed

settings:changed
```

React 监听：

```text
history:changed
```

即可局部刷新。

---

# 49. Backend API

建议暴露给 Wails：

```go
func (a *App) GetHistory(limit int, offset int) ([]ClipboardItemDTO, error)

func (a *App) SearchHistory(query string, favoriteOnly bool) ([]ClipboardItemDTO, error)

func (a *App) PasteItem(id string) error

func (a *App) ToggleFavorite(id string) error

func (a *App) DeleteItem(id string) error

func (a *App) ClearHistory(includeFavorites bool) error

func (a *App) GetSettings() Settings

func (a *App) UpdateSettings(settings Settings) error

func (a *App) HideWindow()

func (a *App) OpenSettings()
```

---

# 50. DTO

不要把内部数据库模型直接暴露给前端。

例如：

```go
type ClipboardItemDTO struct {
    ID          string `json:"id"`
    Type        string `json:"type"`

    Text        string `json:"text,omitempty"`
    PreviewURL  string `json:"previewUrl,omitempty"`

    SourceName  string `json:"sourceName,omitempty"`
    SourceIcon  string `json:"sourceIcon,omitempty"`

    CharCount   int    `json:"charCount,omitempty"`
    Width       int    `json:"width,omitempty"`
    Height      int    `json:"height,omitempty"`

    Favorite    bool   `json:"favorite"`

    CreatedAt   int64  `json:"createdAt"`
    LastUsedAt  int64  `json:"lastUsedAt"`
}
```

---

# 51. 建议的 UI Store

前端状态：

```text
activeTab
items
query
selectedIndex
visible
settings
```

不需要 Redux。

可以使用：

```text
Zustand
```

或者普通 React Context。

第一版建议 Zustand。

---

# 52. 首屏加载

不要把所有历史一次性加载到前端。

默认：

```text
50
```

卡片。

如果用户向右移动接近末尾：

```text
load next 50
```

搜索：

```text
backend query
limit 100
```

---

# 53. 卡片虚拟化

第一版 50 条无需虚拟化。

只有历史达到几千条且允许横向无限浏览时，再加入：

```text
react-window
```

不要提前增加复杂度。

---

# 54. Hotkey 冲突

注册失败时：

```text
Global shortcut is already in use.
```

设置页要求用户换一个。

不能 silent failure。

---

# 55. 多显示器

唤出时：

> SailBoard 应该出现在“当前鼠标所在屏幕”或“当前活动窗口所在屏幕”。

推荐优先：

```text
previous foreground window所在屏幕
```

拿不到则：

```text
cursor所在屏幕
```

拿不到再：

```text
primary display
```

---

# 56. 窗口位置

定位：

```text
x = screen.x
y = screen.y + screen.height - panel.height
width = screen.width
```

如果 macOS Dock 位于底部：

建议留出安全空间或使用系统 usable frame。

---

# 57. 交互细节

唤出以后：

```text
第一张卡片默认 selected
```

如果搜索结果变化：

```text
selectedIndex = 0
```

如果当前结果为空：

```text
No matching clipboard items
```

Enter 不做任何事。

---

# 58. 删除单条

虽然不是核心功能，但建议支持：

```text
Delete / Backspace + Modifier
```

为了避免和搜索输入冲突，首版推荐：

```text
右键菜单 → Delete
```

卡片右键菜单：

```text
Paste
Favorite
Delete
```

---

# 59. 卡片右键菜单

建议：

```text
Paste
Copy Only
Favorite / Remove Favorite
Delete
```

其中：

```text
Copy Only
```

只写入系统 clipboard，不自动模拟粘贴。

---

# 60. 自动粘贴失败的降级策略

完整逻辑：

```text
WriteClipboard
    ↓
Hide SailBoard
    ↓
Restore focus
    ↓
Try SendPasteKey
    ↓
success
```

如果失败：

```text
toast:
Copied to clipboard
```

这使软件不会因为 OS 权限问题不可用。

---

# 61. 首版视觉建议

SailBoard 不需要复制 Paste 的视觉细节。

建议自己的语言：

```text
半透明浅色/深色背景
圆角卡片
低饱和
明显选中状态
简洁 icon
```

支持：

```text
follow system appearance
```

第一版：

```text
Light
Dark
System
```

可后置到 v0.2，如果影响开发速度。

---

# 62. 应用名称与标识

建议：

```text
Product Name: SailBoard
Bundle ID: com.sailboard.app
Executable: SailBoard
```

macOS：

```text
SailBoard.app
```

Windows：

```text
SailBoard.exe
```

---

# 63. 数据目录

macOS：

```text
~/Library/Application Support/SailBoard/
```

Windows：

```text
%APPDATA%\SailBoard\
```

不要把运行数据写在应用安装目录。

---

# 64. 数据迁移

数据库必须有 schema version。

例如：

```sql
PRAGMA user_version;
```

未来：

```text
v1 → v2
```

通过 migration 完成。

不要假设用户会删除数据库升级。

---

# 65. Crash Safety

写图片：

```text
temp file
  ↓
fsync / close
  ↓
rename to final hash path
  ↓
insert DB
```

避免：

```text
DB 已经引用文件
但文件写到一半程序崩溃
```

---

# 66. Repository 接口

```go
type ClipboardRepository interface {
    Upsert(ctx context.Context, item ClipboardItem) (ClipboardItem, error)

    GetByID(ctx context.Context, id string) (*ClipboardItem, error)

    GetByHash(ctx context.Context, hash string) (*ClipboardItem, error)

    List(ctx context.Context, limit, offset int) ([]ClipboardItem, error)

    Search(ctx context.Context, query string, favorites bool, limit int) ([]ClipboardItem, error)

    SetFavorite(ctx context.Context, id string, value bool) error

    Touch(ctx context.Context, id string, now time.Time) error

    Delete(ctx context.Context, id string) error

    Cleanup(ctx context.Context, policy CleanupPolicy) error
}
```

这样未来可更换数据库而不影响 UI。

---

# 67. Clipboard Service

核心业务只放在：

```text
ClipboardService
```

伪代码：

```go
func (s *ClipboardService) OnClipboardChanged(raw RawClipboardContent) {
    normalized, err := s.parser.Parse(raw)
    if err != nil {
        return
    }

    hash := ComputeContentHash(normalized)

    if s.shouldIgnore(hash) {
        return
    }

    existing, _ := s.repo.GetByHash(ctx, hash)

    if existing != nil {
        s.repo.Touch(ctx, existing.ID, time.Now())
        s.events.Emit("history:changed")
        return
    }

    app := s.source.GetForegroundApp()

    item := BuildItem(normalized, app, hash)

    s.repo.Upsert(ctx, item)

    s.events.Emit("clipboard:item-added", item.ID)
    s.events.Emit("history:changed")
}
```

---

# 68. Paste Service

```go
func (s *PasteService) PasteItem(id string) error {
    item, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return err
    }

    s.ignoreNextHash.Store(item.Hash)

    if err := s.clipboard.Write(*item); err != nil {
        return err
    }

    s.repo.Touch(ctx, item.ID, time.Now())

    s.window.Hide()

    time.Sleep(50 * time.Millisecond)

    if err := s.paste.PasteToPreviousApp(); err != nil {
        s.events.Emit("paste:fallback-copy-only", item.ID)
        return nil
    }

    return nil
}
```

实际实现不要粗暴固定 sleep，可以通过 platform adapter 做适配，但首版允许 50~100ms。

---

# 69. Coding Agent 执行方式

Coding Agent 不要一次性完成所有功能后再测试。

建议每个 Phase：

```text
实现
↓
go test ./...
↓
frontend test
↓
手工 smoke test
↓
git commit
```

commit 示例：

```text
feat: initialize Wails application
feat: add clipboard watcher
feat: persist clipboard history
feat: add clipboard deduplication
feat: implement bottom clipboard panel
feat: register global hotkey
feat: implement quick paste
feat: add history search
feat: add favorites
feat: add cleanup policies
```

---

# 70. 第一阶段应优先做的 10 件事

严格按照这个顺序：

1. 建 Wails + Go + React 项目
2. SQLite + migrations
3. ClipboardItem model
4. clipboard watcher
5. text/image/url parser
6. hash + dedup
7. history API
8. bottom card UI
9. global shortcut
10. paste selected item

完成这 10 个以后：

> SailBoard 已经是一个真正能用的软件。

再做搜索、收藏、预览、来源应用、设置和打包优化。

---

# 71. 最终工程原则

整个项目始终围绕四个核心对象：

```text
ClipboardItem
ClipboardService
ClipboardRepository
Platform
```

不要让：

```text
React
OS-specific code
SQLite
```

互相直接耦合。

推荐依赖方向：

```text
UI
 ↓
App API
 ↓
Service
 ↓
Repository / Platform interfaces
 ↓
SQLite / macOS / Windows
```

---

# 72. v0.1 完成定义

当用户能够：

```text
复制一段内容
复制另一段内容
按快捷键
看到历史
搜索过去内容
选中
Enter
内容立即出现在当前输入框
```

并且：

```text
历史不会无限增长
重复内容不会重复保存
常用内容可以收藏
```

就认为 SailBoard v0.1 完成。

此后再做体验优化，而不是在 MVP 阶段继续扩展功能。

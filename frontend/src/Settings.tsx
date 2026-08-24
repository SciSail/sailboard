import { useEffect, useState } from "react";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { BeginShortcutCapture, ClearHistory, CloseWindow, EndShortcutCapture, GetCacheDir, GetDefaultSettings, GetSettings, GetStorageUsage, IsAccessibilityTrusted, OpenCacheFolder, OpenGitHub, SaveSettings, type SettingsData } from "./settingsBindings";
import "./Settings.css";

const formatBytes = (bytes: number) => {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / 1024;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) { value /= 1024; unit++; }
  return `${value >= 100 ? Math.round(value) : value.toFixed(1)} ${units[unit]}`;
};

// Same navigator-sniff App.tsx already uses to tell Mac from Windows for platform-specific UI text.
const isMac = /Mac/i.test(navigator.platform || navigator.userAgent);

// Keys the OS reports on their own while a modifier is still being held down; a capture isn't
// finished until a real (non-modifier) key follows one of these.
const MODIFIER_KEYS = new Set(["Control", "Shift", "Alt", "Meta"]);

// Turns a captured keydown into the "Ctrl+Shift+V" spec string platform.ParseHotkeySpec expects
// (see internal/platform/hotkey.go), or null if the key can't be a hotkey (unsupported key, or no
// modifier held — a bare unmodified key would hijack that key from every other app system-wide).
// The Meta/Super key's label is platform-specific ("Cmd" on macOS, "Win" on Windows) purely for
// display — internal/platform/hotkey.go's ParseHotkeySpec already treats cmd/command/super/win/
// windows/meta as interchangeable aliases for the same modifier, so this only changes what the
// button/notice text says, never what gets registered.
function formatCapturedKey(event: KeyboardEvent): string | null {
  const key = event.key;
  let name: string;
  if (/^[a-zA-Z0-9]$/.test(key)) name = key.toUpperCase();
  else if (/^F(1[0-9]|2[0-4]|[1-9])$/.test(key)) name = key.toUpperCase();
  else if (key === " ") name = "SPACE";
  else if (key === "Tab") name = "TAB";
  else if (key === "Enter") name = "ENTER";
  else return null;

  const mods: string[] = [];
  if (event.ctrlKey) mods.push("Ctrl");
  if (event.shiftKey) mods.push("Shift");
  if (event.altKey) mods.push("Alt");
  if (event.metaKey) mods.push(isMac ? "Cmd" : "Win");
  if (mods.length === 0) return null;
  return [...mods, name].join("+");
}

// A curated (not exhaustive) list of extremely common system/application shortcuts SailBoard
// refuses to let the user claim as their own global hotkey. RegisterHotKey doesn't stop anyone
// from claiming Ctrl+C any more than it would some obscure combo — it just means Ctrl+C stops
// reaching whatever app the user is actually trying to copy from the instant it's saved, system-
// wide, silently. Checked against formatCapturedKey's exact output format ("Ctrl+Shift+V"-style,
// mods in Ctrl/Shift/Alt/Win order).
const RESERVED_COMBOS = new Set([
  "Ctrl+C", "Ctrl+V", "Ctrl+X", "Ctrl+Z", "Ctrl+Y", "Ctrl+A", "Ctrl+S",
  "Ctrl+F", "Ctrl+N", "Ctrl+O", "Ctrl+P", "Ctrl+W", "Ctrl+TAB",
  "Alt+TAB", "Alt+F4",
]);

export default function Settings() {
  const [settings, setSettings] = useState<SettingsData | null>(null);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState("");
  const [capturing, setCapturing] = useState(false);
  const [cacheDir, setCacheDir] = useState("");
  const [usageBytes, setUsageBytes] = useState<number | null>(null);

  useEffect(() => { void GetSettings().then(setSettings); }, []);
  useEffect(() => { void GetCacheDir().then(setCacheDir); }, []);
  const refreshUsage = () => { void GetStorageUsage().then(setUsageBytes); };
  useEffect(refreshUsage, []);

  // Checked fresh every time the settings window opens (see IsAccessibilityTrusted's Go doc
  // comment for why that freshness matters — checking from this window's own short-lived process
  // sidesteps a staleness quirk the main panel's long-running process is otherwise subject to).
  // Always resolves true on Windows/other platforms, so this never fires there. Previously this
  // was only surfaced reactively, inline in the main panel, after a failed paste attempt.
  useEffect(() => {
    void IsAccessibilityTrusted().then(trusted => {
      if (!trusted) setNotice("未开启“辅助功能”权限，SailBoard 无法自动粘贴——请前往 系统设置 › 隐私与安全性 › 辅助功能，勾选 SailBoard 后重试");
    });
  }, []);

  // Armed only while capturing, so every other keystroke in the window behaves normally.
  useEffect(() => {
    if (!capturing) return;
    const onKeyDown = (event: KeyboardEvent) => {
      event.preventDefault();
      event.stopPropagation();
      if (event.key === "Escape") { setCapturing(false); return; }
      if (MODIFIER_KEYS.has(event.key)) return; // still waiting for the real key
      const spec = formatCapturedKey(event);
      if (!spec) {
        setNotice(`不支持这个按键，或者没有按住 Ctrl / Alt / Shift / ${isMac ? "Cmd" : "Win"} 等修饰键`);
        setCapturing(false);
        return;
      }
      if (RESERVED_COMBOS.has(spec)) {
        setNotice(spec + " 是系统或软件的常用快捷键，占用它会导致其失效，建议换一个组合");
        setCapturing(false);
        return;
      }
      setNotice("");
      setSettings(prev => (prev ? { ...prev, shortcut: spec } : prev));
      setCapturing(false);
    };
    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
  }, [capturing]);

  // The *current* global hotkey is still live while capturing a new one — without this, pressing
  // it mid-capture pops the main panel over the settings window. Every exit from capturing mode
  // (a valid combo, Escape, an unsupported key, or the hotkey:reserved handler below) goes through
  // setCapturing(false), so bracketing begin/end here covers all of them uniformly.
  useEffect(() => {
    if (capturing) void BeginShortcutCapture();
    else void EndShortcutCapture();
  }, [capturing]);

  // Some Alt combos (a bare Alt/F10 tap, Alt+Space — Windows' own reserved system-menu keys) never
  // reach the browser as a 'keydown' at all, so the capturing effect above has no way to see them;
  // see platform.SetOnSystemMenuKeyDirect's doc comment for how the backend detects this instead
  // and emits this event, resolving exactly which combo it was where it can (currently just
  // Alt+Space — RegisterHotKey genuinely accepts it as a real global hotkey, verified directly
  // against Win32, so it's offered as a successful capture rather than rejected outright; a bare
  // Alt/F10 tap has no second key to resolve and still can't be used). Only acts while actually
  // capturing — plenty of unrelated Alt taps can happen in this window otherwise.
  useEffect(() => EventsOn("hotkey:reserved", (resolvedSpec: string) => {
    setCapturing(prev => {
      if (!prev) return prev;
      // RESERVED_COMBOS doesn't currently list anything resolveReservedKeySpec can produce (it
      // only resolves Alt+Space today), but routing through the same check keeps this correct by
      // construction if that native resolver ever recognizes more combos later.
      if (resolvedSpec && !RESERVED_COMBOS.has(resolvedSpec)) {
        setNotice("已设置为 " + resolvedSpec + "。提示：系统里所有窗口的 Alt+Space 系统菜单快捷键都会改由 SailBoard 接管");
        setSettings(s => (s ? { ...s, shortcut: resolvedSpec } : s));
      } else if (resolvedSpec) {
        setNotice(resolvedSpec + " 是系统或软件的常用快捷键，占用它会导致其失效，建议换一个组合");
      } else {
        setNotice("Alt+F10 等属于 Windows 系统保留组合键，无法用作快捷键，请换一个组合");
      }
      return false;
    });
  }), []);

  const save = async () => {
    if (!settings) return;
    setSaving(true);
    try {
      await SaveSettings(settings);
      await CloseWindow();
    } catch (error) {
      setNotice(String(error));
      setSaving(false);
    }
  };

  const clearHistory = async () => {
    if (!window.confirm("确定要清空非收藏的剪贴板历史吗？此操作无法撤销。")) return;
    try {
      await ClearHistory(false);
      setNotice("已清空非收藏历史");
      refreshUsage();
    } catch (error) {
      setNotice(String(error));
    }
  };

  const resetDefaults = async () => {
    const defaults = await GetDefaultSettings();
    setSettings(defaults);
    setNotice("已恢复默认设置，点击保存以生效");
  };

  if (!settings) return <main className="settings-page loading">加载中…</main>;

  return <main className="settings-page">
    <div className="settings-row">
      <label>保留历史
        <select value={settings.retentionDays} onChange={event => setSettings({ ...settings, retentionDays: Number(event.target.value) })}>
          <option value="1">1 天</option>
          <option value="7">7 天</option>
          <option value="30">30 天</option>
          <option value="90">90 天</option>
          <option value="0">永久</option>
        </select>
      </label>
      <label>空间限制
        <select value={settings.maxStorageBytes} onChange={event => setSettings({ ...settings, maxStorageBytes: Number(event.target.value) })}>
          <option value={100 * 1024 * 1024}>100 MB</option>
          <option value={500 * 1024 * 1024}>500 MB</option>
          <option value={1024 * 1024 * 1024}>1 GB</option>
          <option value={5 * 1024 * 1024 * 1024}>5 GB</option>
          <option value={0}>不限制</option>
        </select>
      </label>
    </div>
    <label>全局快捷键
      <button type="button" className={capturing ? "shortcut-capture capturing" : "shortcut-capture"} onClick={() => { setNotice(""); setCapturing(true); }}>
        {capturing ? "请按下快捷键组合…（Esc 取消）" : settings.shortcut || "点击设置快捷键"}
      </button>
    </label>
    <label className="check">
      <input type="checkbox" checked={settings.launchAtLogin} onChange={event => setSettings({ ...settings, launchAtLogin: event.target.checked })} />
      登录时启动 SailBoard
    </label>

    <label>缓存目录
      <div className="cache-dir-row">
        <span className="cache-dir-path" title={cacheDir}>{cacheDir || "加载中…"}</span>
        <button type="button" className="secondary small" onClick={() => void OpenCacheFolder()}>打开文件夹</button>
      </div>
    </label>

    <div className="storage-usage">
      <div className="storage-usage-label">
        <span>缓存空间占用</span>
        <span>{usageBytes === null ? "计算中…" : settings.maxStorageBytes > 0 ? `${formatBytes(usageBytes)} / ${formatBytes(settings.maxStorageBytes)}` : formatBytes(usageBytes)}</span>
      </div>
      {usageBytes !== null && settings.maxStorageBytes > 0 && <div className="storage-usage-bar">
        <div className="storage-usage-fill" style={{ width: `${Math.min(100, (usageBytes / settings.maxStorageBytes) * 100)}%` }} />
      </div>}
    </div>

    <button className="secondary full-width" onClick={() => void clearHistory()}>清空非收藏历史</button>

    {notice && <div className="notice">
      <span className="notice-text">{notice}</span>
      <button type="button" className="notice-close" aria-label="关闭提示" onClick={() => setNotice("")}>×</button>
    </div>}

    <button className="footnote-link" onClick={() => void OpenGitHub()}>关于 SailBoard</button>

    <div className="actions">
      <button className="secondary" onClick={() => void resetDefaults()}>恢复默认设置</button>
      <button className="primary" disabled={saving} onClick={() => void save()}>{saving ? "保存中…" : "保存"}</button>
    </div>
  </main>;
}

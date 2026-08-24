// Hand-written bindings for the standalone SettingsApp Go struct (see settings_app.go).
//
// `wails generate module` can't produce these for us: it only introspects whichever struct is
// actually passed to Bind() in main()'s real execution path, which in a normal build is `App`
// (see main.go's branch on the --settings flag) — SettingsApp is only bound when
// runSettingsWindow() actually runs, which the codegen step never takes. Every `wails build` /
// `wails generate module` therefore wipes frontend/wailsjs/go/main/ down to just App's bindings,
// so anything for SettingsApp has to live outside that directory to survive a build. This
// follows the exact call pattern Wails' own codegen produces (window.go.<package>.<struct>.
// <Method>), verified once against a real generated build before being hand-maintained here —
// keep it in sync with SettingsApp's methods in settings_app.go.
export interface SettingsData {
  retentionDays: number;
  maxStorageBytes: number;
  shortcut: string;
  launchAtLogin: boolean;
}

declare global {
  interface Window {
    go: {
      main: {
        SettingsApp: {
          ClearHistory(includeFavorites: boolean): Promise<void>;
          CloseWindow(): Promise<void>;
          GetSettings(): Promise<SettingsData>;
          SaveSettings(settings: SettingsData): Promise<void>;
          GetDefaultSettings(): Promise<SettingsData>;
          GetCacheDir(): Promise<string>;
          OpenCacheFolder(): Promise<void>;
          GetStorageUsage(): Promise<number>;
          OpenGitHub(): Promise<void>;
          BeginShortcutCapture(): Promise<void>;
          EndShortcutCapture(): Promise<void>;
          IsAccessibilityTrusted(): Promise<boolean>;
        };
      };
    };
  }
}

export function ClearHistory(includeFavorites: boolean): Promise<void> {
  return window.go.main.SettingsApp.ClearHistory(includeFavorites);
}
export function CloseWindow(): Promise<void> {
  return window.go.main.SettingsApp.CloseWindow();
}
export function GetSettings(): Promise<SettingsData> {
  return window.go.main.SettingsApp.GetSettings();
}
export function SaveSettings(settings: SettingsData): Promise<void> {
  return window.go.main.SettingsApp.SaveSettings(settings);
}
export function GetDefaultSettings(): Promise<SettingsData> {
  return window.go.main.SettingsApp.GetDefaultSettings();
}
export function GetCacheDir(): Promise<string> {
  return window.go.main.SettingsApp.GetCacheDir();
}
export function OpenCacheFolder(): Promise<void> {
  return window.go.main.SettingsApp.OpenCacheFolder();
}
export function GetStorageUsage(): Promise<number> {
  return window.go.main.SettingsApp.GetStorageUsage();
}
export function OpenGitHub(): Promise<void> {
  return window.go.main.SettingsApp.OpenGitHub();
}
export function BeginShortcutCapture(): Promise<void> {
  return window.go.main.SettingsApp.BeginShortcutCapture();
}
export function EndShortcutCapture(): Promise<void> {
  return window.go.main.SettingsApp.EndShortcutCapture();
}
export function IsAccessibilityTrusted(): Promise<boolean> {
  return window.go.main.SettingsApp.IsAccessibilityTrusted();
}

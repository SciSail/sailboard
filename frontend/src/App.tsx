import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { DeleteItem, GetHistory, GetPaused, HideWindowAnimated, OpenSettingsWindow, PasteItem, PreviewSelection, SearchHistory, ToggleFavorite, TogglePaused } from "../wailsjs/go/main/App";
import "./App.css";
import ImageCardContent from "./ImageCardContent";
import UrlCardContent from "./UrlCardContent";
import FileCardContent from "./FileCardContent";
import ColorCardContent from "./ColorCardContent";

type Item = { id: string; type: "text" | "url" | "image" | "file" | "color"; text: string; sourceName: string; sourceIcon: string; charCount: number; width: number; height: number; favorite: boolean; lastUsedAt: number; fileNames?: string[] };

const DELETE_ANIMATION_MS = 220;

// Space-to-preview (below) only does anything on macOS — app.go's PreviewSelection calls
// platform.Controller.PreviewFile, which is a real Quick Look panel on darwin and an
// unconditional no-op on Windows/stub (see that method's doc comment). The hint text is gated on
// this so Windows users aren't told about a shortcut that silently does nothing for them; the key
// handler itself needs no such gating since it's already harmless there.
const isMac = /Mac/i.test(navigator.platform || navigator.userAgent);

const relativeTime = (timestamp: number) => {
  const diff = Math.max(0, Date.now() - timestamp);
  const minute = 60_000, hour = 3_600_000, day = 86_400_000;
  if (diff < 45_000) return "刚刚";
  if (diff < hour) return `${Math.round(diff / minute)} 分钟前`;
  if (diff < day) return `${Math.round(diff / hour)} 小时前`;
  if (diff < day * 7) return `${Math.round(diff / day)} 天前`;
  return new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric" }).format(timestamp);
};

const typeGlyph = (type: Item["type"]) => (type === "url" ? "🔗" : type === "image" ? "🖼" : type === "file" ? "📁" : type === "color" ? "🎨" : "文");
const typeLabel = (type: Item["type"]) => (type === "url" ? "链接" : type === "image" ? "图片" : type === "file" ? "文件" : type === "color" ? "颜色" : "文本");

// Text shown in the card footer, below the preview — the one place a file's name(s) or a link's
// URL now live, since the card body itself is preview-only (image/thumbnail/favicon).
const metaText = (item: Item) => {
  if (item.type === "image") return `${item.width} × ${item.height}`;
  if (item.type === "file") {
    const names = item.fileNames ?? [];
    return names.length <= 1 ? (names[0] ?? "") : `${names[0]} 等 ${names.length} 个文件`;
  }
  if (item.type === "url") return item.text;
  return `${item.charCount} 个字符`;
};

export default function App() {
  const [items, setItems] = useState<Item[]>([]);
  const [tab, setTab] = useState<"history" | "favorites">("history");
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState(0);
  const [notice, setNotice] = useState("");
  const [paused, setPaused] = useState(false);
  const [visible, setVisible] = useState(false);
  const [exitingIds, setExitingIds] = useState<Set<string>>(new Set());
  const searchInputRef = useRef<HTMLInputElement | null>(null);
  const sheetRef = useRef<HTMLDivElement | null>(null);
  const cardRailRef = useRef<HTMLElement | null>(null);
  const cardRefs = useRef<(HTMLElement | null)[]>([]);

  useEffect(() => { void GetPaused().then(setPaused); }, []);
  useEffect(() => EventsOn("paused:changed", (value: boolean) => setPaused(value)), []);

  // app.go's HideWindowAnimated does the actual native background cross-fade (see its doc
  // comment) and blocks for that duration before returning, so — unlike the old plain-HideWindow
  // design, where the backend hid the window instantly and the frontend had to run its own CSS
  // animation first and delay the backend call until it finished — there's nothing left for the
  // frontend to sequence here: firing setVisible(false) and calling HideWindowAnimated together
  // lets .sheet's CSS slide and the native fade run concurrently, and they're tuned (App.css /
  // app.go's panelAnimationMs) to finish at the same time.
  const requestHide = useCallback(() => {
    setVisible(false);
    void HideWindowAnimated();
  }, []);

  // The panel starts off-screen (see .shell in App.css) and slides up the moment the backend
  // shows the native window, alongside that window's own native background fade-in.
  //
  // Bringing the native window to the foreground (app.go's ShowWindow -> platform.FocusSelf)
  // does not by itself hand keyboard focus to the embedded WebView2 control's document — the OS
  // considers our top-level window focused while every keydown still no-ops inside the page.
  // Explicitly focusing a DOM element from inside the page is what actually grabs it, and it has
  // to be redone on every show (React's `autoFocus` only fires once, on that element's initial
  // mount, not on every reveal of this long-lived component).
  //
  // Deliberately focusing the *sheet* here, not the search input: landing focus straight in the
  // input meant every keydown (including Left/Right) looked like "typing" and got swallowed by
  // the input-guard below, so ←/→ card selection silently did nothing right after summoning the
  // panel. The sheet is just a focus anchor so the page has focus at all; onKeyDown redirects
  // focus into the search box itself only once the user actually starts typing.
  useEffect(() => EventsOn("window:shown", () => {
    setQuery("");
    setSelected(0);
    setVisible(true);
    cardRailRef.current?.scrollTo({ left: 0 });
    sheetRef.current?.focus();
    window.setTimeout(() => sheetRef.current?.focus(), 60);
  }), []);

  // Losing focus to another app (or another of SailBoard's own windows, like Settings) should
  // hide the panel immediately — there should never be a state where it's still visible while
  // the user is typing somewhere else. See app.go's WatchFocusLoss.
  useEffect(() => EventsOn("focus:lost", () => requestHide()), [requestHide]);

  const load = useCallback(async () => {
    try {
      const result = query ? await SearchHistory(query, tab === "favorites") : await GetHistory(50, 0);
      const fetched = result as unknown as Item[];
      setItems(tab === "favorites" ? fetched.filter(item => item.favorite) : fetched);
      setSelected(0);
    } catch (error) { setNotice(String(error)); }
  }, [query, tab]);

  useEffect(() => { void load(); }, [load]);
  useEffect(() => EventsOn("history:changed", () => void load()), [load]);

  // ←/→ can move selection to a card that's scrolled out of view, since
  // the rail doesn't otherwise follow keyboard selection — bring it fully into view whenever the
  // selected index changes. "nearest" means an already-visible card doesn't cause any scrolling.
  useEffect(() => {
    cardRefs.current[selected]?.scrollIntoView({ behavior: "smooth", inline: "nearest", block: "nearest" });
  }, [selected]);

  // Selection no longer follows mouse hover at all (that fought with ←/→ keyboard nav — moving
  // the mouse anywhere over the rail silently stole selection back from the keyboard). A click's
  // only job is to move `selected` to the clicked card; only a *second* click, landing on the
  // card that's already selected/highlighted, pastes.
  const onCardClick = (item: Item, index: number) => {
    if (selected === index) {
      void paste(item.id);
      return;
    }
    setSelected(index);
  };

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      // Esc must always work regardless of focus — it's the hint bar's documented behaviour
      // ("Esc 清空搜索/关闭"), not something that should depend on where the caret happens to be.
      if (event.key === "Escape") {
        event.preventDefault();
        if (query) setQuery("");
        else requestHide();
        return;
      }
      // Enter accepts the selected card whether or not focus happens to be in the search box
      // (standard "type to filter, Enter to pick the top result" search-box UX).
      if (event.key === "Enter" && items[selected]) { void paste(items[selected].id); return; }

      // Focus starts on the sheet, not the search input (see window:shown above), specifically
      // so ←/→ select cards immediately without first needing to click or tab out of a text
      // field. Once focus *is* in the input, let it behave like a normal text field — don't
      // hijack Left/Right/digits as card shortcuts while the user is editing their query.
      if (event.target instanceof HTMLInputElement) return;

      if (event.key === "ArrowRight") { event.preventDefault(); setSelected(value => Math.min(value + 1, items.length - 1)); return; }
      if (event.key === "ArrowLeft") { event.preventDefault(); setSelected(value => Math.max(value - 1, 0)); return; }
      if (event.key === "Delete" && items[selected]) { event.preventDefault(); void removeItem(items[selected].id); return; }
      // Quick Look (macOS only — see previewItem). Guarded ahead of the printable-character
      // fallback below, which would otherwise treat " " like any other typed character and shove
      // focus into the search box.
      if (event.key === " " && items[selected]) { event.preventDefault(); void previewItem(items[selected].id); return; }
      // Any other real keystroke — a letter, digit, punctuation, Backspace — means the user wants to
      // search: hand focus to the search box so this same keystroke's default action (character
      // insertion) lands there, instead of requiring a click first.
      if ((event.key.length === 1 || event.key === "Backspace") && !event.ctrlKey && !event.metaKey && !event.altKey) {
        searchInputRef.current?.focus();
      }
    };
    window.addEventListener("keydown", onKeyDown); return () => window.removeEventListener("keydown", onKeyDown);
  }, [items, selected, query, requestHide]);

  const paste = async (id: string) => { try { await PasteItem(id); } catch (error) { setNotice(`已复制到剪贴板：${String(error)}`); } };
  // Errors here (no platform Quick Look support, or the item isn't a file/image) are expected and
  // silent — pressing Space on a text/link/color card, or on Windows, should just do nothing.
  const previewItem = async (id: string) => { try { await PreviewSelection(id); } catch { /* not previewable here — no-op */ } };
  const toggleFavorite = async (id: string, event: React.MouseEvent) => { event.stopPropagation(); await ToggleFavorite(id); await load(); };
  const removeItem = (id: string) => {
    setExitingIds(prev => new Set(prev).add(id));
    window.setTimeout(async () => {
      try { await DeleteItem(id); } finally {
        setExitingIds(prev => { const next = new Set(prev); next.delete(id); return next; });
        await load();
      }
    }, DELETE_ANIMATION_MS);
  };
  const deleteItem = (id: string, event: React.MouseEvent) => { event.stopPropagation(); removeItem(id); };
  const togglePaused = async () => { setPaused(await TogglePaused()); };
  const title = useMemo(() => tab === "history" ? "剪贴板" : "收藏", [tab]);

  // The rail only ever scrolls horizontally (see .card-rail in App.css, scrollbar hidden), but a
  // plain mouse wheel only ever reports vertical delta — redirect it to horizontal scrolling so
  // hovering the rail and spinning the wheel pages through cards, the way a trackpad's native
  // horizontal swipe already does. A real horizontal gesture (deltaX already set, deltaY 0) is
  // left alone to scroll natively.
  const onRailWheel = (event: React.WheelEvent<HTMLElement>) => {
    if (event.deltaY === 0) return;
    event.currentTarget.scrollLeft += event.deltaY;
    event.preventDefault();
  };

  return <main className={visible ? "shell visible" : "shell"}>
    <div className="sheet" ref={sheetRef} tabIndex={-1}>
      <header className="toolbar">
        <label className="search"><span>⌕</span><input ref={searchInputRef} value={query} onChange={event => setQuery(event.target.value)} placeholder={`搜索${title}…`} /></label>
        <div className="tabs" data-active={tab}>
          <span className="tabs-indicator" />
          <button className={tab === "history" ? "tab active" : "tab"} onClick={() => setTab("history")}>剪贴板</button>
          <button className={tab === "favorites" ? "tab active" : "tab"} onClick={() => setTab("favorites")}>收藏</button>
        </div>
        <div className="hints">
          <span>← → 选择</span><span>Enter 粘贴</span>{isMac && <span>Space 预览</span>}<span>Delete 删除</span><span>Esc {query ? "清空搜索" : "关闭"}</span>
          {notice && <span className="notice">{notice}</span>}
        </div>
        <div className="toolbar-actions">
          <button className={paused ? "icon-button active" : "icon-button"} title={paused ? "恢复记录" : "暂停记录"} onClick={() => void togglePaused()}>{paused ? "▶" : "⏸"}</button>
          <button className="icon-button" title="设置" onClick={() => void OpenSettingsWindow()}>⋯</button>
        </div>
      </header>

      <section className="card-rail" aria-label={title} ref={cardRailRef} onWheel={onRailWheel}>
        {items.length === 0 && <div className="empty"><strong>{query ? "没有匹配的剪贴板内容" : "剪贴板历史为空"}</strong><span>复制文本或链接后，它会自动出现在这里。</span></div>}
        {items.map((item, index) => {
          const classNames = ["card", `kind-${item.type}`];
          if (index === selected) classNames.push("selected");
          if (exitingIds.has(item.id)) classNames.push("exiting");
          return <article key={item.id} ref={el => { cardRefs.current[index] = el; }} className={classNames.join(" ")} style={{ "--i": index } as React.CSSProperties} onClick={() => onCardClick(item, index)}>
            <div className="card-head">
              <div className="card-head-info">
                <span className="type-label">{typeLabel(item.type)}</span>
                <span className="time">{relativeTime(item.lastUsedAt)}</span>
              </div>
              {item.sourceIcon ? <img className="app-icon" src={item.sourceIcon} alt="" /> : <span className="app-icon glyph">{typeGlyph(item.type)}</span>}
            </div>
            <div className="card-content">{item.type === "url" ? <UrlCardContent id={item.id} text={item.text} /> : item.type === "image" ? <ImageCardContent id={item.id} width={item.width} height={item.height} /> : item.type === "file" ? <FileCardContent id={item.id} /> : item.type === "color" ? <ColorCardContent text={item.text} /> : <p>{item.text}</p>}</div>
            <footer>
              <span className="meta">{metaText(item)}</span>
              <div className="card-actions">
                <button onClick={event => void toggleFavorite(item.id, event)} title={item.favorite ? "取消收藏" : "收藏"} className={item.favorite ? "fav active" : "fav"}>{item.favorite ? "★" : "☆"}</button>
                <button onClick={event => deleteItem(item.id, event)} title="删除">×</button>
              </div>
            </footer>
          </article>;
        })}
      </section>
    </div>
  </main>;
}

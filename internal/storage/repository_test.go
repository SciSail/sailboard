package storage

import (
	"SailBoard/internal/clipboard"
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestUpsertDeduplicatesAndMovesItemToFront(t *testing.T) {
	r, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()
	makeItem := func(id, text string, now time.Time) clipboard.Item {
		return clipboard.Item{ID: id, Type: clipboard.ContentText, Text: text, Hash: clipboard.HashText(text), CreatedAt: now, LastUsedAt: now, CharCount: len(text), ByteSize: int64(len(text))}
	}
	first := time.Now().Add(-time.Minute)
	if _, _, err = r.Upsert(ctx, makeItem("a", "A", first)); err != nil {
		t.Fatal(err)
	}
	if _, _, err = r.Upsert(ctx, makeItem("b", "B", time.Now())); err != nil {
		t.Fatal(err)
	}
	item, created, err := r.Upsert(ctx, makeItem("new-a", "A", time.Now().Add(time.Minute)))
	if err != nil || created || item.ID != "a" {
		t.Fatalf("item=%+v created=%v err=%v", item, created, err)
	}
	items, err := r.List(ctx, 50, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "a" {
		t.Fatalf("items=%+v", items)
	}
}

// TestUpsertRefreshesSourceAppOnReMerge guards the merge behavior requested for identical content
// copied from a different app: dedup is already purely content-hash based (source app never
// factors into the hash), so re-copying the same text from a second app must hit the re-pin path
// and update the row's source app fields to the new app, not just its last_used_at.
func TestUpsertRefreshesSourceAppOnReMerge(t *testing.T) {
	r, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()
	appA := clipboard.AppInfo{Name: "AppA", Identifier: "appA.exe", IconPath: "icon-a.png"}
	appB := clipboard.AppInfo{Name: "AppB", Identifier: "appB.exe", IconPath: "icon-b.png"}
	makeItem := func(id, text string, app clipboard.AppInfo, now time.Time) clipboard.Item {
		return clipboard.Item{ID: id, Type: clipboard.ContentText, Text: text, Hash: clipboard.HashText(text), SourceApp: app, CreatedAt: now, LastUsedAt: now, CharCount: len(text), ByteSize: int64(len(text))}
	}
	first := time.Now().Add(-time.Minute)
	if _, _, err = r.Upsert(ctx, makeItem("a", "same text", appA, first)); err != nil {
		t.Fatal(err)
	}
	second := time.Now()
	item, created, err := r.Upsert(ctx, makeItem("new-a", "same text", appB, second))
	if err != nil || created {
		t.Fatalf("item=%+v created=%v err=%v", item, created, err)
	}
	if item.SourceApp != appB {
		t.Fatalf("SourceApp = %+v, want re-merge to refresh to the newer app %+v", item.SourceApp, appB)
	}
	if !item.LastUsedAt.Equal(second) {
		t.Fatalf("LastUsedAt = %v, want refreshed to %v", item.LastUsedAt, second)
	}

	// A capture whose source app couldn't be resolved (empty AppInfo, e.g. a transient ActiveApp
	// lookup failure) must not blank out the already-known-good attribution from appB.
	third := time.Now().Add(time.Minute)
	item, created, err = r.Upsert(ctx, makeItem("new-a-2", "same text", clipboard.AppInfo{}, third))
	if err != nil || created {
		t.Fatalf("item=%+v created=%v err=%v", item, created, err)
	}
	if item.SourceApp != appB {
		t.Fatalf("SourceApp = %+v, want it to still be %+v (unresolved source app must not overwrite)", item.SourceApp, appB)
	}
	if !item.LastUsedAt.Equal(third) {
		t.Fatalf("LastUsedAt = %v, want refreshed to %v even when source app is unresolved", item.LastUsedAt, third)
	}
}

// TestUpsertPersistsRichTextPayload guards the html_content/rtf_content columns added by
// migrateRichText: an item captured with a rich-text payload (an Office-style copy) must survive
// an insert + reopen round trip with both fields intact, and a plain item (no payload) must come
// back with them empty rather than some placeholder/NULL-derived value.
func TestUpsertPersistsRichTextPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now()
	rich := clipboard.Item{
		ID: "a", Type: clipboard.ContentText, Text: "Hello", Hash: clipboard.HashText("Hello"),
		HTML: "<p>Hello</p>", RTF: `{\rtf1 Hello}`, CreatedAt: now, LastUsedAt: now,
	}
	if _, _, err := r.Upsert(ctx, rich); err != nil {
		t.Fatal(err)
	}
	plain := clipboard.Item{
		ID: "b", Type: clipboard.ContentText, Text: "World", Hash: clipboard.HashText("World"),
		CreatedAt: now, LastUsedAt: now,
	}
	if _, _, err := r.Upsert(ctx, plain); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()

	got, err := reopened.GetByID(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if got.HTML != rich.HTML || got.RTF != rich.RTF {
		t.Fatalf("rich item HTML/RTF = %q/%q, want %q/%q", got.HTML, got.RTF, rich.HTML, rich.RTF)
	}

	gotPlain, err := reopened.GetByID(ctx, "b")
	if err != nil {
		t.Fatal(err)
	}
	if gotPlain.HTML != "" || gotPlain.RTF != "" {
		t.Fatalf("plain item HTML/RTF = %q/%q, want empty", gotPlain.HTML, gotPlain.RTF)
	}
}

func TestUpsertWithAssetsReplacesRichPayloadAndCountsUniqueAssets(t *testing.T) {
	r, err := Open(filepath.Join(t.TempDir(), "assets.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()
	now := time.Now()
	item := clipboard.Item{ID: "a", Type: clipboard.ContentText, Text: "same", Hash: clipboard.HashText("same"), HTML: "<b>old</b>", ByteSize: int64(len("same") + len("<b>old</b>")), CreatedAt: now, LastUsedAt: now}
	asset1 := clipboard.AssetRef{Hash: "asset-one", Path: filepath.Join(t.TempDir(), "one.png"), MIME: "image/png", ByteSize: 10}
	if _, _, err := r.UpsertWithAssets(ctx, item, []clipboard.AssetRef{asset1}); err != nil {
		t.Fatal(err)
	}
	usage, err := r.HistoryStorageUsage(ctx)
	if err != nil || usage != item.ByteSize+10 {
		t.Fatalf("usage=%d err=%v", usage, err)
	}
	item.HTML = "<i>new</i>"
	item.ByteSize = int64(len(item.Text) + len(item.HTML))
	asset2 := clipboard.AssetRef{Hash: "asset-two", Path: filepath.Join(t.TempDir(), "two.png"), MIME: "image/png", ByteSize: 20}
	got, created, err := r.UpsertWithAssets(ctx, item, []clipboard.AssetRef{asset2})
	if err != nil || created || got.HTML != item.HTML {
		t.Fatalf("got=%+v created=%v err=%v", got, created, err)
	}
	usage, err = r.HistoryStorageUsage(ctx)
	if err != nil || usage != item.ByteSize+20 {
		t.Fatalf("updated usage=%d err=%v", usage, err)
	}
	if err := r.Delete(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	usage, err = r.HistoryStorageUsage(ctx)
	if err != nil || usage != 0 {
		t.Fatalf("usage after delete=%d err=%v", usage, err)
	}
}

// TestMigrateIsIdempotentAcrossReopen guards against a real regression: migrate() used to reset
// PRAGMA user_version=1 unconditionally on every startup, which erased the version a later
// migration (migrateSourceIcon) had bumped it to, causing that migration's ALTER TABLE ADD
// COLUMN to re-run against a column that already existed and fail every second app launch.
func TestMigrateIsIdempotentAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	first, err := Open(path)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(path)
	if err != nil {
		t.Fatalf("second Open() (simulating app restart) error = %v", err)
	}
	defer second.Close()

	if _, err := second.GetSettings(context.Background()); err != nil {
		t.Fatalf("GetSettings() after reopen error = %v", err)
	}
}

func TestHasSettingsDistinguishesFreshInstallFromUnsavedDefaults(t *testing.T) {
	r, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()

	if has, err := r.HasSettings(ctx); err != nil || has {
		t.Fatalf("HasSettings() on fresh db = %v, %v; want false, nil", has, err)
	}
	// GetSettings() falling back to defaults must not itself count as "settings saved" —
	// otherwise every fresh install would look identical to a returning user on the very next
	// check, and first-run detection (app.go) would never fire.
	if _, err := r.GetSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if has, err := r.HasSettings(ctx); err != nil || has {
		t.Fatalf("HasSettings() after GetSettings() = %v, %v; want false, nil", has, err)
	}

	if err := r.SaveSettings(ctx, DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	if has, err := r.HasSettings(ctx); err != nil || !has {
		t.Fatalf("HasSettings() after SaveSettings() = %v, %v; want true, nil", has, err)
	}
}

func TestListSupportsConfiguredLargeAndUnlimitedLimits(t *testing.T) {
	r, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()
	now := time.Now()
	for i := 0; i < 350; i++ {
		text := fmt.Sprintf("item-%03d", i)
		item := clipboard.Item{ID: text, Type: clipboard.ContentText, Text: text, Hash: clipboard.HashText(text), CreatedAt: now.Add(time.Duration(i) * time.Second), LastUsedAt: now.Add(time.Duration(i) * time.Second)}
		if _, _, err := r.Upsert(ctx, item); err != nil {
			t.Fatal(err)
		}
	}

	items, err := r.List(ctx, 300, 0, false)
	if err != nil || len(items) != 300 {
		t.Fatalf("List(300) returned %d items, err=%v", len(items), err)
	}
	items, err = r.List(ctx, 0, 0, false)
	if err != nil || len(items) != 350 {
		t.Fatalf("List(unlimited) returned %d items, err=%v", len(items), err)
	}
}

func TestHistoryDisplayLimitDefaultsAndPersists(t *testing.T) {
	r, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()

	settings, err := r.GetSettings(ctx)
	if err != nil || settings.HistoryDisplayLimit != 50 {
		t.Fatalf("default HistoryDisplayLimit=%d, err=%v; want 50", settings.HistoryDisplayLimit, err)
	}
	settings.HistoryDisplayLimit = 300
	if err := r.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	settings, err = r.GetSettings(ctx)
	if err != nil || settings.HistoryDisplayLimit != 300 {
		t.Fatalf("saved HistoryDisplayLimit=%d, err=%v; want 300", settings.HistoryDisplayLimit, err)
	}
}

func TestSearchMatchesChineseContentTypeLabels(t *testing.T) {
	r, err := Open(filepath.Join(t.TempDir(), "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()
	now := time.Now()
	items := []clipboard.Item{
		{ID: "file", Type: clipboard.ContentFile, Text: "report.docx", FilePath: "C:\\report.docx", Hash: "hash-file", CreatedAt: now, LastUsedAt: now},
		{ID: "text", Type: clipboard.ContentText, Text: "plain note", Hash: "hash-text", CreatedAt: now.Add(-time.Second), LastUsedAt: now.Add(-time.Second)},
		{ID: "image", Type: clipboard.ContentImage, Text: "", Hash: "hash-image", CreatedAt: now.Add(-2 * time.Second), LastUsedAt: now.Add(-2 * time.Second)},
	}
	for _, item := range items {
		if _, _, err := r.Upsert(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	assertIDs := func(query string, want ...string) {
		t.Helper()
		got, err := r.Search(ctx, query, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Fatalf("Search(%q) returned %d items, want %d", query, len(got), len(want))
		}
		seen := map[string]bool{}
		for _, item := range got {
			seen[item.ID] = true
		}
		for _, id := range want {
			if !seen[id] {
				t.Fatalf("Search(%q) missing %q; got=%v", query, id, got)
			}
		}
	}
	assertIDs("文件", "file")
	assertIDs("文", "file", "text")
	assertIDs("文本", "text")
	assertIDs("图片", "image")
}

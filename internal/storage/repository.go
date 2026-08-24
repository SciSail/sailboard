package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"SailBoard/internal/clipboard"
	_ "modernc.org/sqlite"
)

type Repository struct{ db *sql.DB }

type Settings struct {
	RetentionDays   int    `json:"retentionDays"`
	MaxStorageBytes int64  `json:"maxStorageBytes"`
	Shortcut        string `json:"shortcut"`
	LaunchAtLogin   bool   `json:"launchAtLogin"`
}

func Open(path string) (*Repository, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	r := &Repository{db: db}
	if err := r.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return r, nil
}

func (r *Repository) Close() error { return r.db.Close() }

func (r *Repository) migrate() error {
	_, err := r.db.Exec(`
		PRAGMA journal_mode=WAL;
		CREATE TABLE IF NOT EXISTS clipboard_items (
			id TEXT PRIMARY KEY, content_type TEXT NOT NULL, text_content TEXT, file_path TEXT,
			content_hash TEXT NOT NULL UNIQUE, source_app_name TEXT, source_app_identifier TEXT,
			char_count INTEGER NOT NULL DEFAULT 0, image_width INTEGER NOT NULL DEFAULT 0,
			image_height INTEGER NOT NULL DEFAULT 0, is_favorite INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL, last_used_at INTEGER NOT NULL, byte_size INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_clipboard_last_used ON clipboard_items(last_used_at DESC);
		CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);`)
	if err != nil {
		return err
	}
	// user_version is deliberately left untouched here (the CREATE TABLE/INDEX statements above
	// are already idempotent via IF NOT EXISTS) — it is owned solely by the versioned migration
	// steps below, e.g. migrateSourceIcon. Setting it unconditionally on every startup would
	// erase whatever a later migration had bumped it to, causing that migration to re-run and
	// fail on already-applied schema changes such as a duplicate ALTER TABLE ADD COLUMN.
	if err := r.migrateSourceIcon(); err != nil {
		return err
	}
	return r.migrateRichText()
}

// migrateSourceIcon adds the source-app icon column introduced after v1. SQLite's ALTER TABLE
// ADD COLUMN isn't repeatable, so this is gated on user_version rather than re-run unconditionally.
func (r *Repository) migrateSourceIcon() error {
	var version int
	if err := r.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version >= 2 {
		return nil
	}
	if _, err := r.db.Exec("ALTER TABLE clipboard_items ADD COLUMN source_icon_path TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	_, err := r.db.Exec("PRAGMA user_version=2")
	return err
}

// migrateRichText adds the HTML/RTF payload columns for Office-style formatted-text copies (see
// clipboard.Item.HTML/RTF's doc comment). Same ALTER TABLE ADD COLUMN gating as migrateSourceIcon.
func (r *Repository) migrateRichText() error {
	var version int
	if err := r.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version >= 3 {
		return nil
	}
	if _, err := r.db.Exec(`ALTER TABLE clipboard_items ADD COLUMN html_content TEXT NOT NULL DEFAULT '';
		ALTER TABLE clipboard_items ADD COLUMN rtf_content TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	_, err := r.db.Exec("PRAGMA user_version=3")
	return err
}

func (r *Repository) Upsert(ctx context.Context, item clipboard.Item) (clipboard.Item, bool, error) {
	var existing clipboard.Item
	err := r.scanRow(r.db.QueryRowContext(ctx, selectItem+" WHERE content_hash = ?", item.Hash), &existing)
	if err == nil {
		// Dedup is purely by content hash, which never factors in the source app — so the exact
		// same content copied again from a *different* app (or the same app re-launched under a
		// different icon path) still hits this re-pin path, not a fresh insert. Refresh the source
		// app fields along with last_used_at, or the merged row would keep pointing at whichever
		// app happened to copy it first, misattributing every later re-copy to the wrong source.
		// Only overwrite when the new capture actually resolved a source app, though — a transient
		// ActiveApp() lookup failure (see app.go's resolveSourceApp) reports an empty AppInfo, and
		// that should never blank out a previously-known-good attribution.
		if item.SourceApp.Name != "" {
			_, err = r.db.ExecContext(ctx, "UPDATE clipboard_items SET last_used_at=?, source_app_name=?, source_app_identifier=?, source_icon_path=? WHERE id=?",
				item.LastUsedAt.UnixMilli(), item.SourceApp.Name, item.SourceApp.Identifier, item.SourceApp.IconPath, existing.ID)
			existing.SourceApp = item.SourceApp
		} else {
			_, err = r.db.ExecContext(ctx, "UPDATE clipboard_items SET last_used_at=? WHERE id=?", item.LastUsedAt.UnixMilli(), existing.ID)
		}
		existing.LastUsedAt = item.LastUsedAt
		return existing, false, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return clipboard.Item{}, false, err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO clipboard_items
		(id,content_type,text_content,file_path,content_hash,source_app_name,source_app_identifier,source_icon_path,char_count,image_width,image_height,is_favorite,created_at,last_used_at,byte_size,html_content,rtf_content)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.Type, item.Text, item.FilePath, item.Hash, item.SourceApp.Name, item.SourceApp.Identifier, item.SourceApp.IconPath,
		item.CharCount, item.ImageWidth, item.ImageHeight, item.Favorite, item.CreatedAt.UnixMilli(), item.LastUsedAt.UnixMilli(), item.ByteSize, item.HTML, item.RTF)
	return item, true, err
}

const selectItem = `SELECT id,content_type,COALESCE(text_content,''),COALESCE(file_path,''),content_hash,
	COALESCE(source_app_name,''),COALESCE(source_app_identifier,''),COALESCE(source_icon_path,''),char_count,image_width,image_height,is_favorite,created_at,last_used_at,byte_size,
	COALESCE(html_content,''),COALESCE(rtf_content,'') FROM clipboard_items`

func (r *Repository) scanRow(row *sql.Row, item *clipboard.Item) error {
	var favorite int
	var created, used int64
	err := row.Scan(&item.ID, &item.Type, &item.Text, &item.FilePath, &item.Hash, &item.SourceApp.Name, &item.SourceApp.Identifier, &item.SourceApp.IconPath,
		&item.CharCount, &item.ImageWidth, &item.ImageHeight, &favorite, &created, &used, &item.ByteSize, &item.HTML, &item.RTF)
	item.Favorite = favorite != 0
	item.CreatedAt = time.UnixMilli(created)
	item.LastUsedAt = time.UnixMilli(used)
	return err
}

func (r *Repository) List(ctx context.Context, limit, offset int, favoritesOnly bool) ([]clipboard.Item, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := selectItem + " WHERE (? = 0 OR is_favorite=1) ORDER BY last_used_at DESC LIMIT ? OFFSET ?"
	return r.query(ctx, query, boolInt(favoritesOnly), limit, offset)
}

func (r *Repository) Search(ctx context.Context, value string, favoritesOnly bool) ([]clipboard.Item, error) {
	needle := "%" + value + "%"
	query := selectItem + ` WHERE (? = 0 OR is_favorite=1) AND (text_content LIKE ? COLLATE NOCASE OR source_app_name LIKE ? COLLATE NOCASE) ORDER BY last_used_at DESC LIMIT 100`
	return r.query(ctx, query, boolInt(favoritesOnly), needle, needle)
}

func (r *Repository) query(ctx context.Context, query string, args ...any) ([]clipboard.Item, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []clipboard.Item{}
	for rows.Next() {
		var item clipboard.Item
		var favorite int
		var created, used int64
		if err := rows.Scan(&item.ID, &item.Type, &item.Text, &item.FilePath, &item.Hash, &item.SourceApp.Name, &item.SourceApp.Identifier, &item.SourceApp.IconPath, &item.CharCount, &item.ImageWidth, &item.ImageHeight, &favorite, &created, &used, &item.ByteSize, &item.HTML, &item.RTF); err != nil {
			return nil, err
		}
		item.Favorite = favorite != 0
		item.CreatedAt = time.UnixMilli(created)
		item.LastUsedAt = time.UnixMilli(used)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id string) (*clipboard.Item, error) {
	var item clipboard.Item
	err := r.scanRow(r.db.QueryRowContext(ctx, selectItem+" WHERE id=?", id), &item)
	if err != nil {
		return nil, err
	}
	return &item, nil
}
func (r *Repository) Touch(ctx context.Context, id string, now time.Time) error {
	_, err := r.db.ExecContext(ctx, "UPDATE clipboard_items SET last_used_at=? WHERE id=?", now.UnixMilli(), id)
	return err
}
func (r *Repository) SetFavorite(ctx context.Context, id string, value bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE clipboard_items SET is_favorite=? WHERE id=?", boolInt(value), id)
	return err
}
func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM clipboard_items WHERE id=?", id)
	return err
}
func (r *Repository) ClearHistory(ctx context.Context, includeFavorites bool) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM clipboard_items WHERE ?=1 OR is_favorite=0", boolInt(includeFavorites))
	return err
}

func (r *Repository) Cleanup(ctx context.Context, policy Settings) error {
	if policy.RetentionDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -policy.RetentionDays).UnixMilli()
		if _, err := r.db.ExecContext(ctx, "DELETE FROM clipboard_items WHERE is_favorite=0 AND last_used_at<?", cutoff); err != nil {
			return err
		}
	}
	if policy.MaxStorageBytes > 0 {
		for {
			var total int64
			if err := r.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(byte_size),0) FROM clipboard_items").Scan(&total); err != nil {
				return err
			}
			if total <= policy.MaxStorageBytes {
				break
			}
			var id string
			err := r.db.QueryRowContext(ctx, "SELECT id FROM clipboard_items WHERE is_favorite=0 ORDER BY last_used_at ASC LIMIT 1").Scan(&id)
			if errors.Is(err, sql.ErrNoRows) {
				break
			}
			if err != nil {
				return err
			}
			if err := r.Delete(ctx, id); err != nil {
				return err
			}
		}
	}
	return nil
}

// HasSettings reports whether any settings row has ever been saved, so the caller can tell a
// genuinely fresh install (never persisted anything) from a returning user who just hasn't
// touched a particular setting — GetSettings alone can't distinguish those, since it silently
// falls back to DefaultSettings() either way.
func (r *Repository) HasSettings(ctx context.Context) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM settings").Scan(&count)
	return count > 0, err
}

func (r *Repository) GetSettings(ctx context.Context) (Settings, error) {
	s := DefaultSettings()
	rows, err := r.db.QueryContext(ctx, "SELECT key,value FROM settings")
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return s, err
		}
		switch key {
		case "retention_days":
			fmt.Sscan(value, &s.RetentionDays)
		case "max_storage_bytes":
			fmt.Sscan(value, &s.MaxStorageBytes)
		case "shortcut":
			s.Shortcut = value
		case "launch_at_login":
			s.LaunchAtLogin = value == "true"
		}
	}
	return s, rows.Err()
}
func (r *Repository) SaveSettings(ctx context.Context, s Settings) error {
	values := map[string]string{"retention_days": fmt.Sprint(s.RetentionDays), "max_storage_bytes": fmt.Sprint(s.MaxStorageBytes), "shortcut": s.Shortcut, "launch_at_login": fmt.Sprint(s.LaunchAtLogin)}
	for k, v := range values {
		if _, err := r.db.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", k, v); err != nil {
			return err
		}
	}
	return nil
}

// DefaultSettings returns the settings a fresh install starts with — also what "restore
// defaults" in the settings UI resets the form to (see SettingsApp.GetDefaultSettings).
func DefaultSettings() Settings {
	return Settings{RetentionDays: 30, MaxStorageBytes: 1 << 30, Shortcut: "Ctrl+Shift+V"}
}
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

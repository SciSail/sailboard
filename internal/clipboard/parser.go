package clipboard

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Parse classifies clipboard data with files taking priority over images, and images over URLs
// and text.
func Parse(raw RawContent) (ContentType, string, string, int64, bool) {
	if len(raw.FilePaths) > 0 {
		names := make([]string, len(raw.FilePaths))
		for i, p := range raw.FilePaths {
			names[i] = filepath.Base(p)
		}
		return ContentFile, strings.Join(names, ", "), hashPaths(raw.FilePaths), 0, true
	}
	if len(raw.ImageBytes) > 0 {
		return ContentImage, "", hashBytes(raw.ImageBytes), int64(len(raw.ImageBytes)), true
	}
	text := raw.Text
	if text == "" {
		return "", "", "", 0, false
	}
	typ := ContentText
	if isHTTPURL(text) {
		typ = ContentURL
	} else if trimmedColor, ok := matchColorValue(text); ok {
		typ = ContentColor
		text = trimmedColor // store/hash the cleaned-up value, not incidental surrounding whitespace/tabs
	}
	return typ, text, HashText(text), int64(len([]byte(text))), true
}

// HashText preserves the copied text but normalises Windows line endings for deduplication.
func HashText(text string) string {
	return hashBytes([]byte(strings.ReplaceAll(text, "\r\n", "\n")))
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// hashPaths dedups a file/folder clipboard capture by the set of paths, independent of the order
// the OS reported them in.
func hashPaths(paths []string) string {
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	return hashBytes([]byte(strings.Join(sorted, "\n")))
}

func isHTTPURL(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed != value || trimmed == "" {
		return false
	}
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

var (
	hexColorPattern = regexp.MustCompile(`^#(?:[0-9A-Fa-f]{3}|[0-9A-Fa-f]{4}|[0-9A-Fa-f]{6}|[0-9A-Fa-f]{8})$`)
	// Optional leading "rgb"/"rgba" (case-insensitive); parens are optional and, when present, may
	// be ASCII or full-width (design tools and IME-typed input use either, and a bare "255, 0, 0"
	// with no parens at all is common too); separators accept ASCII or full-width commas with
	// arbitrary whitespace/tabs around them. The numeric ranges checked below (not expressible in
	// the regex itself) are what actually distinguish a color from arbitrary "(1, 2, 3)" text,
	// since \d{1,3} alone would also match out-of-range values like 999.
	rgbTuplePattern = regexp.MustCompile(`(?i)^(?:rgba?)?\s*[(（]?\s*(\d{1,3})\s*[,，]\s*(\d{1,3})\s*[,，]\s*(\d{1,3})\s*(?:[,，]\s*([\d.]+)\s*)?[)）]?\s*$`)
)

// matchColorValue reports whether value, once trimmed of surrounding whitespace/tabs, is exactly
// a hex color (#rgb/#rgba/#rrggbb/#rrggbbaa) or an (R, G, B)/(R, G, B, A) tuple — see
// rgbTuplePattern's comment for the accepted punctuation variants. Alpha is accepted either as
// 0-1 (CSS convention) or 0-255 (matching R/G/B's own range), since callers copy both. Anything
// that doesn't match falls back to plain ContentText in Parse — this never errors, only reports
// ok=false. Returns the trimmed value to use as the item's actual text/hash source in place of
// the untrimmed original, so stray whitespace doesn't end up in what's stored or dedupe-hashed.
func matchColorValue(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	if hexColorPattern.MatchString(trimmed) {
		return trimmed, true
	}
	m := rgbTuplePattern.FindStringSubmatch(trimmed)
	if m == nil {
		return "", false
	}
	for _, s := range m[1:4] {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 || n > 255 {
			return "", false
		}
	}
	if m[4] != "" {
		a, err := strconv.ParseFloat(m[4], 64)
		if err != nil || a < 0 || a > 255 {
			return "", false
		}
	}
	return trimmed, true
}

func CharCount(text string) int { return utf8.RuneCountInString(text) }

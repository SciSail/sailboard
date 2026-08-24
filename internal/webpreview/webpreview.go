// Package webpreview fetches a lightweight preview (title, favicon) for a copied URL. Per
// design doc §17: the URL itself is saved immediately on capture with no network access, and a
// preview is only fetched later, on demand, with a hard timeout so a slow or dead site can never
// block the clipboard watcher.
package webpreview

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Preview is what a successful (or partially successful) fetch produces. A zero-value Preview
// (every field empty) means "just show the URL", per §17's failure behaviour. Description and
// ImageURL are best-effort extras (og:description/meta description, og:image) — a page missing
// either simply leaves that field empty, same as a missing title or favicon.
type Preview struct {
	Title       string `json:"title"`
	FaviconURL  string `json:"faviconUrl"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
}

const (
	// Timeout is the hard cap on the whole fetch, per design doc §17.
	Timeout  = 3 * time.Second
	maxBytes = 128 * 1024 // enough for <head> on effectively every page
)

var httpClient = &http.Client{Timeout: Timeout}

// Fetch downloads pageURL and extracts a title and favicon URL. It never returns an error for a
// "successful but empty" page; it only errors when the request itself fails outright (bad URL,
// network error, non-2xx status), letting the caller fall back to showing the bare URL.
func Fetch(ctx context.Context, pageURL string) (Preview, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return Preview{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SailBoard/0.1; +clipboard-manager)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := httpClient.Do(req)
	if err != nil {
		return Preview{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Preview{}, &httpStatusError{pageURL: pageURL, status: resp.StatusCode}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil && len(body) == 0 {
		return Preview{}, err
	}
	html := string(body)

	return Preview{
		Title:       ExtractTitle(html),
		FaviconURL:  ExtractFaviconURL(html, pageURL),
		Description: ExtractDescription(html),
		ImageURL:    ExtractImageURL(html, pageURL),
	}, nil
}

type httpStatusError struct {
	pageURL string
	status  int
}

func (e *httpStatusError) Error() string {
	return "fetch " + e.pageURL + ": unexpected status " + http.StatusText(e.status)
}

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// ExtractTitle pulls the first <title> element's text, decoding the handful of HTML entities
// that show up in real-world titles and collapsing whitespace. Pure string logic, no network.
func ExtractTitle(html string) string {
	m := titleRe.FindStringSubmatch(html)
	if m == nil {
		return ""
	}
	return collapseWhitespace(decodeEntities(m[1]))
}

var (
	metaTagRe          = regexp.MustCompile(`(?is)<meta\s+[^>]*>`)
	metaContentAttrRe  = regexp.MustCompile(`(?is)content\s*=\s*["']([^"']*)["']`)
	ogDescriptionRe    = regexp.MustCompile(`(?is)(?:property|name)\s*=\s*["']og:description["']`)
	plainDescriptionRe = regexp.MustCompile(`(?is)(?:property|name)\s*=\s*["']description["']`)
	ogImageRe          = regexp.MustCompile(`(?is)(?:property|name)\s*=\s*["']og:image["']`)
)

// findMeta returns the first <meta> tag's content="..." attribute whose name/property matches
// keyRe, or "" if none is found. Shared by ExtractDescription/ExtractImageURL.
func findMeta(html string, keyRe *regexp.Regexp) string {
	for _, tag := range metaTagRe.FindAllString(html, -1) {
		if !keyRe.MatchString(tag) {
			continue
		}
		if m := metaContentAttrRe.FindStringSubmatch(tag); m != nil {
			return m[1]
		}
	}
	return ""
}

// ExtractDescription looks for <meta property="og:description" content="...">, falling back to
// the plain <meta name="description" content="...">. Pure string logic, no network.
func ExtractDescription(html string) string {
	if content := findMeta(html, ogDescriptionRe); content != "" {
		return collapseWhitespace(decodeEntities(content))
	}
	return collapseWhitespace(decodeEntities(findMeta(html, plainDescriptionRe)))
}

// ExtractImageURL looks for <meta property="og:image" content="...">, resolved against pageURL.
// Returns "" when absent — unlike ExtractFaviconURL, there's no reasonable fallback path to try
// for a preview image.
func ExtractImageURL(html string, pageURL string) string {
	content := findMeta(html, ogImageRe)
	if content == "" {
		return ""
	}
	base, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}
	resolved, err := base.Parse(decodeEntities(content))
	if err != nil {
		return ""
	}
	return resolved.String()
}

func collapseWhitespace(s string) string { return strings.Join(strings.Fields(s), " ") }

var iconLinkRe = regexp.MustCompile(`(?is)<link\s+[^>]*rel\s*=\s*["']?(?:shortcut icon|icon|apple-touch-icon)["']?[^>]*>`)
var hrefRe = regexp.MustCompile(`(?is)href\s*=\s*["']([^"']+)["']`)

// ExtractFaviconURL looks for a <link rel="icon"|"shortcut icon"|"apple-touch-icon"> tag and
// resolves its href against pageURL; falling back to the conventional /favicon.ico path when no
// such tag is present.
func ExtractFaviconURL(html string, pageURL string) string {
	base, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}
	if link := iconLinkRe.FindString(html); link != "" {
		if hrefMatch := hrefRe.FindStringSubmatch(link); hrefMatch != nil {
			if resolved, err := base.Parse(decodeEntities(hrefMatch[1])); err == nil {
				return resolved.String()
			}
		}
	}
	fallback := *base
	fallback.Path = "/favicon.ico"
	fallback.RawQuery = ""
	fallback.Fragment = ""
	return fallback.String()
}

var entityReplacer = strings.NewReplacer(
	"&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'", "&apos;", "'", "&nbsp;", " ",
)

func decodeEntities(s string) string {
	return entityReplacer.Replace(s)
}

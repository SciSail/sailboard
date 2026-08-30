package clipboard

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

var (
	cfHTMLStart = regexp.MustCompile(`(?im)^StartFragment:\s*(\d+)\s*\r?$`)
	cfHTMLEnd   = regexp.MustCompile(`(?im)^EndFragment:\s*(\d+)\s*\r?$`)
)

// MaterializeLocalImages externalizes local file:// and data: images from a
// CF_HTML payload into the injected asset store. Remote URLs are deliberately
// left untouched, so capture never performs network I/O.
func MaterializeLocalImages(payload string, store AssetStore) (string, []AssetRef, error) {
	return materializeLocalImages(payload, store, os.ReadFile)
}

func materializeLocalImages(payload string, store AssetStore, read func(string) ([]byte, error)) (string, []AssetRef, error) {
	if payload == "" || store == nil || read == nil {
		return payload, nil, nil
	}
	fragment, wrapped := extractCFHTMLFragment(payload)
	doc, err := html.Parse(strings.NewReader(fragment))
	if err != nil {
		return payload, nil, nil
	}
	var refs []AssetRef
	seen := map[string]bool{}
	var walk func(*html.Node) error
	walk = func(n *html.Node) error {
		if n.Type == html.ElementNode && (strings.EqualFold(n.Data, "img") || strings.EqualFold(n.Data, "imagedata")) {
			for i := range n.Attr {
				if !strings.EqualFold(n.Attr[i].Key, "src") {
					continue
				}
				src := strings.TrimSpace(n.Attr[i].Val)
				data, mimeType, ok := localImageBytes(src, read)
				if !ok || len(data) == 0 {
					continue
				}
				h := sha256.Sum256(data)
				hash := hex.EncodeToString(h[:])
				if !seen[hash] {
					ref, err := store.SaveAsset(data, mimeType)
					if err != nil {
						return err
					}
					if ref.Hash == "" {
						ref.Hash = hash
					}
					if ref.MIME == "" {
						ref.MIME = mimeType
					}
					if ref.ByteSize == 0 {
						ref.ByteSize = int64(len(data))
					}
					refs = append(refs, ref)
					seen[hash] = true
				}
				n.Attr[i].Val = assetPlaceholder(hash, src)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(doc); err != nil {
		return payload, nil, err
	}
	if len(refs) == 0 {
		return payload, nil, nil
	}
	fragment = renderBodyFragment(doc)
	if wrapped {
		return BuildCFHTML(fragment), refs, nil
	}
	return fragment, refs, nil
}

// HydrateLocalImages replaces internal asset placeholders with data URLs at
// paste time and rebuilds CF_HTML offsets. Missing assets fall back to the
// original source encoded in the placeholder.
func HydrateLocalImages(payload string, load func(hash string) ([]byte, string, error)) string {
	if payload == "" || load == nil {
		return payload
	}
	fragment, wrapped := extractCFHTMLFragment(payload)
	doc, err := html.Parse(strings.NewReader(fragment))
	if err != nil {
		return payload
	}
	changed := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for i := range n.Attr {
				if !strings.EqualFold(n.Attr[i].Key, "src") || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(n.Attr[i].Val)), "sailboard-asset://") {
					continue
				}
				hash, original, ok := parseAssetPlaceholder(n.Attr[i].Val)
				if !ok {
					continue
				}
				data, mimeType, loadErr := load(hash)
				if loadErr == nil && len(data) > 0 {
					n.Attr[i].Val = dataURL(data, mimeType)
					changed = true
				} else if original != "" {
					n.Attr[i].Val = original
					changed = true
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if !changed {
		return payload
	}
	fragment = renderBodyFragment(doc)
	if wrapped {
		return BuildCFHTML(fragment)
	}
	return fragment
}

func assetPlaceholder(hash, original string) string {
	return "sailboard-asset://" + hash + "?src=" + url.QueryEscape(original)
}

func parseAssetPlaceholder(value string) (string, string, bool) {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || !strings.EqualFold(u.Scheme, "sailboard-asset") {
		return "", "", false
	}
	hash := strings.TrimPrefix(u.Host+u.Path, "/")
	if len(hash) != 64 {
		return "", "", false
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return "", "", false
	}
	original, _ := url.QueryUnescape(u.Query().Get("src"))
	return hash, original, true
}

func localImageBytes(src string, read func(string) ([]byte, error)) ([]byte, string, bool) {
	lower := strings.ToLower(strings.TrimSpace(src))
	if strings.HasPrefix(lower, "data:") {
		data, mimeType, ok := decodeDataImageURL(src)
		return data, mimeType, ok
	}
	if !strings.HasPrefix(lower, "file://") {
		return nil, "", false
	}
	u, err := url.Parse(src)
	if err != nil || u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
		return nil, "", false
	}
	p, err := url.PathUnescape(u.Path)
	if err != nil {
		return nil, "", false
	}
	if len(p) > 2 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	if p == "" || filepath.IsAbs(p) == false {
		return nil, "", false
	}
	info, err := os.Stat(p)
	if err != nil || info.IsDir() || info.Size() > 16<<20 {
		return nil, "", false
	}
	data, err := read(p)
	if err != nil || len(data) == 0 {
		return nil, "", false
	}
	mimeType := sniffImageMIME(data)
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(p))
	}
	if !allowedImageMIME(mimeType) {
		return nil, "", false
	}
	return data, mimeType, true
}

func decodeDataImageURL(src string) ([]byte, string, bool) {
	rest := strings.TrimSpace(src[len("data:"):])
	meta, payload, ok := strings.Cut(rest, ",")
	if !ok {
		return nil, "", false
	}
	parts := strings.Split(meta, ";")
	mimeType := strings.ToLower(strings.TrimSpace(parts[0]))
	if !allowedImageMIME(mimeType) {
		return nil, "", false
	}
	// Bound the encoded representation before decoding so a hostile/accidental
	// data URL cannot allocate an unbounded temporary buffer. The decoded local
	// asset limit is 16 MiB; base64 expands by roughly 4/3.
	if len(payload) > (16<<20)*4/3+4 {
		return nil, "", false
	}
	if strings.EqualFold(parts[len(parts)-1], "base64") {
		b, err := base64.StdEncoding.DecodeString(payload)
		return b, mimeType, err == nil && len(b) <= 16<<20
	}
	decoded, err := url.PathUnescape(payload)
	return []byte(decoded), mimeType, err == nil && len(decoded) <= 16<<20
}

func allowedImageMIME(m string) bool {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp", "image/tiff":
		return true
	}
	return false
}

func sniffImageMIME(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png"
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "image/jpeg"
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return "image/gif"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp"
	case len(data) >= 2 && data[0] == 'B' && data[1] == 'M':
		return "image/bmp"
	}
	return ""
}

func dataURL(data []byte, mimeType string) string {
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func extractCFHTMLFragment(payload string) (string, bool) {
	var start, end int
	if m := cfHTMLStart.FindStringSubmatchIndex(payload); m != nil {
		start, _ = strconv.Atoi(payload[m[2]:m[3]])
	}
	if m := cfHTMLEnd.FindStringSubmatchIndex(payload); m != nil {
		end, _ = strconv.Atoi(payload[m[2]:m[3]])
	}
	if start > 0 && end > start && end <= len(payload) {
		return payload[start:end], true
	}
	return payload, false
}

func renderBodyFragment(doc *html.Node) string {
	body := doc
	var find func(*html.Node)
	find = func(n *html.Node) {
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "body") {
			body = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
	var b bytes.Buffer
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		_ = html.Render(&b, c)
	}
	return b.String()
}

// BuildCFHTML wraps a fragment with a valid byte-offset header.
func BuildCFHTML(fragment string) string {
	head := `<html><head><meta charset="utf-8"></head><body><!--StartFragment-->`
	tail := `<!--EndFragment--></body></html>`
	h0 := fmt.Sprintf("Version:0.9\r\nStartHTML:%010d\r\nEndHTML:%010d\r\nStartFragment:%010d\r\nEndFragment:%010d\r\n", 0, 0, 0, 0)
	startHTML := len(h0) + 2
	startFragment := startHTML + len(head)
	endFragment := startFragment + len(fragment)
	endHTML := endFragment + len(tail)
	h := fmt.Sprintf("Version:0.9\r\nStartHTML:%010d\r\nEndHTML:%010d\r\nStartFragment:%010d\r\nEndFragment:%010d\r\n", startHTML, endHTML, startFragment, endFragment)
	return h + "\r\n" + head + fragment + tail
}

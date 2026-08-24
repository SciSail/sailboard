package webpreview

import "testing"

func TestExtractTitleDecodesEntitiesAndCollapsesWhitespace(t *testing.T) {
	html := "<html><head>\n<title>  Foo &amp; Bar\n\t Baz  </title></head></html>"
	if got, want := ExtractTitle(html), "Foo & Bar Baz"; got != want {
		t.Fatalf("ExtractTitle() = %q, want %q", got, want)
	}
}

func TestExtractTitleMissing(t *testing.T) {
	if got := ExtractTitle("<html><head></head></html>"); got != "" {
		t.Fatalf("ExtractTitle() = %q, want empty", got)
	}
}

func TestExtractFaviconURLFromLinkTag(t *testing.T) {
	html := `<html><head><link rel="icon" href="/static/icon.png"></head></html>`
	got := ExtractFaviconURL(html, "https://example.com/page")
	want := "https://example.com/static/icon.png"
	if got != want {
		t.Fatalf("ExtractFaviconURL() = %q, want %q", got, want)
	}
}

func TestExtractFaviconURLFallsBackToWellKnownPath(t *testing.T) {
	got := ExtractFaviconURL("<html><head></head></html>", "https://example.com/page?x=1")
	want := "https://example.com/favicon.ico"
	if got != want {
		t.Fatalf("ExtractFaviconURL() = %q, want %q", got, want)
	}
}

func TestExtractFaviconURLAbsoluteHref(t *testing.T) {
	html := `<html><head><link rel="shortcut icon" href="https://cdn.example.com/icon.ico"></head></html>`
	got := ExtractFaviconURL(html, "https://example.com/page")
	want := "https://cdn.example.com/icon.ico"
	if got != want {
		t.Fatalf("ExtractFaviconURL() = %q, want %q", got, want)
	}
}

func TestExtractDescriptionPrefersOpenGraph(t *testing.T) {
	html := `<html><head>
		<meta name="description" content="Plain description">
		<meta property="og:description" content="OG &amp;  description  here">
	</head></html>`
	if got, want := ExtractDescription(html), "OG & description here"; got != want {
		t.Fatalf("ExtractDescription() = %q, want %q", got, want)
	}
}

func TestExtractDescriptionFallsBackToPlainMeta(t *testing.T) {
	html := `<html><head><meta name="description" content="Just the plain one"></head></html>`
	if got, want := ExtractDescription(html), "Just the plain one"; got != want {
		t.Fatalf("ExtractDescription() = %q, want %q", got, want)
	}
}

func TestExtractDescriptionMissing(t *testing.T) {
	if got := ExtractDescription("<html><head></head></html>"); got != "" {
		t.Fatalf("ExtractDescription() = %q, want empty", got)
	}
}

func TestExtractDescriptionAttributeOrderIndependent(t *testing.T) {
	html := `<html><head><meta content="Content comes first here" property="og:description"></head></html>`
	if got, want := ExtractDescription(html), "Content comes first here"; got != want {
		t.Fatalf("ExtractDescription() = %q, want %q", got, want)
	}
}

func TestExtractImageURLResolvesRelative(t *testing.T) {
	html := `<html><head><meta property="og:image" content="/static/preview.png"></head></html>`
	got := ExtractImageURL(html, "https://example.com/page")
	want := "https://example.com/static/preview.png"
	if got != want {
		t.Fatalf("ExtractImageURL() = %q, want %q", got, want)
	}
}

func TestExtractImageURLMissing(t *testing.T) {
	if got := ExtractImageURL("<html><head></head></html>", "https://example.com/page"); got != "" {
		t.Fatalf("ExtractImageURL() = %q, want empty", got)
	}
}

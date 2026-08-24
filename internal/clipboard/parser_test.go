package clipboard

import "testing"

func TestHashTextNormalisesOnlyCRLF(t *testing.T) {
	if HashText("one\r\ntwo") != HashText("one\ntwo") {
		t.Fatal("CRLF must deduplicate with LF")
	}
	if HashText("text") == HashText(" text") {
		t.Fatal("leading whitespace is meaningful")
	}
}

func TestParseRecognisesOnlyCompleteHTTPURL(t *testing.T) {
	typ, _, _, _, ok := Parse(RawContent{Text: "https://example.com/a"})
	if !ok || typ != ContentURL {
		t.Fatalf("got %v", typ)
	}
	typ, _, _, _, _ = Parse(RawContent{Text: "visit https://example.com"})
	if typ != ContentText {
		t.Fatalf("got %v", typ)
	}
}

func TestParseFilesHashIsOrderIndependent(t *testing.T) {
	typ, text, hashA, size, ok := Parse(RawContent{FilePaths: []string{`C:\a.txt`, `C:\b.txt`}})
	if !ok || typ != ContentFile {
		t.Fatalf("got type=%v ok=%v, want ContentFile", typ, ok)
	}
	if text != "a.txt, b.txt" {
		t.Fatalf("text = %q, want basenames joined", text)
	}
	if size != 0 {
		t.Fatalf("size = %d, want 0 (no bytes duplicated)", size)
	}

	_, _, hashB, _, _ := Parse(RawContent{FilePaths: []string{`C:\b.txt`, `C:\a.txt`}})
	if hashA != hashB {
		t.Fatal("hash must not depend on the order paths were reported in")
	}

	_, _, hashC, _, _ := Parse(RawContent{FilePaths: []string{`C:\a.txt`, `C:\c.txt`}})
	if hashA == hashC {
		t.Fatal("hash must change when the path set changes")
	}
}

func TestParseRecognisesColorValues(t *testing.T) {
	cases := []string{
		"#fff", "#FFF", "#ff0000", "#ff0000cc", "#abcd",
		"(255, 0, 0)", "(255,0,0)", "rgb(255, 0, 0)", "RGB(255, 0, 0)",
		"(255, 0, 0, 0.5)", "rgba(255, 0, 0, 0.5)", "(255, 0, 0, 255)",
	}
	for _, text := range cases {
		typ, gotText, _, _, ok := Parse(RawContent{Text: text})
		if !ok || typ != ContentColor {
			t.Errorf("Parse(%q) type = %v ok=%v, want ContentColor", text, typ, ok)
		}
		if gotText != text {
			t.Errorf("Parse(%q) text = %q, want the original text preserved verbatim", text, gotText)
		}
	}
}

func TestParseRecognisesLooseColorPunctuationAndWhitespace(t *testing.T) {
	cases := []struct{ in, want string }{
		{"（255，0，0）", "（255，0，0）"},                     // full-width parens and commas
		{"(255，0, 0)", "(255，0, 0)"},                   // mixed ASCII/full-width comma
		{"255, 0, 0", "255, 0, 0"},                     // no parens at all
		{"255，0，0", "255，0，0"},                         // no parens, full-width commas
		{"  #FF0000  ", "#FF0000"},                     // surrounding spaces trimmed
		{"\t(255, 0, 0)\t", "(255, 0, 0)"},             // surrounding tabs trimmed
		{"rgba(255, 0, 0，0.5)", "rgba(255, 0, 0，0.5)"}, // mixed comma before alpha
	}
	for _, c := range cases {
		typ, gotText, _, _, ok := Parse(RawContent{Text: c.in})
		if !ok || typ != ContentColor {
			t.Errorf("Parse(%q) type = %v ok=%v, want ContentColor", c.in, typ, ok)
			continue
		}
		if gotText != c.want {
			t.Errorf("Parse(%q) text = %q, want %q (surrounding whitespace trimmed, punctuation preserved)", c.in, gotText, c.want)
		}
	}
}

func TestParseRejectsNonColorLookalikes(t *testing.T) {
	cases := []string{
		"#ff",             // wrong hex digit count
		"#gggggg",         // not hex digits
		"(999, 0, 0)",     // out of 0-255 range
		"(255, 0)",        // too few components
		"just some text",  // not a color at all
		"rgb 255, 0, 0 5", // missing parens/commas entirely, extra token
	}
	for _, text := range cases {
		typ, gotText, _, _, _ := Parse(RawContent{Text: text})
		if typ == ContentColor {
			t.Errorf("Parse(%q) classified as ContentColor, want anything else", text)
		}
		// Falling back to plain text must leave the content usable, not erroring or discarding it.
		if gotText != text {
			t.Errorf("Parse(%q) fell back with text = %q, want the original text unchanged", text, gotText)
		}
	}
}

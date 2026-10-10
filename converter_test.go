package main

import (
	"html"
	"strings"
	"testing"
)

func TestDetectParagraphs(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		want        int
	}{
		{"blank lines", "First.\n\nSecond.\n\n\nThird.", 3},
		{"windows", "First.\r\n\r\nSecond.", 2},
		{"unix", "First.\n\nSecond.", 2},
		{"wrapped", "This is a wrapped\nline of text.", 1},
		{"sentence line breaks", "First sentence.\nSecond sentence.", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := detectParagraphs(tc.input)
			if len(got) != tc.want {
				t.Fatalf("got %d paragraphs: %#v", len(got), got)
			}
		})
	}
}

func TestSplitLongParagraphs(t *testing.T) {
	words := make([]string, 1200)
	for i := range words {
		words[i] = "word"
	}
	p := strings.Join(words, " ")
	parts := splitLongParagraphs([]string{p}, 512)
	if len(parts) < 3 {
		t.Fatalf("expected chunks, got %d", len(parts))
	}
	for _, part := range parts {
		if wordCount(part) > 512 {
			t.Fatalf("chunk has %d words", wordCount(part))
		}
	}
	if strings.Join(parts, "") != p {
		t.Fatal("text changed during split")
	}
}

func TestSplitLongParagraphsAtPeriods(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 600; i++ {
		b.WriteString("one two three.")
		if i < 599 {
			b.WriteByte(' ')
		}
	}
	parts := splitLongParagraphs([]string{b.String()}, 512)
	for _, p := range parts {
		if wordCount(p) > 512 {
			t.Fatalf("too many words: %d", wordCount(p))
		}
	}
	if strings.Join(parts, "") != b.String() {
		t.Fatal("text changed")
	}
}

func TestHTMLAndEdgeCases(t *testing.T) {
	if got := detectParagraphs(""); len(got) != 0 {
		t.Fatalf("empty input: %#v", got)
	}
	unicodeText := "こんにちは世界\n次の行"
	parts := detectParagraphs(unicodeText)
	if len(parts) != 1 || strings.ReplaceAll(parts[0], " ", "\n") != unicodeText {
		t.Fatalf("unicode lost: %#v", parts)
	}
	out := toHTML("x&y", []string{"<script>alert('&')</script>"})
	if strings.Contains(out, "<script>") || !strings.Contains(out, html.EscapeString("<script>alert('&')</script>")) || !strings.Contains(out, "x&amp;y") {
		t.Fatalf("not escaped: %s", out)
	}
}

func TestDecodeTextWindows1252AndUTF8(t *testing.T) {
	if got := decodeText([]byte{0x63, 0x61, 0x66, 0xe9, 0x20, 0x80}); got != "café €" {
		t.Fatalf("Windows-1252 decode = %q", got)
	}
	if got := decodeText([]byte("Zażółć 世界")); got != "Zażółć 世界" {
		t.Fatalf("UTF-8 changed: %q", got)
	}
}

package main

import (
	"html"
	"strings"
	"unicode"
	"unicode/utf8"
)

var windows1252High = [32]rune{
	'\u20ac', '\u0081', '\u201a', '\u0192', '\u201e', '\u2026', '\u2020', '\u2021',
	'\u02c6', '\u2030', '\u0160', '\u2039', '\u0152', '\u008d', '\u017d', '\u008f',
	'\u0090', '\u2018', '\u2019', '\u201c', '\u201d', '\u2022', '\u2013', '\u2014',
	'\u02dc', '\u2122', '\u0161', '\u203a', '\u0153', '\u009d', '\u017e', '\u0178',
}

func decodeText(data []byte) string {
	if utf8.Valid(data) {
		return strings.TrimPrefix(string(data), "\uFEFF")
	}
	var decoded strings.Builder
	decoded.Grow(len(data))
	for _, b := range data {
		r := rune(b)
		if b >= 0x80 && b <= 0x9f {
			r = windows1252High[b-0x80]
		}
		decoded.WriteRune(r)
	}
	return decoded.String()
}

func decodeUploadedText(data []byte, preserveBytes bool) string {
	if preserveBytes {
		return string(data)
	}
	return decodeText(data)
}

func detectParagraphs(input string) []string {
	s := strings.ReplaceAll(strings.ReplaceAll(input, "\r\n", "\n"), "\r", "\n")
	if strings.TrimSpace(s) == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	var paragraphs []string
	var current []string
	flush := func() {
		p := strings.TrimSpace(strings.Join(current, " "))
		if p != "" {
			paragraphs = append(paragraphs, p)
		}
		current = nil
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		line = strings.TrimSpace(line)
		// A line ending in a sentence period is a useful boundary; other line breaks
		// are treated as visual wrapping and joined into the same paragraph.
		if len(current) > 0 && endsSentence(current[len(current)-1]) {
			flush()
		}
		current = append(current, line)
	}
	flush()
	return paragraphs
}

func endsSentence(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasSuffix(s, ".") || strings.HasSuffix(s, "!") || strings.HasSuffix(s, "?")
}

// splitLongParagraphs preserves every source byte except whitespace used as a
// split separator; the separator is retained at the start of the next chunk.
func splitLongParagraphs(paragraphs []string, maxWords int) []string {
	if maxWords < 1 {
		maxWords = 300
	}
	var result []string
	for _, p := range paragraphs {
		for wordCount(p) > maxWords {
			end := wordEndAt(p, maxWords)
			cut := strings.LastIndex(p[:end], ".")
			if cut >= 0 && (cut+1 == len(p) || unicode.IsSpace(rune(p[cut+1]))) {
				end = cut + 1
			}
			if end <= 0 {
				end = wordEndAt(p, maxWords)
			}
			result = append(result, p[:end])
			p = p[end:]
		}
		if strings.TrimSpace(p) != "" {
			result = append(result, p)
		}
	}
	return result
}

func wordCount(s string) int {
	count := 0
	in := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			in = false
		} else if !in {
			count++
			in = true
		}
	}
	return count
}

func wordEndAt(s string, limit int) int {
	count := 0
	in := false
	for i, r := range s {
		if unicode.IsSpace(r) {
			in = false
		} else if !in {
			count++
			in = true
			if count > limit {
				return i
			}
		}
	}
	return len(s)
}

func toHTML(title string, paragraphs []string) string {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title>\n<style>body { font-family: Arial, Helvetica, sans-serif; font-size: 1.05em; padding-left: 2em; padding-right: 2em; }</style>\n</head>\n<body>\n")
	for i, p := range paragraphs {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("<p>")
		b.WriteString(html.EscapeString(p))
		b.WriteString("</p>")
	}
	b.WriteString("\n</body>\n</html>\n")
	return b.String()
}

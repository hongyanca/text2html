package main

import (
	"html"
	"strings"
	"unicode"
)

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
	b.WriteString("</title>\n<style>body { font-family: Georgia, serif; font-size: 1.2em; }</style>\n</head>\n<body>\n")
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

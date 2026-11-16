package feed

import (
	"html"
	"regexp"
	"strings"
)

var (
	scriptRe   = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(script|style)\s*>`)
	brRe       = regexp.MustCompile(`(?i)<br\s*/?>`)
	blockEndRe = regexp.MustCompile(`(?i)</(p|div|li|h[1-6]|blockquote|tr|pre|ul|ol|table)\s*>`)
	tagRe      = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceRe    = regexp.MustCompile(`[ \t\r\f\v]+`)
	nlRe       = regexp.MustCompile(`\n{3,}`)
)

// HTMLToText converts an HTML fragment to plain text, keeping paragraph
// breaks, and truncates the result to max runes (max <= 0 = no limit).
func HTMLToText(s string, max int) string {
	s = scriptRe.ReplaceAllString(s, "")
	s = brRe.ReplaceAllString(s, "\n")
	s = blockEndRe.ReplaceAllString(s, "\n\n")
	s = tagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")

	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(spaceRe.ReplaceAllString(l, " "))
	}
	s = strings.Join(lines, "\n")
	s = nlRe.ReplaceAllString(s, "\n\n")
	s = strings.TrimSpace(s)

	if max > 0 {
		if r := []rune(s); len(r) > max {
			s = strings.TrimSpace(string(r[:max]))
		}
	}
	return s
}

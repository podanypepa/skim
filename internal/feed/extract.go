package feed

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	dropTags  = []string{"script", "style", "nav", "header", "footer", "aside", "form", "noscript", "svg"}
	dropRes   = make([]*regexp.Regexp, len(dropTags))
	regionRes = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<article\b[^>]*>(.*?)</article\s*>`),
		regexp.MustCompile(`(?is)<main\b[^>]*>(.*?)</main\s*>`),
		regexp.MustCompile(`(?is)<body\b[^>]*>(.*?)</body\s*>`),
	}
	textElemRe = regexp.MustCompile(`(?is)<(?:p|h[1-4]|li|pre|blockquote)\b[^>]*>(.*?)</(?:p|h[1-4]|li|pre|blockquote)\s*>`)
)

// minRegionText is how much text a region needs to be taken as the article.
const minRegionText = 300

func regionText(region string) string {
	var parts []string
	for _, m := range textElemRe.FindAllStringSubmatch(region, -1) {
		if t := HTMLToText(m[1], 0); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n\n")
}

func init() {
	for i, t := range dropTags {
		dropRes[i] = regexp.MustCompile(`(?is)<` + t + `\b[^>]*>.*?</` + t + `\s*>`)
	}
}

// ExtractText pulls the readable article text out of a web page: it drops
// navigation and boilerplate, picks <article>, else <main>, else <body>,
// and joins its paragraphs, headings, list items, code and quotes.
// The result is truncated to max runes (max <= 0 = no limit).
func ExtractText(html string, max int) string {
	for _, re := range dropRes {
		html = re.ReplaceAllString(html, " ")
	}
	// Try <article>, <main>, <body>, whole page in order; the first region
	// with enough text wins (a tiny <article> card must not hide <main>),
	// otherwise the longest result.
	regions := []string{}
	for _, re := range regionRes {
		if m := re.FindStringSubmatch(html); m != nil {
			regions = append(regions, m[1])
		}
	}
	regions = append(regions, html)
	out := ""
	for _, region := range regions {
		t := regionText(region)
		if utf8.RuneCountInString(t) >= minRegionText {
			out = t
			break
		}
		if len(t) > len(out) {
			out = t
		}
	}
	if max > 0 {
		if r := []rune(out); len(r) > max {
			out = strings.TrimSpace(string(r[:max]))
		}
	}
	return out
}

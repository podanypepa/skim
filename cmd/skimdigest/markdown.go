package main

import (
	"fmt"
	"strings"
	"time"
)

// renderMarkdown renders a digest with the fixed template; all links come
// from the archive (Ref), never from the skill's input.
func renderMarkdown(d Digest) string {
	var b strings.Builder
	date := d.Day
	if t, err := time.Parse("2006-01-02", d.Day); err == nil {
		date = fmt.Sprintf("%d. %d. %d", t.Day(), int(t.Month()), t.Year())
	}
	var topics []string
	seen := map[string]bool{}
	articles := len(d.More)
	for _, s := range d.Stories {
		articles += len(s.Articles)
		if !seen[s.Topic] {
			seen[s.Topic] = true
			topics = append(topics, s.Topic)
		}
	}
	fmt.Fprintf(&b, "# Skim – %s\n\n", date)
	fmt.Fprintf(&b, "%d %s z %d %s · %s\n",
		len(d.Stories), plural(len(d.Stories), "příběh", "příběhy", "příběhů"),
		articles, plural(articles, "článku", "článků", "článků"),
		strings.Join(topics, " · "))

	for _, topic := range topics {
		fmt.Fprintf(&b, "\n## %s\n", topic)
		for _, s := range d.Stories {
			if s.Topic != topic {
				continue
			}
			fmt.Fprintf(&b, "\n### %d. %s `%d`\n%s\n\n", s.N, s.Title, s.Score, s.Summary)
			links := make([]string, len(s.Articles))
			for i, a := range s.Articles {
				links[i] = link(a.FeedTitle+" – "+a.Title, a.URL)
			}
			b.WriteString(strings.Join(links, " · ") + "\n")
		}
	}
	if len(d.More) > 0 {
		b.WriteString("\n## Další zprávy\n\n")
		for _, a := range d.More {
			fmt.Fprintf(&b, "- %s – %s\n", link(a.Title, a.URL), a.FeedTitle)
		}
	}
	return b.String()
}

var linkTextEscaper = strings.NewReplacer(
	`\`, `\\`, `[`, `\[`, `]`, `\]`, "`", "\\`",
	`<`, `\<`, `>`, `\>`, `*`, `\*`, `_`, `\_`,
)

// link builds a markdown link that survives brackets in the text and
// spaces or parentheses in the URL.
func link(text, url string) string {
	text = linkTextEscaper.Replace(text)
	if strings.ContainsAny(url, " ()<>") {
		url = "<" + strings.NewReplacer("<", "%3C", ">", "%3E").Replace(url) + ">"
	}
	return "[" + text + "](" + url + ")"
}

// plural picks the Czech form for n: one (1), few (2–4, not 12–14), many.
func plural(n int, one, few, many string) string {
	switch {
	case n == 1:
		return one
	case n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14):
		return few
	default:
		return many
	}
}

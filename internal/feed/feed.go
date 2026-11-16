// Package feed parses RSS 2.0, RSS 1.0 (RDF) and Atom feeds into a common
// structure. It uses only the standard library.
package feed

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// Feed is a parsed feed document.
type Feed struct {
	Format string // "rss" | "atom"
	Title  string
	Site   string
	Items  []Item
}

// Item is one entry of a feed.
type Item struct {
	GUID      string    // RSS <guid>, Atom <id>
	Title     string    // whitespace-normalized
	Link      string    // RSS <link>; Atom <link rel="alternate"|"">
	Author    string    // RSS <author> / dc:creator; Atom author/name
	Content   string    // raw HTML: content:encoded > description; Atom content > summary
	Published time.Time // pubDate > dc:date; Atom published > updated; zero = unknown
}

// nsText collects all elements with a given local name; get() keeps only
// those in the RSS namespace (none for RSS 2.0, RSS 1.0 for RDF), so that
// e.g. <media:title> or <itunes:author> never override the real field.
type nsText []struct {
	XMLName xml.Name
	V       string `xml:",chardata"`
}

const nsRSS1 = "http://purl.org/rss/1.0/"

func (t nsText) get() string {
	for _, e := range t {
		if e.XMLName.Space != "" && e.XMLName.Space != nsRSS1 {
			continue
		}
		if v := strings.TrimSpace(e.V); v != "" {
			return v
		}
	}
	return ""
}

type rssDoc struct {
	Channel struct {
		Title nsText    `xml:"title"`
		Links nsText    `xml:"link"`
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
	Items []rssItem `xml:"item"` // RSS 1.0 puts items next to channel
}

type rssItem struct {
	Title       nsText `xml:"title"`
	GUID        nsText `xml:"guid"`
	Links       nsText `xml:"link"`
	Author      nsText `xml:"author"`
	Creator     string `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Description nsText `xml:"description"`
	Encoded     string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	PubDate     nsText `xml:"pubDate"`
	DCDate      string `xml:"http://purl.org/dc/elements/1.1/ date"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

type atomText struct {
	Type  string `xml:"type,attr"`
	Text  string `xml:",chardata"`
	Inner string `xml:",innerxml"`
}

func (t *atomText) value() string {
	if t == nil {
		return ""
	}
	if strings.EqualFold(t.Type, "xhtml") {
		return strings.TrimSpace(t.Inner)
	}
	return strings.TrimSpace(t.Text)
}

// Atom fields carry the Atom namespace explicitly so that extension
// elements with the same local name (media:title, …) are ignored.
type atomDoc struct {
	Title   string     `xml:"http://www.w3.org/2005/Atom title"`
	Links   []atomLink `xml:"http://www.w3.org/2005/Atom link"`
	Entries []struct {
		ID     string     `xml:"http://www.w3.org/2005/Atom id"`
		Title  string     `xml:"http://www.w3.org/2005/Atom title"`
		Links  []atomLink `xml:"http://www.w3.org/2005/Atom link"`
		Author struct {
			Name string `xml:"http://www.w3.org/2005/Atom name"`
		} `xml:"http://www.w3.org/2005/Atom author"`
		Summary   *atomText `xml:"http://www.w3.org/2005/Atom summary"`
		Content   *atomText `xml:"http://www.w3.org/2005/Atom content"`
		Updated   string    `xml:"http://www.w3.org/2005/Atom updated"`
		Published string    `xml:"http://www.w3.org/2005/Atom published"`
	} `xml:"http://www.w3.org/2005/Atom entry"`
}

// Parse parses an RSS 2.0, RSS 1.0 or Atom document.
func Parse(body []byte) (Feed, error) {
	root, err := rootElement(body)
	if err != nil {
		return Feed{}, err
	}
	switch strings.ToLower(root) {
	case "feed":
		return parseAtom(body)
	case "rss", "rdf":
		return parseRSS(body)
	default:
		return Feed{}, fmt.Errorf("not a feed: root element <%s>", root)
	}
}

func rootElement(body []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.Strict = false
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", errors.New("not a feed: no root element")
			}
			return "", fmt.Errorf("xml: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se.Name.Local, nil
		}
	}
}

func parseRSS(body []byte) (Feed, error) {
	var d rssDoc
	if err := xml.Unmarshal(body, &d); err != nil {
		return Feed{}, fmt.Errorf("rss parse: %w", err)
	}
	f := Feed{Format: "rss", Title: Clean(d.Channel.Title.get()), Site: d.Channel.Links.get()}
	for _, it := range append(d.Channel.Items, d.Items...) {
		date := it.PubDate.get()
		if date == "" {
			date = it.DCDate
		}
		content := strings.TrimSpace(it.Encoded)
		if content == "" {
			content = it.Description.get()
		}
		author := strings.TrimSpace(it.Creator)
		if author == "" {
			author = it.Author.get()
		}
		f.Items = append(f.Items, Item{
			GUID:      it.GUID.get(),
			Title:     Clean(it.Title.get()),
			Link:      it.Links.get(),
			Author:    Clean(author),
			Content:   content,
			Published: ParseDate(date),
		})
	}
	return f, nil
}

func parseAtom(body []byte) (Feed, error) {
	var a atomDoc
	if err := xml.Unmarshal(body, &a); err != nil {
		return Feed{}, fmt.Errorf("atom parse: %w", err)
	}
	f := Feed{Format: "atom", Title: Clean(a.Title), Site: alternate(a.Links)}
	for _, e := range a.Entries {
		date := e.Published
		if strings.TrimSpace(date) == "" {
			date = e.Updated
		}
		content := e.Content.value()
		if content == "" {
			content = e.Summary.value()
		}
		f.Items = append(f.Items, Item{
			GUID:      strings.TrimSpace(e.ID),
			Title:     Clean(e.Title),
			Link:      alternate(e.Links),
			Author:    Clean(e.Author.Name),
			Content:   content,
			Published: ParseDate(date),
		})
	}
	return f, nil
}

func alternate(links []atomLink) string {
	for _, l := range links {
		if (l.Rel == "" || l.Rel == "alternate") && strings.TrimSpace(l.Href) != "" {
			return strings.TrimSpace(l.Href)
		}
	}
	return ""
}

// LooksLikeFeed reports whether a response body / content type is a feed.
func LooksLikeFeed(body []byte, contentType string) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "rss") || strings.Contains(ct, "atom") {
		return true
	}
	head := strings.ToLower(string(body[:min(len(body), 2048)]))
	return strings.Contains(head, "<rss") || strings.Contains(head, "<feed") || strings.Contains(head, "<rdf:rdf")
}

var dateLayouts = []string{
	time.RFC1123Z, time.RFC1123, time.RFC3339, time.RFC3339Nano,
	"Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST",
	"Mon, 02 Jan 2006 15:04 -0700", "2 Jan 2006 15:04:05 -0700",
	"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02",
}

// ParseDate parses common feed date formats; zero time if unknown.
func ParseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

var wsRe = regexp.MustCompile(`\s+`)

// Clean collapses whitespace and trims.
func Clean(s string) string { return strings.TrimSpace(wsRe.ReplaceAllString(s, " ")) }

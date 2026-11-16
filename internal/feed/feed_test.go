package feed

import (
	"testing"
	"time"
)

const rssFull = `<?xml version="1.0"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:dc="http://purl.org/dc/elements/1.1/">
<channel><title>T</title><link>https://e.com</link>
<item>
  <title>  First
   item </title>
  <guid>g1</guid>
  <link>https://e.com/1</link>
  <dc:creator>Jana</dc:creator>
  <description>short</description>
  <content:encoded><![CDATA[<p>full</p>]]></content:encoded>
  <pubDate>Mon, 09 Nov 2026 10:00:00 +0000</pubDate>
</item>
</channel></rss>`

func TestParseRSSFull(t *testing.T) {
	f, err := Parse([]byte(rssFull))
	if err != nil {
		t.Fatal(err)
	}
	if f.Format != "rss" || f.Title != "T" || f.Site != "https://e.com" || len(f.Items) != 1 {
		t.Fatalf("bad feed: %+v", f)
	}
	it := f.Items[0]
	if it.GUID != "g1" || it.Link != "https://e.com/1" || it.Author != "Jana" || it.Content != "<p>full</p>" || it.Title != "First item" {
		t.Fatalf("bad item: %+v", it)
	}
	if !it.Published.Equal(time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("bad date: %v", it.Published)
	}
}

func TestParseRSSDescriptionFallback(t *testing.T) {
	f, err := Parse([]byte(`<rss><channel><title>T</title><item><title>a</title><author>x@y.z (X)</author><description>short</description></item></channel></rss>`))
	if err != nil {
		t.Fatal(err)
	}
	if f.Items[0].Content != "short" || f.Items[0].Author != "x@y.z (X)" {
		t.Fatalf("bad item: %+v", f.Items[0])
	}
}

func TestParseRDF(t *testing.T) {
	doc := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns="http://purl.org/rss/1.0/" xmlns:dc="http://purl.org/dc/elements/1.1/">
<channel><title>R</title><link>https://r.com</link></channel>
<item><title>r1</title><link>https://r.com/1</link><dc:date>2026-11-10T08:00:00Z</dc:date></item>
</rdf:RDF>`
	f, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Items) != 1 || f.Items[0].Link != "https://r.com/1" {
		t.Fatalf("bad feed: %+v", f)
	}
	if !f.Items[0].Published.Equal(time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("bad date: %v", f.Items[0].Published)
	}
}

func TestParseAtom(t *testing.T) {
	doc := `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>A</title>
<link href="https://a.com/" rel="alternate"/>
<entry>
  <id>tag:x,1</id><title>e1</title>
  <link rel="self" href="https://a.com/self"/>
  <link rel="alternate" href="https://a.com/1"/>
  <author><name>Petr</name></author>
  <summary>s</summary>
  <content type="html">&lt;b&gt;c&lt;/b&gt;</content>
  <updated>2026-11-15T08:00:00Z</updated>
</entry>
<entry><id>tag:x,2</id><title>e2</title><link href="https://a.com/2"/><summary>only summary</summary><published>2026-11-01T08:00:00Z</published><updated>2026-11-02T08:00:00Z</updated></entry>
</feed>`
	f, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if f.Format != "atom" || f.Title != "A" || f.Site != "https://a.com/" || len(f.Items) != 2 {
		t.Fatalf("bad feed: %+v", f)
	}
	e := f.Items[0]
	if e.GUID != "tag:x,1" || e.Link != "https://a.com/1" || e.Author != "Petr" || e.Content != "<b>c</b>" {
		t.Fatalf("bad entry: %+v", e)
	}
	e2 := f.Items[1]
	if e2.Link != "https://a.com/2" || e2.Content != "only summary" || !e2.Published.Equal(time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("bad entry2: %+v", e2)
	}
}

func TestParseGarbage(t *testing.T) {
	if _, err := Parse([]byte("<html>")); err == nil {
		t.Fatal("expected error")
	}
}

func TestNamespacedElementsDoNotOverride(t *testing.T) {
	rssDocText := `<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
<channel><title>Chan</title><itunes:title>iChan</itunes:title><link>https://e.com</link>
<item><title>Real</title><media:title>Media</media:title><author>a@b.c</author><itunes:author>iAuth</itunes:author>
<description>desc</description><media:description>mdesc</media:description><link>https://e.com/1</link></item>
</channel></rss>`
	f, err := Parse([]byte(rssDocText))
	if err != nil {
		t.Fatal(err)
	}
	it := f.Items[0]
	if f.Title != "Chan" || it.Title != "Real" || it.Author != "a@b.c" || it.Content != "desc" {
		t.Fatalf("namespaced override: feed %q item %+v", f.Title, it)
	}
	atomText := `<feed xmlns="http://www.w3.org/2005/Atom" xmlns:media="http://search.yahoo.com/mrss/"><title>A</title>
<entry><id>1</id><title>Real</title><media:title>Media</media:title><link href="https://a/1"/></entry></feed>`
	fa, err := Parse([]byte(atomText))
	if err != nil {
		t.Fatal(err)
	}
	if fa.Items[0].Title != "Real" {
		t.Fatalf("atom namespaced override: %+v", fa.Items[0])
	}
}

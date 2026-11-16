package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const rssSample = `<?xml version="1.0"?><rss version="2.0"><channel><title>Test RSS</title><link>https://example.com</link>
<item><title>Newest</title><pubDate>Mon, 09 Nov 2026 10:00:00 +0000</pubDate></item>
<item><title>Older</title><pubDate>Mon, 02 Nov 2026 10:00:00 +0000</pubDate></item>
<item><title>Oldest</title><pubDate>Mon, 26 Oct 2026 10:00:00 +0000</pubDate></item>
</channel></rss>`

const atomSample = `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>Test Atom</title>
<link href="https://atom.example.com/" rel="alternate"/>
<entry><title>A1</title><updated>2026-11-15T08:00:00Z</updated></entry>
<entry><title>A2</title><published>2026-11-01T08:00:00Z</published></entry>
</feed>`

func server() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/rss.xml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(rssSample))
	})
	mux.HandleFunc("/atom", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(atomSample))
	})
	mux.HandleFunc("/site", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><link rel="alternate" type="application/atom+xml" href="/atom"></head></html>`))
	})
	mux.HandleFunc("/plain", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head></head></html>`))
	})
	return httptest.NewServer(mux)
}

var now = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

func TestRSS(t *testing.T) {
	s := server()
	defer s.Close()
	r := check(s.URL+"/rss.xml", 2, now)
	if !r.OK || r.Format != "rss" || r.Title != "Test RSS" || r.Items != 3 {
		t.Fatalf("bad result: %+v", r)
	}
	if r.Latest != "2026-11-09" || *r.DaysSince != 7 || r.PostsPerWeek != 1 {
		t.Fatalf("bad stats: %+v", r)
	}
	if len(r.Samples) != 2 || r.Samples[0] != "Newest" {
		t.Fatalf("bad samples: %v", r.Samples)
	}
}

func TestAutodiscoveryAtom(t *testing.T) {
	s := server()
	defer s.Close()
	r := check(s.URL+"/site", 5, now)
	if !r.OK || r.Format != "atom" || r.FeedURL != s.URL+"/atom" || r.Samples[0] != "A1" {
		t.Fatalf("bad result: %+v", r)
	}
}

func TestCommonPathFallback(t *testing.T) {
	s := server()
	defer s.Close()
	r := check(s.URL+"/plain", 5, now)
	// /plain has no <link>, but /rss.xml exists at the site root.
	if !r.OK || r.FeedURL != s.URL+"/rss.xml" {
		t.Fatalf("bad result: %+v", r)
	}
}

func TestNotFound(t *testing.T) {
	s := server()
	defer s.Close()
	r := check(s.URL+"/missing", 5, now)
	if r.OK || r.Error == "" {
		t.Fatalf("expected error: %+v", r)
	}
}

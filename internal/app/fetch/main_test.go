package fetch

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/podanypepa/skim/internal/store"
)

var now = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

// fakeServer serves per-path handlers that tests can swap between runs.
type fakeServer struct {
	*httptest.Server
	mu       sync.Mutex
	handlers map[string]http.HandlerFunc
	headers  map[string]http.Header // last request headers per path
}

func newServer(t *testing.T) *fakeServer {
	fs := &fakeServer{handlers: map[string]http.HandlerFunc{}, headers: map[string]http.Header{}}
	fs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.mu.Lock()
		h := fs.handlers[r.URL.Path]
		fs.headers[r.URL.Path] = r.Header.Clone()
		fs.mu.Unlock()
		if h == nil {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}))
	t.Cleanup(fs.Close)
	return fs
}

func (fs *fakeServer) set(path string, h http.HandlerFunc) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.handlers[path] = h
}

func (fs *fakeServer) lastHeader(path, key string) string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.headers[path].Get(key)
}

func serveXML(body string, hdr map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for k, v := range hdr {
			w.Header().Set(k, v)
		}
		if et := hdr["ETag"]; et != "" && r.Header.Get("If-None-Match") == et {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(body))
	}
}

func rss(items ...string) string {
	return `<?xml version="1.0"?><rss version="2.0"><channel><title>Feed</title><link>https://e.com</link>` +
		strings.Join(items, "") + `</channel></rss>`
}

func item(guid, title string, age time.Duration) string {
	g := ""
	if guid != "" {
		g = "<guid>" + guid + "</guid>"
	}
	return fmt.Sprintf(`<item>%s<title>%s</title><link>https://e.com/%s</link><description>&lt;p&gt;Body %s&lt;/p&gt;</description><pubDate>%s</pubDate></item>`,
		g, title, title, title, now.Add(-age).Format(time.RFC1123Z))
}

const day = 24 * time.Hour

type feedDef struct{ url, topic string }

func setup(t *testing.T, feeds ...feedDef) Options {
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("feeds:\n")
	for i, f := range feeds {
		fmt.Fprintf(&b, "  - title: \"Feed %d\"\n    url: \"%s\"\n    topic: %s\n", i, f.url, f.topic)
	}
	path := filepath.Join(dir, "feeds.yaml")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return Options{
		FeedsPath:    path,
		DataDir:      filepath.Join(dir, "data"),
		BackfillDays: 7,
		Now:          now,
		Client:       &http.Client{Timeout: 5 * time.Second},
	}
}

func run(t *testing.T, opts Options) (Summary, []FeedResult) {
	t.Helper()
	var out bytes.Buffer
	sum, err := Run(opts, &out)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var results []FeedResult
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	for _, l := range lines[:len(lines)-1] {
		var r FeedResult
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatalf("bad line %q: %v", l, err)
		}
		results = append(results, r)
	}
	var last Summary
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil || !last.Summary {
		t.Fatalf("bad summary line %q: %v", lines[len(lines)-1], err)
	}
	if last != sum {
		t.Fatalf("printed summary %+v != returned %+v", last, sum)
	}
	return sum, results
}

func readArticles(t *testing.T, opts Options) []map[string]any {
	t.Helper()
	f, err := os.Open(filepath.Join(opts.DataDir, "articles", "2026-11-16.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var arts []map[string]any
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var a map[string]any
		if err := json.Unmarshal(sc.Bytes(), &a); err != nil {
			t.Fatal(err)
		}
		arts = append(arts, a)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return arts
}

func TestFirstRunAndDedup(t *testing.T) {
	s := newServer(t)
	s.set("/a", serveXML(rss(item("1", "one", day), item("2", "two", 2*day), item("3", "three", 3*day)), nil))
	opts := setup(t, feedDef{s.URL + "/a", "ai"})

	sum, res := run(t, opts)
	if sum.New != 3 || res[0].Status != "new" || !res[0].OK || res[0].Topic != "ai" {
		t.Fatalf("run1: %+v %+v", sum, res)
	}
	arts := readArticles(t, opts)
	if len(arts) != 3 {
		t.Fatalf("want 3 articles, got %d", len(arts))
	}
	a := arts[0]
	if a["title"] != "one" || a["url"] != "https://e.com/one" || a["content"] != "Body one" ||
		a["feed_title"] != "Feed 0" || a["topic"] != "ai" || a["fetched_at"] != now.Format(time.RFC3339) || a["published"] == nil {
		t.Fatalf("bad article: %v", a)
	}

	sum, res = run(t, opts)
	if sum.New != 0 || res[0].Status != "no_new" {
		t.Fatalf("run2: %+v %+v", sum, res)
	}
	if n := len(readArticles(t, opts)); n != 3 {
		t.Fatalf("after run2 want 3 articles, got %d", n)
	}
}

func TestConditionalGet(t *testing.T) {
	s := newServer(t)
	s.set("/a", serveXML(rss(item("1", "one", day)), map[string]string{"ETag": `"v1"`, "Last-Modified": "Mon, 16 Nov 2026 10:00:00 GMT"}))
	opts := setup(t, feedDef{s.URL + "/a", "ai"})
	run(t, opts)
	_, res := run(t, opts)
	if res[0].Status != "not_modified" || !res[0].OK {
		t.Fatalf("want not_modified: %+v", res)
	}
	if s.lastHeader("/a", "If-None-Match") != `"v1"` || s.lastHeader("/a", "If-Modified-Since") != "Mon, 16 Nov 2026 10:00:00 GMT" {
		t.Fatal("conditional headers not sent")
	}
}

func TestBackfill(t *testing.T) {
	s := newServer(t)
	s.set("/a", serveXML(rss(item("1", "recent", day), item("2", "ancient", 30*day)), nil))
	opts := setup(t, feedDef{s.URL + "/a", "ai"})
	if sum, _ := run(t, opts); sum.New != 1 {
		t.Fatalf("run1 want 1 new, got %+v", sum)
	}
	if sum, _ := run(t, opts); sum.New != 0 {
		t.Fatalf("run2 want 0 new, got %+v", sum)
	}
}

func TestNewItemAfterFirstRunIgnoresBackfill(t *testing.T) {
	s := newServer(t)
	s.set("/a", serveXML(rss(item("1", "recent", day)), nil))
	opts := setup(t, feedDef{s.URL + "/a", "ai"})
	run(t, opts)
	// A late-arriving old-dated item after the first fetch is still new.
	s.set("/a", serveXML(rss(item("1", "recent", day), item("9", "late", 20*day)), nil))
	if sum, _ := run(t, opts); sum.New != 1 {
		t.Fatalf("want 1 new, got %+v", sum)
	}
}

func TestBackfillKeepsUndated(t *testing.T) {
	s := newServer(t)
	s.set("/a", serveXML(rss(`<item><guid>u</guid><title>undated</title></item>`), nil))
	opts := setup(t, feedDef{s.URL + "/a", "ai"})
	if sum, _ := run(t, opts); sum.New != 1 {
		t.Fatalf("want 1 new, got %+v", sum)
	}
	if _, ok := readArticles(t, opts)[0]["published"]; ok {
		t.Fatal("published should be omitted")
	}
}

func TestNoGUIDNoLinkStableID(t *testing.T) {
	s := newServer(t)
	doc := rss(fmt.Sprintf(`<item><title>bare</title><pubDate>%s</pubDate></item>`, now.Add(-day).Format(time.RFC1123Z)))
	s.set("/a", serveXML(doc, nil))
	opts := setup(t, feedDef{s.URL + "/a", "ai"})
	run(t, opts)
	run(t, opts)
	if n := len(readArticles(t, opts)); n != 1 {
		t.Fatalf("want 1 article, got %d", n)
	}
}

func TestDuplicateItemsInOneDocument(t *testing.T) {
	s := newServer(t)
	s.set("/a", serveXML(rss(item("dup", "one", day), item("dup", "one again", day)), nil))
	opts := setup(t, feedDef{s.URL + "/a", "ai"})
	if sum, _ := run(t, opts); sum.New != 1 {
		t.Fatalf("want 1 new, got %+v", sum)
	}
}

func TestHTMLInsteadOfFeed(t *testing.T) {
	s := newServer(t)
	s.set("/a", serveXML(rss(item("1", "one", day)), map[string]string{"ETag": `"v1"`}))
	opts := setup(t, feedDef{s.URL + "/a", "ai"})
	run(t, opts)
	before, _ := store.LoadState(opts.DataDir)
	seenBefore := len(before.Feeds[s.URL+"/a"].Seen)

	s.set("/a", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body>maintenance</body></html>"))
	})
	sum, res := run(t, opts)
	if res[0].OK || res[0].Status != "error" || res[0].Failures != 1 || res[0].Error == "" || sum.Failed != 1 {
		t.Fatalf("want error result: %+v", res)
	}
	st, _ := store.LoadState(opts.DataDir)
	fs := st.Feeds[s.URL+"/a"]
	if fs.ETag != `"v1"` || len(fs.Seen) != seenBefore || fs.Failures != 1 || fs.LastError == "" {
		t.Fatalf("state changed: %+v", fs)
	}
}

func TestOneFeedFailsOthersContinue(t *testing.T) {
	s := newServer(t)
	s.set("/ok", serveXML(rss(item("1", "a", day), item("2", "b", day), item("3", "c", day)), nil))
	s.set("/bad", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) })
	opts := setup(t, feedDef{s.URL + "/bad", "ai"}, feedDef{s.URL + "/ok", "ai"})
	sum, res := run(t, opts)
	want := Summary{Summary: true, Feeds: 2, OK: 1, Failed: 1, New: 3}
	if sum != want {
		t.Fatalf("summary %+v, want %+v", sum, want)
	}
	if res[0].FeedURL != s.URL+"/bad" || res[0].OK || res[1].FeedURL != s.URL+"/ok" {
		t.Fatalf("results order/content: %+v", res)
	}
}

func TestTopicFilter(t *testing.T) {
	s := newServer(t)
	s.set("/a", serveXML(rss(item("1", "a", day)), nil))
	s.set("/b", serveXML(rss(item("1", "b", day)), nil))
	opts := setup(t, feedDef{s.URL + "/a", "ai"}, feedDef{s.URL + "/b", "other"})
	opts.Topic = "other"
	sum, res := run(t, opts)
	if sum.Feeds != 1 || len(res) != 1 || res[0].Topic != "other" {
		t.Fatalf("filter: %+v %+v", sum, res)
	}
}

func TestDryRun(t *testing.T) {
	s := newServer(t)
	s.set("/a", serveXML(rss(item("1", "a", day)), nil))
	opts := setup(t, feedDef{s.URL + "/a", "ai"})
	opts.DryRun = true
	if sum, _ := run(t, opts); sum.New != 1 {
		t.Fatalf("dry run should still report new: %+v", sum)
	}
	if _, err := os.Stat(opts.DataDir); !os.IsNotExist(err) {
		t.Fatal("dry run must not write data dir")
	}
}

func TestBadFeedsFile(t *testing.T) {
	opts := setup(t)
	opts.FeedsPath = filepath.Join(t.TempDir(), "missing.yaml")
	if _, err := Run(opts, &bytes.Buffer{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestBackfillAppliesAfterFailedFirstFetch(t *testing.T) {
	s := newServer(t)
	s.set("/a", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "down", 503) })
	opts := setup(t, feedDef{s.URL + "/a", "ai"})
	run(t, opts)
	s.set("/a", serveXML(rss(item("1", "recent", day), item("2", "old", 100*day), item("3", "older", 300*day)), nil))
	if sum, _ := run(t, opts); sum.New != 1 {
		t.Fatalf("want 1 new (backfill), got %+v", sum)
	}
}

func TestRelativeLinksResolved(t *testing.T) {
	s := newServer(t)
	doc := rss(fmt.Sprintf(`<item><guid>r</guid><title>rel</title><link>/clanek/1</link><pubDate>%s</pubDate></item>`, now.Add(-day).Format(time.RFC1123Z)))
	s.set("/feeds/a", serveXML(doc, nil))
	opts := setup(t, feedDef{s.URL + "/feeds/a", "ai"})
	run(t, opts)
	if got := readArticles(t, opts)[0]["url"]; got != s.URL+"/clanek/1" {
		t.Fatalf("url not resolved: %v", got)
	}
}

func TestUnknownTopicIsError(t *testing.T) {
	opts := setup(t, feedDef{"http://127.0.0.1:1/a", "ai"})
	opts.Topic = "aj-novinky"
	if _, err := Run(opts, &bytes.Buffer{}); err == nil {
		t.Fatal("expected error for unknown topic")
	}
}

func TestBackfillSkipsUndatedWhenFeedHasDates(t *testing.T) {
	s := newServer(t)
	s.set("/a", serveXML(rss(
		item("1", "recent", day),
		`<item><guid>p1</guid><title>Training page</title><pubDate>Mon, 01 Jan 0001 00:00:00 +0000</pubDate></item>`,
		`<item><guid>p2</guid><title>Another page</title></item>`,
	), nil))
	opts := setup(t, feedDef{s.URL + "/a", "ai"})
	if sum, _ := run(t, opts); sum.New != 1 {
		t.Fatalf("want only the dated recent item, got %+v", sum)
	}
	if sum, _ := run(t, opts); sum.New != 0 {
		t.Fatalf("undated pages must not come back later, got %+v", sum)
	}
}

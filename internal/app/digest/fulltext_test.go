package digest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/podanypepa/skim/internal/store"
)

var longText = strings.Repeat("Toto je dlouhý odstavec článku. ", 20) // ~640 runes

func page(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(body))
	}
}

type ftServer struct {
	*httptest.Server
	mux  *http.ServeMux
	hits map[string]*int32
}

func newFT(t *testing.T) *ftServer {
	s := &ftServer{mux: http.NewServeMux(), hits: map[string]*int32{}}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, ok := s.hits[r.URL.Path]; ok {
			atomic.AddInt32(c, 1)
		}
		s.mux.ServeHTTP(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *ftServer) handle(path string, h http.HandlerFunc) {
	var n int32
	s.hits[path] = &n
	s.mux.HandleFunc(path, h)
}

func ftOpts(dir string) FulltextOptions {
	return FulltextOptions{DataDir: dir, Day: day, Min: 3, Short: 600, Client: &http.Client{Timeout: 5 * time.Second}, Now: now}
}

func runFT(t *testing.T, o FulltextOptions) (map[string]string, map[string]any) {
	t.Helper()
	var out bytes.Buffer
	if err := runFulltext(o, &out); err != nil {
		t.Fatal(err)
	}
	st := map[string]string{}
	var sum map[string]any
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var m map[string]any
		json.Unmarshal([]byte(l), &m)
		if m["summary"] == true {
			sum = m
			continue
		}
		st[m["id"].(string)] = m["status"].(string)
	}
	return st, sum
}

func artAt(id, url string) store.Article {
	a := art(id)
	a.URL = url
	a.Content = "krátký perex"
	return a
}

func TestFulltextBasic(t *testing.T) {
	s := newFT(t)
	s.handle("/a", page(`<html><body><nav><p>menu</p></nav><article><p>`+longText+`</p></article></body></html>`))
	dir := t.TempDir()
	addArticles(t, dir, day, artAt("a", s.URL+"/a"))
	addTriage(t, dir, day, tri("a", 4))
	st, sum := runFT(t, ftOpts(dir))
	if st["a"] != "ok" || sum["ok"] != 1.0 || sum["tried"] != 1.0 {
		t.Fatalf("st %v sum %v", st, sum)
	}
	recs, _ := store.ReadFulltext(dir, day)
	if len(recs) != 1 || recs[0].URL != s.URL+"/a" || strings.Contains(recs[0].Content, "menu") {
		t.Fatalf("recs = %+v", recs)
	}
	cs, _ := candidates(t, copts(dir))
	if !strings.HasPrefix(cs[0].Excerpt, "Toto je dlouhý odstavec") {
		t.Fatalf("candidates excerpt = %q", cs[0].Excerpt)
	}
}

func TestFulltextSelectsOnlyShortHighScored(t *testing.T) {
	s := newFT(t)
	s.handle("/low", page("<p>"+longText+"</p>"))
	s.handle("/long", page("<p>"+longText+"</p>"))
	s.handle("/done", page("<p>"+longText+"</p>"))
	dir := t.TempDir()
	long := artAt("long", s.URL+"/long")
	long.Content = longText
	addArticles(t, dir, day, artAt("low", s.URL+"/low"), long, artAt("done", s.URL+"/done"))
	addTriage(t, dir, day, tri("low", 2), tri("long", 5), tri("done", 4))
	store.AppendFulltext(dir, day, []store.Fulltext{{ID: "done", Status: "error"}})
	_, sum := runFT(t, ftOpts(dir))
	if sum["tried"] != 0.0 {
		t.Fatalf("sum = %v", sum)
	}
	for _, p := range []string{"/low", "/long", "/done"} {
		if *s.hits[p] != 0 {
			t.Fatalf("%s fetched", p)
		}
	}
}

func TestFulltextEmptyKeepsExcerpt(t *testing.T) {
	s := newFT(t)
	s.handle("/js", page(`<html><body><div id="app"></div><p>Enable JavaScript.</p></body></html>`))
	dir := t.TempDir()
	addArticles(t, dir, day, artAt("js", s.URL+"/js"))
	addTriage(t, dir, day, tri("js", 4))
	st, _ := runFT(t, ftOpts(dir))
	if st["js"] != "empty" {
		t.Fatalf("st = %v", st)
	}
	cs, _ := candidates(t, copts(dir))
	if cs[0].Excerpt != "krátký perex" {
		t.Fatalf("excerpt = %q", cs[0].Excerpt)
	}
}

func TestFulltextRobots(t *testing.T) {
	s := newFT(t)
	s.handle("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("User-agent: *\nDisallow: /private\n"))
	})
	s.handle("/private/a", page("<p>"+longText+"</p>"))
	s.handle("/public/b", page("<p>"+longText+"</p>"))
	dir := t.TempDir()
	addArticles(t, dir, day, artAt("a", s.URL+"/private/a"), artAt("b", s.URL+"/public/b"))
	addTriage(t, dir, day, tri("a", 4), tri("b", 4))
	st, _ := runFT(t, ftOpts(dir))
	if st["a"] != "robots" || st["b"] != "ok" || *s.hits["/private/a"] != 0 {
		t.Fatalf("st = %v", st)
	}

	// No robots.txt (404) → allowed.
	s2 := newFT(t)
	s2.handle("/x", page("<p>"+longText+"</p>"))
	dir2 := t.TempDir()
	addArticles(t, dir2, day, artAt("x", s2.URL+"/x"))
	addTriage(t, dir2, day, tri("x", 4))
	if st, _ := runFT(t, ftOpts(dir2)); st["x"] != "ok" {
		t.Fatalf("404 robots: st = %v", st)
	}
}

func TestFulltextRedirect(t *testing.T) {
	target := newFT(t)
	target.handle("/final", page("<p>"+longText+"</p>"))
	s := newFT(t)
	s.handle("/short", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/final", http.StatusFound)
	})
	dir := t.TempDir()
	addArticles(t, dir, day, artAt("r", s.URL+"/short"))
	addTriage(t, dir, day, tri("r", 4))
	if st, _ := runFT(t, ftOpts(dir)); st["r"] != "ok" {
		t.Fatalf("st = %v", st)
	}
	recs, _ := store.ReadFulltext(dir, day)
	if recs[0].URL != target.URL+"/final" {
		t.Fatalf("url = %s", recs[0].URL)
	}
}

func TestFulltextReddit(t *testing.T) {
	s := newFT(t)
	s.handle("/ext", page("<article><p>"+longText+"</p></article>"))
	s.handle("/r/golang/comments/aaa/link_post.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"data":{"children":[{"data":{"url":"` + s.URL + `/ext","selftext":"","is_self":false}}]}}]`))
	})
	s.handle("/r/golang/comments/bbb/self_post.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"data":{"children":[{"data":{"url":"x","selftext":"` + longText + `","is_self":true}}]}}]`))
	})
	dir := t.TempDir()
	addArticles(t, dir, day,
		artAt("link", s.URL+"/r/golang/comments/aaa/link_post/"),
		artAt("self", s.URL+"/r/golang/comments/bbb/self_post/"))
	addTriage(t, dir, day, tri("link", 4), tri("self", 4))
	st, _ := runFT(t, ftOpts(dir))
	if st["link"] != "ok" || st["self"] != "ok" || *s.hits["/ext"] != 1 {
		t.Fatalf("st = %v hits = %d", st, *s.hits["/ext"])
	}
	recs, _ := store.ReadFulltext(dir, day)
	for _, r := range recs {
		if r.ID == "link" && r.URL != s.URL+"/ext" {
			t.Fatalf("link url = %s", r.URL)
		}
	}
}

func TestFulltextHTTPError(t *testing.T) {
	s := newFT(t)
	s.handle("/err", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "gone", 404) })
	s.handle("/pdf", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Write([]byte("%PDF"))
	})
	dir := t.TempDir()
	addArticles(t, dir, day, artAt("e", s.URL+"/err"), artAt("p", s.URL+"/pdf"))
	addTriage(t, dir, day, tri("e", 4), tri("p", 4))
	st, sum := runFT(t, ftOpts(dir))
	if st["e"] != "error" || st["p"] != "error" || sum["failed"] != 2.0 {
		t.Fatalf("st = %v sum = %v", st, sum)
	}
	if _, sum := runFT(t, ftOpts(dir)); sum["tried"] != 0.0 || *s.hits["/err"] != 1 {
		t.Fatalf("retried: %v", sum)
	}
}

func TestFulltextKeepsAngleBrackets(t *testing.T) {
	s := newFT(t)
	s.handle("/g", page("<article><p>Generics: Vec&lt;T&gt; and a &lt; b. "+longText+"</p></article>"))
	dir := t.TempDir()
	addArticles(t, dir, day, artAt("g", s.URL+"/g"))
	addTriage(t, dir, day, tri("g", 4))
	runFT(t, ftOpts(dir))
	recs, _ := store.ReadFulltext(dir, day)
	if !strings.HasPrefix(recs[0].Content, "Generics: Vec<T> and a < b.") {
		t.Fatalf("content = %q", recs[0].Content[:60])
	}
}

func TestFulltextRedditRespectsRobots(t *testing.T) {
	s := newFT(t)
	s.handle("/robots.txt", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("User-agent: *\nDisallow: /\n")) })
	s.handle("/r/golang/comments/aaa/x.json", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`[]`)) })
	dir := t.TempDir()
	addArticles(t, dir, day, artAt("r", s.URL+"/r/golang/comments/aaa/x/"))
	addTriage(t, dir, day, tri("r", 4))
	if st, _ := runFT(t, ftOpts(dir)); st["r"] != "robots" || *s.hits["/r/golang/comments/aaa/x.json"] != 0 {
		t.Fatalf("st = %v", st)
	}
}

func TestFulltextRedirectChecksRobotsOfTarget(t *testing.T) {
	target := newFT(t)
	target.handle("/robots.txt", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("User-agent: *\nDisallow: /\n")) })
	target.handle("/final", page("<p>"+longText+"</p>"))
	s := newFT(t)
	s.handle("/go", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/final", http.StatusFound)
	})
	dir := t.TempDir()
	addArticles(t, dir, day, artAt("x", s.URL+"/go"))
	addTriage(t, dir, day, tri("x", 4))
	if st, _ := runFT(t, ftOpts(dir)); st["x"] != "robots" || *target.hits["/final"] != 0 {
		t.Fatalf("st = %v hits = %d", st, *target.hits["/final"])
	}
}

func TestFulltextTransientNotPersisted(t *testing.T) {
	s := newFT(t)
	var fail int32 = 1
	s.handle("/flaky", func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&fail) == 1 {
			http.Error(w, "busy", 503)
			return
		}
		page("<p>"+longText+"</p>")(w, r)
	})
	dir := t.TempDir()
	addArticles(t, dir, day, artAt("f", s.URL+"/flaky"))
	addTriage(t, dir, day, tri("f", 4))
	if st, _ := runFT(t, ftOpts(dir)); st["f"] != "retry" {
		t.Fatalf("st = %v", st)
	}
	if recs, _ := store.ReadFulltext(dir, day); len(recs) != 0 {
		t.Fatalf("transient error persisted: %+v", recs)
	}
	atomic.StoreInt32(&fail, 0)
	if st, _ := runFT(t, ftOpts(dir)); st["f"] != "ok" {
		t.Fatalf("second run st = %v", st)
	}
}

func TestFulltextNetworkErrorNotPersisted(t *testing.T) {
	s := newFT(t)
	url := s.URL + "/gone"
	s.Close()
	dir := t.TempDir()
	addArticles(t, dir, day, artAt("n", url))
	addTriage(t, dir, day, tri("n", 4))
	if st, _ := runFT(t, ftOpts(dir)); st["n"] != "retry" {
		t.Fatalf("st = %v", st)
	}
	if recs, _ := store.ReadFulltext(dir, day); len(recs) != 0 {
		t.Fatalf("persisted: %+v", recs)
	}
}

func TestFulltextNonUTF8IsEmpty(t *testing.T) {
	s := newFT(t)
	s.handle("/latin2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=iso-8859-2")
		w.Write(append([]byte("<p>"), append(bytes.Repeat([]byte{0xe8, 0x61, 0xb9, 0x20}, 200), []byte("</p>")...)...))
	})
	dir := t.TempDir()
	addArticles(t, dir, day, artAt("l", s.URL+"/latin2"))
	addTriage(t, dir, day, tri("l", 4))
	if st, _ := runFT(t, ftOpts(dir)); st["l"] != "empty" {
		t.Fatalf("st = %v", st)
	}
}

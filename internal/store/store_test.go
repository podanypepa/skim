package store

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

func TestArticleID(t *testing.T) {
	hex16 := regexp.MustCompile(`^[0-9a-f]{16}$`)
	a := ArticleID("f", "g1", "l1", "t", now)
	if !hex16.MatchString(a) {
		t.Fatalf("bad id %q", a)
	}
	if ArticleID("f", "g1", "l2", "x", time.Time{}) != a {
		t.Fatal("guid must take priority")
	}
	if ArticleID("f", "", "l1", "t", now) != ArticleID("f", "", "l1", "other", time.Time{}) {
		t.Fatal("link must take priority over title")
	}
	if ArticleID("f", "", "", "t", now) == ArticleID("f", "", "", "t", now.Add(time.Hour)) {
		t.Fatal("title+date fallback must include date")
	}
	if ArticleID("f", "g1", "", "", time.Time{}) == ArticleID("f2", "g1", "", "", time.Time{}) {
		t.Fatal("feed url must be part of id")
	}
}

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := LoadState(dir)
	if err != nil || len(s.Feeds) != 0 {
		t.Fatalf("empty state: %+v, %v", s, err)
	}
	s.Feeds["https://a/feed"] = &FeedState{ETag: `"v1"`, Failures: 2, Seen: map[string]string{"x": now.Format(time.RFC3339)}}
	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}
	s2, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, s2) {
		t.Fatalf("round trip mismatch:\n%+v\n%+v", s.Feeds["https://a/feed"], s2.Feeds["https://a/feed"])
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "state.json" {
		t.Fatalf("unexpected files: %v", entries)
	}
}

func TestPruneKeepsIDsStillInFeed(t *testing.T) {
	old := now.Add(-100 * 24 * time.Hour).Format(time.RFC3339)
	fs := &FeedState{Seen: map[string]string{
		"old":   old,
		"gone":  old,
		"fresh": now.Add(-24 * time.Hour).Format(time.RFC3339),
	}}
	fs.Prune(now, map[string]bool{"old": true}, 90*24*time.Hour)
	if _, ok := fs.Seen["gone"]; ok {
		t.Fatal("gone should be pruned")
	}
	if fs.Seen["old"] != now.Format(time.RFC3339) {
		t.Fatalf("old should be kept and refreshed: %q", fs.Seen["old"])
	}
	if _, ok := fs.Seen["fresh"]; !ok {
		t.Fatal("fresh should be kept")
	}
}

func TestAppendArticles(t *testing.T) {
	dir := t.TempDir()
	mk := func(id string) Article {
		return Article{ID: id, Title: "a <b> & c", Content: "x<y", FetchedAt: now.Format(time.RFC3339)}
	}
	if err := AppendArticles(dir, now, []Article{mk("1"), mk("2")}); err != nil {
		t.Fatal(err)
	}
	if err := AppendArticles(dir, now, []Article{mk("3"), mk("4")}); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "articles", "2026-11-16.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var ids []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, `\u003c`) || strings.Contains(line, `\u0026`) {
			t.Fatalf("html escaped: %s", line)
		}
		var a Article
		if err := json.Unmarshal([]byte(line), &a); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, a.ID)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(ids, ",") != "1,2,3,4" {
		t.Fatalf("bad ids: %v", ids)
	}
}

func TestAppendNothing(t *testing.T) {
	dir := t.TempDir()
	if err := AppendArticles(dir, now, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "articles", "2026-11-16.jsonl")); !os.IsNotExist(err) {
		t.Fatal("empty append should not create file")
	}
}

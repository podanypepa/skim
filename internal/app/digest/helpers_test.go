package digest

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/podanypepa/skim/internal/store"
)

const day = "2026-11-16"

var now = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

func addArticles(t *testing.T, dir, d string, arts ...store.Article) {
	t.Helper()
	tm, _ := time.Parse("2006-01-02", d)
	if err := store.AppendArticles(dir, tm, arts); err != nil {
		t.Fatal(err)
	}
}

func addTriage(t *testing.T, dir, d string, recs ...store.Triage) {
	t.Helper()
	if err := store.AppendTriage(dir, d, recs); err != nil {
		t.Fatal(err)
	}
}

func art(id string) store.Article {
	return store.Article{ID: id, Topic: "ai", FeedTitle: "Feed " + id, Title: "Title " + id, URL: "https://e.com/" + id, Content: "obsah " + id}
}

func tri(id string, score int) store.Triage {
	return store.Triage{ID: id, Topic: "ai", Score: score, Reason: "důvod " + id}
}

func candidates(t *testing.T, o CandOptions) ([]Candidate, CandSummary) {
	t.Helper()
	var out bytes.Buffer
	if err := runCandidates(o, &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var cs []Candidate
	for _, l := range lines[:len(lines)-1] {
		var c Candidate
		if err := json.Unmarshal([]byte(l), &c); err != nil {
			t.Fatalf("bad line %q: %v", l, err)
		}
		cs = append(cs, c)
	}
	var sum CandSummary
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &sum); err != nil || !sum.Summary {
		t.Fatalf("bad summary %q: %v", lines[len(lines)-1], err)
	}
	return cs, sum
}

func copts(dir string) CandOptions { return CandOptions{DataDir: dir, Day: day, Min: 3, Excerpt: 2000} }

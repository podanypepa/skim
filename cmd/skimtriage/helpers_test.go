package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"skim/internal/store"
)

var now = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

func dayTime(t *testing.T, day string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", day)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func addArticles(t *testing.T, dir, day string, arts ...store.Article) {
	t.Helper()
	if err := store.AppendArticles(dir, dayTime(t, day), arts); err != nil {
		t.Fatal(err)
	}
}

func art(id, topic string) store.Article {
	return store.Article{ID: id, Topic: topic, FeedTitle: "Feed", Title: "Title " + id, URL: "https://e.com/" + id, Content: "obsah " + id}
}

func pending(t *testing.T, opts PendingOptions) ([]PendingItem, PendingSummary) {
	t.Helper()
	var out bytes.Buffer
	if err := runPending(opts, &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var items []PendingItem
	for _, l := range lines[:len(lines)-1] {
		var it PendingItem
		if err := json.Unmarshal([]byte(l), &it); err != nil {
			t.Fatalf("bad line %q: %v", l, err)
		}
		items = append(items, it)
	}
	var sum PendingSummary
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &sum); err != nil || !sum.Summary {
		t.Fatalf("bad summary %q: %v", lines[len(lines)-1], err)
	}
	return items, sum
}

func opts(dir string) PendingOptions {
	return PendingOptions{DataDir: dir, Limit: 25, Days: 7, Excerpt: 1500, Now: now}
}

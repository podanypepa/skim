package main

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"skim/internal/store"
)

func TestPendingBasic(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, "2026-11-16", art("c", "ai"))
	addArticles(t, dir, "2026-11-15", art("a", "ai"), art("b", "ai"))
	store.AppendTriage(dir, "2026-11-15", []store.Triage{{ID: "a", Score: 3, Reason: "x"}})
	items, sum := pending(t, opts(dir))
	if len(items) != 2 || items[0].ID != "b" || items[0].Day != "2026-11-15" || items[1].ID != "c" {
		t.Fatalf("items = %+v", items)
	}
	if items[0].Title != "Title b" || items[0].FeedTitle != "Feed" || items[0].Topic != "ai" || items[0].URL != "https://e.com/b" || items[0].Excerpt != "obsah b" {
		t.Fatalf("fields = %+v", items[0])
	}
	if sum.Pending != 2 || sum.Shown != 2 {
		t.Fatalf("sum = %+v", sum)
	}
}

func TestPendingLimit(t *testing.T) {
	dir := t.TempDir()
	for i := range 30 {
		addArticles(t, dir, "2026-11-16", art(fmt.Sprint(i), "ai"))
	}
	items, sum := pending(t, opts(dir))
	if len(items) != 25 || sum.Pending != 30 || sum.Shown != 25 || items[0].ID != "0" {
		t.Fatalf("len %d sum %+v", len(items), sum)
	}
}

func TestPendingDaysWindow(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, "2026-11-10", art("in", "ai"))
	addArticles(t, dir, "2026-11-09", art("out", "ai"))
	items, sum := pending(t, opts(dir))
	if len(items) != 1 || items[0].ID != "in" || sum.Pending != 1 {
		t.Fatalf("items = %+v", items)
	}
}

func TestPendingTopicFilter(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, "2026-11-16", art("a", "ai"), art("b", "other"), art("c", "other"))
	o := opts(dir)
	o.Topic = "other"
	items, sum := pending(t, o)
	if len(items) != 2 || items[0].Topic != "other" || sum.Pending != 2 {
		t.Fatalf("items = %+v sum %+v", items, sum)
	}
}

func TestPendingExcerptRunes(t *testing.T) {
	dir := t.TempDir()
	a := art("a", "ai")
	a.Content = strings.Repeat("ř", 2000)
	addArticles(t, dir, "2026-11-16", a)
	items, _ := pending(t, opts(dir))
	if n := utf8.RuneCountInString(items[0].Excerpt); n != 1500 || !utf8.ValidString(items[0].Excerpt) {
		t.Fatalf("excerpt runes = %d", n)
	}
}

func TestPendingDedupAcrossDays(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, "2026-11-15", art("dup", "ai"))
	addArticles(t, dir, "2026-11-16", art("dup", "ai"))
	items, sum := pending(t, opts(dir))
	if len(items) != 1 || items[0].Day != "2026-11-15" || sum.Pending != 1 {
		t.Fatalf("items = %+v", items)
	}
}

func TestPendingEmpty(t *testing.T) {
	items, sum := pending(t, opts(t.TempDir()))
	if len(items) != 0 || sum.Pending != 0 || sum.Shown != 0 {
		t.Fatalf("items %+v sum %+v", items, sum)
	}
}

func TestPendingRejectsBadDays(t *testing.T) {
	o := opts(t.TempDir())
	o.Days = 0
	if err := runPending(o, &strings.Builder{}); err == nil {
		t.Fatal("expected error for days 0")
	}
}

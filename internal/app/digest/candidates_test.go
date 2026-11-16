package digest

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func ids(cs []Candidate) string {
	var s []string
	for _, c := range cs {
		s = append(s, c.ID)
	}
	return strings.Join(s, ",")
}

func TestCandidatesBasic(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, day, art("a"), art("b"), art("c"), art("d"))
	addTriage(t, dir, day, tri("a", 3), tri("b", 5), tri("c", 1), tri("d", 4))
	cs, sum := candidates(t, copts(dir))
	if ids(cs) != "b,d,a" {
		t.Fatalf("order = %s", ids(cs))
	}
	if sum.Candidates != 3 || sum.Untriaged != 0 || sum.Day != day ||
		sum.ByScore["5"] != 1 || sum.ByScore["4"] != 1 || sum.ByScore["3"] != 1 {
		t.Fatalf("sum = %+v", sum)
	}
	b := cs[0]
	if b.Reason != "důvod b" || b.URL != "https://e.com/b" || b.FeedTitle != "Feed b" || b.Score != 5 || b.Excerpt != "obsah b" || b.Topic != "ai" {
		t.Fatalf("fields = %+v", b)
	}
}

func TestCandidatesTieKeepsFileOrder(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, day, art("x"), art("y"))
	addTriage(t, dir, day, tri("y", 4), tri("x", 4))
	cs, _ := candidates(t, copts(dir))
	if ids(cs) != "x,y" {
		t.Fatalf("order = %s", ids(cs))
	}
}

func TestCandidatesMin(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, day, art("a"), art("b"), art("c"))
	addTriage(t, dir, day, tri("a", 3), tri("b", 5), tri("c", 4))
	o := copts(dir)
	o.Min = 4
	cs, sum := candidates(t, o)
	if ids(cs) != "b,c" || sum.ByScore["3"] != 0 || sum.Candidates != 2 {
		t.Fatalf("cs %s sum %+v", ids(cs), sum)
	}
}

func TestCandidatesUntriaged(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, day, art("a"), art("b"), art("c"))
	addTriage(t, dir, day, tri("a", 4), tri("b", 2))
	_, sum := candidates(t, copts(dir))
	if sum.Untriaged != 1 || sum.Candidates != 1 {
		t.Fatalf("sum = %+v", sum)
	}
}

func TestCandidatesNoTriage(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, day, art("a"))
	_, sum := candidates(t, copts(dir))
	if sum.Candidates != 0 || sum.Untriaged != 1 {
		t.Fatalf("sum = %+v", sum)
	}
	_, sum = candidates(t, copts(t.TempDir()))
	if sum.Candidates != 0 || sum.Untriaged != 0 {
		t.Fatalf("empty day sum = %+v", sum)
	}
}

func TestCandidatesExcerptRunes(t *testing.T) {
	dir := t.TempDir()
	a := art("a")
	a.Content = strings.Repeat("ř", 3000)
	addArticles(t, dir, day, a)
	addTriage(t, dir, day, tri("a", 4))
	cs, _ := candidates(t, copts(dir))
	if utf8.RuneCountInString(cs[0].Excerpt) != 2000 {
		t.Fatalf("runes = %d", utf8.RuneCountInString(cs[0].Excerpt))
	}
}

func TestCandidatesIgnoresIDsFromEarlierDays(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, "2026-11-15", art("dup"))
	addTriage(t, dir, "2026-11-15", tri("dup", 4))
	addArticles(t, dir, day, art("dup"), art("new"))
	addTriage(t, dir, day, tri("new", 4))
	cs, sum := candidates(t, copts(dir))
	if sum.Untriaged != 0 || ids(cs) != "new" {
		t.Fatalf("cs %s sum %+v", ids(cs), sum)
	}
}

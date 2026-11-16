package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDays(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"2026-11-16.jsonl", "2026-11-14.jsonl", "notes.txt", "bad.jsonl"} {
		writeFile(t, filepath.Join(dir, "articles", n), "")
	}
	days, err := Days(dir, "articles")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(days, []string{"2026-11-14", "2026-11-16"}) {
		t.Fatalf("days = %v", days)
	}
	days, err = Days(dir, "triage")
	if err != nil || len(days) != 0 {
		t.Fatalf("missing dir: %v, %v", days, err)
	}
}

func TestReadSkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "articles", "2026-11-16.jsonl"),
		`{"id":"a","title":"A"}`+"\n\n"+`{"id":"x","ti`+"\n"+`{"title":"no id"}`+"\n"+`{"id":"b","title":"B"}`+"\n")
	arts, err := ReadArticles(dir, "2026-11-16")
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 2 || arts[0].ID != "a" || arts[1].ID != "b" {
		t.Fatalf("arts = %+v", arts)
	}
	writeFile(t, filepath.Join(dir, "triage", "2026-11-16.jsonl"),
		`{"id":"a","score":4}`+"\n"+`garbage`+"\n"+`{"id":"b","score":1}`+"\n")
	recs, err := ReadTriage(dir, "2026-11-16")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 || recs[1].Score != 1 {
		t.Fatalf("recs = %+v", recs)
	}
}

func TestTriageRoundTrip(t *testing.T) {
	dir := t.TempDir()
	r := func(id string) Triage {
		return Triage{ID: id, Topic: "ai", Score: 4, Reason: "nový <model> & nástroj", TriagedAt: "2026-11-16T12:00:00Z"}
	}
	if err := AppendTriage(dir, "2026-11-15", []Triage{r("a"), r("b")}); err != nil {
		t.Fatal(err)
	}
	if err := AppendTriage(dir, "2026-11-16", []Triage{r("c")}); err != nil {
		t.Fatal(err)
	}
	recs, err := ReadTriage(dir, "2026-11-15")
	if err != nil || len(recs) != 2 || recs[0] != r("a") {
		t.Fatalf("recs = %+v, %v", recs, err)
	}
	ids, err := TriagedIDs(dir)
	if err != nil || !ids["a"] || !ids["b"] || !ids["c"] || len(ids) != 3 {
		t.Fatalf("ids = %v, %v", ids, err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "triage", "2026-11-15.jsonl"))
	if !strings.Contains(string(b), "<model> &") {
		t.Fatalf("html escaped: %s", b)
	}
}

func TestAppendAfterTruncatedLine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "triage", "2026-11-16.jsonl"), `{"id":"a","sco`)
	if err := AppendTriage(dir, "2026-11-16", []Triage{{ID: "b", Score: 3, Reason: "x"}}); err != nil {
		t.Fatal(err)
	}
	ids, err := TriagedIDs(dir)
	if err != nil || !ids["b"] {
		t.Fatalf("b lost after truncated line: %v, %v", ids, err)
	}
}

func TestAppendTriageDaysRollsBack(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "triage", "2026-11-14.jsonl"), `{"id":"old","score":2,"reason":"x"}`+"\n")
	// Make the day-16 target unwritable: a directory in place of the file.
	if err := os.MkdirAll(filepath.Join(dir, "triage", "2026-11-16.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := AppendTriageDays(dir, map[string][]Triage{
		"2026-11-14": {{ID: "a", Score: 3, Reason: "x"}},
		"2026-11-15": {{ID: "b", Score: 3, Reason: "x"}},
		"2026-11-16": {{ID: "c", Score: 3, Reason: "x"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	ids, _ := TriagedIDs(dir)
	if len(ids) != 1 || !ids["old"] {
		t.Fatalf("not rolled back: %v", ids)
	}
	if _, err := os.Stat(filepath.Join(dir, "triage", "2026-11-15.jsonl")); !os.IsNotExist(err) {
		t.Fatal("newly created file not removed")
	}
}

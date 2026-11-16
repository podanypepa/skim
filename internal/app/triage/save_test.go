package triage

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/podanypepa/skim/internal/store"
)

func save(dir, input string) (string, error) {
	var out bytes.Buffer
	err := runSave(dir, strings.NewReader(input), now, &out)
	return out.String(), err
}

func noTriage(t *testing.T, dir string) {
	t.Helper()
	if days, _ := store.Days(dir, "triage"); len(days) != 0 {
		t.Fatalf("triage written: %v", days)
	}
}

func TestSaveOK(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, "2026-11-15", art("a", "ai"))
	addArticles(t, dir, "2026-11-16", art("b", "other"))
	out, err := save(dir, `{"id":"a","score":5,"reason":"Nový model."}`+"\n"+`{"id":"b","score":1,"reason":"PR."}`+"\n")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != `{"saved":2}` {
		t.Fatalf("out = %q", out)
	}
	r15, _ := store.ReadTriage(dir, "2026-11-15")
	r16, _ := store.ReadTriage(dir, "2026-11-16")
	want := store.Triage{ID: "a", Topic: "ai", Score: 5, Reason: "Nový model.", TriagedAt: "2026-11-16T12:00:00Z"}
	if len(r15) != 1 || r15[0] != want || len(r16) != 1 || r16[0].Topic != "other" {
		t.Fatalf("r15 %+v r16 %+v", r15, r16)
	}
}

func TestSaveAllOrNothing(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, "2026-11-16", art("a", "ai"))
	_, err := save(dir, `{"id":"a","score":4,"reason":"ok"}`+"\n"+`{"id":"zzz","score":4,"reason":"ok"}`)
	if err == nil || !strings.Contains(err.Error(), "řádek 2") {
		t.Fatalf("err = %v", err)
	}
	noTriage(t, dir)
}

func TestSaveScoreValidation(t *testing.T) {
	for _, sc := range []string{`4.5`, `"4"`, `0`, `6`, `4.0`, `4e0`, ``} {
		dir := t.TempDir()
		addArticles(t, dir, "2026-11-16", art("a", "ai"))
		line := `{"id":"a","score":` + sc + `,"reason":"x"}`
		if sc == "" {
			line = `{"id":"a","reason":"x"}`
		}
		if _, err := save(dir, line); err == nil {
			t.Errorf("score %q: expected error", sc)
		}
		noTriage(t, dir)
	}
}

func TestSaveReasonLimitRunes(t *testing.T) {
	cases := map[string]bool{strings.Repeat("ř", 300): true, strings.Repeat("ř", 301): false, "   ": false}
	for reason, ok := range cases {
		dir := t.TempDir()
		addArticles(t, dir, "2026-11-16", art("a", "ai"))
		_, err := save(dir, `{"id":"a","score":3,"reason":"`+reason+`"}`)
		if (err == nil) != ok {
			t.Errorf("reason len %d: ok=%v err=%v", len([]rune(reason)), ok, err)
		}
	}
}

func TestSaveDuplicates(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, "2026-11-16", art("a", "ai"), art("b", "ai"))
	if _, err := save(dir, `{"id":"a","score":3,"reason":"x"}`+"\n"+`{"id":"a","score":4,"reason":"y"}`); err == nil {
		t.Fatal("duplicate in batch: expected error")
	}
	if _, err := save(dir, `{"id":"b","score":3,"reason":"x"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := save(dir, `{"id":"b","score":3,"reason":"x"}`); err == nil {
		t.Fatal("already triaged: expected error")
	}
}

func TestSaveUnknownField(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, "2026-11-16", art("a", "ai"))
	if _, err := save(dir, `{"id":"a","score":4,"reason":"x","foo":1}`); err == nil {
		t.Fatal("expected error")
	}
}

func TestSaveUsesFirstDay(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, "2026-11-15", art("dup", "ai"))
	addArticles(t, dir, "2026-11-16", art("dup", "ai"))
	if _, err := save(dir, `{"id":"dup","score":4,"reason":"x"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "triage", "2026-11-16.jsonl")); !os.IsNotExist(err) {
		t.Fatal("must not write second day")
	}
	if r, _ := store.ReadTriage(dir, "2026-11-15"); len(r) != 1 {
		t.Fatalf("first day: %+v", r)
	}
}

func TestSaveEmptyInput(t *testing.T) {
	dir := t.TempDir()
	for _, in := range []string{"", "\n\n  \n"} {
		if _, err := save(dir, in); err == nil {
			t.Errorf("input %q: expected error", in)
		}
	}
	noTriage(t, dir)
}

func TestSaveCollectsAllErrors(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, "2026-11-16", art("a", "ai"))
	_, err := save(dir, `{"id":"x","score":4,"reason":"r"}`+"\n"+`not json`+"\n"+`{"id":"a","score":9,"reason":"r"}`)
	if err == nil {
		t.Fatal("expected error")
	}
	for _, l := range []string{"řádek 1", "řádek 2", "řádek 3"} {
		if !strings.Contains(err.Error(), l) {
			t.Errorf("missing %q in %v", l, err)
		}
	}
}

func TestSaveRejectsTrailingContent(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, "2026-11-16", art("a", "ai"), art("b", "ai"))
	for _, in := range []string{
		`{"id":"a","score":4,"reason":"x"} {"id":"b","score":4,"reason":"y"}`,
		`{"id":"a","score":4,"reason":"x"}}garbage`,
	} {
		if _, err := save(dir, in); err == nil {
			t.Errorf("input %q: expected error", in)
		}
		noTriage(t, dir)
	}
}

func TestSaveAllOrNothingAcrossDays(t *testing.T) {
	dir := t.TempDir()
	addArticles(t, dir, "2026-11-15", art("a", "ai"))
	addArticles(t, dir, "2026-11-16", art("b", "ai"))
	if err := os.MkdirAll(filepath.Join(dir, "triage", "2026-11-16.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := save(dir, `{"id":"a","score":4,"reason":"x"}`+"\n"+`{"id":"b","score":4,"reason":"y"}`)
	if err == nil || !strings.Contains(err.Error(), "nic neuloženo") {
		t.Fatalf("err = %v", err)
	}
	if ids, _ := store.TriagedIDs(dir); len(ids) != 0 {
		t.Fatalf("partial write: %v", ids)
	}
}

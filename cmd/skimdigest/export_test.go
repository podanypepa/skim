package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeDigest(t *testing.T, dir, d string, dg Digest) {
	t.Helper()
	p := filepath.Join(dir, "digest", d+".json")
	os.MkdirAll(filepath.Dir(p), 0o755)
	b, _ := json.Marshal(dg)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readExport(t *testing.T, path string) Export {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var e Export
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	return e
}

func sampleDigest(d string) Digest {
	return Digest{Day: d,
		Stories: []Story{{N: 1, Topic: "ai", Title: "T " + d, Summary: "S", Score: 5,
			Articles: []Ref{{ID: "a", FeedTitle: "F", Title: "A", URL: "https://e.com/a", Score: 5}}}},
		More: []Ref{{ID: "m", FeedTitle: "F", Title: "M", URL: "https://e.com/m", Score: 3}}}
}

func TestExportBasic(t *testing.T) {
	dir := t.TempDir()
	writeDigest(t, dir, "2026-11-15", sampleDigest("2026-11-15"))
	writeDigest(t, dir, "2026-11-16", sampleDigest("2026-11-16"))
	out := filepath.Join(dir, "web", "digests.json")
	if err := runExport(dir, 14, out, now); err != nil {
		t.Fatal(err)
	}
	e := readExport(t, out)
	if e.GeneratedAt != "2026-11-16T12:00:00Z" || len(e.Days) != 2 || e.Days[0].Day != "2026-11-16" {
		t.Fatalf("e = %+v", e)
	}
	s := e.Days[0].Stories[0]
	if s.N != 1 || s.Title != "T 2026-11-16" || s.Score != 5 || s.Articles[0].URL != "https://e.com/a" || s.Articles[0].FeedTitle != "F" || e.Days[0].More[0].Title != "M" {
		t.Fatalf("story = %+v", e.Days[0])
	}
}

func TestExportFewDays(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "web", "digests.json")
	if err := runExport(dir, 14, out, now); err != nil {
		t.Fatal(err)
	}
	if e := readExport(t, out); e.Days == nil || len(e.Days) != 0 {
		t.Fatalf("empty: %+v", e)
	}
	for _, d := range []string{"2026-11-14", "2026-11-15", "2026-11-16"} {
		writeDigest(t, dir, d, sampleDigest(d))
	}
	runExport(dir, 2, out, now)
	if e := readExport(t, out); len(e.Days) != 2 || e.Days[1].Day != "2026-11-15" {
		t.Fatalf("limit: %+v", e.Days)
	}
}

func TestExportSkipsJunk(t *testing.T) {
	dir := t.TempDir()
	writeDigest(t, dir, "2026-11-16", sampleDigest("2026-11-16"))
	os.WriteFile(filepath.Join(dir, "digest", "2026-11-16.md"), []byte("# md"), 0o644)
	os.WriteFile(filepath.Join(dir, "digest", "2026-11-15.json"), []byte("{broken"), 0o644)
	out := filepath.Join(dir, "web", "digests.json")
	if err := runExport(dir, 14, out, now); err != nil {
		t.Fatal(err)
	}
	if e := readExport(t, out); len(e.Days) != 1 || e.Days[0].Day != "2026-11-16" {
		t.Fatalf("e = %+v", e.Days)
	}
}

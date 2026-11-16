package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"skim/internal/store"
)

// ExportArticle is an article reference on the digest page.
type ExportArticle struct {
	ID        string `json:"id"`
	FeedTitle string `json:"feed_title"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Score     int    `json:"score,omitempty"`
}

// ExportStory is one story on the digest page.
type ExportStory struct {
	N        int             `json:"n"`
	Topic    string          `json:"topic"`
	Title    string          `json:"title"`
	Summary  string          `json:"summary"`
	Score    int             `json:"score"`
	Articles []ExportArticle `json:"articles"`
}

// ExportDay is one day's digest.
type ExportDay struct {
	Day     string          `json:"day"`
	Stories []ExportStory   `json:"stories"`
	More    []ExportArticle `json:"more"`
}

// Export is the data file the digest page reads (digests.json).
type Export struct {
	GeneratedAt string      `json:"generated_at"`
	Days        []ExportDay `json:"days"`
}

// runExport writes the last `days` digests, newest first, to outPath.
func runExport(dataDir string, days int, outPath string, now time.Time) error {
	all, err := store.DaysExt(dataDir, "digest", ".json")
	if err != nil {
		return err
	}
	ex := Export{GeneratedAt: now.Format(time.RFC3339), Days: []ExportDay{}}
	for i := len(all) - 1; i >= 0 && len(ex.Days) < days; i-- {
		b, err := os.ReadFile(filepath.Join(dataDir, "digest", all[i]+".json"))
		if err != nil {
			return err
		}
		var dg Digest
		if json.Unmarshal(b, &dg) != nil || len(dg.Stories) == 0 {
			continue // skip broken or empty digests
		}
		ex.Days = append(ex.Days, exportDay(all[i], dg))
	}
	js, err := json.MarshalIndent(ex, "", " ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	tmp, err := writeTemp(filepath.Dir(outPath), append(js, '\n'))
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	return os.Rename(tmp, outPath)
}

func exportDay(day string, dg Digest) ExportDay {
	d := ExportDay{Day: day, Stories: []ExportStory{}, More: []ExportArticle{}}
	ref := func(r Ref) ExportArticle {
		return ExportArticle{ID: r.ID, FeedTitle: r.FeedTitle, Title: r.Title, URL: r.URL, Score: r.Score}
	}
	for _, s := range dg.Stories {
		es := ExportStory{N: s.N, Topic: s.Topic, Title: s.Title, Summary: s.Summary, Score: s.Score, Articles: []ExportArticle{}}
		for _, a := range s.Articles {
			es.Articles = append(es.Articles, ref(a))
		}
		d.Stories = append(d.Stories, es)
	}
	for _, m := range dg.More {
		d.More = append(d.More, ref(m))
	}
	return d
}

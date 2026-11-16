package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Triage is one relevance judgement (one JSONL line in data/triage/).
type Triage struct {
	ID        string `json:"id"`
	Topic     string `json:"topic"`
	Score     int    `json:"score"`
	Reason    string `json:"reason"`
	TriagedAt string `json:"triaged_at"` // RFC3339
}

// ReadTriage reads dataDir/triage/<day>.jsonl, skipping malformed lines.
func ReadTriage(dataDir, day string) ([]Triage, error) {
	return readJSONL(filepath.Join(dataDir, "triage", day+".jsonl"), func(t Triage) bool { return t.ID != "" })
}

// TriagedIDs returns the ids of all triaged articles.
func TriagedIDs(dataDir string) (map[string]bool, error) {
	days, err := Days(dataDir, "triage")
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, d := range days {
		recs, err := ReadTriage(dataDir, d)
		if err != nil {
			return nil, err
		}
		for _, r := range recs {
			ids[r.ID] = true
		}
	}
	return ids, nil
}

// AppendTriage appends records to dataDir/triage/<day>.jsonl.
func AppendTriage(dataDir, day string, recs []Triage) error {
	return appendJSONL(filepath.Join(dataDir, "triage", day+".jsonl"), recs)
}

// AppendTriageDays appends records for several days as one unit: if any
// append fails, every touched file is restored to its previous size (or
// removed if it did not exist), so the batch is all or nothing.
func AppendTriageDays(dataDir string, byDay map[string][]Triage) error {
	days := make([]string, 0, len(byDay))
	for d := range byDay {
		days = append(days, d)
	}
	sort.Strings(days)

	type orig struct {
		path   string
		size   int64
		exists bool
	}
	var touched []orig
	rollback := func() {
		for _, o := range touched {
			if o.exists {
				os.Truncate(o.path, o.size)
			} else {
				os.Remove(o.path)
			}
		}
	}
	for _, d := range days {
		path := filepath.Join(dataDir, "triage", d+".jsonl")
		o := orig{path: path}
		st, err := os.Stat(path)
		switch {
		case err == nil && st.IsDir():
			rollback()
			return fmt.Errorf("%s is a directory", path)
		case err == nil:
			o.size, o.exists = st.Size(), true
		case !errors.Is(err, fs.ErrNotExist):
			rollback()
			return err
		}
		touched = append(touched, o)
		if err := appendJSONL(path, byDay[d]); err != nil {
			rollback()
			return err
		}
	}
	return nil
}

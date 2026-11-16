package main

import (
	"strings"

	"skim/internal/store"
)

// entry is an article of the day joined with its triage record (if any).
type entry struct {
	store.Article
	Triage *store.Triage
}

// dayData holds the articles of one day (first occurrence per id, file
// order) joined with that day's triage.
type dayData struct {
	Day   string
	Order []string
	ByID  map[string]*entry
}

func loadDay(dataDir, day string) (*dayData, error) {
	arts, err := store.ReadArticles(dataDir, day)
	if err != nil {
		return nil, err
	}
	recs, err := store.ReadTriage(dataDir, day)
	if err != nil {
		return nil, err
	}
	earlier, err := earlierIDs(dataDir, day)
	if err != nil {
		return nil, err
	}
	d := &dayData{Day: day, ByID: map[string]*entry{}}
	for _, a := range arts {
		if _, dup := d.ByID[a.ID]; dup || earlier[a.ID] {
			continue // duplicates belong to the day they first appeared (as in skimtriage)
		}
		d.ByID[a.ID] = &entry{Article: a}
		d.Order = append(d.Order, a.ID)
	}
	for i := range recs {
		if e, ok := d.ByID[recs[i].ID]; ok && e.Triage == nil {
			e.Triage = &recs[i]
		}
	}
	return d, nil
}

// earlierIDs returns ids of articles archived on days before day.
func earlierIDs(dataDir, day string) (map[string]bool, error) {
	days, err := store.Days(dataDir, "articles")
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, d := range days {
		if d >= day {
			break
		}
		arts, err := store.ReadArticles(dataDir, d)
		if err != nil {
			return nil, err
		}
		for _, a := range arts {
			ids[a.ID] = true
		}
	}
	return ids, nil
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	n := 0
	for i := range s {
		if n == max {
			return strings.TrimSpace(s[:i])
		}
		n++
	}
	return s
}

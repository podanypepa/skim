package main

import (
	"sort"

	"skim/internal/store"
)

// vote is the latest vote on one article.
type vote struct {
	ID, Vote, Note, Day, At string
}

// latestVotes flattens story votes to article votes; for each article the
// vote with the latest `at` wins. Result is newest first.
func latestVotes(dataDir string) ([]vote, error) {
	fb, err := store.ReadFeedback(dataDir)
	if err != nil {
		return nil, err
	}
	byID := map[string]vote{}
	for _, f := range fb {
		for _, id := range f.IDs {
			if prev, ok := byID[id]; ok && prev.At > f.At {
				continue
			}
			byID[id] = vote{ID: id, Vote: f.Vote, Note: f.Note, Day: f.Day, At: f.At}
		}
	}
	out := make([]vote, 0, len(byID))
	for _, v := range byID {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].At != out[j].At {
			return out[i].At > out[j].At
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// triageIndex returns every triage record by article id (first wins).
func triageIndex(dataDir string) (map[string]store.Triage, error) {
	days, err := store.Days(dataDir, "triage")
	if err != nil {
		return nil, err
	}
	idx := map[string]store.Triage{}
	for _, d := range days {
		recs, err := store.ReadTriage(dataDir, d)
		if err != nil {
			return nil, err
		}
		for _, r := range recs {
			if _, ok := idx[r.ID]; !ok {
				idx[r.ID] = r
			}
		}
	}
	return idx, nil
}

func isMismatch(vote string, score int) bool {
	return (vote == "down" && score >= 4) || (vote == "up" && score <= 3)
}

package main

import (
	"encoding/json"
	"io"
)

// Example is a past vote shown to triage as guidance.
type Example struct {
	ID        string `json:"id"`
	Topic     string `json:"topic"`
	FeedTitle string `json:"feed_title"`
	Title     string `json:"title"`
	Score     int    `json:"score"`
	Reason    string `json:"reason"`
	Vote      string `json:"vote"`
	Note      string `json:"note,omitempty"`
	Day       string `json:"day"`
	Mismatch  bool   `json:"mismatch"`
}

// runExamples prints up to n recent votes joined with their articles and
// triage, disagreements (mismatch) first.
func runExamples(dataDir string, n int, out io.Writer) error {
	votes, err := latestVotes(dataDir)
	if err != nil {
		return err
	}
	arts, err := loadArchive(dataDir)
	if err != nil {
		return err
	}
	byID := make(map[string]dayArticle, len(arts))
	for _, a := range arts {
		byID[a.ID] = a
	}
	tri, err := triageIndex(dataDir)
	if err != nil {
		return err
	}

	var mism, rest []Example
	for _, v := range votes {
		a, okA := byID[v.ID]
		t, okT := tri[v.ID]
		if !okA || !okT {
			continue
		}
		e := Example{ID: v.ID, Topic: a.Topic, FeedTitle: a.FeedTitle, Title: a.Title,
			Score: t.Score, Reason: t.Reason, Vote: v.Vote, Note: v.Note, Day: v.Day,
			Mismatch: isMismatch(v.Vote, t.Score)}
		if e.Mismatch {
			mism = append(mism, e)
		} else {
			rest = append(rest, e)
		}
	}
	all := append(mism, rest...)
	if n > 0 && len(all) > n {
		all = all[:n]
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	m := 0
	for _, e := range all {
		if e.Mismatch {
			m++
		}
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return enc.Encode(map[string]any{"summary": true, "examples": len(all), "mismatches": m})
}

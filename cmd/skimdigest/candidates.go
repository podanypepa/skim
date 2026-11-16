package main

import (
	"encoding/json"
	"io"
	"sort"
	"strconv"
)

// CandOptions configures `skimdigest candidates`.
type CandOptions struct {
	DataDir, Day string
	Min, Excerpt int
}

// Candidate is a triaged article offered for the digest.
type Candidate struct {
	ID        string `json:"id"`
	Topic     string `json:"topic"`
	Score     int    `json:"score"`
	Reason    string `json:"reason"`
	FeedTitle string `json:"feed_title"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Published string `json:"published,omitempty"`
	Excerpt   string `json:"excerpt"`
}

// CandSummary is the last output line of `candidates`.
type CandSummary struct {
	Summary    bool           `json:"summary"`
	Day        string         `json:"day"`
	Candidates int            `json:"candidates"`
	ByScore    map[string]int `json:"by_score"`
	Untriaged  int            `json:"untriaged"`
}

func runCandidates(o CandOptions, out io.Writer) error {
	d, err := loadDay(o.DataDir, o.Day)
	if err != nil {
		return err
	}
	sum := CandSummary{Summary: true, Day: o.Day, ByScore: map[string]int{}}
	var cs []Candidate
	for _, id := range d.Order {
		e := d.ByID[id]
		if e.Triage == nil {
			sum.Untriaged++
			continue
		}
		if e.Triage.Score < o.Min {
			continue
		}
		cs = append(cs, Candidate{
			ID: id, Topic: e.Topic, Score: e.Triage.Score, Reason: e.Triage.Reason,
			FeedTitle: e.FeedTitle, Title: e.Title, URL: e.URL, Published: e.Published,
			Excerpt: truncateRunes(bestText(e), o.Excerpt),
		})
		sum.ByScore[strconv.Itoa(e.Triage.Score)]++
	}
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].Score > cs[j].Score })
	sum.Candidates = len(cs)

	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	for _, c := range cs {
		if err := enc.Encode(c); err != nil {
			return err
		}
	}
	return enc.Encode(sum)
}

// bestText prefers fetched full text over a shorter feed excerpt.
func bestText(e *entry) string {
	if f := e.Fulltext; f != nil && f.Status == "ok" && len(f.Content) > len(e.Content) {
		return f.Content
	}
	return e.Content
}

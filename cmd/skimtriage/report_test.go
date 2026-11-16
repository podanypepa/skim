package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"skim/internal/store"
)

var reportNow = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

func report(t *testing.T, dir string, mark bool, now time.Time) Report {
	t.Helper()
	var out bytes.Buffer
	if err := runReport(dir, mark, now, &out); err != nil {
		t.Fatal(err)
	}
	var r Report
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("bad json %q: %v", out.String(), err)
	}
	return r
}

func TestReportAggregates(t *testing.T) {
	dir := examplesSetup(t)
	addVotes(t, dir,
		store.Feedback{Day: "2026-11-16", Story: 1, IDs: []string{"a", "b"}, Vote: "down", Note: "PR", At: "2026-11-16T09:00:00Z"},
		fbk("c", "up", "2026-11-16T10:00:00Z"))
	r := report(t, dir, false, reportNow)
	if r.TotalVotes != 3 || r.NewVotes != 3 {
		t.Fatalf("votes = %+v", r)
	}
	if len(r.ByFeed) != 1 || r.ByFeed[0].FeedTitle != "Feed" || r.ByFeed[0].Down != 2 || r.ByFeed[0].Up != 1 || r.ByFeed[0].AvgScore != 4.0 {
		t.Fatalf("by_feed = %+v", r.ByFeed)
	}
	if r.ByScore["5"]["down"] != 1 || r.ByScore["4"]["down"] != 1 || r.ByScore["3"]["up"] != 1 {
		t.Fatalf("by_score = %+v", r.ByScore)
	}
	if len(r.Notes) != 1 || r.Notes[0] != "PR" {
		t.Fatalf("notes = %v", r.Notes)
	}
}

func TestReportLastVoteWins(t *testing.T) {
	dir := examplesSetup(t)
	addVotes(t, dir, fbk("a", "up", "2026-11-16T09:00:00Z"), fbk("a", "down", "2026-11-16T11:00:00Z"))
	r := report(t, dir, false, reportNow)
	if r.TotalVotes != 1 || r.ByScore["5"]["down"] != 1 || r.ByScore["5"]["up"] != 0 {
		t.Fatalf("r = %+v", r)
	}
}

func TestReportDue(t *testing.T) {
	cases := []struct {
		votes   int
		ageDays int
		due     bool
	}{{20, 0, true}, {5, 8, true}, {5, 2, false}, {4, 30, false}}
	for _, c := range cases {
		dir := t.TempDir()
		var arts []store.Article
		var tri []store.Triage
		var fb []store.Feedback
		at := reportNow.Add(-time.Duration(c.ageDays) * 24 * time.Hour).Format(time.RFC3339)
		for i := range c.votes {
			id := fmt.Sprint("x", i)
			arts = append(arts, art(id, "ai"))
			tri = append(tri, store.Triage{ID: id, Topic: "ai", Score: 4, Reason: "r"})
			fb = append(fb, fbk(id, "up", at))
		}
		addArticles(t, dir, "2026-11-16", arts...)
		store.AppendTriage(dir, "2026-11-16", tri)
		addVotes(t, dir, fb...)
		if r := report(t, dir, false, reportNow); r.Due != c.due {
			t.Errorf("votes %d age %d: due = %v, want %v", c.votes, c.ageDays, r.Due, c.due)
		}
	}
}

func TestReportMark(t *testing.T) {
	dir := examplesSetup(t)
	addVotes(t, dir, store.Feedback{Day: "2026-11-16", Story: 1, IDs: []string{"a"}, Vote: "down", Note: "x", At: "2026-11-16T09:00:00Z"})
	report(t, dir, true, reportNow)
	r := report(t, dir, false, reportNow.Add(time.Hour))
	if r.NewVotes != 0 || len(r.Notes) != 0 || r.TotalVotes != 1 || r.DaysSinceLast == nil || *r.DaysSinceLast != 0 {
		t.Fatalf("after mark: %+v", r)
	}
}

func TestReportEmpty(t *testing.T) {
	r := report(t, examplesSetup(t), false, reportNow)
	if r.Due || r.TotalVotes != 0 || r.DaysSinceLast != nil {
		t.Fatalf("r = %+v", r)
	}
}

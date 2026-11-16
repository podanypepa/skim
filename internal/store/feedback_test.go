package store

import "testing"

func TestFeedbackRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if fb, err := ReadFeedback(dir); err != nil || fb != nil {
		t.Fatalf("missing file: %v %v", fb, err)
	}
	recs := []Feedback{{Day: "2026-11-16", Story: 1, IDs: []string{"a"}, Vote: "up", At: "2026-11-16T12:00:00Z"}}
	if err := AppendFeedback(dir, recs); err != nil {
		t.Fatal(err)
	}
	fb, err := ReadFeedback(dir)
	if err != nil || len(fb) != 1 || fb[0].Vote != "up" || fb[0].IDs[0] != "a" {
		t.Fatalf("fb = %+v %v", fb, err)
	}
}

func TestProposals(t *testing.T) {
	dir := t.TempDir()
	if p, err := LastProposal(dir); err != nil || p != nil {
		t.Fatalf("none: %v %v", p, err)
	}
	AppendProposal(dir, Proposal{At: "2026-11-01T00:00:00Z", Votes: 3})
	AppendProposal(dir, Proposal{At: "2026-11-09T00:00:00Z", Votes: 9})
	p, err := LastProposal(dir)
	if err != nil || p == nil || p.Votes != 9 {
		t.Fatalf("last = %+v %v", p, err)
	}
}

func TestFulltextRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := AppendFulltext(dir, "2026-11-16", []Fulltext{{ID: "a", URL: "https://e.com/a", Status: "ok", Content: "text <b>", FetchedAt: "x"}}); err != nil {
		t.Fatal(err)
	}
	r, err := ReadFulltext(dir, "2026-11-16")
	if err != nil || len(r) != 1 || r[0].Content != "text <b>" || r[0].Status != "ok" {
		t.Fatalf("r = %+v %v", r, err)
	}
}

package main

import (
	"encoding/json"
	"io"
	"math"
	"sort"
	"strconv"
	"time"

	"skim/internal/store"
)

const (
	dueVotes    = 20 // propose after this many new votes …
	dueMinVotes = 5  // … or after at least this many …
	dueMinDays  = 7  // … once this many days have passed
)

// FeedStat aggregates votes for one feed.
type FeedStat struct {
	FeedTitle string  `json:"feed_title"`
	Topic     string  `json:"topic"`
	Up        int     `json:"up"`
	Down      int     `json:"down"`
	AvgScore  float64 `json:"avg_score"`
}

// Report is the output of `skimtriage report`.
type Report struct {
	Due           bool                      `json:"due"`
	NewVotes      int                       `json:"new_votes"`
	TotalVotes    int                       `json:"total_votes"`
	DaysSinceLast *int                      `json:"days_since_last"`
	ByFeed        []FeedStat                `json:"by_feed"`
	ByScore       map[string]map[string]int `json:"by_score"`
	Notes         []string                  `json:"notes"`
}

// runReport aggregates votes (latest per article) and says whether a
// topics.md proposal is due; with mark it records that one was made.
func runReport(dataDir string, mark bool, now time.Time, out io.Writer) error {
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
	last, err := store.LastProposal(dataDir)
	if err != nil {
		return err
	}
	since := ""
	if last != nil {
		since = last.At
	}

	r := Report{ByFeed: []FeedStat{}, ByScore: map[string]map[string]int{}, Notes: []string{}}
	type acc struct {
		FeedStat
		sum int
	}
	feeds := map[string]*acc{}
	oldest := ""
	for _, v := range votes {
		a, okA := byID[v.ID]
		t, okT := tri[v.ID]
		if !okA || !okT {
			continue
		}
		r.TotalVotes++
		if v.At > since {
			r.NewVotes++
		}
		if oldest == "" || v.At < oldest {
			oldest = v.At
		}
		f := feeds[a.FeedTitle]
		if f == nil {
			f = &acc{FeedStat: FeedStat{FeedTitle: a.FeedTitle, Topic: a.Topic}}
			feeds[a.FeedTitle] = f
		}
		sc := strconv.Itoa(t.Score)
		if r.ByScore[sc] == nil {
			r.ByScore[sc] = map[string]int{"up": 0, "down": 0}
		}
		r.ByScore[sc][v.Vote]++
		if v.Vote == "up" {
			f.Up++
		} else {
			f.Down++
		}
		f.sum += t.Score
	}
	for _, f := range feeds {
		n := f.Up + f.Down
		f.AvgScore = math.Round(float64(f.sum)/float64(n)*10) / 10
		r.ByFeed = append(r.ByFeed, f.FeedStat)
	}
	sort.Slice(r.ByFeed, func(i, j int) bool {
		if r.ByFeed[i].Down != r.ByFeed[j].Down {
			return r.ByFeed[i].Down > r.ByFeed[j].Down
		}
		return r.ByFeed[i].FeedTitle < r.ByFeed[j].FeedTitle
	})

	fb, err := store.ReadFeedback(dataDir)
	if err != nil {
		return err
	}
	seenNote := map[string]bool{}
	for _, f := range fb {
		if f.At > since && f.Note != "" && !seenNote[f.Note] {
			seenNote[f.Note] = true
			r.Notes = append(r.Notes, f.Note)
		}
	}

	ref := since
	if ref == "" {
		ref = oldest
	}
	if t, err := time.Parse(time.RFC3339, ref); err == nil {
		d := int(now.Sub(t).Hours() / 24)
		r.DaysSinceLast = &d
	}
	r.Due = r.NewVotes >= dueVotes || (r.NewVotes >= dueMinVotes && r.DaysSinceLast != nil && *r.DaysSinceLast >= dueMinDays)

	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(r); err != nil {
		return err
	}
	if mark {
		return store.AppendProposal(dataDir, store.Proposal{At: now.Format(time.RFC3339), Votes: r.TotalVotes})
	}
	return nil
}

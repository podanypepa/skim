package triage

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/podanypepa/skim/internal/store"
)

// PendingOptions configures `skimtriage pending`.
type PendingOptions struct {
	DataDir, Topic       string
	Limit, Days, Excerpt int
	Now                  time.Time
}

// PendingItem is one article waiting for triage.
type PendingItem struct {
	ID        string `json:"id"`
	Day       string `json:"day"`
	Topic     string `json:"topic"`
	FeedTitle string `json:"feed_title"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Published string `json:"published,omitempty"`
	Excerpt   string `json:"excerpt"`
}

// PendingSummary is the last output line of `pending`.
type PendingSummary struct {
	Summary bool `json:"summary"`
	Pending int  `json:"pending"`
	Shown   int  `json:"shown"`
}

func runPending(opts PendingOptions, out io.Writer) error {
	if opts.Days < 1 {
		return fmt.Errorf("-days musí být alespoň 1, je %d", opts.Days)
	}
	arts, err := loadArchive(opts.DataDir)
	if err != nil {
		return err
	}
	done, err := store.TriagedIDs(opts.DataDir)
	if err != nil {
		return err
	}
	from := opts.Now.Local().AddDate(0, 0, -(opts.Days - 1)).Format("2006-01-02")

	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	sum := PendingSummary{Summary: true}
	for _, a := range arts {
		if a.Day < from || done[a.ID] || (opts.Topic != "" && a.Topic != opts.Topic) {
			continue
		}
		sum.Pending++
		if opts.Limit > 0 && sum.Shown >= opts.Limit {
			continue
		}
		sum.Shown++
		if err := enc.Encode(PendingItem{
			ID: a.ID, Day: a.Day, Topic: a.Topic, FeedTitle: a.FeedTitle,
			Title: a.Title, URL: a.URL, Published: a.Published,
			Excerpt: truncateRunes(a.Content, opts.Excerpt),
		}); err != nil {
			return err
		}
	}
	return enc.Encode(sum)
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

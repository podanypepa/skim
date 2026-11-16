// Command skimfetch downloads new articles from all feeds in feeds.yaml into
// the local archive: data/articles/YYYY-MM-DD.jsonl (by fetch date, append
// only) plus fetch state in data/state.json.
//
// Usage:
//
//	go run ./cmd/skimfetch [-feeds feeds.yaml] [-data data] [-topic slug] [-backfill-days 7] [-dry-run]
//
// Output is JSONL: one line per feed, then a summary line.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"skim/internal/feed"
	"skim/internal/feedsfile"
	"skim/internal/store"
)

const (
	userAgent   = "skim-fetch/0.1 (+personal RSS reader)"
	concurrency = 8
	maxContent  = 20000
	seenKeep    = 90 * 24 * time.Hour
	maxBody     = 20 << 20
)

// Options configures a fetch run.
type Options struct {
	FeedsPath, DataDir, Topic string
	BackfillDays              int
	DryRun                    bool
	Now                       time.Time
	Client                    *http.Client
}

// FeedResult is the per-feed output line.
type FeedResult struct {
	FeedURL  string `json:"feed_url"`
	Title    string `json:"title"`
	Topic    string `json:"topic"`
	OK       bool   `json:"ok"`
	Status   string `json:"status"` // "new" | "not_modified" | "no_new" | "error"
	New      int    `json:"new"`
	Error    string `json:"error,omitempty"`
	Failures int    `json:"failures"`
}

// Summary is the final output line.
type Summary struct {
	Summary bool `json:"summary"`
	Feeds   int  `json:"feeds"`
	OK      int  `json:"ok"`
	Failed  int  `json:"failed"`
	New     int  `json:"new"`
}

func main() {
	opts := Options{Client: &http.Client{Timeout: 20 * time.Second}}
	flag.StringVar(&opts.FeedsPath, "feeds", "feeds.yaml", "path to feeds.yaml")
	flag.StringVar(&opts.DataDir, "data", "data", "data directory")
	flag.StringVar(&opts.Topic, "topic", "", "only fetch feeds of this topic")
	flag.IntVar(&opts.BackfillDays, "backfill-days", 7, "on a feed's first fetch, keep only items newer than this")
	flag.BoolVar(&opts.DryRun, "dry-run", false, "fetch and report, but write nothing")
	flag.Parse()
	opts.Now = time.Now()

	if _, err := Run(opts, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "skimfetch:", err)
		os.Exit(1)
	}
}

type outcome struct {
	res   FeedResult
	arts  []store.Article
	state *store.FeedState
}

// Run fetches all selected feeds and writes results to out. It returns an
// error only for configuration or storage problems; failing feeds are
// reported in the output.
func Run(opts Options, out io.Writer) (Summary, error) {
	all, err := feedsfile.Load(opts.FeedsPath)
	if err != nil {
		return Summary{}, err
	}
	st, err := store.LoadState(opts.DataDir)
	if err != nil {
		return Summary{}, fmt.Errorf("load state: %w", err)
	}

	var feeds []feedsfile.Feed
	seenURL := map[string]bool{}
	for _, f := range all {
		if opts.Topic != "" && f.Topic != opts.Topic {
			continue
		}
		if seenURL[f.URL] { // duplicate entry: fetch once
			continue
		}
		seenURL[f.URL] = true
		feeds = append(feeds, f)
	}
	if opts.Topic != "" && len(feeds) == 0 {
		return Summary{}, fmt.Errorf("no feeds for topic %q in %s", opts.Topic, opts.FeedsPath)
	}

	outs := make([]outcome, len(feeds))
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for i, f := range feeds {
		wg.Add(1)
		go func(i int, f feedsfile.Feed) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			outs[i] = fetchFeed(opts, f, st.Feeds[f.URL])
		}(i, f)
	}
	wg.Wait()

	sum := Summary{Summary: true, Feeds: len(feeds)}
	var arts []store.Article
	for _, o := range outs {
		st.Feeds[o.res.FeedURL] = o.state
		arts = append(arts, o.arts...)
		sum.New += o.res.New
		if o.res.OK {
			sum.OK++
		} else {
			sum.Failed++
		}
	}

	if !opts.DryRun {
		// Articles first, state second: a crash in between can only cause
		// duplicates on the next run, never lost articles.
		if err := store.AppendArticles(opts.DataDir, opts.Now.Local(), arts); err != nil {
			return sum, fmt.Errorf("write articles: %w", err)
		}
		if err := st.Save(opts.DataDir); err != nil {
			return sum, fmt.Errorf("save state: %w", err)
		}
	}

	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	for _, o := range outs {
		if err := enc.Encode(o.res); err != nil {
			return sum, err
		}
	}
	return sum, enc.Encode(sum)
}

func fetchFeed(opts Options, f feedsfile.Feed, prev *store.FeedState) outcome {
	// First successful fetch (a feed whose first attempts failed still counts).
	first := prev == nil || prev.LastOK == ""
	fs := &store.FeedState{Seen: map[string]string{}}
	if prev != nil {
		*fs = *prev
		fs.Seen = make(map[string]string, len(prev.Seen))
		maps.Copy(fs.Seen, prev.Seen)
	}
	o := outcome{res: FeedResult{FeedURL: f.URL, Title: f.Title, Topic: f.Topic}, state: fs}
	stamp := opts.Now.Format(time.RFC3339)

	fail := func(err error) outcome {
		fs.Failures++
		fs.LastError = err.Error()
		o.res.Status, o.res.Error, o.res.Failures = "error", err.Error(), fs.Failures
		return o
	}
	succeed := func(status string) outcome {
		fs.Failures, fs.LastError, fs.LastOK = 0, "", stamp
		o.res.OK, o.res.Status, o.res.Failures = true, status, 0
		return o
	}

	req, err := http.NewRequest("GET", f.URL, nil)
	if err != nil {
		return fail(err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml;q=0.9, */*;q=0.5")
	if fs.ETag != "" {
		req.Header.Set("If-None-Match", fs.ETag)
	}
	if fs.LastModified != "" {
		req.Header.Set("If-Modified-Since", fs.LastModified)
	}
	resp, err := opts.Client.Do(req)
	if err != nil {
		return fail(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return succeed("not_modified")
	}
	if resp.StatusCode >= 400 {
		return fail(fmt.Errorf("HTTP %d", resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return fail(err)
	}
	if !feed.LooksLikeFeed(body, resp.Header.Get("Content-Type")) {
		return fail(fmt.Errorf("not a feed (Content-Type %q)", resp.Header.Get("Content-Type")))
	}
	parsed, err := feed.Parse(body)
	if err != nil {
		return fail(err)
	}

	feedTitle := f.Title
	if feedTitle == "" {
		feedTitle = parsed.Title
		o.res.Title = parsed.Title
	}
	base := resp.Request.URL // final URL after redirects
	cutoff := opts.Now.Add(-time.Duration(opts.BackfillDays) * 24 * time.Hour)
	// On a first fetch, undated items are kept only when the feed has no
	// dates at all; in a dated feed they are typically static pages
	// (e.g. Hugo pages with a zero date) rather than articles.
	feedHasDates := false
	for _, it := range parsed.Items {
		if !it.Published.IsZero() {
			feedHasDates = true
			break
		}
	}
	inDoc := map[string]bool{}
	for _, it := range parsed.Items {
		id := store.ArticleID(f.URL, it.GUID, it.Link, it.Title, it.Published)
		if inDoc[id] {
			continue
		}
		inDoc[id] = true
		if _, seen := fs.Seen[id]; seen {
			continue
		}
		fs.Seen[id] = stamp
		if first && (it.Published.IsZero() && feedHasDates || !it.Published.IsZero() && it.Published.Before(cutoff)) {
			continue // backfill limit: remember it, don't store it
		}
		a := store.Article{
			ID:        id,
			FeedURL:   f.URL,
			FeedTitle: feedTitle,
			Topic:     f.Topic,
			Title:     it.Title,
			URL:       resolve(base, it.Link),
			Author:    it.Author,
			FetchedAt: stamp,
			Content:   feed.HTMLToText(it.Content, maxContent),
		}
		if !it.Published.IsZero() {
			a.Published = it.Published.UTC().Format(time.RFC3339)
		}
		o.arts = append(o.arts, a)
	}
	fs.Prune(opts.Now, inDoc, seenKeep)
	fs.ETag = resp.Header.Get("ETag")
	fs.LastModified = resp.Header.Get("Last-Modified")
	o.res.New = len(o.arts)
	if o.res.New > 0 {
		return succeed("new")
	}
	return succeed("no_new")
}

// resolve makes a possibly relative item link absolute against the feed URL.
// The article ID keeps using the raw link, so IDs stay stable.
func resolve(base *url.URL, link string) string {
	if link == "" {
		return ""
	}
	u, err := url.Parse(link)
	if err != nil {
		return link
	}
	return base.ResolveReference(u).String()
}

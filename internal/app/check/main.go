// Package check validates RSS/Atom feeds and reports whether they are alive.
//
// Usage:
//
//	go run ./cmd/feedcheck [-samples N] <url> [<url> ...]
//
// Each URL may be a feed URL or a website homepage. For a homepage, feedcheck
// tries feed autodiscovery (<link rel="alternate">) and common feed paths.
// Output is one JSON object per line (JSONL) so agents can parse it easily.
package check

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/podanypepa/skim/internal/feed"
)

type Result struct {
	Input        string   `json:"input"`
	OK           bool     `json:"ok"`
	FeedURL      string   `json:"feed_url,omitempty"`
	Title        string   `json:"title,omitempty"`
	Site         string   `json:"site,omitempty"`
	Format       string   `json:"format,omitempty"`
	Items        int      `json:"items,omitempty"`
	Latest       string   `json:"latest,omitempty"`
	DaysSince    *int     `json:"days_since_latest,omitempty"`
	PostsPerWeek float64  `json:"posts_per_week,omitempty"`
	Samples      []string `json:"samples,omitempty"`
	Error        string   `json:"error,omitempty"`
}

var client = &http.Client{Timeout: 15 * time.Second}

const userAgent = "skim-feedcheck/0.1 (+personal RSS reader)"

// Main runs feedcheck with args (without the program name) and returns the exit code.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("feedcheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	samples := fs.Int("samples", 5, "number of latest item titles to include")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "usage: feedcheck [-samples N] <url> [<url> ...]")
		return 2
	}

	results := make([]Result, fs.NArg())
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, in := range fs.Args() {
		wg.Add(1)
		go func(i int, in string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = check(in, *samples, time.Now())
		}(i, in)
	}
	wg.Wait()

	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	for _, r := range results {
		_ = enc.Encode(r)
	}
	return 0
}

func check(input string, samples int, now time.Time) Result {
	r := Result{Input: input}
	u := input
	if !strings.Contains(u, "://") {
		u = "https://" + u
	}

	body, ctype, finalURL, err := fetch(u)
	if err != nil {
		r.Error = err.Error()
		return r
	}

	if feed.LooksLikeFeed(body, ctype) {
		parseInto(&r, finalURL, body, samples, now)
		return r
	}

	// Not a feed: treat as a website and try to discover a feed.
	for _, cand := range candidates(finalURL, body) {
		b, ct, fu, err := fetch(cand)
		if err != nil || !feed.LooksLikeFeed(b, ct) {
			continue
		}
		parseInto(&r, fu, b, samples, now)
		if r.OK {
			return r
		}
	}
	r.Error = "no feed found (not a feed, autodiscovery and common paths failed)"
	return r
}

func fetch(u string) ([]byte, string, string, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml;q=0.9, text/html;q=0.8, */*;q=0.5")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, "", "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, "", "", err
	}
	return b, resp.Header.Get("Content-Type"), resp.Request.URL.String(), nil
}

var (
	linkTagRe = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	attrRe    = regexp.MustCompile(`(?is)(\w+)\s*=\s*["']([^"']*)["']`)
)

// candidates returns feed URLs to try for an HTML page: autodiscovered first,
// then common paths.
func candidates(pageURL string, html []byte) []string {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	add := func(ref string) {
		ru, err := base.Parse(strings.TrimSpace(ref))
		if err != nil {
			return
		}
		s := ru.String()
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, tag := range linkTagRe.FindAllString(string(html), -1) {
		attrs := map[string]string{}
		for _, m := range attrRe.FindAllStringSubmatch(tag, -1) {
			attrs[strings.ToLower(m[1])] = m[2]
		}
		t := strings.ToLower(attrs["type"])
		if strings.Contains(strings.ToLower(attrs["rel"]), "alternate") &&
			(strings.Contains(t, "rss") || strings.Contains(t, "atom")) && attrs["href"] != "" {
			add(attrs["href"])
		}
	}
	for _, p := range []string{"/feed", "/rss", "/feed.xml", "/rss.xml", "/atom.xml", "/index.xml", "/feed/", "/blog/feed", "/blog/rss.xml"} {
		add(p)
	}
	return out
}

func parseInto(r *Result, feedURL string, body []byte, samples int, now time.Time) {
	r.FeedURL = feedURL
	f, err := feed.Parse(body)
	if err != nil {
		r.Error = err.Error()
		return
	}
	r.Format = f.Format
	r.Title = f.Title
	r.Site = f.Site
	items := f.Items

	r.OK = true
	r.Items = len(items)
	sort.SliceStable(items, func(i, j int) bool { return items[i].Published.After(items[j].Published) })
	for i := 0; i < len(items) && i < samples; i++ {
		r.Samples = append(r.Samples, items[i].Title)
	}

	var dated []time.Time
	for _, it := range items {
		if !it.Published.IsZero() {
			dated = append(dated, it.Published)
		}
	}
	if len(dated) == 0 {
		return
	}
	latest, oldest := dated[0], dated[len(dated)-1]
	r.Latest = latest.Format("2006-01-02")
	days := int(now.Sub(latest).Hours() / 24)
	r.DaysSince = &days
	if span := latest.Sub(oldest).Hours() / 24 / 7; len(dated) > 1 && span > 0 {
		r.PostsPerWeek = float64(int(float64(len(dated)-1)/span*10+0.5)) / 10
	}
}

// parseExit maps a flag parsing error to an exit code: -h/-help is 0
// (as with flag.ExitOnError), anything else 2.
func parseExit(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	return 2
}

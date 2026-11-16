package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"skim/internal/feed"
	"skim/internal/store"
)

const (
	userAgent     = "skim-fetch/0.1 (+personal RSS reader)"
	ftConcurrency = 4
	ftMaxBody     = 5 << 20
	ftMaxChars    = 20000
	ftMinChars    = 300
)

// FulltextOptions configures `skimdigest fulltext`.
type FulltextOptions struct {
	DataDir, Day string
	Min, Short   int
	Client       *http.Client
	Now          time.Time
}

var redditThreadRe = regexp.MustCompile(`^/r/[^/]+/comments/`)

// runFulltext fetches the article text for the day's candidates (score ≥
// Min) whose feed content is shorter than Short runes and that were not
// tried before; every id is tried at most once.
func runFulltext(o FulltextOptions, out io.Writer) error {
	d, err := loadDay(o.DataDir, o.Day)
	if err != nil {
		return err
	}
	var todo []*entry
	for _, id := range d.Order {
		e := d.ByID[id]
		if e.Triage == nil || e.Triage.Score < o.Min || e.Fulltext != nil ||
			utf8.RuneCountInString(e.Content) >= o.Short || e.URL == "" {
			continue
		}
		todo = append(todo, e)
	}

	robots := newRobotsCache(o.Client)
	// Same client, but every redirect hop must also pass robots.txt.
	client := *o.Client
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if !robots.allowed(req.URL) {
			return errRobots
		}
		return nil
	}
	recs := make([]store.Fulltext, len(todo))
	var wg sync.WaitGroup
	sem := make(chan struct{}, ftConcurrency)
	for i, e := range todo {
		wg.Add(1)
		go func(i int, e *entry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			recs[i] = fetchFulltext(&client, robots, e)
			recs[i].FetchedAt = o.Now.Format(time.RFC3339)
		}(i, e)
	}
	wg.Wait()

	// Transient failures (network, 429, 5xx) are not stored, so the next
	// run tries again; everything else is final.
	var keep []store.Fulltext
	for _, r := range recs {
		if r.Status != "retry" {
			keep = append(keep, r)
		}
	}
	if err := store.AppendFulltext(o.DataDir, o.Day, keep); err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	ok, retry := 0, 0
	for _, r := range recs {
		switch r.Status {
		case "ok":
			ok++
		case "retry":
			retry++
		}
		if err := enc.Encode(map[string]any{"id": r.ID, "url": r.URL, "status": r.Status, "chars": utf8.RuneCountInString(r.Content)}); err != nil {
			return err
		}
	}
	return enc.Encode(map[string]any{"summary": true, "tried": len(recs), "ok": ok, "retry": retry, "failed": len(recs) - ok - retry})
}

var errRobots = errors.New("disallowed by robots.txt")

// statusError is an HTTP error status.
type statusError int

func (s statusError) Error() string { return fmt.Sprintf("HTTP %d", int(s)) }

// failStatus maps a fetch error to robots / retry (transient) / error.
func failStatus(err error) string {
	var se statusError
	switch {
	case errors.Is(err, errRobots):
		return "robots"
	case errors.As(err, &se):
		if se == http.StatusTooManyRequests || se >= 500 {
			return "retry"
		}
		return "error"
	case errors.Is(err, errNotWanted):
		return "error"
	default:
		return "retry" // network, DNS, timeout
	}
}

var errNotWanted = errors.New("unexpected content type")

func fetchFulltext(c *http.Client, robots *robotsCache, e *entry) store.Fulltext {
	r := store.Fulltext{ID: e.ID, URL: e.URL}
	u, err := url.Parse(e.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		r.Status = "error"
		return r
	}
	if redditThreadRe.MatchString(u.Path) {
		target, self, err := redditTarget(c, robots, u)
		if err != nil {
			r.Status = failStatus(err)
			return r
		}
		if target == nil {
			return withText(r, self)
		}
		u = target
		r.URL = u.String()
	}
	if !robots.allowed(u) {
		r.Status = "robots"
		return r
	}
	body, final, err := get(c, u.String(), "text/html")
	if err != nil {
		r.Status = failStatus(err)
		return r
	}
	r.URL = final
	if !utf8.ValidString(body) {
		r.Status = "empty" // non-UTF-8 page: keep the clean feed excerpt
		return r
	}
	return withText(r, feed.ExtractText(body, ftMaxChars))
}

// withText stores already-plain text (no second HTML pass, which would
// eat "<" and double-unescape entities).
func withText(r store.Fulltext, text string) store.Fulltext {
	text = strings.TrimSpace(text)
	if rs := []rune(text); len(rs) > ftMaxChars {
		text = string(rs[:ftMaxChars])
	}
	if utf8.RuneCountInString(text) < ftMinChars {
		r.Status = "empty"
		return r
	}
	r.Status, r.Content = "ok", text
	return r
}

// redditTarget resolves a Reddit thread to the linked article, or returns
// the self post's text when the thread links to itself.
func redditTarget(c *http.Client, robots *robotsCache, thread *url.URL) (*url.URL, string, error) {
	j := *thread
	j.Path = strings.TrimSuffix(j.Path, "/") + ".json"
	j.RawQuery = ""
	if !robots.allowed(&j) {
		return nil, "", errRobots
	}
	body, _, err := get(c, j.String(), "")
	if err != nil {
		return nil, "", err
	}
	var listing []struct {
		Data struct {
			Children []struct {
				Data struct {
					URL      string `json:"url"`
					Selftext string `json:"selftext"`
					IsSelf   bool   `json:"is_self"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &listing); err != nil || len(listing) == 0 || len(listing[0].Data.Children) == 0 {
		return nil, "", fmt.Errorf("%w: reddit json", errNotWanted)
	}
	p := listing[0].Data.Children[0].Data
	if p.IsSelf {
		return nil, p.Selftext, nil
	}
	t, err := url.Parse(p.URL)
	if err != nil {
		return nil, "", err
	}
	return thread.ResolveReference(t), "", nil
}

// get fetches a URL; wantType (if set) must appear in the Content-Type.
func get(c *http.Client, u, wantType string) (string, string, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", "", statusError(resp.StatusCode)
	}
	if wantType != "" && !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), wantType) {
		return "", "", fmt.Errorf("%w: %s", errNotWanted, resp.Header.Get("Content-Type"))
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, ftMaxBody))
	if err != nil {
		return "", "", err
	}
	return string(b), resp.Request.URL.String(), nil
}

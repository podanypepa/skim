// Package store manages skim's on-disk archive: daily append-only JSONL
// article files and the fetch state (data/state.json).
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Article is one stored article (one JSONL line).
type Article struct {
	ID        string `json:"id"`
	FeedURL   string `json:"feed_url"`
	FeedTitle string `json:"feed_title"`
	Topic     string `json:"topic"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Author    string `json:"author,omitempty"`
	Published string `json:"published,omitempty"` // RFC3339, empty when unknown
	FetchedAt string `json:"fetched_at"`          // RFC3339
	Content   string `json:"content"`
}

// FeedState is the persisted fetch state of one feed.
type FeedState struct {
	ETag         string            `json:"etag,omitempty"`
	LastModified string            `json:"last_modified,omitempty"`
	LastOK       string            `json:"last_ok,omitempty"`
	LastError    string            `json:"last_error,omitempty"`
	Failures     int               `json:"failures"`
	Seen         map[string]string `json:"seen"` // article id -> RFC3339 time last seen
}

// State is the whole fetch state, keyed by feed URL.
type State struct {
	Feeds map[string]*FeedState `json:"feeds"`
}

const stateFile = "state.json"

// ArticleID returns a stable 16-hex-char id. Key priority: guid, link,
// title+published.
func ArticleID(feedURL, guid, link, title string, published time.Time) string {
	key := guid
	if key == "" {
		key = link
	}
	if key == "" {
		date := ""
		if !published.IsZero() {
			date = published.UTC().Format(time.RFC3339)
		}
		key = title + "\n" + date
	}
	sum := sha256.Sum256([]byte(feedURL + "\n" + key))
	return hex.EncodeToString(sum[:])[:16]
}

// LoadState reads dataDir/state.json; a missing file yields an empty state.
func LoadState(dataDir string) (*State, error) {
	s := &State{Feeds: map[string]*FeedState{}}
	b, err := os.ReadFile(filepath.Join(dataDir, stateFile))
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, err
	}
	if s.Feeds == nil {
		s.Feeds = map[string]*FeedState{}
	}
	for _, f := range s.Feeds {
		if f.Seen == nil {
			f.Seen = map[string]string{}
		}
	}
	return s, nil
}

// Save writes the state atomically (temp file + rename).
func (s *State) Save(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dataDir, "state-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after successful rename
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dataDir, stateFile))
}

// Prune drops seen ids older than keep, except ids still present in the
// current feed document, which are refreshed to now so they never come
// back as new.
func (fs *FeedState) Prune(now time.Time, inFeed map[string]bool, keep time.Duration) {
	stamp := now.Format(time.RFC3339)
	for id := range inFeed {
		fs.Seen[id] = stamp
	}
	cutoff := now.Add(-keep)
	for id, ts := range fs.Seen {
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil || t.Before(cutoff) {
			delete(fs.Seen, id)
		}
	}
}

// AppendArticles appends articles to dataDir/articles/<day>.jsonl.
func AppendArticles(dataDir string, day time.Time, arts []Article) error {
	return appendJSONL(filepath.Join(dataDir, "articles", day.Format("2006-01-02")+".jsonl"), arts)
}

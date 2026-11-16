package store

import "path/filepath"

// Fulltext is the article text fetched from the web for a digest candidate.
type Fulltext struct {
	ID        string `json:"id"`
	URL       string `json:"url"`    // final URL after redirects
	Status    string `json:"status"` // ok | empty | robots | error
	Content   string `json:"content,omitempty"`
	FetchedAt string `json:"fetched_at"`
}

// ReadFulltext reads dataDir/fulltext/<day>.jsonl, skipping malformed lines.
func ReadFulltext(dataDir, day string) ([]Fulltext, error) {
	return readJSONL(filepath.Join(dataDir, "fulltext", day+".jsonl"), func(f Fulltext) bool { return f.ID != "" })
}

// AppendFulltext appends records to dataDir/fulltext/<day>.jsonl.
func AppendFulltext(dataDir, day string, recs []Fulltext) error {
	return appendJSONL(filepath.Join(dataDir, "fulltext", day+".jsonl"), recs)
}

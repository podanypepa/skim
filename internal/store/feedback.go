package store

import "path/filepath"

// Feedback is one vote on a digest story (one line in data/feedback.jsonl).
type Feedback struct {
	Day   string   `json:"day"`
	Story int      `json:"story"`
	IDs   []string `json:"ids"`
	Vote  string   `json:"vote"` // "up" | "down"
	Note  string   `json:"note,omitempty"`
	At    string   `json:"at"` // RFC3339
}

// AppendFeedback appends records to dataDir/feedback.jsonl.
func AppendFeedback(dataDir string, recs []Feedback) error {
	return appendJSONL(filepath.Join(dataDir, "feedback.jsonl"), recs)
}

// ReadFeedback reads dataDir/feedback.jsonl, skipping malformed lines.
func ReadFeedback(dataDir string) ([]Feedback, error) {
	return readJSONL(filepath.Join(dataDir, "feedback.jsonl"), func(f Feedback) bool { return len(f.IDs) > 0 && f.Vote != "" })
}

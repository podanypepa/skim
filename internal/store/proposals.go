package store

import "path/filepath"

// Proposal records that topics.md edits were proposed to the user.
type Proposal struct {
	At    string `json:"at"`    // RFC3339
	Votes int    `json:"votes"` // total votes at that time
}

func proposalsPath(dataDir string) string { return filepath.Join(dataDir, "proposals.jsonl") }

// AppendProposal appends a record to dataDir/proposals.jsonl.
func AppendProposal(dataDir string, p Proposal) error {
	return appendJSONL(proposalsPath(dataDir), []Proposal{p})
}

// LastProposal returns the most recent proposal, or nil when there is none.
func LastProposal(dataDir string) (*Proposal, error) {
	ps, err := readJSONL(proposalsPath(dataDir), func(p Proposal) bool { return p.At != "" })
	if err != nil || len(ps) == 0 {
		return nil, err
	}
	return &ps[len(ps)-1], nil
}

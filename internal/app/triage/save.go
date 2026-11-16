package triage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/podanypepa/skim/internal/store"
)

const maxReason = 300

type scoreInput struct {
	ID     string          `json:"id"`
	Score  json.RawMessage `json:"score"`
	Reason string          `json:"reason"`
}

// runSave validates a batch of {id, score, reason} lines and appends it to
// data/triage/<day>.jsonl. Any invalid line rejects the whole batch.
func runSave(dataDir string, in io.Reader, now time.Time, out io.Writer) error {
	arts, err := loadArchive(dataDir)
	if err != nil {
		return err
	}
	byID := make(map[string]dayArticle, len(arts))
	for _, a := range arts {
		byID[a.ID] = a
	}
	done, err := store.TriagedIDs(dataDir)
	if err != nil {
		return err
	}

	var (
		problems []string
		inBatch  = map[string]bool{}
		byDay    = map[string][]store.Triage{}
		saved    int
		n        int
	)
	bad := func(format string, a ...any) {
		problems = append(problems, fmt.Sprintf("řádek %d: ", n)+fmt.Sprintf(format, a...))
	}

	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		n++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var rec scoreInput
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&rec); err != nil {
			bad("neplatný JSON: %v", err)
			continue
		}
		if _, err := dec.Token(); err != io.EOF {
			bad("za JSON objektem je další obsah (jeden záznam na řádek)")
			continue
		}
		a, exists := byID[rec.ID]
		switch {
		case rec.ID == "":
			bad("chybí id")
		case !exists:
			bad("článek %q v archivu neexistuje", rec.ID)
		case done[rec.ID]:
			bad("článek %q už je ohodnocený", rec.ID)
		case inBatch[rec.ID]:
			bad("článek %q je v dávce víckrát", rec.ID)
		}
		inBatch[rec.ID] = true
		score, ok := parseScore(rec.Score)
		if !ok {
			bad("score musí být celé číslo 1–5, je %s", orMissing(rec.Score))
		}
		reason := strings.TrimSpace(rec.Reason)
		if reason == "" {
			bad("chybí reason")
		} else if utf8.RuneCountInString(reason) > maxReason {
			bad("reason má %d znaků, max. %d", utf8.RuneCountInString(reason), maxReason)
		}
		if len(problems) > 0 || !exists {
			continue
		}
		byDay[a.Day] = append(byDay[a.Day], store.Triage{
			ID: rec.ID, Topic: a.Topic, Score: score, Reason: reason,
			TriagedAt: now.Format(time.RFC3339),
		})
		saved++
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if len(problems) > 0 {
		return errors.New("nic neuloženo:\n" + strings.Join(problems, "\n"))
	}
	if saved == 0 {
		return errors.New("nic neuloženo: na vstupu nejsou žádné záznamy")
	}
	if err := store.AppendTriageDays(dataDir, byDay); err != nil {
		return fmt.Errorf("nic neuloženo: zápis selhal: %w", err)
	}
	_, err = fmt.Fprintf(out, "{\"saved\":%d}\n", saved)
	return err
}

// parseScore accepts only a bare JSON integer 1–5 (no quotes, decimals or exponent).
func parseScore(raw json.RawMessage) (int, bool) {
	s := string(bytes.TrimSpace(raw))
	if len(s) != 1 || s[0] < '1' || s[0] > '5' {
		return 0, false
	}
	return int(s[0] - '0'), true
}

func orMissing(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "chybí"
	}
	return string(raw)
}

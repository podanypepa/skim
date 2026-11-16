package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"skim/internal/store"
)

const maxNote = 300

type feedbackInput struct {
	Story int    `json:"story"`
	Vote  string `json:"vote"`
	Note  string `json:"note"`
}

// runFeedback maps story numbers of the day's digest to article ids and
// appends the votes to data/feedback.jsonl — all or nothing.
func runFeedback(dataDir, day string, in io.Reader, now time.Time, out io.Writer) error {
	b, err := os.ReadFile(filepath.Join(dataDir, "digest", day+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("digest za %s neexistuje", day)
	}
	if err != nil {
		return err
	}
	var dg Digest
	if err := json.Unmarshal(b, &dg); err != nil {
		return fmt.Errorf("digest za %s je poškozený: %w", day, err)
	}
	stories := map[int]Story{}
	for _, s := range dg.Stories {
		stories[s.N] = s
	}

	var (
		problems []string
		recs     []store.Feedback
		n        int
	)
	bad := func(format string, a ...any) {
		problems = append(problems, fmt.Sprintf("řádek %d: ", n)+fmt.Sprintf(format, a...))
	}
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		n++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var f feedbackInput
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&f); err != nil {
			bad("neplatný JSON: %v", err)
			continue
		}
		if _, err := dec.Token(); err != io.EOF {
			bad("za JSON objektem je další obsah (jeden záznam na řádek)")
			continue
		}
		s, ok := stories[f.Story]
		if !ok {
			bad("příběh %d v digestu za %s není", f.Story, day)
		}
		if f.Vote != "up" && f.Vote != "down" {
			bad("vote musí být \"up\" nebo \"down\", je %q", f.Vote)
		}
		note := strings.TrimSpace(f.Note)
		if utf8.RuneCountInString(note) > maxNote {
			bad("note má %d znaků, max. %d", utf8.RuneCountInString(note), maxNote)
		}
		ids := make([]string, len(s.Articles))
		for i, a := range s.Articles {
			ids[i] = a.ID
		}
		recs = append(recs, store.Feedback{Day: day, Story: f.Story, IDs: ids, Vote: f.Vote, Note: note, At: now.Format(time.RFC3339)})
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if len(problems) > 0 {
		return errors.New("nic neuloženo:\n" + strings.Join(problems, "\n"))
	}
	if len(recs) == 0 {
		return errors.New("nic neuloženo: na vstupu nejsou žádné záznamy")
	}
	if err := store.AppendFeedback(dataDir, recs); err != nil {
		return err
	}
	type echo struct {
		N     int    `json:"n"`
		Title string `json:"title"`
		Vote  string `json:"vote"`
	}
	res := struct {
		Saved   int    `json:"saved"`
		Day     string `json:"day"`
		Stories []echo `json:"stories"`
	}{Saved: len(recs), Day: day}
	for _, r := range recs {
		res.Stories = append(res.Stories, echo{N: r.Story, Title: stories[r.Story].Title, Vote: r.Vote})
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	return enc.Encode(res)
}

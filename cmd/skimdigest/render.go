package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	maxTitle   = 120
	maxSummary = 600
)

// StoryInput is one story as sent by the skill.
type StoryInput struct {
	Topic   string   `json:"topic"`
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	IDs     []string `json:"ids"`
}

// DigestInput is the whole JSON document read by `render`.
type DigestInput struct {
	Stories []StoryInput `json:"stories"`
	More    []string     `json:"more"`
}

// Ref is an article reference with data taken from the archive.
type Ref struct {
	ID        string `json:"id"`
	FeedTitle string `json:"feed_title"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Score     int    `json:"score"`
}

// Story is a rendered story.
type Story struct {
	N        int    `json:"n"`
	Topic    string `json:"topic"`
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Score    int    `json:"score"`
	Articles []Ref  `json:"articles"`
}

// Digest is stored as data/digest/<day>.json.
type Digest struct {
	Day     string  `json:"day"`
	Stories []Story `json:"stories"`
	More    []Ref   `json:"more"`
}

func runRender(dataDir, day string, force bool, in io.Reader, out io.Writer) error {
	d, err := loadDay(dataDir, day)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	var input DigestInput
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		return fmt.Errorf("nic neuloženo: neplatný JSON: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("nic neuloženo: za JSON objektem je další obsah (pošli jeden objekt)")
	}

	var problems []string
	used := map[string]string{}
	ref := func(where, id string) (Ref, bool) {
		if prev, dup := used[id]; dup {
			problems = append(problems, fmt.Sprintf("%s: článek %q je už v %s", where, id, prev))
			return Ref{}, false
		}
		used[id] = where
		e, ok := d.ByID[id]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: článek %q v archivu dne %s není", where, id, day))
			return Ref{}, false
		}
		if e.Triage == nil {
			problems = append(problems, fmt.Sprintf("%s: článek %q není ohodnocený", where, id))
			return Ref{}, false
		}
		return Ref{ID: id, FeedTitle: e.FeedTitle, Title: e.Title, URL: e.URL, Score: e.Triage.Score}, true
	}

	dg := Digest{Day: day, Stories: []Story{}, More: []Ref{}}
	if len(input.Stories) == 0 {
		problems = append(problems, "žádné příběhy")
	}
	for i, s := range input.Stories {
		where := fmt.Sprintf("příběh %d", i+1)
		bad := func(msg string, a ...any) { problems = append(problems, where+": "+fmt.Sprintf(msg, a...)) }
		st := Story{N: i + 1, Topic: strings.TrimSpace(s.Topic), Title: strings.TrimSpace(s.Title), Summary: strings.TrimSpace(s.Summary)}
		switch {
		case st.Topic == "":
			bad("chybí topic")
		case strings.ContainsAny(st.Topic, "\r\n"):
			bad("topic nesmí obsahovat nový řádek")
		}
		switch {
		case st.Title == "":
			bad("chybí title")
		case strings.ContainsAny(st.Title, "\r\n"):
			bad("title nesmí obsahovat nový řádek")
		case utf8.RuneCountInString(st.Title) > maxTitle:
			bad("title má %d znaků, max. %d", utf8.RuneCountInString(st.Title), maxTitle)
		case strings.Contains(st.Title, "`"):
			bad("title nesmí obsahovat zpětný apostrof `")
		case hasLink(st.Title):
			bad("title nesmí obsahovat odkazy ani URL (odkazy doplní nástroj z archivu)")
		}
		switch {
		case st.Summary == "":
			bad("chybí summary")
		case utf8.RuneCountInString(st.Summary) > maxSummary:
			bad("summary má %d znaků, max. %d", utf8.RuneCountInString(st.Summary), maxSummary)
		case hasLink(st.Summary):
			bad("summary nesmí obsahovat odkazy ani URL (odkazy doplní nástroj z archivu)")
		case blockMarkupRe.MatchString(st.Summary):
			bad("summary nesmí mít řádek začínající markdownem (#, >, -, *, +, číslo s tečkou) – piš souvislý text")
		}
		if len(s.IDs) == 0 {
			bad("chybí ids")
		}
		for _, id := range s.IDs {
			if r, ok := ref(where, id); ok {
				st.Articles = append(st.Articles, r)
				st.Score = max(st.Score, r.Score)
			}
		}
		dg.Stories = append(dg.Stories, st)
	}
	for _, id := range input.More {
		if r, ok := ref("more", id); ok {
			dg.More = append(dg.More, r)
		}
	}
	if len(problems) > 0 {
		return errors.New("nic neuloženo:\n" + strings.Join(problems, "\n"))
	}

	dir := filepath.Join(dataDir, "digest")
	mdPath := filepath.Join(dir, day+".md")
	if _, err := os.Stat(mdPath); err == nil && !force {
		return fmt.Errorf("nic neuloženo: %s už existuje (přepsat jen s -force)", mdPath)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	js, err := json.MarshalIndent(dg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writePair(mdPath, []byte(renderMarkdown(dg)), filepath.Join(dir, day+".json"), append(js, '\n')); err != nil {
		return fmt.Errorf("nic neuloženo: zápis selhal: %w", err)
	}

	articles := len(dg.More)
	for _, s := range dg.Stories {
		articles += len(s.Articles)
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	return enc.Encode(map[string]any{"day": day, "stories": len(dg.Stories), "articles": articles, "path": mdPath})
}

// blockMarkupRe matches a summary line that markdown would turn into a
// heading, quote, list item or similar block.
var blockMarkupRe = regexp.MustCompile(`(?m)^\s*(#|>|[-*+]\s|\d+[.)]\s)`)

func hasLink(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "](") || strings.Contains(l, "<http") ||
		strings.Contains(l, "http://") || strings.Contains(l, "https://")
}

// writePair replaces the digest markdown and JSON as one unit: both temp
// files are written first; the markdown is renamed into place, then the
// JSON; if the JSON rename fails the previous markdown is restored, so
// feedback (which reads the JSON) never refers to a digest the user did
// not see.
func writePair(mdPath string, md []byte, jsonPath string, js []byte) error {
	old, oldErr := os.ReadFile(mdPath)
	tmpMD, err := writeTemp(filepath.Dir(mdPath), md)
	if err != nil {
		return err
	}
	defer os.Remove(tmpMD)
	tmpJSON, err := writeTemp(filepath.Dir(jsonPath), js)
	if err != nil {
		return err
	}
	defer os.Remove(tmpJSON)
	if err := os.Rename(tmpMD, mdPath); err != nil {
		return err
	}
	if err := os.Rename(tmpJSON, jsonPath); err != nil {
		if oldErr == nil {
			if t, terr := writeTemp(filepath.Dir(mdPath), old); terr == nil {
				os.Rename(t, mdPath)
			}
		} else {
			os.Remove(mdPath)
		}
		return err
	}
	return nil
}

func writeTemp(dir string, data []byte) (string, error) {
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return "", err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Days returns the dates (YYYY-MM-DD) of dataDir/<sub>/*.jsonl, ascending.
// A missing directory yields no days.
func Days(dataDir, sub string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(dataDir, sub))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var days []string
	for _, e := range entries {
		day, ok := strings.CutSuffix(e.Name(), ".jsonl")
		if !ok || e.IsDir() {
			continue
		}
		if _, err := time.Parse("2006-01-02", day); err == nil {
			days = append(days, day)
		}
	}
	sort.Strings(days)
	return days, nil
}

// ReadArticles reads dataDir/articles/<day>.jsonl, skipping malformed lines.
func ReadArticles(dataDir, day string) ([]Article, error) {
	return readJSONL(filepath.Join(dataDir, "articles", day+".jsonl"), func(a Article) bool { return a.ID != "" })
}

// readJSONL decodes one T per line; lines that fail to decode or are
// rejected by valid are skipped (e.g. a half-written last line).
func readJSONL[T any](path string, valid func(T) bool) ([]T, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []T
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	for sc.Scan() {
		var v T
		if json.Unmarshal(sc.Bytes(), &v) != nil || !valid(v) {
			continue
		}
		out = append(out, v)
	}
	return out, sc.Err()
}

// appendJSONL appends one JSON line per value to path, creating dirs.
func appendJSONL[T any](path string, vals []T) error {
	if len(vals) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := ensureTrailingNewline(path); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, v := range vals {
		if err := enc.Encode(v); err != nil {
			f.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ensureTrailingNewline terminates a half-written last line (crash during
// a previous append) so the next record starts on its own line.
func ensureTrailingNewline(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.Size() == 0 {
		return err
	}
	last := make([]byte, 1)
	if _, err := f.ReadAt(last, st.Size()-1); err != nil {
		return err
	}
	if last[0] == '\n' {
		return nil
	}
	_, err = f.WriteAt([]byte{'\n'}, st.Size())
	return err
}

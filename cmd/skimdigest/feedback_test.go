package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skim/internal/store"
)

func feedback(dir, input string) (string, error) {
	var out bytes.Buffer
	err := runFeedback(dir, day, strings.NewReader(input), now, &out)
	return out.String(), err
}

func withDigest(t *testing.T) string {
	dir := goldenDay(t)
	if _, err := render(dir, goldenInput, false); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readFeedback(t *testing.T, dir string) []store.Feedback {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "feedback.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	var out []store.Feedback
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var f store.Feedback
		if err := json.Unmarshal([]byte(l), &f); err != nil {
			t.Fatal(err)
		}
		out = append(out, f)
	}
	return out
}

func TestFeedbackOK(t *testing.T) {
	dir := withDigest(t)
	out, err := feedback(dir, `{"story":1,"vote":"up","note":"skvělé"}`+"\n"+`{"story":2,"vote":"down"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), `{"saved":2,"day":"2026-11-16"`) {
		t.Fatalf("out = %q", out)
	}
	fb := readFeedback(t, dir)
	if len(fb) != 2 || fb[0].Day != day || fb[0].Story != 1 || strings.Join(fb[0].IDs, ",") != "b,a" ||
		fb[0].Vote != "up" || fb[0].Note != "skvělé" || fb[0].At != "2026-11-16T12:00:00Z" || fb[1].Vote != "down" {
		t.Fatalf("feedback = %+v", fb)
	}
}

func TestFeedbackNoDigest(t *testing.T) {
	dir := goldenDay(t)
	if _, err := feedback(dir, `{"story":1,"vote":"up"}`); err == nil || !strings.Contains(err.Error(), "neexistuje") {
		t.Fatalf("err = %v", err)
	}
}

func TestFeedbackValidation(t *testing.T) {
	for name, in := range map[string]string{
		"story 99":      `{"story":99,"vote":"up"}`,
		"bad vote":      `{"story":1,"vote":"meh"}`,
		"long note":     `{"story":1,"vote":"up","note":"` + strings.Repeat("ř", 301) + `"}`,
		"unknown field": `{"story":1,"vote":"up","x":1}`,
		"empty":         "\n \n",
		"two objects":   `{"story":1,"vote":"up"} {"story":2,"vote":"up"}`,
	} {
		dir := withDigest(t)
		if _, err := feedback(dir, in); err == nil {
			t.Errorf("%s: expected error", name)
		}
		if len(readFeedback(t, dir)) != 0 {
			t.Errorf("%s: written", name)
		}
	}
}

func TestFeedbackAllOrNothing(t *testing.T) {
	dir := withDigest(t)
	if _, err := feedback(dir, `{"story":1,"vote":"up"}`+"\n"+`{"story":7,"vote":"up"}`); err == nil || !strings.Contains(err.Error(), "řádek 2") {
		t.Fatalf("err = %v", err)
	}
	if len(readFeedback(t, dir)) != 0 {
		t.Fatal("partial write")
	}
}

func TestFeedbackEchoesDayAndTitles(t *testing.T) {
	dir := withDigest(t)
	out, err := feedback(dir, `{"story":2,"vote":"up"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"day":"2026-11-16"`) || !strings.Contains(out, `"title":"Příběh dva"`) || !strings.Contains(out, `"saved":1`) {
		t.Fatalf("out = %s", out)
	}
}

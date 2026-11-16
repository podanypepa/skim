package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"skim/internal/store"
)

func addVotes(t *testing.T, dir string, fb ...store.Feedback) {
	t.Helper()
	if err := store.AppendFeedback(dir, fb); err != nil {
		t.Fatal(err)
	}
}

func fbk(id, vote, at string) store.Feedback {
	return store.Feedback{Day: "2026-11-16", Story: 1, IDs: []string{id}, Vote: vote, At: at}
}

func examplesSetup(t *testing.T) string {
	dir := t.TempDir()
	addArticles(t, dir, "2026-11-16", art("a", "ai"), art("b", "ai"), art("c", "go"))
	store.AppendTriage(dir, "2026-11-16", []store.Triage{
		{ID: "a", Topic: "ai", Score: 5, Reason: "ra"},
		{ID: "b", Topic: "ai", Score: 4, Reason: "rb"},
		{ID: "c", Topic: "go", Score: 3, Reason: "rc"},
	})
	return dir
}

func examples(t *testing.T, dir string, n int) ([]Example, map[string]any) {
	t.Helper()
	var out bytes.Buffer
	if err := runExamples(dir, n, &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var ex []Example
	for _, l := range lines[:len(lines)-1] {
		var e Example
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			t.Fatal(err)
		}
		ex = append(ex, e)
	}
	var sum map[string]any
	json.Unmarshal([]byte(lines[len(lines)-1]), &sum)
	return ex, sum
}

func TestExamplesBasic(t *testing.T) {
	dir := examplesSetup(t)
	addVotes(t, dir, fbk("a", "up", "2026-11-16T10:00:00Z"), fbk("b", "down", "2026-11-16T09:00:00Z"))
	ex, sum := examples(t, dir, 30)
	if len(ex) != 2 || ex[0].ID != "b" || !ex[0].Mismatch || ex[0].Score != 4 || ex[0].Reason != "rb" || ex[0].Title != "Title b" || ex[0].FeedTitle != "Feed" {
		t.Fatalf("ex = %+v", ex)
	}
	if ex[1].ID != "a" || ex[1].Mismatch || sum["examples"] != 2.0 || sum["mismatches"] != 1.0 {
		t.Fatalf("ex1 = %+v sum = %v", ex[1], sum)
	}
}

func TestExamplesLastVoteWins(t *testing.T) {
	dir := examplesSetup(t)
	addVotes(t, dir, fbk("a", "up", "2026-11-16T09:00:00Z"), fbk("a", "down", "2026-11-16T11:00:00Z"))
	ex, _ := examples(t, dir, 30)
	if len(ex) != 1 || ex[0].Vote != "down" || !ex[0].Mismatch {
		t.Fatalf("ex = %+v", ex)
	}
}

func TestExamplesSkipsUnknown(t *testing.T) {
	dir := examplesSetup(t)
	addArticles(t, dir, "2026-11-16", art("untriaged", "ai"))
	addVotes(t, dir, fbk("zzz", "up", "2026-11-16T09:00:00Z"), fbk("untriaged", "down", "2026-11-16T09:00:00Z"), fbk("c", "up", "2026-11-16T09:00:00Z"))
	ex, _ := examples(t, dir, 30)
	if len(ex) != 1 || ex[0].ID != "c" || !ex[0].Mismatch {
		t.Fatalf("ex = %+v", ex)
	}
}

func TestExamplesLimit(t *testing.T) {
	dir := examplesSetup(t)
	addVotes(t, dir, fbk("a", "up", "2026-11-16T09:00:00Z"), fbk("b", "up", "2026-11-16T10:00:00Z"), fbk("c", "down", "2026-11-16T11:00:00Z"))
	ex, _ := examples(t, dir, 2)
	if len(ex) != 2 || ex[0].ID != "c" || ex[1].ID != "b" {
		t.Fatalf("ex = %+v", ex)
	}
}

func TestExamplesNoFeedback(t *testing.T) {
	ex, sum := examples(t, examplesSetup(t), 30)
	if len(ex) != 0 || sum["examples"] != 0.0 {
		t.Fatalf("ex %+v sum %v", ex, sum)
	}
}

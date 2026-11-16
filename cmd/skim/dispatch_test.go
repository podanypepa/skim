package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/podanypepa/skim/internal/app/triage"
	"github.com/podanypepa/skim/internal/store"
)

func call(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, strings.NewReader(""), &out, &errb)
	return code, out.String(), errb.String()
}

func TestDispatchUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"foo"}} {
		code, out, errs := call(args...)
		if code != 2 || out != "" || !strings.Contains(errs, "skim fetch") {
			t.Errorf("%v: code=%d out=%q err=%q", args, code, out, errs)
		}
	}
	if code, _, _ := call("triage"); code != 2 {
		t.Errorf("triage without subcommand: code=%d", code)
	}
}

func TestDispatchHelpVersion(t *testing.T) {
	for _, h := range []string{"help", "-h", "--help"} {
		if code, out, _ := call(h); code != 0 || !strings.Contains(out, "skim digest") {
			t.Errorf("%s: code=%d out=%q", h, code, out)
		}
	}
	if code, out, _ := call("version"); code != 0 || strings.TrimSpace(out) != "skim dev" {
		t.Errorf("version: code=%d out=%q", code, out)
	}
}

func TestLegacyAndNewSame(t *testing.T) {
	d := t.TempDir()
	store.AppendArticles(d, time.Now(), []store.Article{{ID: "a", Topic: "ai", Title: "T", URL: "https://e.com/a", Content: "c"}})
	code, out, _ := call("triage", "pending", "-data", d)
	var legacy bytes.Buffer
	lcode := triage.Main([]string{"pending", "-data", d}, strings.NewReader(""), &legacy, &bytes.Buffer{})
	if code != 0 || lcode != 0 || out != legacy.String() || !strings.Contains(out, `"id":"a"`) {
		t.Fatalf("new=%d %q legacy=%d %q", code, out, lcode, legacy.String())
	}
}

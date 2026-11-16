// Package triage keeps the books for article triage (skim triage …); the
// judgement itself is done by the skim-triage skill.
//
// Subcommands:
//
//	pending  [-limit 25] [-days 7] [-topic slug] [-excerpt 1500] [-data data]
//	save     [-data data] < scores.jsonl
//	examples [-n 30] [-data data]
//	report   [-mark] [-data data]
//
// pending prints untriaged articles as JSONL plus a summary line.
// save reads {"id","score","reason"} lines and appends them to
// data/triage/<day>.jsonl — all or nothing.
package triage

import (
	"flag"
	"fmt"
	"io"
	"time"
)

const usageText = "usage: skimtriage pending|save|examples|report [flags]"

// Main runs the triage tool with args (subcommand first) and returns the exit code.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usageText)
		return 2
	}
	newFS := func(name string) *flag.FlagSet {
		fs := flag.NewFlagSet(name, flag.ContinueOnError)
		fs.SetOutput(stderr)
		return fs
	}
	var err error
	switch args[0] {
	case "pending":
		fs := newFS("pending")
		o := PendingOptions{Now: time.Now()}
		fs.StringVar(&o.DataDir, "data", "data", "data directory")
		fs.StringVar(&o.Topic, "topic", "", "only articles of this topic")
		fs.IntVar(&o.Limit, "limit", 25, "max articles to print")
		fs.IntVar(&o.Days, "days", 7, "only articles fetched in the last N days")
		fs.IntVar(&o.Excerpt, "excerpt", 1500, "excerpt length in characters")
		if fs.Parse(args[1:]) != nil {
			return 2
		}
		err = runPending(o, stdout)
	case "save":
		fs := newFS("save")
		dataDir := fs.String("data", "data", "data directory")
		if fs.Parse(args[1:]) != nil {
			return 2
		}
		err = runSave(*dataDir, stdin, time.Now(), stdout)
	case "examples":
		fs := newFS("examples")
		dataDir := fs.String("data", "data", "data directory")
		n := fs.Int("n", 30, "max examples")
		if fs.Parse(args[1:]) != nil {
			return 2
		}
		err = runExamples(*dataDir, *n, stdout)
	case "report":
		fs := newFS("report")
		dataDir := fs.String("data", "data", "data directory")
		mark := fs.Bool("mark", false, "record that a topics.md proposal was made")
		if fs.Parse(args[1:]) != nil {
			return 2
		}
		err = runReport(*dataDir, *mark, time.Now(), stdout)
	default:
		fmt.Fprintln(stderr, usageText)
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, "skimtriage:", err)
		return 1
	}
	return 0
}

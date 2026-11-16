// Command skimtriage keeps the books for article triage; the judgement
// itself is done by the skim-triage skill.
//
// Usage:
//
//	go run ./cmd/skimtriage pending [-limit 25] [-days 7] [-topic slug] [-excerpt 1500] [-data data]
//	go run ./cmd/skimtriage save [-data data] < scores.jsonl
//
// pending prints untriaged articles as JSONL plus a summary line.
// save reads {"id","score","reason"} lines and appends them to
// data/triage/<day>.jsonl — all or nothing.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "pending":
		fs := flag.NewFlagSet("pending", flag.ExitOnError)
		o := PendingOptions{Now: time.Now()}
		fs.StringVar(&o.DataDir, "data", "data", "data directory")
		fs.StringVar(&o.Topic, "topic", "", "only articles of this topic")
		fs.IntVar(&o.Limit, "limit", 25, "max articles to print")
		fs.IntVar(&o.Days, "days", 7, "only articles fetched in the last N days")
		fs.IntVar(&o.Excerpt, "excerpt", 1500, "excerpt length in characters")
		fs.Parse(os.Args[2:])
		err = runPending(o, os.Stdout)
	case "save":
		fs := flag.NewFlagSet("save", flag.ExitOnError)
		dataDir := fs.String("data", "data", "data directory")
		fs.Parse(os.Args[2:])
		err = runSave(*dataDir, os.Stdin, time.Now(), os.Stdout)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "skimtriage:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: skimtriage pending [flags] | skimtriage save [flags] < scores.jsonl")
	os.Exit(2)
}

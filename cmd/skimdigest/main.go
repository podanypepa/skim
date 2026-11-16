// Command skimdigest builds the daily digest. The skim-digest skill groups
// candidates into stories and writes summaries; this tool joins the data,
// validates the skill's output and renders the markdown with links taken
// from the archive.
//
// Usage:
//
//	go run ./cmd/skimdigest fulltext   [-day D] [-min 3] [-short 600] [-data data]
//	go run ./cmd/skimdigest candidates [-day D] [-min 3] [-excerpt 2000] [-data data]
//	go run ./cmd/skimdigest render     [-day D] [-force] [-data data] < digest.json
//	go run ./cmd/skimdigest feedback   [-day D] [-data data] < feedback.jsonl
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
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	dataDir := fs.String("data", "data", "data directory")
	day := fs.String("day", time.Now().Local().Format("2006-01-02"), "digest day (YYYY-MM-DD, day of fetch)")
	var (
		minScore, excerpt int
		force             bool
	)
	switch cmd {
	case "candidates":
		fs.IntVar(&minScore, "min", 3, "minimum triage score")
		fs.IntVar(&excerpt, "excerpt", 2000, "excerpt length in characters (0 = full)")
	case "render":
		fs.BoolVar(&force, "force", false, "overwrite an existing digest")
	case "feedback":
	case "fulltext":
		fs.IntVar(&minScore, "min", 3, "minimum triage score")
		fs.IntVar(&excerpt, "short", 600, "fetch full text when feed content is shorter than this (characters)")
	default:
		usage()
	}
	fs.Parse(os.Args[2:])

	var err error
	if _, perr := time.Parse("2006-01-02", *day); perr != nil {
		err = fmt.Errorf("-day musí být YYYY-MM-DD, je %q", *day)
	} else {
		switch cmd {
		case "candidates":
			err = runCandidates(CandOptions{DataDir: *dataDir, Day: *day, Min: minScore, Excerpt: excerpt}, os.Stdout)
		case "render":
			err = runRender(*dataDir, *day, force, os.Stdin, os.Stdout)
		case "feedback":
			err = runFeedback(*dataDir, *day, os.Stdin, time.Now(), os.Stdout)
		case "fulltext":
			err = runFulltext(FulltextOptions{DataDir: *dataDir, Day: *day, Min: minScore, Short: excerpt,
				Client: newSafeClient(), Now: time.Now()}, os.Stdout)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "skimdigest:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: skimdigest fulltext|candidates|render|feedback [flags]")
	os.Exit(2)
}

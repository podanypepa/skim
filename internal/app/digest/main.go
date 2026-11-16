// Package digest builds the daily digest (skim digest …). The skim-digest
// skill groups candidates into stories and writes summaries; this tool
// joins the data, validates the skill's output and renders the markdown
// with links taken from the archive.
//
// Subcommands:
//
//	export     [-days 14] [-data data]  → data/web/digests.json
//	fulltext   [-day D] [-min 3] [-short 600] [-data data]
//	candidates [-day D] [-min 3] [-excerpt 2000] [-data data]
//	render     [-day D] [-force] [-data data] < digest.json
//	feedback   [-day D] [-data data] < feedback.jsonl
package digest

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"
)

const usageText = "usage: skimdigest fulltext|candidates|render|feedback|export [flags]"

// Main runs the digest tool with args (subcommand first) and returns the exit code.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usageText)
		return 2
	}
	cmd := args[0]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	dataDir := fs.String("data", "data", "data directory")
	day := fs.String("day", time.Now().Local().Format("2006-01-02"), "digest day (YYYY-MM-DD, day of fetch)")
	var (
		minScore, excerpt, days, short int
		force                          bool
	)
	switch cmd {
	case "candidates":
		fs.IntVar(&minScore, "min", 3, "minimum triage score")
		fs.IntVar(&excerpt, "excerpt", 2000, "excerpt length in characters (0 = full)")
	case "render":
		fs.BoolVar(&force, "force", false, "overwrite an existing digest")
	case "feedback":
	case "export":
		fs.IntVar(&days, "days", 14, "how many days of digests to export")
	case "fulltext":
		fs.IntVar(&minScore, "min", 3, "minimum triage score")
		fs.IntVar(&short, "short", 600, "fetch full text when feed content is shorter than this (characters)")
	default:
		fmt.Fprintln(stderr, usageText)
		return 2
	}
	if err := fs.Parse(args[1:]); err != nil {
		return parseExit(err)
	}

	var err error
	if _, perr := time.Parse("2006-01-02", *day); perr != nil {
		err = fmt.Errorf("-day musí být YYYY-MM-DD, je %q", *day)
	} else {
		switch cmd {
		case "candidates":
			err = runCandidates(CandOptions{DataDir: *dataDir, Day: *day, Min: minScore, Excerpt: excerpt}, stdout)
		case "render":
			err = runRender(*dataDir, *day, force, stdin, stdout)
		case "feedback":
			err = runFeedback(*dataDir, *day, stdin, time.Now(), stdout)
		case "export":
			out := filepath.Join(*dataDir, "web", "digests.json")
			err = runExport(*dataDir, days, out, time.Now())
			if err == nil {
				fmt.Fprintln(stdout, out)
			}
		case "fulltext":
			err = runFulltext(FulltextOptions{DataDir: *dataDir, Day: *day, Min: minScore, Short: short,
				Client: newSafeClient(), Now: time.Now()}, stdout)
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "skimdigest:", err)
		return 1
	}
	return 0
}

// parseExit maps a flag parsing error to an exit code: -h/-help is 0
// (as with flag.ExitOnError), anything else 2.
func parseExit(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	return 2
}

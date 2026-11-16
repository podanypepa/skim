package main

import (
	"fmt"
	"io"

	"github.com/podanypepa/skim/internal/app/check"
	"github.com/podanypepa/skim/internal/app/digest"
	"github.com/podanypepa/skim/internal/app/fetch"
	"github.com/podanypepa/skim/internal/app/triage"
)

// version is set at build time: -ldflags "-X main.version=v0.1.0".
var version = "dev"

const helpText = `skim — personal RSS agent tools (used by the skim Claude Code skills)

Usage: skim <command> [arguments]

Commands:
  skim check <url>...                     verify feeds or find a website's feed
  skim fetch [-topic slug] [-dry-run]     fetch new articles into data/
  skim triage pending|save|examples|report  triage bookkeeping and feedback learning
  skim digest fulltext|candidates|render|feedback|export  build the daily digest
  skim version                            print the version
  skim help                               show this help

Run a command with -h for its flags.`

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, helpText)
		return 2
	}
	rest := args[1:]
	switch args[0] {
	case "check":
		return check.Main(rest, stdin, stdout, stderr)
	case "fetch":
		return fetch.Main(rest, stdin, stdout, stderr)
	case "triage":
		return triage.Main(rest, stdin, stdout, stderr)
	case "digest":
		return digest.Main(rest, stdin, stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, "skim", version)
		return 0
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, helpText)
		return 0
	default:
		fmt.Fprintf(stderr, "skim: unknown command %q\n\n%s\n", args[0], helpText)
		return 2
	}
}

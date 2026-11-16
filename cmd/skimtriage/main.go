// Command skimtriage is kept for compatibility; it is the same as "skim triage".
package main

import (
	"os"

	"github.com/podanypepa/skim/internal/app/triage"
)

func main() { os.Exit(triage.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

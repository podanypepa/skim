// Command skimdigest is kept for compatibility; it is the same as "skim digest".
package main

import (
	"os"

	"github.com/podanypepa/skim/internal/app/digest"
)

func main() { os.Exit(digest.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

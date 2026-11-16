// Command skimfetch is kept for compatibility; it is the same as "skim fetch".
package main

import (
	"os"

	"github.com/podanypepa/skim/internal/app/fetch"
)

func main() { os.Exit(fetch.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

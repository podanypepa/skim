// Command feedcheck is kept for compatibility; it is the same as "skim check".
package main

import (
	"os"

	"github.com/podanypepa/skim/internal/app/check"
)

func main() { os.Exit(check.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

// Command lunatic is a passive subdomain reconnaissance tool.
package main

import (
	"os"

	"github.com/lunalully/lunatic/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}

// Command lex is the Lexicon command-line interface. The daemon is a separate
// binary (lexicon-daemon) managed by systemd.
package main

import (
	"context"
	"os"

	"github.com/PandaX185/lexicon/internal/cli"
)

func main() {
	os.Exit(cli.New().Run(context.Background(), os.Args[1:]))
}

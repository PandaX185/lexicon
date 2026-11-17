// Command dictgen builds Lexicon's English SQLite dictionary from Princeton
// WordNet 3.0 and the CMU Pronouncing Dictionary. It is a build-time tool and is
// not installed with the application.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("dictgen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	wordnetDir := flags.String("wordnet-dir", "", "WordNet dict/ directory (required)")
	cmudictFile := flags.String("cmudict-file", "", "CMUdict .dict file (optional)")
	output := flags.String("output", "", "output SQLite database (required)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: dictgen --wordnet-dir DIR --output PATH [--cmudict-file PATH]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	if *wordnetDir == "" || *output == "" {
		flags.Usage()
		return 2
	}

	stats, err := Build(context.Background(), Options{
		WordNetDir:  *wordnetDir,
		CMUDictFile: *cmudictFile,
		OutputPath:  *output,
		Progress:    stderr,
	})
	if err != nil {
		fmt.Fprintln(stderr, "dictgen:", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %d entries, %d definitions, and %d forms to %s\n",
		stats.Entries, stats.Definitions, stats.Forms, *output)
	return 0
}

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/reallyoldfogie/mc-replay-go/mcpr"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <replay.mcpr> [replay2.mcpr ...]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Validates MCPR replay files for ReplayMod compatibility.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	verbose := flag.Bool("v", false, "Verbose output")
	quiet := flag.Bool("q", false, "Quiet mode (errors only)")
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	files := flag.Args()
	exitCode := 0

	for _, file := range files {
		// Check if file exists
		if _, err := os.Stat(file); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %s: file not found\n", file)
			exitCode = 1
			continue
		}

		// Validate the file
		if *verbose {
			fmt.Printf("Validating %s...\n", file)
		}

		var err error
		if *quiet {
			err = mcpr.ValidateFileQuiet(file)
		} else {
			err = mcpr.ValidateFile(file)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ %s: %v\n", filepath.Base(file), err)
			exitCode = 1
		} else {
			if !*quiet {
				fmt.Printf("✅ %s: valid\n", filepath.Base(file))
			}
		}
	}

	if exitCode == 0 && !*quiet {
		if len(files) > 1 {
			fmt.Printf("\nAll %d replay files are valid!\n", len(files))
		}
	}

	os.Exit(exitCode)
}

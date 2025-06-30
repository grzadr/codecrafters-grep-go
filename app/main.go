package main

import (
	"fmt"
	"io"
	"os"

	"github.com/codecrafters-io/grep-starter-go/internal"
)

const errorExitCode = 2

// Usage: echo <input_text> | your_program.sh -E <pattern>.
func main() {
	if len(os.Args) < 3 || os.Args[1] != "-E" {
		fmt.Fprintf(os.Stderr, "usage: mygrep -E <pattern>\n")
		os.Exit(errorExitCode) // 1 means no lines were selected, >1 means error
	}

	pattern := os.Args[2]

	line, err := io.ReadAll(
		os.Stdin,
	) // assume we're only dealing with a single line
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read input text: %v\n", err)
		os.Exit(errorExitCode)
	}

	ok, err := matchLine(line, pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(errorExitCode)
	}

	if !ok {
		os.Exit(1)
	}
	// default exit code is 0 which means succes
}

func matchLine(line []byte, pattern string) (ok bool, err error) {
	p := internal.NewPattern(pattern)

	return p.Match(line), nil
}

package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"

	"github.com/codecrafters-io/grep-starter-go/internal"
)

const (
	errorExitCode = 2
)

type Arguments struct {
	pattern   internal.Pattern
	recursive bool
	files     []string
}

func parseArguments(args []string) *Arguments {
	parsed := Arguments{files: make([]string, 0)}

	flag.BoolVar(&parsed.recursive, "r", false, "recursive mde")
	pattern := flag.String("E", "", "pattern")

	flag.Parse()

	if *pattern == "" {
		log.Println("usage: mygrep -E <pattern> [file]")
		os.Exit(errorExitCode) // 1 means no lines were selected, >1 means error
	}

	parsed.pattern = internal.NewPattern(*pattern)

	if flag.NArg() > 0 {
		parsed.files = flag.Args()
	}

	return &parsed
}

func (a *Arguments) hasFiles() bool {
	return a.files != nil
}

// Usage: echo <input_text> | your_program.sh -E <pattern>.
func main() {
	args := parseArguments(os.Args)

	var ok bool

	var err error

	switch len(args.files) {
	case 0:
		ok, err = matchLine(args.pattern)
	case 1:
		if args.recursive {
			ok, err = matchRecursive(args.pattern, args.files[0])
		} else {
			ok, err = matchFile(args.pattern, args.files[0])
		}
	default:
		ok, err = matchMultipleFiles(args.pattern, args.files)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(errorExitCode)
	}

	if !ok {
		os.Exit(1)
	}
	// default exit code is 0 which means succes
}

func matchLine(pattern internal.Pattern) (ok bool, err error) {
	line, err := io.ReadAll(
		os.Stdin,
	) // assume we're only dealing with a single line
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read input text: %v\n", err)
		os.Exit(errorExitCode)
	}

	return pattern.Match(line), nil
}

func matchFile(pattern internal.Pattern, filename string) (ok bool, err error) {
	for line := range pattern.MatchFile(filename) {
		if line.Err != nil {
			return ok, line.Err
		}

		fmt.Println(string(line.Data))

		ok = true
	}

	return
}

func matchMultipleFiles(
	pattern internal.Pattern,
	files []string,
) (ok bool, err error) {
	for _, filename := range files {
		for line := range pattern.MatchFile(filename) {
			if line.Err != nil {
				return ok, line.Err
			}

			fmt.Printf("%s:%s\n", filename, string(line.Data))

			ok = true
		}
	}

	return
}

func matchRecursive(
	pattern internal.Pattern,
	dirname string,
) (ok bool, err error) {
	files := make([]string, 0)

	fs.WalkDir(
		os.DirFS(dirname),
		".",
		func(fullpath string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}

			files = append(files, path.Join(dirname, fullpath))

			return nil
		},
	)

	return matchMultipleFiles(pattern, files)
}

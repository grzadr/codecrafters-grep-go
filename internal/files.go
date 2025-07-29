package internal

import (
	"bufio"
	"fmt"
	"iter"
	"os"
)

type Line struct {
	Data []byte
	Err  error
}

func readLines(filename string) iter.Seq[Line] {
	return func(yield func(Line) bool) {
		file, err := os.Open(filename)
		if err != nil {
			yield(Line{Err: fmt.Errorf("error opening %q: %w", filename, err)})

			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			if !yield(Line{Data: scanner.Bytes()}) {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			yield(
				Line{
					Err: fmt.Errorf(
						"error performing scan on %q: %w",
						filename,
						err,
					),
				},
			)
		}
	}
}

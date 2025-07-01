package internal

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

type byteReader struct {
	reader *bufio.Reader
	offset int
	done   bool
}

func NewRuneReader(b []byte) *byteReader {
	return &byteReader{reader: bufio.NewReader(bytes.NewReader(b))}
}

func NewRuneReaderFromString(s string) *byteReader {
	return &byteReader{reader: bufio.NewReader(strings.NewReader(s))}
}

func (rr byteReader) isDone() bool {
	return rr.done
}

func (rr *byteReader) readRune() (r rune, n int) {
	if rr.isDone() {
		return
	}

	r, n, err := rr.reader.ReadRune()
	rr.done = err != nil

	if err != nil {
		return
	}

	rr.reader.UnreadRune()

	return
}

func (rr *byteReader) readToken() (token string) {
	if rr.isDone() {
		return
	}

	for range 2 {
		r, n, err := rr.reader.ReadRune()
		if err != nil {
			rr.done = true

			return
		}

		rr.offset += n
		token = fmt.Sprintf("%s%s", token, string(r))

		if r != '\\' {
			break
		}
	}

	if token == AnyOfSymbol && rr.test('^') {
		token = NoneOfSymbol
	}

	return
}

func (rr *byteReader) discard(n int) {
	if rr.isDone() {
		return
	}

	d, err := rr.reader.Discard(n)
	rr.offset += d
	rr.done = err != nil
}

func (rr *byteReader) test(t byte) bool {
	if b, err := rr.reader.ReadByte(); err != nil {
		rr.done = true
	} else if b == t {
		return true
	} else {
		rr.reader.UnreadByte()
	}

	return false
	// d, err := rr.reader.Discard(n)
	// rr.offset += d
	// rr.done = err != nil
}

package internal

import (
	"fmt"
)

type runeReader struct {
	// reader *bufio.Reader
	runes  []rune
	offset int
}

func NewRuneReader(s string) *runeReader {
	return &runeReader{runes: []rune(s)}
}

func NewRuneReaderFromBytes(b []byte) *runeReader {
	// return &runeReader{reader: bufio.NewReader(bytes.NewReader(b))}
	return NewRuneReader(string(b))
}

func (rr runeReader) Len() int {
	return len(rr.runes)
}

func (rr runeReader) Left() int {
	return len(rr.runes) - rr.offset
}

func (rr runeReader) isDone() bool {
	return rr.Left() == 0
}

func (rr runeReader) peek() (r rune) {
	return rr.runes[rr.offset]
}

func (rr runeReader) prev() (r rune) {
	return rr.runes[max(0, rr.offset-1)]
}

func (rr *runeReader) discard(n int) (d int) {
	d = min(rr.Left(), n)
	rr.offset += d

	return
}

func (rr *runeReader) test(t byte) (ok bool) {
	if rr.isDone() {
		return
	}

	return rr.peek() == rune(t)
	// d, err := rr.reader.Discard(n)
	// rr.offset += d
	// rr.done = err != nil
}

func (rr *runeReader) reset(offset int) {
	if offset > rr.Len() {
		panic(
			fmt.Sprintf(
				"cannot reset offset: %d is larger than len %d",
				offset,
				rr.Len(),
			),
		)
	}

	rr.offset = offset
}

func (rr *runeReader) readRune() (r rune, ok bool) {
	if rr.isDone() {
		return
	}

	r = rr.peek()
	rr.offset += 1
	ok = true

	return
}

func (rr *runeReader) unreadRune() {
	rr.offset--
}

func (rr *runeReader) readToken() (token string, ok bool) {
	if rr.isDone() {
		return
	}

	for range 2 {
		var r rune

		if r, ok = rr.readRune(); !ok {
			return
		}

		token = fmt.Sprintf("%s%s", token, string(r))

		if r != '\\' {
			break
		}
	}

	if token == AnyOfSymbol && rr.test('^') {
		token = NoneOfSymbol

		rr.discard(1)
	} else if token == `\\` {
		return rr.readToken()
	}

	return
}

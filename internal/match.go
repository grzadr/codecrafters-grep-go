package internal

import (
	"bufio"
	"bytes"
	"unicode"
)

type RuneReader struct {
	reader *bufio.Reader
	offset int
	done   bool
}

func NewRuneReader(b []byte) *RuneReader {
	return &RuneReader{reader: bufio.NewReader(bytes.NewReader(b))}
}

func (rr RuneReader) isDone() bool {
	return rr.done
}

func (rr *RuneReader) readRune() (r rune) {
	if rr.isDone() {
		return
	}

	r, size, err := rr.reader.ReadRune()
	rr.offset += size
	rr.done = err != nil

	return
}

func (rr *RuneReader) safeReadRune() (r rune, n int) {
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

func (rr *RuneReader) discard(n int) {
	d, err := rr.reader.Discard(n)
	rr.offset += d
	rr.done = err != nil
}

type MatchExpression interface {
	Match(reader *RuneReader) bool
	// SafeMatch(reader *RuneReader) bool
}

type CharacterClass interface {
	MatchExpression
	MatchRune(r rune) bool
	EqualCode(ref string) bool
}

type BaseCharacterClass string

func (b BaseCharacterClass) EqualCode(other string) bool {
	return string(b) == other
}

func (b BaseCharacterClass) MatchRune(r rune) bool {
	return []rune(b)[0] == r
}

func (b *BaseCharacterClass) Match(reader *RuneReader) (matched bool) {
	r, n := reader.safeReadRune()

	if matched = b.MatchRune(r); matched {
		reader.discard(n)
	}

	return
}

type DecimalExpression BaseCharacterClass

func NewDecimalExpression() DecimalExpression {
	return DecimalExpression(`\d`)
}

func (d *DecimalExpression) MatchRune(r rune) bool {
	return unicode.IsNumber(r)
}

func (d *DecimalExpression) Match(reader *RuneReader) (matched bool) {
	r, n := reader.safeReadRune()

	if matched = d.MatchRune(r); matched {
		reader.discard(n)
	}

	return
}

type AlphanumericExpression BaseCharacterClass

func NewAlphanumericExpression() AlphanumericExpression {
	return AlphanumericExpression(`\w`)
}

func (d *AlphanumericExpression) MatchRune(r rune) bool {
	return unicode.IsNumber(r)
}

type LiteralExpression BaseCharacterClass

func NewLiteralExpression(s string) LiteralExpression {
	return LiteralExpression(s)
}

type Pattern struct {
	expressions []MatchExpression
}

func NewPattern(input string) Pattern {
}

func (p Pattern) Match(line []byte) bool

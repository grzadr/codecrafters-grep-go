package internal

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
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

type MatchExpression interface {
	MatchRune(r rune) bool
	Match(reader *byteReader) bool
	MatchesMin() int
	// SafeMatch(reader *RuneReader) bool
}

// type CharacterClass interface {
// 	MatchExpression
// 	MatchRune(r rune) bool
// 	// EqualCode(ref string) bool
// }

type BaseMatchExpression func(r rune) bool

// func (b BaseCharacterClass) EqualCode(other string) bool {
// 	return string(b) == other
// }

type MatchExpressionSymbol string

const (
	DecimalSymbol        = `\d`
	AlphanumericSymbol   = `\w`
	AnyOfSymbol          = `[`
	NoneOfSymbol         = `[^`
	AnyNoneOfSymbolClose = `]`
	AtStartSymbol        = `^`
	AtEndSymbol          = `$`
)

func NewCharacterClass(expr string) BaseMatchExpression {
	switch expr {
	case DecimalSymbol:
		return BaseMatchExpression(unicode.IsNumber)
	case AlphanumericSymbol:
		return BaseMatchExpression(func(r rune) bool {
			return unicode.IsDigit(r) || unicode.IsLetter(r) || r == '_'
		})
	default:
		if utf8.RuneCountInString(expr) != 1 {
			panic("character expression %q must be single rune")
		}

		return BaseMatchExpression(func(r rune) bool {
			return []rune(expr)[0] == r
		})
	}
}

func (b BaseMatchExpression) MatchRune(r rune) bool {
	return b(r)
}

func (b BaseMatchExpression) Match(reader *byteReader) (matched bool) {
	r, n := reader.readRune()

	if matched = b.MatchRune(r); matched {
		reader.discard(n)
	}

	return
}

func (b BaseMatchExpression) MatchesMin() int {
	return 1
}

type AnyOfExpression struct {
	expressions []MatchExpression
}

func NewAnyOfExpression(r *byteReader) (expr AnyOfExpression) {
	expr = AnyOfExpression{
		expressions: make([]MatchExpression, 0),
	}

	for !r.isDone() && !r.test(AnyNoneOfSymbolClose[0]) {
		expr.expressions = append(expr.expressions, NewMatchExpression(r))
	}

	return
}

func (e AnyOfExpression) MatchRune(r rune) (matched bool) {
	for _, expr := range e.expressions {
		if expr.MatchRune(r) {
			return true
		}
	}

	return
}

func (e AnyOfExpression) Match(reader *byteReader) (matched bool) {
	r, n := reader.readRune()

	if matched = e.MatchRune(r); matched {
		reader.discard(n)
	}

	return
}

func (e AnyOfExpression) MatchesMin() int {
	return 1
}

type NoneOfExpression struct {
	AnyOfExpression
}

func NewNoneOfExpression(r *byteReader) (expr NoneOfExpression) {
	expr = NoneOfExpression{
		AnyOfExpression: NewAnyOfExpression(r),
	}

	return
}

func (e NoneOfExpression) MatchRune(r rune) (matched bool) {
	return !e.AnyOfExpression.MatchRune(r)
}

type AtStartExpression struct {
	MatchExpression
}

func NewAtStartExpression(reader *byteReader) AtStartExpression {
	return AtStartExpression{
		MatchExpression: NewMatchExpression(reader),
	}
}

func (e AtStartExpression) Match(reader *byteReader) (matched bool) {
	if reader.offset != 0 {
		return false
	}

	return e.MatchExpression.Match(reader)
}

type AtEndExpression struct {
	MatchExpression
}

func NewAtEndExpression(expr MatchExpression) AtEndExpression {
	return AtEndExpression{
		MatchExpression: expr,
	}
}

func (e AtEndExpression) Match(reader *byteReader) (matched bool) {
	if !e.MatchExpression.Match(reader) {
		return
	}

	_, n := reader.readRune()

	return n == 0
}

func NewMatchExpression(reader *byteReader) MatchExpression {
	switch t := reader.readToken(); t {
	case AnyOfSymbol:
		return NewAnyOfExpression(reader)
	case NoneOfSymbol:
		return NewNoneOfExpression(reader)
	case AtStartSymbol:
		return NewAtStartExpression(reader)
	case AtEndSymbol:
		return AtEndExpression{}
	default:
		return NewCharacterClass(t)
	}
}

type Pattern struct {
	expressions []MatchExpression
}

func NewPattern(expr string) (p Pattern) {
	reader := NewRuneReaderFromString(expr)

	p = Pattern{
		expressions: make([]MatchExpression, 0, len(expr)),
	}

	for !reader.isDone() {
		// p.expressions = append(p.expressions, )
		p.append(NewMatchExpression(reader))
	}

	return
}

func (p Pattern) Len() int {
	return len(p.expressions)
}

func (p Pattern) Last() MatchExpression {
	return p.expressions[p.Len()]
}

func (p Pattern) Match(line []byte) bool {
	reader := NewRuneReader(line)

	for _, expr := range p.expressions {
		if !expr.Match(reader) {
			return false
		}
	}

	return true
}

func (p *Pattern) append(expr MatchExpression) {
	switch expr.(type) {
	case AtEndExpression:
		p.expressions = append(p.expressions, NewAtEndExpression(p.Last()))
		p.expressions = p.expressions[:p.Len()-1]
	default:
		p.expressions = append(p.expressions, expr)
	}
}

package internal

import (
	"fmt"
	"log"
	"unicode"
	"unicode/utf8"
)

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
			panic(
				fmt.Sprintf(
					"character expression %q must be single rune",
					expr,
				),
			)
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

	if n == 0 {
		return
	}

	if matched = b.MatchRune(r); !matched {
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
		log.Printf("%+v %q\n", e.expressions, string(r))

		if expr.MatchRune(r) {
			return true
		}
	}

	return
}

func (e AnyOfExpression) Match(reader *byteReader) (matched bool) {
	r, n := reader.readRune()

	if n == 0 {
		return
	}

	if matched = e.MatchRune(r); !matched {
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

func (e NoneOfExpression) Match(reader *byteReader) (matched bool) {
	if _, err := reader.reader.Peek(1); err != nil {
		return
	}

	return !e.AnyOfExpression.Match(reader)
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
	t := reader.readToken()
	log.Printf("token %q", t)

	switch t {
	case AnyOfSymbol:
		return NewAnyOfExpression(reader)
	case NoneOfSymbol:
		log.Println("none of symbol")

		return NewNoneOfExpression(reader)
	case AtStartSymbol:
		return NewAtStartExpression(reader)
	case AtEndSymbol:
		return AtEndExpression{}
	case "":
		return nil
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

	for !reader.isDone() {
		matched := true

		for _, expr := range p.expressions {
			log.Printf("expr %+v\n", expr)

			if !expr.Match(reader) {
				log.Println("not matched")

				matched = false

				break
			}
		}

		if matched {
			return true
		}

		log.Printf("offset %d", reader.offset)

		reader.discard(1)
	}

	return false
}

func (p *Pattern) append(expr MatchExpression) {
	switch expr.(type) {
	case nil:
	case AtEndExpression:
		p.expressions = append(p.expressions, NewAtEndExpression(p.Last()))
		p.expressions = p.expressions[:p.Len()-1]
	default:
		p.expressions = append(p.expressions, expr)
	}
}

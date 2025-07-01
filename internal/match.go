package internal

import (
	"fmt"
	"log"
	"unicode"
	"unicode/utf8"
)

type MatchExpression interface {
	// MatchRune(r rune) bool
	Match(reader *runeReader) (matched bool, n int)
	// MatchesMin() int
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
	WildcardSymbol       = `.`
	AlterationSymbol     = `|`
)

func NewCharacterClass(expr string) BaseMatchExpression {
	switch expr {
	case DecimalSymbol:
		return BaseMatchExpression(unicode.IsNumber)
	case AlphanumericSymbol:
		return BaseMatchExpression(func(r rune) bool {
			return unicode.IsDigit(r) || unicode.IsLetter(r) || r == '_'
		})
	case WildcardSymbol:
		return BaseMatchExpression(func(r rune) bool {
			return true
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

// func (b BaseMatchExpression) MatchRune(r rune) bool {
// 	return b(r)
// }

func (b BaseMatchExpression) Match(reader *runeReader) (matched bool, n int) {
	r, ok := reader.readRune()

	if !ok {
		return
	}

	matched = b(r)
	n = 1

	return
}

type AnyOfExpression struct {
	expressions []MatchExpression
}

func NewAnyOfExpression(r *runeReader) (expr AnyOfExpression) {
	expr = AnyOfExpression{
		expressions: make([]MatchExpression, 0),
	}

	for !r.isDone() && !r.test(AnyNoneOfSymbolClose[0]) {
		expr.expressions = append(expr.expressions, NewMatchExpression(r))
	}

	r.discard(1)

	return
}

// func (e AnyOfExpression) MatchRune(r rune) (matched bool) {
// 	for _, expr := range e.expressions {
// 		log.Printf("%+v %q\n", e.expressions, string(r))

// 		if expr.MatchRune(r) {
// 			return true
// 		}
// 	}

// 	return
// }

func (e AnyOfExpression) Match(reader *runeReader) (matched bool, n int) {
	if reader.isDone() {
		return
	}

	offset := reader.offset

	for _, expr := range e.expressions {
		reader.reset(offset)

		if matched, n = expr.Match(reader); matched {
			return
		}
	}

	return
}

func (e AnyOfExpression) MatchesMin() int {
	return 1
}

type NoneOfExpression struct {
	AnyOfExpression
}

func NewNoneOfExpression(r *runeReader) (expr NoneOfExpression) {
	expr = NoneOfExpression{
		AnyOfExpression: NewAnyOfExpression(r),
	}

	return
}

func (e NoneOfExpression) Match(reader *runeReader) (matched bool, n int) {
	if reader.isDone() {
		return
	}

	matched, n = e.AnyOfExpression.Match(reader)

	log.Println("noneof matched:", matched)

	return !matched, n
}

type AtStartExpression struct {
	MatchExpression
}

func NewAtStartExpression(reader *runeReader) AtStartExpression {
	return AtStartExpression{
		MatchExpression: NewMatchExpression(reader),
	}
}

func (e AtStartExpression) Match(reader *runeReader) (matched bool, n int) {
	if reader.offset != 0 {
		return
	}

	matched, n = e.MatchExpression.Match(reader)

	return
}

type AtEndExpression struct {
	MatchExpression
}

func NewAtEndExpression(expr MatchExpression) AtEndExpression {
	return AtEndExpression{
		MatchExpression: expr,
	}
}

func (e AtEndExpression) Match(reader *runeReader) (matched bool, n int) {
	matched, n = e.MatchExpression.Match(reader)

	return matched && reader.isDone(), n
}

func NewMatchExpression(reader *runeReader) MatchExpression {
	t, ok := reader.readToken()

	if !ok {
		return nil
	}

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
	default:
		return NewCharacterClass(t)
	}
}

type Pattern struct {
	expressions []MatchExpression
}

func NewPattern(expr string) (p Pattern) {
	reader := NewRuneReader(expr)

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
	reader := NewRuneReaderFromBytes(line)
	offset := reader.offset

	for !reader.isDone() {
		all_matched := true

		for _, expr := range p.expressions {
			log.Printf("expr %+v\n", expr)

			if matched, _ := expr.Match(reader); !matched {
				log.Println("not matched")

				all_matched = false

				break
			}
		}

		if all_matched {
			return true
		}

		offset++
		log.Printf("offset %d reset to %d", reader.offset, offset)
		reader.reset(offset)
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

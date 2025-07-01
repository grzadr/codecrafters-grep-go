package internal

import (
	"fmt"
	"log"
	"unicode"
	"unicode/utf8"
)

type MatchResult struct {
	offset   int
	mainSize int
	size     int
}

func (r MatchResult) Len() int {
	return r.size
}

func (r MatchResult) MainLen() int {
	return r.mainSize
}

func (r MatchResult) Ok() bool {
	return r.Len() > 0
}

type MatchExpression interface {
	// MatchRune(r rune) bool
	Match(reader *runeReader) (result MatchResult)
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
	OneOrMoreSymbol      = `+`
	ZeroOrOneSymbol      = `?`
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

func (b BaseMatchExpression) Match(reader *runeReader) (result MatchResult) {
	if r, ok := reader.readRune(); ok && b(r) {
		result.offset = reader.offset
		result.size = 1
		result.mainSize = 1
	}

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
		expr.expressions = append(expr.expressions, NewMatchExpression(r, nil))
	}

	r.discard(1)

	return
}

func (e AnyOfExpression) Match(reader *runeReader) (result MatchResult) {
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
		MatchExpression: NewMatchExpression(reader, nil),
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

	log.Println("atend:", matched, reader.isDone())

	return matched && reader.isDone(), n
}

type CountExpression struct {
	expr MatchExpression
	from int
	to   int
}

func NewCountExpression(expr MatchExpression, from, to int) CountExpression {
	return CountExpression{
		expr: expr,
		from: from,
		to:   to,
	}
}

func (e CountExpression) Match(reader *runeReader) (matched bool, n int) {
	total_n := 0

	matches := 0

	for {
		if matches == e.to {
			return matched, total_n
		}

		matched, n = e.expr.Match(reader)

		total_n += n

		if matched {
			matches++
		} else if matches >= e.from {
			reader.unreadRune()

			return true, total_n
		} else {
			return matched, total_n
		}
	}
}

func NewMatchExpression(
	reader *runeReader,
	prev MatchExpression,
) MatchExpression {
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
		log.Println("end symbol")

		return NewAtEndExpression(prev)
	case ZeroOrOneSymbol:
		return NewCountExpression(prev, 0, 1)
	case OneOrMoreSymbol:
		return NewCountExpression(prev, 1, -1)
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

	var prev MatchExpression

	for !reader.isDone() {
		expr := NewMatchExpression(reader, prev)
		// p.expressions = append(p.expressions, )
		p.append(expr)
		prev = expr
	}

	return
}

func (p Pattern) Len() int {
	return len(p.expressions)
}

func (p Pattern) Last() MatchExpression {
	return p.expressions[p.Len()-1]
}

func (p Pattern) Match(line []byte) bool {
	reader := NewRuneReaderFromBytes(line)
	offset := reader.offset

	for !reader.isDone() {
		all_matched := true

		var last_expr MatchExpression

		// var last_matched_rune rune

		for _, expr := range p.expressions {
			log.Printf("expr %+v offset %d", expr, reader.offset)

			if matched, n := expr.Match(reader); matched {
				log.Println("matched")

				last_matched_rune = reader.prev()

				if _, ok := expr.(CountExpression); ok {
					last_n = n
				} else {
					last_n = 0
				}

				continue
			}

			if c, ok := expr.(BaseMatchExpression); ok && last_n > 1 &&
				c(last_matched_rune) {
				log.Println("testing last rune")

				last_n = 0

				reader.unreadRune()

				continue
			}

			log.Println("not matched")

			all_matched = false

			break
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
	case AtEndExpression, CountExpression:
		p.expressions[p.Len()-1] = expr
	default:
		p.expressions = append(p.expressions, expr)
	}
}

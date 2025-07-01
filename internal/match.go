package internal

import (
	"fmt"
	"log"
	"unicode"
	"unicode/utf8"
)

type MatchResult struct {
	offset    int
	count     int
	mainCount int
	lengths   []int
}

func NewMatchResultOne(offset int) (result MatchResult) {
	return MatchResult{
		offset:    offset,
		count:     1,
		mainCount: 1,
		lengths:   []int{1},
	}
}

func (r MatchResult) Len() (length int) {
	for _, l := range r.lengths {
		length += l
	}

	return
}

func (r MatchResult) hasRest() bool {
	return r.count-r.mainCount > 0
}

func (r MatchResult) lenMain() (length int) {
	for _, l := range r.lengths[:r.mainCount] {
		length += l
	}

	return
}

func (r MatchResult) offsetRest() (offset int) {
	return r.offset + r.lenMain()
}

func (r MatchResult) lenRest() (length int) {
	for _, l := range r.lengths[r.mainCount:] {
		length += l
	}

	return
}

func (r *MatchResult) append(length int) {
	r.lengths = append(r.lengths, length)
}

func (r MatchResult) ok() bool {
	return r.count > 0
}

type MatchExpression interface {
	// MatchRune(r rune) bool
	Match(reader *runeReader) (result MatchResult)
	MatchesMin() int
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
	AlphanumericSymbol   = `\w`
	AlterationSymbol     = `)`
	AnyNoneOfSymbolClose = `]`
	AnyOfSymbol          = `[`
	AtEndSymbol          = `$`
	AtStartSymbol        = `^`
	CaptureEndSymbol     = `)`
	CaptureStartSymbol   = `(`
	DecimalSymbol        = `\d`
	NoneOfSymbol         = `[^`
	OneOrMoreSymbol      = `+`
	WildcardSymbol       = `.`
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

func (e BaseMatchExpression) MatchesMin() int {
	return 1
}

func (e BaseMatchExpression) Match(reader *runeReader) (result MatchResult) {
	if r, ok := reader.readRune(); ok && e(r) {
		result = NewMatchResultOne(reader.offset - 1)
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

func (e AnyOfExpression) MatchesMin() int {
	return 1
}

func (e AnyOfExpression) Match(reader *runeReader) (result MatchResult) {
	if reader.isDone() {
		return
	}

	offset := reader.offset

	for _, expr := range e.expressions {
		reader.reset(offset)

		if result = expr.Match(reader); result.ok() {
			return
		}
	}

	result = MatchResult{}

	return
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

func (e NoneOfExpression) Match(reader *runeReader) (result MatchResult) {
	if reader.isDone() {
		return
	}

	result = NewMatchResultOne(reader.offset)

	if r := e.AnyOfExpression.Match(reader); r.ok() {
		result = MatchResult{}
	}

	return
}

type AtStartExpression struct {
	MatchExpression
}

func NewAtStartExpression(reader *runeReader) AtStartExpression {
	return AtStartExpression{
		MatchExpression: NewMatchExpression(reader, nil),
	}
}

func (e AtStartExpression) Match(reader *runeReader) (result MatchResult) {
	if reader.offset != 0 {
		return
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

func (e AtEndExpression) Match(reader *runeReader) (result MatchResult) {
	result = e.MatchExpression.Match(reader)

	// log.Println("atend:", matched, reader.isDone())

	if !result.ok() || !reader.isDone() {
		result = MatchResult{}
	}

	return
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

func (e CountExpression) MatchesMin() int {
	return e.from
}

func (e CountExpression) Match(reader *runeReader) (result MatchResult) {
	result.offset = reader.offset

	for {
		offset := reader.offset

		r := e.expr.Match(reader)

		if r.ok() {
			result.count++
			result.append(r.Len())
			result.mainCount = min(result.count, e.from)

			continue
		}

		if result.mainCount >= e.from {
			reader.reset(offset)

			result.mainCount = max(1, result.mainCount)
			result.count = max(result.mainCount, result.count)
		}

		break
	}

	return
}

// type AlterationExpression AnyOfExpression

// func NewAlterationExpression(prev MatchExpression) (expr
// AlterationExpression) {
// 	switch p := prev.(type) {
// 	case AlterationExpression:
// 	}

// 	expr = AlterationExpression{
// 		expressions: []MatchExpression{prev},
// 	}

// 	return
// }

// func (e *AlterationExpression) append(expr MatchExpression) {
// }

type AllOfExpression struct {
	expr []MatchExpression
}

func NewAllOfExpressionDefault() AllOfExpression {
	return AllOfExpression{
		expr: make([]MatchExpression, 0),
	}
}

func NewAllOfExpression(reader *runeReader) (expr AllOfExpression) {
	expr = NewAllOfExpressionDefault()

	for !reader.isDone() && !reader.test(CaptureEndSymbol[0]) {
		if reader.test(AlterationSymbol[0]) {
			log.Println("found |")
			reader.discard(1)

			break
		}

		expr.expr = append(expr.expr, NewMatchExpression(reader, nil))
	}

	return
}

func (e AllOfExpression) Match(reader *runeReader) (result MatchResult) {
	if reader.isDone() {
		return
	}

	result.offset = reader.offset

	for _, ex := range e.expr {
		if r := ex.Match(reader); r.ok() {
			result.count++
			result.mainCount++
			result.lengths = append(result.lengths, r.lengths...)

			continue
		}

		return MatchResult{}
	}

	return
}

func (e AllOfExpression) MatchesMin() int {
	return len(e.expr)
}

type CaptureExpression struct {
	expr []AllOfExpression
	num  int
}

func NewCaptureExpression(reader *runeReader) (expr CaptureExpression) {
	log.Println("new capture")

	expr = CaptureExpression{
		expr: make([]AllOfExpression, 0),
	}

	for !reader.isDone() && !reader.test(CaptureEndSymbol[0]) {
		expr.expr = append(expr.expr, NewAllOfExpression(reader))
	}

	reader.discard(1)

	return
}

func (e CaptureExpression) Match(reader *runeReader) (result MatchResult) {
	if reader.isDone() {
		return
	}

	result.offset = reader.offset

	for _, expr := range e.expr {
		reader.reset(result.offset)

		if result = expr.Match(reader); result.ok() {
			break
		}
	}

	return
}

func (e CaptureExpression) MatchesMin() int {
	return 1
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
		return NewNoneOfExpression(reader)
	case AtStartSymbol:
		return NewAtStartExpression(reader)
	case AtEndSymbol:
		return NewAtEndExpression(prev)
	case ZeroOrOneSymbol:
		return NewCountExpression(prev, 0, 1)
	case OneOrMoreSymbol:
		return NewCountExpression(prev, 1, -1)
	case CaptureStartSymbol:
		return NewCaptureExpression(reader)
	case AlterationSymbol:
		return AllOfExpression{}
	// return NewAlterationExpression(prev)
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

	last_result := MatchResult{}

	for !reader.isDone() {
		matched := p.Len()

		// var last_matched_rune rune

		for _, expr := range p.expressions {
			log.Printf("expr %+v offset %d", expr, reader.offset)

			// offsetBefore := reader.offset

			if result := expr.Match(reader); result.ok() {
				log.Println("matched")

				matched--

				last_result = result

				continue
			}

			if last_result.hasRest() {
				log.Println("checking rest")
				reader.reset(last_result.offsetRest())
				result := p.matchAt(reader, last_result.lenRest(), expr)

				if result.ok() {
					log.Println("check matched")

					matched--

					last_result = result

					continue
				}
			}

			break
		}

		if matched == 0 {
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

func (p Pattern) matchAt(
	reader *runeReader,
	n int,
	expr MatchExpression,
) (result MatchResult) {
	offset := reader.offset

	// n -= expr.MatchesMin()

	log.Println("check init", offset, n)

	for i := n - 1; i >= 0; i-- {
		log.Println("check", offset, i)
		reader.reset(offset + i)

		if reader.isDone() {
			return
		}

		if r := expr.Match(reader); r.ok() {
			return r
		}
	}

	return
}

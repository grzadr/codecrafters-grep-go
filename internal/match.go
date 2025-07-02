package internal

import (
	"fmt"
	"log"
	"slices"
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
	String() string
}

type MatchExpressionSymbol string

const (
	AlphanumericSymbol   = `\w`
	AlterationSymbol     = `|`
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

const defaultMatchSliceCapacity = 32

type MatchSlice []MatchExpression

func NewMatchSlice() MatchSlice {
	return make(MatchSlice, 0, defaultMatchSliceCapacity)
}

func (s MatchSlice) MatchesMin() int {
	return s.len()
}

func (s MatchSlice) String() string {
	// repr := make([]string, len(s))
	// for i, expr := range s {
	// 	repr[i] = expr.String()
	// }
	return fmt.Sprintf("%T<%+v>", s, []MatchExpression(s))
}

func (s MatchSlice) Match(reader *runeReader) (result MatchResult) {
	if reader.isDone() {
		return result
	}

	result.offset = reader.offset

	log.Printf(
		"matching %+v on %d",
		s,
		result.offset,
	)

	for _, expr := range s {
		if r := expr.Match(reader); r.ok() {
			result.count++
			result.mainCount++
			result.lengths = append(result.lengths, r.lengths...)

			continue
		} else {
			log.Printf(
				"expr %+v failed %+v",
				expr,
				result,
			)
		}

		return MatchResult{}
	}

	log.Printf(
		"expr %+v matched %q",
		s,
		string(reader.runes[result.offset:reader.offset]),
	)

	return result
}

func (s MatchSlice) len() int {
	return len(s)
}

func (s MatchSlice) last() MatchExpression {
	if s.len() == 0 {
		return nil
	}

	return s[s.len()-1]
}

func (s *MatchSlice) append(
	expr MatchExpression,
) (prev MatchExpression) {
	log.Printf("%T appending to slice\n%+v", s, expr)

	switch expr.(type) {
	case nil:
	case AtEndExpression, CountExpression:
		log.Println("replace last")

		(*s)[s.len()-1] = expr
	default:
		log.Println("append")

		*s = append(*s, expr)
	}

	return s.last()
}

type BaseMatchExpression func(r rune) bool

func (e BaseMatchExpression) MatchesMin() int {
	return 1
}

func (e BaseMatchExpression) String() string {
	return fmt.Sprintf("%T", e)
}

func (e BaseMatchExpression) Match(reader *runeReader) (result MatchResult) {
	if r, ok := reader.readRune(); ok && e(r) {
		result = NewMatchResultOne(reader.offset - 1)
	}

	return
}

type DecimalExpression struct{ BaseMatchExpression }

func NewDecimalExpression() DecimalExpression {
	return DecimalExpression{BaseMatchExpression: unicode.IsNumber}
}

func (e DecimalExpression) String() string {
	return fmt.Sprintf("%T<>", e)
}

type AlphanumericExpression struct{ BaseMatchExpression }

func NewAlphanumericExpression() AlphanumericExpression {
	return AlphanumericExpression{BaseMatchExpression: func(r rune) bool {
		return unicode.IsDigit(r) || unicode.IsLetter(r) || r == '_'
	}}
}

func (e AlphanumericExpression) String() string {
	return fmt.Sprintf("%T<>", e)
}

type WildcardExpression struct{ BaseMatchExpression }

func NewWildcardExpression() WildcardExpression {
	return WildcardExpression{BaseMatchExpression: func(r rune) bool {
		return true
	}}
}

func (e WildcardExpression) String() string {
	return fmt.Sprintf("%T<>", e)
}

type CharacterExpression struct {
	BaseMatchExpression
	char rune
}

func NewCharacterExpression(char rune) CharacterExpression {
	return CharacterExpression{
		BaseMatchExpression: func(r rune) bool {
			return char == r
		},
		char: char,
	}
}

func (e CharacterExpression) String() string {
	return fmt.Sprintf("%T<%q>", e, string(e.char))
}

func NewCharacterClass(expr string) MatchExpression {
	switch expr {
	case DecimalSymbol:
		return NewDecimalExpression()
	case AlphanumericSymbol:
		return NewAlphanumericExpression()
	case WildcardSymbol:
		return NewWildcardExpression()
	default:
		if utf8.RuneCountInString(expr) != 1 {
			panic(
				fmt.Sprintf(
					"character expression %q must be single rune",
					expr,
				),
			)
		}

		return NewCharacterExpression([]rune(expr)[0])
	}
}

type AnyOfExpression struct {
	expr []MatchExpression
}

func NewAnyOfExpressionDefault() AnyOfExpression {
	return AnyOfExpression{
		expr: make([]MatchExpression, 0),
	}
}

func NewAnyOfExpression(r *runeReader) (expr AnyOfExpression) {
	expr = NewAnyOfExpressionDefault()

	for !r.isDone() && !r.test(AnyNoneOfSymbolClose[0]) {
		expr.expr = append(expr.expr, NewMatchExpression(r, nil))
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

	for _, expr := range e.expr {
		reader.reset(offset)

		if result = expr.Match(reader); result.ok() {
			return
		}
	}

	result = MatchResult{}

	return
}

func (e AnyOfExpression) String() string {
	return fmt.Sprintf("%T<%+v>", e, e.expr)
}

func (e *AnyOfExpression) append(expr MatchExpression) {
	e.expr = append(e.expr, expr)
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

func (e NoneOfExpression) String() string {
	return fmt.Sprintf("%T<%+v>", e, e.AnyOfExpression)
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

func (e AtStartExpression) String() string {
	return fmt.Sprintf("%T<%+v>", e, e.MatchExpression)
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
		log.Printf(
			"expr %+v failed %d, %t",
			e,
			reader.offset,
			reader.isDone(),
		)

		result = MatchResult{}
	} else {
		log.Printf(
			"expr %+v matched %q",
			e,
			string(reader.runes[result.offset:reader.offset]),
		)
	}

	return
}

func (e AtEndExpression) String() string {
	return fmt.Sprintf("%T<%+v>", e, e.MatchExpression)
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

		log.Printf("%+v current offset %d", e, reader.offset)

		r := e.expr.Match(reader)

		if r.ok() {
			result.count++
			result.append(r.Len())
			result.mainCount = min(result.count, e.from)

			log.Printf(
				"expr %+v matched %q",
				e.expr,
				string(reader.runes[result.offset:reader.offset]),
			)

			continue
		}

		log.Printf(
			"expr %+v failed %d %t %+v",
			e.expr,
			reader.offset, reader.isDone(), result,
		)

		if result.mainCount >= e.from {
			reader.reset(offset)

			result.mainCount = max(1, result.mainCount)
			result.count = max(result.mainCount, result.count)
		}

		break
	}

	return result
}

func (e CountExpression) String() string {
	return fmt.Sprintf("%T{%d, %d}<%+v>", e, e.from, e.to, e.expr)
}

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
		log.Printf("current %q", string(reader.peek()))

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

func (e AllOfExpression) String() string {
	return fmt.Sprintf("%T<%+v>", e, e.expr)
}

func (e AllOfExpression) len() int {
	return len(e.expr)
}

func (e *AllOfExpression) append(expr MatchExpression) {
	log.Printf("appending %+v", expr)

	switch expr.(type) {
	case nil:
	case AtEndExpression, CountExpression:
		log.Println("replace last")

		e.expr[e.len()-1] = expr
	default:
		log.Println("append")

		e.expr = append(e.expr, expr)
	}
}

type CaptureExpression struct {
	expr       []MatchSlice
	alteration bool
}

func NewCaptureExpression(reader *runeReader) (capture CaptureExpression) {
	log.Println("new capture")

	capture = CaptureExpression{
		expr: slices.Grow(
			[]MatchSlice{NewMatchSlice()},
			defaultMatchSliceCapacity,
		),
	}

	var prev MatchExpression

	for !reader.isDone() && !reader.test(CaptureEndSymbol[0]) {
		log.Printf("prev expression: %+v", prev)
		expr := NewMatchExpression(reader, prev)
		log.Printf("new expression: %+v", expr)
		// p.expressions = append(p.expressions, )
		prev = capture.append(expr)
	}

	reader.discard(1)

	return
}

func (e CaptureExpression) Match(reader *runeReader) (result MatchResult) {
	if reader.isDone() {
		return
	}

	offset := reader.offset

	for _, expr := range e.expr {
		reader.reset(offset)

		if result = expr.Match(reader); result.ok() {
			// result.count++
			// result.mainCount++
			// result.lengths = append(result.lengths, r.lengths...)
			// continue
			return
		}
	}

	return MatchResult{}
}

func (e CaptureExpression) MatchesMin() int {
	return len(e.expr)
}

func (e CaptureExpression) String() string {
	return fmt.Sprintf("%T<%+v>", e, e.expr)
}

func (e CaptureExpression) len() int {
	return len(e.expr)
}

func (e CaptureExpression) last() MatchExpression {
	return e.expr[e.len()-1].last()
}

func (e *CaptureExpression) append(
	expr MatchExpression,
) (prev MatchExpression) {
	log.Printf("%T appending %+v", e, expr)

	switch v := expr.(type) {
	case nil:
	// case AtEndExpression, CountExpression:
	// 	log.Println("replace last")

	case MatchSlice:
		e.expr = append(e.expr, v)
	default:
		log.Println("append")

		e.expr[e.len()-1].append(expr)
	}

	return e.last()
}

// type AlterationExpression struct {
// 	expr
// }

// func NewAlterationExpression() AlterationExpression {
// 	return AlterationExpression{AnyOfExpression: NewAnyOfExpressionDefault()}
// }

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
		return NewMatchSlice()
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
	log.Printf("appending %+v", expr)

	switch expr.(type) {
	case nil:
	case AtEndExpression, CountExpression:
		log.Println("replace last")

		p.expressions[p.Len()-1] = expr
	default:
		log.Println("append")

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

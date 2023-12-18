package calcu

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/shopspring/decimal"
)

const (
	invalid = iota - 1
	eof     = 0
)

var (
	rules = []rule{
		{regexp.MustCompile("^[_a-zA-Z][_a-zA-Z0-9]*"), IDENT},
		{regexp.MustCompile("^[0-9]*\\.?[0-9]+([eE][-+]?[0-9]+)?"), NUM},
		{regexp.MustCompile(`^"[^"]*"`), LITERALSTR},
	}
)

type rule struct {
	re    *regexp.Regexp
	token int
}

type lexer struct {
	in    string
	rules []rule

	um UnitManager

	root      Node
	lastError error
}

func newLexer(expr string) *lexer {
	return &lexer{
		in:    expr,
		rules: rules,
		um:    StdUm,
	}
}

func (l *lexer) Lex(lval *exprSymType) int {
	// Skip spaces.
	for len(l.in) > 0 && isSpace(l.in[0]) {
		l.in = l.in[1:]
	}

	// Check if the input has ended.
	if len(l.in) == 0 {
		lval.token = eof
		return eof
	}

	// Try math on unit first
	{
		if n, ok := l.um.Peek(l.in); ok {
			str := l.in[:n]
			l.in = l.in[n:]
			if str[0] == '(' {
				// remove brackets
				str = str[1 : len(str)-1]
			}
			lval.str = str
			lval.token = UNIT
			return UNIT
		}
	}

	for _, r := range rules {
		str := r.re.FindString(l.in)
		if str != "" {
			l.in = l.in[len(str):]
			switch r.token {
			case IDENT:
				lval.str = str
			case NUM:
				lval.str = str
			case UNIT:
				lval.str = str
			case LITERALSTR:
				// remove quote
				str = str[1 : len(str)-1]
				r.token = l.lexLiteralStr(str, lval)
			}
			lval.token = r.token
			return r.token
		}
	}

	// Otherwise return the next letter.
	lval.token = invalid
	ret := int(l.in[0])
	l.in = l.in[1:]
	return ret
}

func (l *lexer) lexLiteralStr(s string, lval *exprSymType) int {
	if ok := l.um.IsUnit(s); ok {
		lval.str = s
		return UNIT
	}
	if _, err := NewMeasureValueFromString(s); err == nil {
		lval.str = s
		return LITERALMV
	}
	lval.str = s
	return LITERALSTR
}

func (l *lexer) Error(e string) {
	l.lastError = errors.New(e)
}

func (l *lexer) setErr(err error) {
	l.lastError = err
}

func (l *lexer) setRoot(node Node) {
	l.root = node
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n'
}

func NewMeasureValueFromString(s string) (*MeasureValue, error) {
	l := newLexer(s)
	var lvals []exprSymType
	for {
		var lval exprSymType
		ret := l.Lex(&lval)
		if ret == eof {
			break
		}
		if lval.token != invalid {
			lvals = append(lvals, lval)
		}
	}
	if len(lvals) != 2 || (lvals[0].token != NUM && lvals[1].token != UNIT) {
		return nil, fmt.Errorf("invalid measure value: %s", s)
	}
	num, unit := lvals[0].str, lvals[1].str
	d, _ := decimal.NewFromString(num)
	return &MeasureValue{um: StdUm, unit: unit, value: d}, nil
}

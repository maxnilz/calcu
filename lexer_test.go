package calcu

import (
	"github.com/shopspring/decimal"
	"reflect"
	"strconv"
	"testing"
)

func TestMeasureValueFromString(t *testing.T) {
	cases := []struct {
		s        string
		expected func() MeasureValue
	}{
		{"1(10^3m3)", func() MeasureValue {
			return MeasureValue{value: decimal.NewFromInt(1), unit: "10^3m3"}
		}},
		{"1kg/m3", func() MeasureValue {
			return MeasureValue{value: decimal.NewFromInt(1), unit: "kg/m3"}
		}},
		{"1kg", func() MeasureValue {
			return MeasureValue{value: decimal.NewFromInt(1), unit: "kg"}
		}},
		{"1.1E-04(Gg/10^3m3)", func() MeasureValue {
			d, err := decimal.NewFromString("1.1E-04")
			if err != nil {
				panic(err)
			}
			return MeasureValue{value: d, unit: "Gg/10^3m3"}
		}},
		{"1.1E-04Gg/10^3m3", func() MeasureValue {
			d, err := decimal.NewFromString("1.1E-04")
			if err != nil {
				panic(err)
			}
			return MeasureValue{value: d, unit: "Gg/10^3m3"}
		}},
	}

	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			mv, err := NewMeasureValueFromString(c.s)
			if err != nil {
				t.Fatal(err)
			}
			expected := c.expected()
			if !mv.value.Equal(expected.value) {
				t.Fatalf("expectd decimal value: %v, got: %v", expected.value, mv.value)
			}
			if mv.unit != expected.unit {
				t.Fatalf("expectd unit: %v, got: %v", expected.unit, mv.unit)
			}
		})
	}
}

func TestLexer(t *testing.T) {
	cases := []struct {
		expr     string
		expected []int
	}{
		{expr: `1kg/m3, 1kg`, expected: []int{NUM, UNIT, NUM, UNIT}},
		{
			expr:     `SI = Convert(activity_value, activity_unit, "10^3m3", "hello", 123.123, "10(10^3m3)")`,
			expected: []int{IDENT, IDENT, IDENT, IDENT, UNIT, LITERALSTR, NUM, LITERALMV},
		},
	}

	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			l := newLexer(c.expr)
			var lvals []*exprSymType
			var tokens []int
			for {
				var lval exprSymType
				ret := l.Lex(&lval)
				if ret == eof {
					break
				}
				if lval.token != invalid {
					tokens = append(tokens, lval.token)
				}
			}
			_ = lvals
			if !reflect.DeepEqual(c.expected, tokens) {
				t.Fatalf("%d: %q: expected %d, but found %d", i, c.expr, c.expected, tokens)
			}
		})
	}
}

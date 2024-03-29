package calcu

import (
	"reflect"
	"strconv"
	"testing"
)

func mustMV(s string, unitless bool) *MeasureValue {
	if unitless {
		mv, err := makeUnitlessMeasureValue(s)
		if err != nil {
			panic(err)
		}
		return mv
	}
	mv, err := makeMeasureValueFromString(s)
	if err != nil {
		panic(err)
	}
	return mv
}

func TestMeasureOps(t *testing.T) {
	type opfun func(value *MeasureValue) (*MeasureValue, error)
	cases := []struct {
		a        string
		aul      bool
		b        string
		bul      bool
		expected []string // expected +, -, *, /
	}{
		// unitless
		{a: "1", aul: true, b: "2", bul: true, expected: []string{"3", "-1", "2", "0.5"}},
		{a: "2", aul: true, b: "1", bul: true, expected: []string{"3", "1", "2", "2"}},
		// meta and si
		{a: "1kg", b: "2kg", expected: []string{"3kg", "-1kg", "2kg", "0.5"}},
		{a: "2kg", b: "1kg", expected: []string{"3kg", "1kg", "2kg", "2"}},
		// meta and one is si, another is not
		{a: "1kg", b: "2Mg", expected: []string{"2001kg", "-1999kg", "2000kg", "0.0005"}},
		{a: "2Mg", b: "1kg", expected: []string{"2001kg", "1999kg", "2000kg", "2000"}},
		// meta, both are not si
		{a: "1Mg", b: "2Mg", expected: []string{"3000kg", "-1000kg", "2000000kg", "0.5"}},
		{a: "2Mg", b: "1Mg", expected: []string{"3000kg", "1000kg", "2000000kg", "2"}},
		// both compound, one is si, another is not
		{a: "1kg/m3", b: "2Mg/m3", expected: []string{"2001kg/m3", "-1999kg/m3", "2000kg/m3", "0.0005"}},
		{a: "2Mg/m3", b: "1kg/m3", expected: []string{"2001kg/m3", "1999kg/m3", "2000kg/m3", "2000"}},
		{a: "1kg/m3", b: "2Mg/10^3m3", expected: []string{"3kg/m3", "-1kg/m3", "2kg/m3", "0.5"}},
		{a: "2Mg/10^3m3", b: "1kg/m3", expected: []string{"3kg/m3", "1kg/m3", "2kg/m3", "2"}},
	}
	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			a, b := mustMV(c.a, c.aul), mustMV(c.b, c.bul)
			var gots []string
			funs := []opfun{a.Add, a.Sub, a.Mul, a.Div}
			for _, f := range funs {
				got, err := f(b)
				if err != nil {
					t.Fatal(err)
				}
				gots = append(gots, got.String())
			}
			// check
			if !reflect.DeepEqual(c.expected, gots) {
				t.Fatalf("expectd: %v, got: %v", c.expected, gots)
			}
		})
	}
}

func TestMeasureValueMul2(t *testing.T) {
	cases := []struct {
		a        string
		b        string
		expected string
	}{
		// one is compound, one is meta
		{a: "1kg/m3", b: "2m3", expected: "2kg"},
		{a: "2kg/m3", b: "2m3", expected: "4kg"},
		{a: "2kg/m3", b: "2(10^3m3)", expected: "4000kg"},
		{a: "2kg/10^3m3", b: "2m3", expected: "0.004kg"},
		// both are compound
		{a: "2kg/Gg", b: "2Gg/kg", expected: "4"},
		{a: "2kg/kg", b: "2kg/kg", expected: "4"},
		{a: "2kg/10^3m3", b: "2(10^3m3/kg)", expected: "4"},
	}
	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			a, b := mustMV(c.a, false), mustMV(c.b, false)
			got, err := a.Mul(b)
			if err != nil {
				t.Fatal(err)
			}
			if c.expected != got.String() {
				t.Fatalf("expected: %v, got: %v", c.expected, got.String())
			}
		})
	}
}

func TestMeasureValueMul3(t *testing.T) {
	cases := []struct {
		a        string
		auitless bool
		b        string
		buitless bool
		expected string
	}{
		// one is unitless, one is unit
		{a: "1kg/m3", b: "2", buitless: true, expected: "2kg/m3"},
		{a: "2", auitless: true, b: "2m3", expected: "4m3"},
	}
	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			a, b := mustMV(c.a, c.auitless), mustMV(c.b, c.buitless)
			got, err := a.Mul(b)
			if err != nil {
				t.Fatal(err)
			}
			if c.expected != got.String() {
				t.Fatalf("expected: %v, got: %v", c.expected, got.String())
			}
		})
	}
}

func TestMeasureValueDiv1(t *testing.T) {
	cases := []struct {
		a        string
		auitless bool
		b        string
		buitless bool
		expected string
	}{
		// both unitless
		{a: "1", auitless: true, b: "2", buitless: true, expected: "0.5"},
		// one is unitless, one is unit
		{a: "1kg/m3", b: "2", buitless: true, expected: "0.5kg/m3"},
		{a: "2", auitless: true, b: "2m3", expected: "1m3"},
		// both are mata
		{a: "2m3", b: "2m3", expected: "1"},
		// both are compound
		{a: "2kg/m3", b: "2kg/m3", expected: "1"},
	}
	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			a, b := mustMV(c.a, c.auitless), mustMV(c.b, c.buitless)
			got, err := a.Div(b)
			if err != nil {
				t.Fatal(err)
			}
			if c.expected != got.String() {
				t.Fatalf("expected: %v, got: %v", c.expected, got.String())
			}
		})
	}
}

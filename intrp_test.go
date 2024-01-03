package calcu

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"testing"
)

func TestGetFuncInfo(t *testing.T) {
	add := func(a int, b MeasureValue, c *MeasureValue) (n int, err error, mv *MeasureValue) {
		return a, nil, nil
	}
	fi := getFuncInfo(add)
	if len(fi.paramTypes) != 3 {
		t.Fatalf("expect 3 params, got %d", len(fi.paramTypes))
	}
	if len(fi.returnTypes) != 3 {
		t.Fatalf("expect 3 returns, got %d", len(fi.returnTypes))
	}
}

func TestInterpreterSimple(t *testing.T) {
	exprs := `
CO2 = activity_value * CO2Factor;
CH2 = activity_value * CH2Factor;
N2O = activity_value * N2OFactor;
GHG = CO2 + CH2 + N2O;
a = CO2 * CH2 * (1 + 2);
b = CO2 * CH2 * (1 - 2);
c = CO2 * CH2 * 2/1;
d = CO2 * CH2 * (2/1);
print(CO2, CH2, N2O, GHG, a, b, c, d);
`
	vars := map[string]string{
		"activity_value": "1(10^3m3)",
		"CO2Factor":      "1.1E-04Gg/10^3m3",
		"CH2Factor":      "7.2E-06Gg/10^3m3",
		"N2OFactor":      "1.1E-03Gg/10^3m3",
	}
	intrp, err := NewInterpreter(vars)
	if err != nil {
		t.Fatal(err)
	}
	rd := bytes.NewBufferString(exprs)
	outvars, err := intrp.Interpret(rd)
	if err != nil {
		t.Fatal(err)
	}
	co2 := outvars["CO2"]
	ch2 := outvars["CH2"]
	n2o := outvars["N2O"]
	ghg := outvars["GHG"]
	a := outvars["a"]
	b := outvars["b"]
	c := outvars["c"]
	d := outvars["d"]
	gots := []string{co2.String(), ch2.String(), n2o.String(), ghg.String(), a.String(), b.String(), c.String(), d.String()}
	expected := []string{"110kg", "7.2kg", "1100kg", "1217.2kg", "2376kg", "-792kg", "1584kg", "1584kg"}
	if !reflect.DeepEqual(expected, gots) {
		t.Fatalf("exptectd: %v, got: %v", expected, gots)
	}
}

func TestInterpreterUnitless(t *testing.T) {
	exprs := `
CH4 = activity_value * FractionofGassyCoalMines * CH4Factor * CH4ConversionFactor;
GHG = CH4;
print(CH4, GHG);
`
	vars := map[string]string{
		"activity_value":           "30",
		"FractionofGassyCoalMines": "0.1",
		"CH4Factor":                "0.402m3",
		"CH4ConversionFactor":      "1.1E-03Gg/m3",
	}
	intrp, err := NewInterpreter(vars)
	if err != nil {
		t.Fatal(err)
	}
	rd := bytes.NewBufferString(exprs)
	outvars, err := intrp.Interpret(rd)
	if err != nil {
		t.Fatal(err)
	}
	ch4 := outvars["CH4"]
	ghg := outvars["GHG"]
	gots := []string{ch4.String(), ghg.String()}
	expected := []string{"1326.6kg", "1326.6kg"}
	if !reflect.DeepEqual(expected, gots) {
		t.Fatalf("exptectd: %v, got: %v", expected, gots)
	}
}

func TestInterpreterReuseVar(t *testing.T) {
	exprs := `
a = a + 1kg;
a = a + 2kg;
b = a;
b = b * b + 2kg;
print(a, b);
`
	vars := map[string]string{"a": "1kg"}
	intrp, err := NewInterpreter(vars)
	if err != nil {
		t.Fatal(err)
	}
	rd := bytes.NewBufferString(exprs)
	outvars, err := intrp.Interpret(rd)
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"4kg", "18kg"}
	a := outvars["a"].String()
	b := outvars["b"].String()
	gots := []string{a, b}
	if !reflect.DeepEqual(expected, gots) {
		t.Fatalf("expected: %s, got: %s", expected, gots)
	}
}

func TestInterpreterSyntaxError(t *testing.T) {
	cases := []struct {
		expr string
		ok   bool
		hint string
	}{
		{expr: "print(a);", ok: true},
		{expr: `a = a + 2kg;`, ok: true},
		{expr: `b = b + "10(10^3m3)";`, ok: true},
		{expr: `a = a + "2kg";`, ok: true},
		{expr: "print(a)", ok: false, hint: "have no ; in the end"},
		{expr: "a = a + 1kg \n a = a + a", hint: "missing ; between lines"},
		{expr: "a=1print(a);", ok: false, hint: "unexpected ident, missing ;"},
	}
	vars := map[string]string{"a": "1kg", "b": "1(10^3m3)"}
	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			intrp, err := NewInterpreter(vars)
			if err != nil {
				t.Fatal(err)
			}
			rd := bytes.NewBufferString(c.expr)
			_, err = intrp.Interpret(rd)
			switch c.ok {
			case false:
				if err == nil {
					t.Fatalf("expected err, got nil")
				}
				t.Log(err)
			case true:
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func fAny(args ...interface{}) (*MeasureValue, error) {
	// just use the num of args as an indicator
	// to test return error or not.
	if len(args) == 0 {
		return nil, fmt.Errorf("i'm an error")
	}
	var outs []string
	for _, a := range args {
		outs = append(outs, fmt.Sprintf("%v", a))
	}
	fmt.Println(outs)
	return mustMV("1kg", false), nil
}

func fTypes(s string, mv1, mv2, mv3, mv4 *MeasureValue) *MeasureValue {
	fmt.Println(s, mv1, mv2, mv3, mv4)
	return mv1
}

type Stub struct {
	s string
}

func (s *Stub) mTypesF(str string, mv1, mv2, mv3, mv4 *MeasureValue) *MeasureValue {
	fmt.Println(s.s, str, mv1, mv2, mv3, mv4)
	return mv1
}

func fInt(a *MeasureValue) *MeasureValue {
	return a
}

func TestInterpreterFuncs(t *testing.T) {
	cases := []struct {
		expr string
		ok   bool
		hint string
	}{
		{expr: "print(a);", ok: true},
		{expr: "fInt(1);", ok: true, hint: "int literal 1, will be wrapped as *MeasureValue"},
		{expr: `fAny("1kg");`, ok: true, hint: `"1kg" is pass as *MeasureValue to func(because it a LITERALMV token`},
		{expr: `fTypes("hello world", a, 1kg, "1kg", "10(10^3m3)");`, ok: true, hint: `"hello world" is pass as str to func(because it a LITERALSTR token`},
		{expr: `mTypesF("hello world", a, 1kg, "1kg", "10(10^3m3)");`, ok: true, hint: `"hello world" is pass as str to func(because it a LITERALSTR token`},
		{expr: `fAny("1kg", a);`, ok: true},
		{expr: `fAny();`, ok: false, hint: "error raise by func"},
	}
	vars := map[string]string{"a": "1kg", "b": "1(10^3m3)"}
	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			stub := &Stub{s: "stateful method call"}
			intrp, err := NewInterpreter(vars, fInt, fAny, fTypes, stub.mTypesF)
			if err != nil {
				t.Fatal(err)
			}
			rd := bytes.NewBufferString(c.expr)
			_, err = intrp.Interpret(rd)
			switch c.ok {
			case false:
				if err == nil {
					t.Fatalf("expected err, got nil")
				}
				t.Log(err)
			case true:
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

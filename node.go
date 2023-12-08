//go:generate goyacc -o expr.go -p "expr" expr.y

package calcu

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/shopspring/decimal"
)

type NodeType int

const (
	NodeTypeMV NodeType = iota
	NodeTypeLiteralStr
	NodeTypeVar
	NodeTypeBinaryExpr
	NodeTypeUnaryExpr
	NodeTypeFuncCall
	NodeTypeList
	NodeTypeAssignment
)

func (t NodeType) String() string {
	return reflect.TypeOf(t).String()
}

type Node interface {
	Type() NodeType
}

type MeasureValue struct {
	um UnitManager

	unit     string
	unitless bool
	value    decimal.Decimal
}

func makeUnitlessMeasureValue(value string) (*MeasureValue, error) {
	d, err := decimal.NewFromString(value)
	if err != nil {
		return nil, err
	}
	return &MeasureValue{
		um:       StdUm,
		value:    d,
		unitless: true,
	}, nil
}

func makeMeasureValue(value, unit string) (*MeasureValue, error) {
	d, err := decimal.NewFromString(value)
	if err != nil {
		return nil, err
	}
	return &MeasureValue{um: StdUm, value: d, unit: unit}, nil
}

func makeLiteralMeasureValue(s string) (*MeasureValue, error) {
	return NewMeasureValueFromString(s)
}

func (mv *MeasureValue) Type() NodeType {
	return NodeTypeMV
}

type mvopstat struct {
	unitless bool
	// left side measure value in si
	lmv *MeasureValue
	// right side measure value in si
	rmv *MeasureValue

	targetUnit string
}

func (mv *MeasureValue) toSi(mvUnit Unit) *MeasureValue {
	siName := mvUnit.SiName()
	siFactor, siOffset := mvUnit.SiFactors()
	d := mv.value.Mul(siFactor)
	d = d.Add(siOffset)
	return &MeasureValue{
		um:       mv.um,
		unit:     siName,
		unitless: false,
		value:    d,
	}
}

func (mv *MeasureValue) parseAdd(other *MeasureValue) (*mvopstat, bool) {
	// either both unitless or unit measured
	// are allowed, otherwise not allowed
	if mv.unitless && !other.unitless {
		return nil, false
	}
	if !mv.unitless && other.unitless {
		return nil, false
	}
	// if both are unitless, allow
	if mv.unitless && other.unitless {
		return &mvopstat{
			unitless: true,
			lmv:      mv,
			rmv:      other,
		}, true
	}
	// both are unit measured value onwards.
	// the unit dimension should be same,
	// otherwise not allowed
	u, ok := mv.um.GetByName(mv.unit)
	if !ok {
		return nil, false
	}
	ou, ok := other.um.GetByName(other.unit)
	if !ok {
		return nil, false
	}
	if u.Dimension() != ou.Dimension() {
		return nil, false
	}
	lmv, rmv := mv.toSi(u), other.toSi(ou)
	return &mvopstat{
		unitless:   false,
		lmv:        lmv,
		rmv:        rmv,
		targetUnit: lmv.unit,
	}, true
}

func (mv *MeasureValue) parseSub(other *MeasureValue) (*mvopstat, bool) {
	// sub is same as add
	return mv.parseAdd(other)
}

func (mv *MeasureValue) parseMul(other *MeasureValue) (*mvopstat, bool) {
	// either both unitless or unit measured
	// are allowed, otherwise not allowed
	if mv.unitless && !other.unitless {
		return nil, false
	}
	if !mv.unitless && other.unitless {
		return nil, false
	}
	// if both unitless, allow
	if mv.unitless && other.unitless {
		return &mvopstat{
			unitless: true,
			lmv:      mv,
			rmv:      other,
		}, true
	}

	// both are unit measured value onwards
	u, ok := mv.um.GetByName(mv.unit)
	if !ok {
		return nil, false
	}
	ou, ok := mv.um.GetByName(other.unit)
	if !ok {
		return nil, false
	}
	// if both are meta unit, compatible if dimension is same,
	// otherwise not allowed.
	if u.IsMeta() && ou.IsMeta() {
		if u.Dimension() != ou.Dimension() {
			return nil, false
		}
		lmv, rmv := mv.toSi(u), other.toSi(ou)
		return &mvopstat{
			unitless:   false,
			lmv:        lmv,
			rmv:        rmv,
			targetUnit: lmv.unit,
		}, true
	}
	// if both are compound unit, compatible if:
	//  1. the dimension of Numerator same and
	//  2. the dimension of Denominator is same
	if !u.IsMeta() && !ou.IsMeta() {
		cu1, cu2 := u.(*CompoundUnit), ou.(*CompoundUnit)
		if cu1.Numerator.Dimension() != cu2.Numerator.Dimension() {
			return nil, false
		}
		if cu1.Denominator.Dimension() != cu2.Denominator.Dimension() {
			return nil, false
		}
		lmv, rmv := mv.toSi(u), other.toSi(ou)
		return &mvopstat{
			unitless:   false,
			lmv:        lmv,
			rmv:        rmv,
			targetUnit: lmv.unit,
		}, true
	}

	// one of the unit is meta and another is compound onwards

	// figure out meta unit(mu), and compound unit(cu)
	// and put meta unit as the left side, compound unit
	// as the right side.
	var mu *MetaUnit
	var cu *CompoundUnit
	var lmv, rmv *MeasureValue
	if u.IsMeta() {
		lmv, rmv = mv, other
		mu = u.(*MetaUnit)
		cu = ou.(*CompoundUnit)
	} else {
		lmv, rmv = other, mv
		mu = ou.(*MetaUnit)
		cu = u.(*CompoundUnit)
	}
	// if the dimension of denominator of compound is same as the dimension
	// of meta unit, otherwise not allowed.
	if cu.Denominator.Dimension() != mu.Dimension() {
		return nil, false
	}

	// the si of the outcome later should be the si of numerator of the compound unit
	// e.g, meta unit is: m3, compound unit is: kg/m3(numerator is kg, denominator is m3)
	// m3 * (kg/m3) == kg.
	targetSIName := cu.Numerator.SiName()
	return &mvopstat{
		unitless:   false,
		lmv:        lmv.toSi(mu), // meta unit as left side
		rmv:        rmv.toSi(cu), // compound unit as right side
		targetUnit: targetSIName,
	}, true
}

func (mv *MeasureValue) parseDiv(other *MeasureValue) (*mvopstat, bool) {
	// div support the operations that same as add
	// TODO: consider support case like: (kg/m3) / m3
	//  where one of unit is meta unit, and another is
	//  compound unit.
	return mv.parseAdd(other)
}

func (mv *MeasureValue) Add(other *MeasureValue) (*MeasureValue, error) {
	mvos, ok := mv.parseAdd(other)
	if !ok {
		return nil, fmt.Errorf("(%s)+(%s) is unsupported", mv.unit, other.unit)
	}
	d := mvos.lmv.value.Add(mvos.rmv.value)
	return &MeasureValue{um: mv.um, value: d, unit: mvos.targetUnit}, nil
}

func (mv *MeasureValue) Sub(other *MeasureValue) (*MeasureValue, error) {
	mvos, ok := mv.parseAdd(other)
	if !ok {
		return nil, fmt.Errorf("(%s)-(%s) is unsupported", mv.unit, other.unit)
	}
	d := mvos.lmv.value.Sub(mvos.rmv.value)
	return &MeasureValue{um: mv.um, value: d, unit: mvos.targetUnit}, nil
}

func (mv *MeasureValue) Mul(other *MeasureValue) (*MeasureValue, error) {
	mvos, ok := mv.parseMul(other)
	if !ok {
		return nil, fmt.Errorf("(%s)*(%s) is unsupported", mv.unit, other.unit)
	}
	d := mvos.lmv.value.Mul(mvos.rmv.value)
	return &MeasureValue{um: mv.um, value: d, unit: mvos.targetUnit}, nil
}

func (mv *MeasureValue) Div(other *MeasureValue) (*MeasureValue, error) {
	mvos, ok := mv.parseDiv(other)
	if !ok {
		return nil, fmt.Errorf("(%s)/(%s) is unsupported", mv.unit, other.unit)
	}
	d := mvos.lmv.value.Div(mvos.rmv.value)
	return &MeasureValue{um: mv.um, value: d, unit: mvos.targetUnit}, nil
}

func (mv *MeasureValue) Neg() *MeasureValue {
	return &MeasureValue{value: mv.value.Neg()}
}

func (mv *MeasureValue) String() string {
	ans := bytes.NewBufferString(mv.value.String())
	if mv.unit != "" {
		s, _ := maybeAmbiguousUnitName(mv.unit)
		ans.WriteString(s)
	}
	return ans.String()
}

type LiteralString struct {
	s string
}

func makeLiteralString(s string) *LiteralString {
	return &LiteralString{s: s}
}

func (n *LiteralString) Type() NodeType {
	return NodeTypeLiteralStr
}

type Variable struct {
	Name string
}

func makeVariable(name string) *Variable {
	return &Variable{Name: name}
}

func (v *Variable) Type() NodeType {
	return NodeTypeVar
}

type BinaryExpr struct {
	Op  string
	lhs Node
	rhs Node
}

const (
	OpAdd = "+"
	OpSub = "-"
	OpMul = "*"
	OpDiv = "/"
)

func makeBinaryExpr(lhs, rhs Node, op string) *BinaryExpr {
	return &BinaryExpr{
		Op:  op,
		lhs: lhs,
		rhs: rhs,
	}
}

func (n *BinaryExpr) Type() NodeType {
	return NodeTypeBinaryExpr
}

type UnaryExpr struct {
	expr Node
}

func makeUnaryExpr(expr Node) *UnaryExpr {
	return &UnaryExpr{expr: expr}
}

func (n *UnaryExpr) Type() NodeType {
	return NodeTypeUnaryExpr
}

type FuncCall struct {
	fn   string
	args []Node
}

func makeFuncCall(fn string, args ...Node) (*FuncCall, error) {
	return &FuncCall{fn: fn, args: args}, nil
}

func (fc *FuncCall) Type() NodeType {
	return NodeTypeFuncCall
}

type List struct {
	elements []Node
}

func makeList() *List {
	return &List{}
}

func (l *List) Append(node Node) {
	l.elements = append(l.elements, node)
}

func (l *List) Type() NodeType {
	return NodeTypeList
}

type Assignment struct {
	variable string
	node     Node
}

func makeAssignment(variable string, node Node) (*Assignment, error) {
	return &Assignment{
		variable: variable,
		node:     node,
	}, nil
}

func (n *Assignment) Type() NodeType {
	return NodeTypeAssignment
}

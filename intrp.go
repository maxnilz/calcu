package calcu

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strings"

	"github.com/shopspring/decimal"
)

type MeasureVars map[string]*MeasureValue

func (m MeasureVars) Get(key string) (*MeasureValue, bool) {
	mv, ok := m[key]
	return mv, ok
}

func (m MeasureVars) Decimal(key string) decimal.NullDecimal {
	mv, ok := m[key]
	if !ok {
		return decimal.NullDecimal{}
	}
	return decimal.NullDecimal{Valid: true, Decimal: mv.Value()}
}

type Interpreter struct {
	mvvars MeasureVars
	funcs  map[string]*function
	kfuncs map[string]*function

	outvars   MeasureVars
	lastError error
}

func NewInterpreter(vars map[string]string, fns ...interface{}) (*Interpreter, error) {
	mvvars := make(map[string]*MeasureValue)
	for k, s := range vars {
		mv, err := makeMeasureValueFromString(s)
		if err != nil {
			return nil, err
		}
		mvvars[k] = mv
	}

	intrp := Interpreter{
		mvvars:  mvvars,
		funcs:   make(map[string]*function),
		kfuncs:  make(map[string]*function),
		outvars: make(map[string]*MeasureValue),
	}

	// register kernel funcs
	intrp.registerKFuncs()

	// register user funcs
	// func name is case-sensitive.
	for _, fn := range fns {
		if err := intrp.registerUFunc(fn); err != nil {
			return nil, err
		}
	}

	return &intrp, nil
}

func (i *Interpreter) Interpret(rd io.Reader) (MeasureVars, error) {
	r := bufio.NewScanner(rd)
	for r.Scan() {
		expr := r.Text()
		root, err := i.parseOneExpr(expr)
		if err != nil {
			return nil, err
		}
		if root == nil {
			continue // empty statement
		}
		if err = i.visitRoot(root); err != nil {
			return nil, err
		}
		if i.lastError != nil {
			return nil, i.lastError
		}
	}
	return i.outvars, nil
}

func (i *Interpreter) registerKFuncs() {
	fns := []interface{}{i.print}
	for _, fn := range fns {
		fi := getFuncInfo(fn)
		i.kfuncs[fi.funcName] = fi
	}
}

// registerUFunc register expr functions
// add func check to make sure
// the func with the following return
// signature:
//  1. no return: func(....)
//  2. one return with *MeasureValue: func(....) *MeasureValue
//  3. two return with *MeasureValue and an error: func(....) (*MeasureValue, error)
func (i *Interpreter) registerUFunc(fn interface{}) error {
	fi := getFuncInfo(fn)
	if _, ok := i.kfuncs[fi.funcName]; ok {
		return fmt.Errorf("overwriting kernel func %v not allowed", fi.funcName)
	}
	if _, ok := i.funcs[fi.funcName]; ok {
		return fmt.Errorf("found reregistered func %s", fi.funcName)
	}
	switch len(fi.returnTypes) {
	case 1:
		rt := fi.returnTypeNames[0]
		if rt != "*MeasureValue" {
			return errors.New("unsupported func return type, expect *MeasureValue")
		}
	case 2:
		rt := fi.returnTypeNames[0]
		if rt != "*MeasureValue" {
			return errors.New("unsupported func return types, expect (*MeasureValue, error)")
		}
		rt = fi.returnTypeNames[1]
		if rt != "error" {
			return errors.New("unsupported func return types, expect (*MeasureValue, error)")
		}
	}
	i.funcs[fi.funcName] = fi
	return nil
}

func (i *Interpreter) parseOneExpr(expr string) (Node, error) {
	l := newLexer(expr)
	if ret := exprParse(l); ret != 0 {
		return nil, l.lastError
	}
	return l.root, nil
}

func (i *Interpreter) visitRoot(root Node) error {
	switch root.Type() {
	case NodeTypeAssignment:
		if err := i.visitAssignment(root.(*Assignment)); err != nil {
			return err
		}
	case NodeTypeFuncCall:
		// since we are on root node, ignoring the return mv
		if _, err := i.visitFuncCall(root.(*FuncCall)); err != nil {
			return err
		}
	}
	return nil
}

func (i *Interpreter) visitAssignment(a *Assignment) error {
	switch a.node.Type() {
	case NodeTypeFuncCall:
		mv, err := i.visitFuncCall(a.node.(*FuncCall))
		if err != nil {
			return err
		}
		// we are expecting func call
		// returning either mv or void
		// so if it returns valid mv,
		// assign the mv to the var,
		// otherwise ignore it.
		if mv != nil {
			i.mvvars[a.variable] = mv
		}
	default:
		ans, err := i.visitAExpr(a.node)
		if err != nil {
			return err
		}
		i.mvvars[a.variable] = ans
	}
	return nil
}

// visitAExpr visits expr node, return either *MeasureValue
// or decimal.Decimal
func (i *Interpreter) visitAExpr(a Node) (*MeasureValue, error) {
	switch a.Type() {
	case NodeTypeMV:
		mv := i.visitMeasuredValue(a.(*MeasureValue))
		return mv, nil
	case NodeTypeVar:
		var_ := i.visitVariable(a.(*Variable))
		if value, ok := i.mvvars[var_.Name]; ok {
			return value, nil
		}
		return nil, fmt.Errorf("found undefined var %s" + var_.Name)
	case NodeTypeBinaryExpr:
		mv, err := i.visitBinaryExpr(a.(*BinaryExpr))
		if err != nil {
			return nil, err
		}
		return mv, nil
	case NodeTypeUnaryExpr:
		mv, err := i.visitUnaryExpr(a.(*UnaryExpr))
		if err != nil {
			return nil, err
		}
		return mv, nil
	default:
		return nil, fmt.Errorf("found unsupported expr node: %v", a.Type())
	}
}

func (i *Interpreter) visitFuncCall(a *FuncCall) (*MeasureValue, error) {
	if kf, ok := i.kfuncs[a.fn]; ok {
		// we have a kernel func call
		var args []interface{}
		for _, argnode := range a.args {
			arg, err := i.visitKFuncArg(argnode)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
		return i.call(kf, args...)
	}
	// we have a user func call
	f, ok := i.funcs[a.fn]
	if !ok {
		return nil, fmt.Errorf("unknow func: %s", a.fn)
	}
	var args []interface{}
	for _, argnode := range a.args {
		arg, err := i.visitFuncArg(argnode)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}
	return i.call(f, args...)
}

func (i *Interpreter) visitKFuncArg(a Node) (interface{}, error) {
	switch a.Type() {
	case NodeTypeLiteralStr:
		str := i.visitLiteralStr(a.(*LiteralString))
		return str, nil
	case NodeTypeVar:
		// instead of evaluate a var in visitFuncArg,
		// we have var return directly. A direct use
		// case is the print func.
		return a, nil
	default:
		return i.visitAExpr(a)
	}
}

func (i *Interpreter) visitFuncArg(a Node) (interface{}, error) {
	switch a.Type() {
	case NodeTypeLiteralStr:
		str := i.visitLiteralStr(a.(*LiteralString))
		return str, nil
	case NodeTypeVar:
		varname := a.(*Variable).Name
		return i.mvvars[varname], nil
	default:
		return i.visitAExpr(a)
	}
}

func (i *Interpreter) call(f *function, args ...interface{}) (*MeasureValue, error) {
	defer func() {
		if r := recover(); r != nil {
			err, ok := r.(error)
			if !ok {
				err = errors.New(fmt.Sprint(r))
			}
			i.lastError = fmt.Errorf("call func %s failed: %v", f.funcName, err)
		}
	}()

	// assuming the f is valid if we
	// can get it from the funcs pool.
	var rargs []reflect.Value
	for _, arg := range args {
		rargs = append(rargs, reflect.ValueOf(arg))
	}

	results, err := f.call(rargs...)
	if err != nil {
		return nil, err
	}
	switch len(results) {
	case 0:
		return nil, nil
	case 1, 2:
		// if the result has one or two results
		// it is guaranteed that it is *MeasureValue
		val := results[0].Interface()
		if val == nil {
			return nil, nil
		}
		mv := results[0].Interface().(*MeasureValue)
		return mv, nil
	default:
		return nil, fmt.Errorf("unexpected number of func call results, expected at most 2, got %d", len(results))
	}
}

func (i *Interpreter) visitLiteralStr(a *LiteralString) string {
	return a.s
}

func (i *Interpreter) visitMeasuredValue(a *MeasureValue) *MeasureValue {
	return a
}

func (i *Interpreter) visitVariable(a *Variable) *Variable {
	return a
}

func (i *Interpreter) visitBinaryExpr(a *BinaryExpr) (*MeasureValue, error) {
	lhs, err := i.visitAExpr(a.lhs)
	if err != nil {
		return nil, err
	}
	rhs, err := i.visitAExpr(a.rhs)
	if err != nil {
		return nil, err
	}
	switch a.Op {
	case OpAdd:
		return lhs.Add(rhs)
	case OpSub:
		return lhs.Sub(rhs)
	case OpMul:
		return lhs.Mul(rhs)
	case OpDiv:
		return lhs.Sub(rhs)
	default:
		return nil, fmt.Errorf("unsupported op %s", a.Op)
	}
}

func (i *Interpreter) visitUnaryExpr(a *UnaryExpr) (*MeasureValue, error) {
	ans, err := i.visitAExpr(a.expr)
	if err != nil {
		return nil, err
	}
	return ans.Neg(), nil
}

// print is the kernel func of the expr
// it will save the given name of the varname
// to outvars, the given varname should be
// MeasureValue var only, any non-MeasureValue
// var will cause error.
func (i *Interpreter) print(args ...interface{}) {
	for _, arg := range args {
		switch a := arg.(type) {
		case *Variable:
			value, ok := i.mvvars[a.Name]
			if !ok {
				continue
			}
			i.outvars[a.Name] = value
		default:
			i.lastError = fmt.Errorf("expect variable as the arg of print, found: %T", a)
		}
	}
}

type function struct {
	paramTypeNames  []string
	paramTypes      []reflect.Type
	returnTypeNames []string
	returnTypes     []reflect.Type
	errorIndexes    []int
	fnValue         reflect.Value
	funcName        string
}

func getFuncInfo(fn interface{}) *function {
	// Check if the argument is a function
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		panic("argument must be a function")
	}

	// Get the function's value
	fnValue := reflect.ValueOf(fn)

	// Extract the function name
	strs := strings.Split(runtime.FuncForPC(fnValue.Pointer()).Name(), ".")
	funcName := strs[len(strs)-1]
	// A function from method would have -fm suffix, remove it
	// https://groups.google.com/g/golang-nuts/c/nZtpSK3SOGE?pli=1
	funcName = strings.TrimSuffix(funcName, "-fm")

	var paramTypeNames []string
	var paramTypes []reflect.Type
	var returnTypeNames []string
	var returnTypes []reflect.Type

	// Extract parameter information
	for i := 0; i < fnType.NumIn(); i++ {
		it := fnType.In(i)
		paramType := it
		paramName := it.Name()
		if it.Kind() == reflect.Ptr {
			paramName = "*" + it.Elem().Name()
		}
		paramTypeNames = append(paramTypeNames, paramName)
		paramTypes = append(paramTypes, paramType)
	}

	// Extract return information
	var errorIndexes []int
	for i := 0; i < fnType.NumOut(); i++ {
		it := fnType.Out(i)
		returnType := it
		returnName := it.Name()
		if returnName == "error" {
			errorIndexes = append(errorIndexes, i)
		}
		if it.Kind() == reflect.Ptr {
			returnName = "*" + it.Elem().Name()
		}
		returnTypeNames = append(returnTypeNames, returnName)
		returnTypes = append(returnTypes, returnType)
	}

	return &function{
		paramTypeNames:  paramTypeNames,
		paramTypes:      paramTypes,
		returnTypeNames: returnTypeNames,
		returnTypes:     returnTypes,
		errorIndexes:    errorIndexes,
		fnValue:         fnValue,
		funcName:        funcName,
	}
}

func (f *function) call(args ...reflect.Value) ([]reflect.Value, error) {
	results := f.fnValue.Call(args)
	for _, ind := range f.errorIndexes {
		itf := results[ind].Interface()
		if itf == nil {
			return results, nil
		}
		err := itf.(error)
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

func init() {
	exprErrorVerbose = true
}

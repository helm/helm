/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package engine

import (
	"C"
	"reflect"
	_ "unsafe"
)

// jsonNumber is an interface that mocks json.Number behavior. The method set is
// completely identical to the original struct definition. Using an internal
// interface allows us to get rid of explicit encoding/json dependency in this
// package.
type jsonNumber interface {
	Float64() (float64, error)
	Int64() (int64, error)
	String() string
}

type argctx uint8

const (
	ctxInt argctx = 1 << (iota + 1)
	ctxFloat
	ctxAllref
)

var (
	// A hack to get a type of an empty interface
	intfType    reflect.Type = reflect.ValueOf(func(interface{}) {}).Type().In(0)
	intType                  = reflect.TypeOf(int(0))
	int64Type                = reflect.TypeOf(int64(0))
	float64Type              = reflect.TypeOf(float64(0))
)

var castNumericTo map[reflect.Kind]reflect.Kind
var typeConverters map[reflect.Kind]reflect.Type

func init() {
	castNumericTo = make(map[reflect.Kind]reflect.Kind)
	castNumericTo[reflect.Interface] = 0
	for _, kind := range []reflect.Kind{reflect.Int, reflect.Uint} {
		castNumericTo[kind] = reflect.Int
	}
	for _, kind := range []reflect.Kind{reflect.Int32, reflect.Int64, reflect.Uint32, reflect.Uint64} {
		castNumericTo[kind] = reflect.Int64
	}
	for _, kind := range []reflect.Kind{reflect.Float32, reflect.Float64} {
		castNumericTo[kind] = reflect.Float64
	}
	typeConverters = map[reflect.Kind]reflect.Type{
		reflect.Int:     intType,
		reflect.Int64:   int64Type,
		reflect.Float64: float64Type,
	}
}

func isIntKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	}
	return false
}

func isFloatKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

// guessArgsCtx iterates over the input arguments and tries to guess a common
// value type numeric context. The return value is a bit mask holding bit flags
// for float and int types. If all arguments are of type reflect.Value, the
// result bitmask will contain a dedicated flag ctxAllref set to 1. For
// variadic functions the last argument is expected to be a slice and is handled
// the same way as the main list.
func guessArgsCtx(args []reflect.Value, isvariadic bool) argctx {
	var ctx, msk argctx
	ctx |= ctxAllref
	for _, arg := range args {
		msk = ^ctxAllref
		kind := arg.Kind()
		switch kind {
		case reflect.Struct:
			if v, ok := arg.Interface().(reflect.Value); ok {
				kind = v.Kind()
			}
			msk |= ctxAllref
		case reflect.Interface:
			v := reflect.ValueOf(arg.Interface())
			kind = v.Kind()
		}
		if isFloatKind(kind) {
			ctx |= ctxFloat
		} else if isIntKind(kind) {
			ctx |= ctxInt
		}
		ctx &= msk
	}
	// Variadic functions are handled in a slightly special way
	if isvariadic && len(args) > 0 {
		// The last argument in variadic functions is a slice and we should
		// iterate all over the variadic arguments the same way we do it for the
		// regular args.
		varg := args[len(args)-1]
		varargs := make([]reflect.Value, 0, varg.Len())
		for i := 0; i < varg.Len(); i++ {
			varargs = append(varargs, varg.Index(i))
		}
		// We call the same routine with an explicit flag that the argument list
		// is not variadic
		varctx := guessArgsCtx(varargs, false)
		varmsk := ^ctxAllref
		if ctx&ctxAllref > 0 {
			varmsk |= ctxAllref
		}
		return (varctx | ctx) & varmsk
	}
	return ctx
}

// convJSONNumber converts a jsonNumber argument to a particular primitive
// defined by a wanted kind or an argument context.
// The context would be used if the wanted kind is unknown (we use 0 as an
// undefined value. This can happen if the argument wanted kind is interface{}
// or reflect.Value, which defines no specific primitive to convert to. In this
// case a broader observaion is used to define the optimal conversion strategy.
func convJSONNumber(jsnum jsonNumber, wantkind reflect.Kind, ctx argctx) (interface{}, error) {
	switch wantkind {
	case reflect.Int:
		int64val, err := jsnum.Int64()
		if err != nil {
			return nil, err
		}
		return int(int64val), nil
	case reflect.Int64:
		return jsnum.Int64()
	case reflect.Float64:
		return jsnum.Float64()
	// The wanted kind is unknown yet, we should guess it from the context
	case 0:
		switch {
		case ctx&ctxInt > 0:
			if intval, err := convJSONNumber(jsnum, reflect.Int64, ctx); err == nil {
				return intval, nil
			}
		case ctx&ctxFloat > 0:
			if floatval, err := convJSONNumber(jsnum, reflect.Float64, ctx); err == nil {
				return floatval, nil
			}
		}
	}
	return jsnum.String(), nil
}

// convIntf converts a given value to a wanted kind within a provided argument
// context. If the value conforms to jsonNumber interface, the conversion is
// delegated to convJSONNumber. If the value is of a numeric type, conversion is
// performed according to the conversion table defined by typeConverters.
func convIntf(val reflect.Value, wantkind reflect.Kind, ctx argctx) reflect.Value {
	intf := val.Interface()
	if jsnum, ok := intf.(jsonNumber); ok {
		if convval, err := convJSONNumber(jsnum, castNumericTo[wantkind], ctx); err == nil {
			return reflect.ValueOf(convval)
		}
	}
	if convtype, ok := typeConverters[wantkind]; ok {
		if reflect.TypeOf(intf).ConvertibleTo(convtype) {
			return reflect.ValueOf(intf).Convert(convtype)
		}
	}
	// If no conversion was performed, we return the value as is
	return val
}

// convFuncArg converts an argument of 2 particular types: interface{} and reflect.Value
// to a primitive. Both types provide no certainty on the final type and kind and
// therefore the function converts the input value using convIntf. If the input
// value is of type reflect.Value (means: reflect.ValueOf(reflect.ValueOf(...)))
// and the context has both int and float bits set, convVal forces conversion
// type to float64.
func convFuncArg(val reflect.Value, wantkind reflect.Kind, ctx argctx) reflect.Value {
	conv := val
	switch val.Kind() {
	case reflect.Interface:
		conv = convIntf(val, wantkind, ctx)
	case reflect.Struct:
		if rv, ok := val.Interface().(reflect.Value); ok {
			if ((ctx & ctxAllref) > 0) && ((ctx & ctxFloat) > 0) {
				wantkind = reflect.Float64
			}
			conv = reflect.ValueOf(convIntf(rv, wantkind, ctx))
		}
	}
	return conv
}

// convFuncArgs accepts a list of factual arguments, corresponding expected types
// and returns a list of converted arguments. The last argument is a flag
// indicating the list of values is invoked on a variadic function (in this case
// the last argument in the returned list would be safely converted to a
// variadic-friendly slice.
func convFuncArgs(args []reflect.Value, wantkind []reflect.Kind, isvariadic bool) []reflect.Value {
	ctx := guessArgsCtx(args, isvariadic)
	newargs := make([]reflect.Value, 0, len(args))
	for i, arg := range args {
		convarg := convFuncArg(arg, wantkind[i], ctx)
		newargs = append(newargs, convarg)
	}
	if isvariadic && len(newargs) > 0 {
		varargs := newargs[len(newargs)-1]
		for i := 0; i < varargs.Len(); i++ {
			vararg := varargs.Index(i)
			convarg := convFuncArg(vararg, reflect.Interface, ctx)
			vararg.Set(convarg)
		}
	}
	return newargs
}

// getArgTypes takes a function type as an argument and returns 2 lists: return
// argument types and return argument kinds. The returned type list will contain
// pre-casted types for all known types from the conversion table: e.g. uint8
// would be pre-casted to int64.
func getArgTypes(functype reflect.Type) ([]reflect.Type, []reflect.Kind) {
	newargs := make([]reflect.Type, 0, functype.NumIn())
	wantkind := make([]reflect.Kind, 0, functype.NumIn())
	for i := 0; i < functype.NumIn(); i++ {
		newtype := functype.In(i)
		argkind := functype.In(i).Kind()
		wantkind = append(wantkind, argkind)
		// This is a bit cryptic: if there is a converter for a provided
		// function argument type, we substitute it with an interface type so we
		// can do an ad-hoc conversion when the overloaded function would be
		// invoked.
		//
		// For example, if a template function is defined as:
		// ```func foo(bar int64)```,
		//
		// The argument type list would look like:
		// `[]reflect.Type{reflect.Int64}`
		//
		// What it means in fact is: when the template rendering engine will
		// invoke the function, the factual argument will be of any type
		// convertible to int64 (from reflect's POV). When we allow external
		// types (like: JSONNumber), we have to convert them to int64 explicitly
		// and on top of that we have to relax the rendering function formal
		// argument type check strictness. In other words, we let any value in
		// by using interface{} type instead of int64 so the reflect-backed
		// gotmpl invocation of rendering functions keeps working.
		//
		// An overloaded foo(1) will have the following signature:
		// ```func foo(bar interface{})```.
		if _, ok := castNumericTo[argkind]; ok {
			newtype = intfType
		}
		newargs = append(newargs, newtype)
	}
	return newargs, wantkind
}

// overloadFunc modifies the input function so it can handle JSONNumber
// arguments as regular numeric values. It relaxes formal argument type
// if needed. For example: if a function signature expects and argument of type
// int64, the overloaded function will expect an interface{} argument and
// perform the corresponding conversion and type checking in the runtime.
// It mainly searches for 3 categories of arguments:
//   * numeric arguments
//   * interface{}
//   * reflect.Value
// If the input function signature expects a specific type, override will
// preserve it and convert during the runtime before propagating the invocation
// to the input func. If the type is vague (e.g. interface{}), the best cast
// option would be guessed from the argument list context. For example: if the
// input function expects 2 arguments: float64 and interface{}, and the 2nd one
// is of jsonNumber type, it would be casted to float64 as well.
func overloadFunc(fn interface{}) interface{} {
	funcval := reflect.ValueOf(fn)
	functype := funcval.Type()

	newargs, wantkind := getArgTypes(functype)
	newreturn := make([]reflect.Type, 0, functype.NumOut())
	for i := 0; i < functype.NumOut(); i++ {
		newreturn = append(newreturn, functype.Out(i))
	}

	newfunctype := reflect.FuncOf(newargs, newreturn, functype.IsVariadic())
	newfunc := func(args []reflect.Value) []reflect.Value {
		convargs := convFuncArgs(args, wantkind, functype.IsVariadic())
		if functype.IsVariadic() {
			return funcval.CallSlice(convargs)
		}
		return funcval.Call(convargs)
	}

	return reflect.MakeFunc(newfunctype, newfunc).Interface()
}

// These 6 functions are imported from text/template and are meant to be
// overloaded in funcMap call.

//go:linkname _templateBuiltinEq text/template.eq
func _templateBuiltinEq(arg1 reflect.Value, arg2 ...reflect.Value) (bool, error)

//go:linkname _templateBuiltinGe text/template.ge
func _templateBuiltinGe(arg1, arg2 reflect.Value) (bool, error)

//go:linkname _templateBuiltinGt text/template.gt
func _templateBuiltinGt(arg1, arg2 reflect.Value) (bool, error)

//go:linkname _templateBuiltinLe text/template.le
func _templateBuiltinLe(arg1, arg2 reflect.Value) (bool, error)

//go:linkname _templateBuiltinLt text/template.lt
func _templateBuiltinLt(arg1, arg2 reflect.Value) (bool, error)

//go:linkname _templateBuiltinNe text/template.ne
func _templateBuiltinNe(arg1, arg2 reflect.Value) (bool, error)

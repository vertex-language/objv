package analyzer

import "github.com/vertex-language/objv/types"

// The compiler's own functions.
//
// A builtin is declared by no header. Every spelling beginning __builtin_ is
// reserved to the implementation, __has_builtin is how a program asks whether
// this compiler has a particular one, and the signature is the compiler's to
// know — which is why the table has to live here. An expression whose
// function has no type is an expression with no type, and a phase further
// down reports that fact once per operator built on it, in words that name
// the wrong thing:
//
//	error: internal: no type recorded for *ast.Ident
//	error: internal: no type recorded for *ast.CallExpr
//	error: internal: no type recorded for *ast.BinaryExpr
//
// for the one call the compiler could not type.
//
// What the table holds is what objv *implements* — every entry is emitted by
// lower/builtin.go, and TestBuiltinsAreLowered asserts the two agree. A
// __builtin_ name outside it is reported by name, once, rather than waved
// through: `#import <Foundation/Foundation.h>` reaches a dozen of these
// through <math.h> and <libkern/OSByteOrder.h> alone, and accepting one the
// compiler cannot emit only moves the failure somewhere it cannot be
// explained.
//
// The selection is not a guess about what programs want. It is what VIR has
// an operation for: abs, sqrt, ceil, floor, trunc, copysign, minnum, maxnum,
// fma, clz, ctz, popcnt, bswap. A builtin in that correspondence costs one
// instruction and nothing else; one outside it would have to be a call to a
// function of the same name, which is a decision to make when something needs
// it rather than in advance.

// builtinSpec is one builtin's type.
//
// A spec is stated in kinds rather than types because every one of these is
// built from basics: no builtin here takes a pointer, a struct, or an array.
type builtinSpec struct {
	ret    types.Kind
	params []types.Kind

	// any marks the one builtin whose argument has no type of its own.
	// __builtin_constant_p takes an expression of any type and does not
	// evaluate it, which is a shape a signature cannot state.
	any bool
}

// The table. Read it as four groups: the floating-point operations VIR has a
// verb for, the two constants <math.h> is written in terms of, the bit
// operations, and the three that are about control rather than arithmetic.
var builtins = func() map[string]builtinSpec {
	m := map[string]builtinSpec{}

	// A float builtin comes in three widths with one suffix convention:
	// none for double, f for float, l for long double. The suffix is part
	// of the name and not a property of the argument, because a builtin has
	// no overloading — __builtin_fabs(x) converts x to double.
	widths := []struct {
		suffix string
		kind   types.Kind
	}{
		{"", types.Double},
		{"f", types.Float},
		{"l", types.LongDouble},
	}
	for _, w := range widths {
		unary := []string{"fabs", "sqrt", "ceil", "floor", "trunc"}
		for _, n := range unary {
			m["__builtin_"+n+w.suffix] = builtinSpec{ret: w.kind, params: []types.Kind{w.kind}}
		}
		binary := []string{"copysign", "fmin", "fmax"}
		for _, n := range binary {
			m["__builtin_"+n+w.suffix] = builtinSpec{ret: w.kind, params: []types.Kind{w.kind, w.kind}}
		}
		m["__builtin_fma"+w.suffix] = builtinSpec{ret: w.kind,
			params: []types.Kind{w.kind, w.kind, w.kind}}

		// The two infinities. huge_val is C89's spelling and inf is gcc's;
		// they are the same value, and <math.h> defines HUGE_VAL as one of
		// them and INFINITY as the other.
		m["__builtin_inf"+w.suffix] = builtinSpec{ret: w.kind}
		m["__builtin_huge_val"+w.suffix] = builtinSpec{ret: w.kind}
	}

	// The bit operations. Each comes in three widths too, but the suffix
	// selects the *argument* and the result is always int: clz counts
	// leading zeros and the count fits in an int whatever it counted.
	for _, w := range []struct {
		suffix string
		kind   types.Kind
	}{
		{"", types.UInt},
		{"l", types.ULong},
		{"ll", types.ULongLong},
	} {
		for _, n := range []string{"clz", "ctz", "popcount", "parity"} {
			m["__builtin_"+n+w.suffix] = builtinSpec{ret: types.Int, params: []types.Kind{w.kind}}
		}
	}

	// bswap is named by width rather than by C type, being about bytes.
	m["__builtin_bswap16"] = builtinSpec{ret: types.UShort, params: []types.Kind{types.UShort}}
	m["__builtin_bswap32"] = builtinSpec{ret: types.UInt, params: []types.Kind{types.UInt}}
	m["__builtin_bswap64"] = builtinSpec{ret: types.ULongLong, params: []types.Kind{types.ULongLong}}

	// Control, not arithmetic.
	m["__builtin_expect"] = builtinSpec{ret: types.Long, params: []types.Kind{types.Long, types.Long}}
	m["__builtin_unreachable"] = builtinSpec{ret: types.Void}
	m["__builtin_trap"] = builtinSpec{ret: types.Void}
	m["__builtin_constant_p"] = builtinSpec{ret: types.Int, any: true}
	return m
}()

// BuiltinNames is every builtin objv implements. It exists for the test that
// holds this table and lower's emission to the same list.
func BuiltinNames() []string {
	out := make([]string, 0, len(builtins))
	for n := range builtins {
		out = append(out, n)
	}
	return out
}

// builtinType is the type of a builtin used as a value, or nil for a name
// objv does not implement.
func builtinType(name string) types.Type {
	spec, ok := builtins[name]
	if !ok {
		return nil
	}
	ft := &types.Func{Ret: types.Typ(spec.ret), Proto: true}
	if spec.any {
		// An unprototyped type, which is what "one argument, any type"
		// amounts to in C's type system: the call is not checked against a
		// parameter list because there is no list to check it against.
		ft.Proto = false
		return ft
	}
	if len(spec.params) == 0 {
		// `f(void)`, not `f()`: a builtin taking nothing takes nothing, and
		// __builtin_inf(1) should be a diagnostic.
		ft.Params = nil
		return ft
	}
	for _, k := range spec.params {
		ft.Params = append(ft.Params, types.Param{Type: types.Typ(k)})
	}
	return ft
}

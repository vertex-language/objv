package analyzer

import "github.com/vertex-language/objv/types"

// Builtin function signatures recognized by the compiler.
// All functions listed here map directly to VIR operations implemented in lower/builtin.go.

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

	// The variadic machinery, §7.16. objv's own <stdarg.h> spells the four
	// macros in terms of these, taking the address of the list rather than
	// the list itself, so that one signature serves whatever shape
	// __builtin_va_list has on the target.
	//
	// __builtin_va_arg_ref is the odd one: it advances the list past one
	// argument and yields that argument's *address*, and its type is the
	// type of the pointer it was handed — `(T *)0`, which is how the macro
	// smuggles a type into an expression. See builtinResult.
	m["__builtin_va_start"] = builtinSpec{ret: types.Void, params: []types.Kind{types.PointerKind}}
	m["__builtin_va_end"] = builtinSpec{ret: types.Void, params: []types.Kind{types.PointerKind}}
	m["__builtin_va_copy"] = builtinSpec{ret: types.Void,
		params: []types.Kind{types.PointerKind, types.PointerKind}}
	m["__builtin_va_arg_ref"] = builtinSpec{ret: types.Void, any: true}

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
	if _, ok := AtomicBuiltins[name]; ok {
		// Unprototyped, which is what "one name, every width" amounts to in
		// C's type system: there is no parameter list to check the call
		// against, and the result comes from the operand at the call site.
		// See atomicResult.
		return &types.Func{Ret: types.Typ(types.Int), Proto: false}
	}
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
		ft.Params = append(ft.Params, types.Param{Type: paramType(k)})
	}
	return ft
}

// paramType is a spec's kind as a type. Every kind but one is a basic type;
// PointerKind stands for `void *`, which is what the variadic builtins take
// and the only non-basic parameter in the table.
func paramType(k types.Kind) types.Type {
	if k == types.PointerKind {
		return &types.Pointer{Elem: types.Typ(types.Void)}
	}
	return types.Typ(k)
}

// builtinResult is the type of a call to a builtin whose result depends on
// its arguments rather than on its name.
//
// There is one, and it is __builtin_va_arg_ref: `va_arg(ap, T)` expands to
// `*(T *)__builtin_va_arg_ref(&ap, (T *)0)`, and the null pointer is there
// only to carry T into an expression — C has no other way to pass a type to
// something that is not a keyword. So the call's type is that pointer's.
func builtinResult(name string, args []types.Type) (types.Type, bool) {
	if t, ok := atomicResult(name, args); ok {
		return t, true
	}
	if name != "__builtin_va_arg_ref" || len(args) != 2 || args[1] == nil {
		return nil, false
	}
	if !types.IsPointer(args[1]) {
		return nil, false
	}
	return args[1], true
}

// AtomicBuiltins is §7.17's read-modify-write operations, which objv's own
// <stdatomic.h> is written in terms of.
//
// They are not in the table above because they have no signature: the type
// is the *pointee* of the first argument, and a builtin has no overloading
// to state that with. atomic_fetch_add on an _Atomic(long) is a long
// operation and on an _Atomic(int) an int one, and the same name is both.
var AtomicBuiltins = map[string]string{
	"__builtin_atomic_exchange":         "xchg",
	"__builtin_atomic_fetch_add":        "add",
	"__builtin_atomic_fetch_sub":        "sub",
	"__builtin_atomic_fetch_and":        "and",
	"__builtin_atomic_fetch_or":         "or",
	"__builtin_atomic_fetch_xor":        "xor",
	"__builtin_atomic_compare_exchange": "cas",
}

// atomicResult types one of them: the value read for an exchange or a fetch,
// and whether the exchange happened for a compare-exchange.
func atomicResult(name string, args []types.Type) (types.Type, bool) {
	op, ok := AtomicBuiltins[name]
	if !ok || len(args) == 0 || args[0] == nil {
		return nil, false
	}
	p := types.AsPointer(types.Unqualify(args[0]))
	if p == nil {
		return nil, false
	}
	if op == "cas" {
		return types.Typ(types.Bool), true
	}
	return types.Unqualify(p.Elem), true
}

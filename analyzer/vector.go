package analyzer

import (
	"strings"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// Type checking for clang extended vector types and swizzles (__ext_vector_type__).
//
// Supports lane-wise arithmetic and member swizzles using position (.xyzw),
// color (.rgba), hex indices (.s0-.sF), or sub-vector halves (.lo, .hi, .even, .odd).

// vectorMember is the type of `v.name` where v is a vector, and whether name
// is a swizzle at all.
func (c *checker) vectorMember(at ast.Node, v *types.Vector, base types.Type, name string) (types.Type, bool) {
	quals := types.QualsOf(base)
	elem := types.Qualify(v.Elem, quals)

	// The halves, which are not swizzles and do not mix with them.
	switch name {
	case "lo", "hi", "even", "odd":
		n := (v.Len + 1) / 2
		if v.Len < 2 {
			c.report(at, "'"+name+"' needs a vector of at least two elements, not "+v.String())
			return nil, true
		}
		if n == 1 {
			return elem, true
		}
		return types.Qualify(&types.Vector{Elem: v.Elem, Len: n}, quals), true
	}

	lanes, ok := swizzleLanes(name)
	if !ok {
		return nil, false
	}
	// A swizzle is as long as it is written, not as long as its alphabet.
	// The position names run out at four, but the *lanes* may repeat:
	// `x.xyzwxyzw` is how <simd/logic.h> makes an eight-element vector out
	// of a four-element one, and `v.xxxx` is how a header broadcasts one
	// lane. Sixteen is clang's limit and the widest vector there is.
	if len(lanes) > 16 {
		c.report(at, "'"+name+"' selects more than sixteen lanes")
		return nil, true
	}
	for _, l := range lanes {
		if l >= v.Len {
			c.report(at, "lane "+string("0123456789abcdef"[l])+" is out of range for "+v.String())
			return nil, true
		}
	}
	if len(lanes) == 1 {
		// One lane is the element, not a vector of one. `p.x + p.y` is
		// float arithmetic, and every simd header relies on it.
		return elem, true
	}
	return types.Qualify(&types.Vector{Elem: v.Elem, Len: int64(len(lanes))}, quals), true
}

// swizzleLanes is the lane indices a member name selects, or ok=false when
// the name is not one of the four alphabets.
//
// The alphabets do not mix: `v.xg` is not the first two lanes, it is a
// mistake, and clang rejects it. `s` is its own alphabet and takes a
// hexadecimal digit per lane, which is how a sixteen-element vector names
// lanes past the fourth.
func swizzleLanes(name string) ([]int64, bool) {
	if name == "" {
		return nil, false
	}
	if name[0] == 's' || name[0] == 'S' {
		rest := name[1:]
		if rest == "" {
			return nil, false
		}
		out := make([]int64, 0, len(rest))
		for i := 0; i < len(rest); i++ {
			d := hexDigit(rest[i])
			if d < 0 {
				return nil, false
			}
			out = append(out, int64(d))
		}
		return out, true
	}
	const position = "xyzw"
	const colour = "rgba"
	var set string
	switch {
	case strings.IndexByte(position, name[0]) >= 0:
		set = position
	case strings.IndexByte(colour, name[0]) >= 0:
		set = colour
	default:
		return nil, false
	}
	out := make([]int64, 0, len(name))
	for i := 0; i < len(name); i++ {
		j := strings.IndexByte(set, name[i])
		if j < 0 {
			return nil, false
		}
		out = append(out, int64(j))
	}
	return out, true
}

func hexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// vectorBinary is the type of an elementwise operation on vectors, and
// whether either operand was one.
//
// §clang: the operands must be vectors of the same shape, or one of them a
// scalar — which is spread across every lane before the operation. A
// comparison yields a vector of *signed integers* of the element's width,
// all ones where the comparison held: there is no vector of bool, and
// `simd_all(a == b)` is how a program asks whether every lane agreed.
func (c *checker) vectorBinary(at ast.Node, op string, x, y types.Type, comparison bool) (types.Type, bool) {
	vx, vy := types.AsVector(x), types.AsVector(y)
	if vx == nil && vy == nil {
		return nil, false
	}
	var v *types.Vector
	switch {
	case vx != nil && vy != nil:
		if vx.Len != vy.Len || !types.Compatible(types.Unqualify(vx.Elem), types.Unqualify(vy.Elem)) {
			c.report(at, "cannot apply '"+op+"' to "+x.String()+" and "+y.String()+
				": vector operands must have the same element type and count")
			return nil, true
		}
		v = vx
	case vx != nil:
		if !types.IsArithmetic(types.Decay(y)) {
			c.report(at, "cannot apply '"+op+"' to "+x.String()+" and "+y.String())
			return nil, true
		}
		v = vx
	default:
		if !types.IsArithmetic(types.Decay(x)) {
			c.report(at, "cannot apply '"+op+"' to "+x.String()+" and "+y.String())
			return nil, true
		}
		v = vy
	}
	if comparison {
		return &types.Vector{Elem: c.vectorMaskElem(v.Elem), Len: v.Len}, true
	}
	return &types.Vector{Elem: v.Elem, Len: v.Len}, true
}

// vectorMaskElem is the element type a comparison on this element yields: a
// signed integer as wide as the element, because the result has to live in
// the same register lanes the comparison read.
//
// *Which* signed integer matters, because C has two names for sixty-four
// bits and they are not compatible with each other. clang takes the first
// standard type of the right width, which on LP64 makes a comparison of
// doubles a vector of `long` — and <simd/vector_types.h> agrees, declaring
// simd_long1 as `long` under __LP64__ and `long long` otherwise. Answering
// `long long` there is a type error at every use, on a line nobody wrote.
func (c *checker) vectorMaskElem(elem types.Type) types.Type {
	sz, ok := c.model.Sizeof(elem)
	if !ok {
		return types.Typ(types.Int)
	}
	for _, k := range []types.Kind{
		types.SChar, types.Short, types.Int, types.Long, types.LongLong,
	} {
		if w, ok := c.model.Sizeof(types.Typ(k)); ok && w == sz {
			return types.Typ(k)
		}
	}
	return types.Typ(types.Int)
}

// vectorBuiltin types the two builtins that make a vector out of others.
//
// They are not functions and cannot be declared as any: the result's shape
// comes from the *arguments*, and in one case from a type named in argument
// position. <simd/matrix.h> builds a quaternion product out of
// __builtin_shufflevector and <simd/conversion.h> is __builtin_convertvector
// the whole way down, so a compiler without them reads neither.
//
//	__builtin_shufflevector(a, b, i…)  a vector of len(i) lanes, taken from
//	                                   the concatenation of a and b
//	__builtin_convertvector(v, T)      v with each lane converted to T's
func (c *checker) vectorBuiltin(e *ast.CallExpr, args []types.Type) (types.Type, bool) {
	id, ok := stripParens(e.Fun).(*ast.Ident)
	if !ok {
		return nil, false
	}
	switch c.name(id) {
	case "__builtin_shufflevector":
		if len(args) < 3 {
			c.report(e, "__builtin_shufflevector takes two vectors and one index per lane")
			return nil, true
		}
		v := types.AsVector(args[0])
		w := types.AsVector(args[1])
		if v == nil || w == nil {
			c.report(e, "__builtin_shufflevector takes two vectors")
			return nil, true
		}
		n := int64(len(args) - 2)
		for i := 2; i < len(args); i++ {
			if args[i] != nil && !types.IsInteger(args[i]) {
				c.report(e.Args[i], "a shuffle index must be an integer constant")
			}
		}
		if n == 1 {
			return v.Elem, true
		}
		return &types.Vector{Elem: v.Elem, Len: n}, true

	case "__builtin_convertvector":
		// The second argument is a type name, which the parser read as an
		// expression. c.typeOfArg is what recovers it.
		if len(e.Args) != 2 {
			c.report(e, "__builtin_convertvector takes a vector and a type")
			return nil, true
		}
		if types.AsVector(args[0]) == nil {
			c.report(e, "__builtin_convertvector takes a vector")
			return nil, true
		}
		t := c.typeOfArg(e.Args[1])
		if t == nil || types.AsVector(t) == nil {
			c.report(e, "__builtin_convertvector's second argument names the vector type to convert to")
			return nil, true
		}
		return t, true
	}
	return nil, false
}

// typeOfArg reads a type name written in argument position, which is how
// __builtin_convertvector takes its destination type. The parser has no way
// to know a type was coming, so it parsed an expression; what it produced for
// a typedef name is an identifier that names one.
func (c *checker) typeOfArg(e ast.Expr) types.Type {
	id, ok := stripParens(e).(*ast.Ident)
	if !ok {
		return nil
	}
	s := c.lookup(c.name(id))
	if s == nil || s.kind != symTypedef {
		return nil
	}
	return s.typ
}

// isVectorBuiltin reports whether the callee is one of the two builtins
// vectorBuiltin types.
func isVectorBuiltin(c *checker, e *ast.CallExpr) bool {
	id, ok := stripParens(e.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	switch c.name(id) {
	case "__builtin_shufflevector", "__builtin_convertvector":
		return true
	}
	return false
}

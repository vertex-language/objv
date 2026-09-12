package analyzer

import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// Resolution for __attribute__((overloadable)) functions.
//
// Overload candidates are scored based on argument conversions (exact match = 2,
// compatible conversion = 1, incompatible = 0). Ties are reported as ambiguous calls.

// overloadSet is the candidates a name stands for, or nil when it is an
// ordinary function.
func (c *checker) overloadSet(fun ast.Expr) []*symbol {
	id, ok := stripParens(fun).(*ast.Ident)
	if !ok {
		return nil
	}
	s := c.lookup(c.name(id))
	if s == nil || len(s.overloads) < 2 {
		return nil
	}
	return s.overloads
}

// resolveOverload picks the declaration a call means and records it, so that
// the identifier's type is the one chosen rather than the one declared first.
//
// It returns the chosen function type, or nil where nothing matched — in
// which case it has reported, because a call that resolves to nothing is not
// a call the program can make.
func (c *checker) resolveOverload(e *ast.CallExpr, set []*symbol, args []types.Type) types.Type {
	best, bestScore, tied := -1, -1, false
	for i, cand := range set {
		ft := types.AsFunc(types.Unqualify(cand.typ))
		if ft == nil {
			continue
		}
		score, ok := c.overloadScore(ft, args)
		if !ok {
			continue
		}
		switch {
		case score > bestScore:
			best, bestScore, tied = i, score, false
		case score == bestScore:
			tied = true
		}
	}
	if best < 0 {
		c.report(e, "no overload of '"+c.callName(e)+"' takes "+
			plural(len(args), "argument")+" of these types")
		return nil
	}
	if tied {
		c.report(e, "the call to '"+c.callName(e)+"' is ambiguous: "+
			"more than one overload matches equally well")
		return nil
	}
	chosen := set[best]
	// The identifier's own type is the overload chosen, so that lower emits
	// a call to this declaration rather than to the first one read. Nothing
	// below this package repeats the resolution.
	if id, ok := stripParens(e.Fun).(*ast.Ident); ok {
		c.info.Types[id] = chosen.typ
		c.info.Overloads[id] = chosen.node
	}
	return chosen.typ
}

// overloadScore ranks one candidate against the arguments, and says whether
// it is viable at all.
func (c *checker) overloadScore(ft *types.Func, args []types.Type) (int, bool) {
	np := len(ft.Params)
	if np == 1 && types.IsVoid(ft.Params[0].Type) {
		np = 0
	}
	if len(args) < np || (len(args) > np && !ft.Variadic) {
		return 0, false
	}
	score := 0
	for i := 0; i < np; i++ {
		a := args[i]
		if a == nil {
			return 0, false
		}
		p := types.AdjustParam(ft.Params[i].Type)
		switch {
		case types.Compatible(types.Unqualify(p), types.Unqualify(types.Decay(a))):
			score += 6
		case c.isPromotion(p, a):
			// §6.3.1.1's promotion, which ranks above an ordinary
			// conversion for the same reason it does in C++: a char
			// becomes an int on its way into any call, so a candidate
			// taking int is what the argument already is. `which(c)` where
			// c is a char and the overloads take int and double is not
			// ambiguous, and calling it so would reject a line clang
			// compiles.
			score += 4
		case c.sameWidthVectorElem(p, a):
			// `char` and `signed char` are distinct types and the same
			// eight bits. A comparison of simd_char2 yields the signed
			// one, simd_all takes the plain one, and <simd/common.h>
			// passes the first to the second on every other line. Ranking
			// that below an exact match and above a lax conversion is what
			// picks simd_all(simd_char2) out of the eight candidates whose
			// parameter is two bytes wide.
			score += 5
		case laxVectorConversion(p, a):
			// Worth less than any other conversion. A lax conversion
			// relates every integer vector of a size to every other, so it
			// makes `simd_abs(x)` viable against eight candidates at once;
			// ranking it with ordinary conversions would call that
			// ambiguous and reject the line rather than pick the one whose
			// parameter the argument actually is.
			score++
		case types.Assignable(p, a, false) == types.AssignOK:
			score += 3
		default:
			return 0, false
		}
	}
	return score, true
}

// callName is what to call the callee in a diagnostic.
func (c *checker) callName(e *ast.CallExpr) string {
	if id, ok := stripParens(e.Fun).(*ast.Ident); ok {
		return c.name(id)
	}
	return "this function"
}

// sameSignature reports whether two function types are the same overload —
// the same parameter list, whatever the return type. Two declarations that
// agree are one declaration written twice, which a header does often; two
// that differ only in the return type are a mistake §6.7p4 already reports.
func sameSignature(a, b types.Type) bool {
	fa, fb := types.AsFunc(types.Unqualify(a)), types.AsFunc(types.Unqualify(b))
	if fa == nil || fb == nil {
		return false
	}
	if len(fa.Params) != len(fb.Params) || fa.Variadic != fb.Variadic {
		return false
	}
	for i := range fa.Params {
		pa := types.AdjustParam(fa.Params[i].Type)
		pb := types.AdjustParam(fb.Params[i].Type)
		if !types.Compatible(types.Unqualify(pa), types.Unqualify(pb)) {
			return false
		}
	}
	return true
}

// laxVectorConversion reports whether the only thing making this argument
// viable is clang's lax vector conversion: two integer vectors of the same
// size but different element types. See types.Assignable.
func laxVectorConversion(p, a types.Type) bool {
	pv, av := types.AsVector(p), types.AsVector(types.Decay(a))
	if pv == nil || av == nil {
		return false
	}
	if pv.Len == av.Len && types.Compatible(types.Unqualify(pv.Elem), types.Unqualify(av.Elem)) {
		return false
	}
	return types.Assignable(p, a, false) == types.AssignOK
}

// sameWidthVectorElem reports whether two vectors differ only in the *name*
// of an integer element type that is the same width and signedness — the
// char/signed char distinction, and long/long long on LP64.
func (c *checker) sameWidthVectorElem(p, a types.Type) bool {
	pv, av := types.AsVector(p), types.AsVector(types.Decay(a))
	if pv == nil || av == nil || pv.Len != av.Len {
		return false
	}
	pe, ae := types.Unqualify(pv.Elem), types.Unqualify(av.Elem)
	if !types.IsInteger(pe) || !types.IsInteger(ae) {
		return false
	}
	sp, okp := c.model.Sizeof(pe)
	sa, oka := c.model.Sizeof(ae)
	return okp && oka && sp == sa && c.signedElem(pe) == c.signedElem(ae)
}

// signedElem is whether an integer element is signed, with plain char
// answered by the target rather than by the language: §6.2.5p15 leaves it
// implementation-defined, and it is signed on every target objv emits for.
// Treating it as neither signed nor unsigned made `simd_all(x == y)` tie
// between the char and unsigned char overloads, which is a real ambiguity
// only if the compiler has forgotten what a char is.
func (c *checker) signedElem(t types.Type) bool {
	if types.Unqualify(t).Kind() == types.Char {
		return c.model.CharSigned
	}
	return types.IsSigned(t)
}

// isPromotion reports whether the parameter is what §6.3.1.1 promotes the
// argument to: a narrow integer to int, or a float to double.
func (c *checker) isPromotion(p, a types.Type) bool {
	pu, au := types.Unqualify(p), types.Unqualify(types.Decay(a))
	if !types.IsArithmetic(pu) || !types.IsArithmetic(au) {
		return false
	}
	if types.Compatible(pu, c.model.Promote(au)) {
		return true
	}
	// The other half of the default argument promotions: a float widens to
	// a double wherever one is wanted.
	return types.Unqualify(au).Kind() == types.Float && pu.Kind() == types.Double
}

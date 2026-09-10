package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/analyzer"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// File-scope initializers, which are values and not code.
//
// §6.7.9p4 makes a static object's initializer a constant expression, so
// nothing here evaluates anything at run time: what it produces is an
// ir.Init tree whose shape matches the declared type exactly, because that
// is what §19.10 checks. The one thing a constant expression may name that
// is not a number is an address — &x, a string literal, a function — which
// becomes a relocation rather than a value.
//
// The brace walk is init.go's, with the same elision and the same
// designators. It is written twice because the two produce different things:
// one emits stores into an object that exists, and this builds the object.

// constInit folds a file-scope initializer to an ir.Init.
func (u *unit) constInit(e ast.Expr, t types.Type) (ir.Init, bool) {
	if list, ok := e.(*ast.InitList); ok {
		c := &initCursor{items: list.Items}
		return u.constFill(t, c)
	}
	return u.constScalar(e, t)
}

// constScalar folds one initializer that is not a braced list.
func (u *unit) constScalar(e ast.Expr, t types.Type) (ir.Init, bool) {
	if s, ok := stripParens(e).(*ast.StringLit); ok && !s.Object {
		if types.IsArray(t) {
			return u.constStringArray(s, t)
		}
		if sym := u.stringSymbol(s); sym != nil {
			return ir.RelocInit(sym), true
		}
		return ir.Init{}, false
	}
	if types.IsFloat(t) {
		if v, ok := u.foldFloat(e); ok {
			return ir.Lit(ir.Float(v)), true
		}
		return ir.Init{}, false
	}
	if v, ok := u.info.Consts[e]; ok {
		return ir.Lit(ir.Int(v)), true
	}
	if v, ok := u.foldInt(e); ok {
		return ir.Lit(ir.Int(v)), true
	}
	if types.IsPointer(t) || types.IsObjectPointer(t) {
		return u.constAddress(e)
	}
	return ir.Init{}, false
}

// constAddress folds an address constant: &x, a function or array name, a
// string literal, or one of those plus a constant displacement.
func (u *unit) constAddress(e ast.Expr) (ir.Init, bool) {
	switch e := stripParens(e).(type) {
	case *ast.UnaryExpr:
		if e.Op.String() == "&" {
			return u.constAddress(e.X)
		}
	case *ast.Ident:
		st := u.lookup(u.name(e))
		if st == nil || st.sym == nil {
			return ir.Init{}, false
		}
		return ir.RelocInit(st.sym), true
	case *ast.StringLit:
		if sym := u.stringSymbol(e); sym != nil {
			return ir.RelocInit(sym), true
		}
	case *ast.CastExpr:
		return u.constAddress(e.X)
	}
	return ir.Init{}, false
}

// constStringArray is `char s[8] = "abc"` at file scope. The literal is the
// object's own contents, truncated or zero-filled to the declared length.
func (u *unit) constStringArray(s *ast.StringLit, t types.Type) (ir.Init, bool) {
	val := analyzer.DecodeString(u.src, s, u.model, func(string) {})
	a := types.AsArray(t)
	if a == nil {
		return ir.Init{}, false
	}
	esz, _ := u.sizeAlign(a.Elem)
	if esz != 1 {
		items := make([]ir.Init, 0, len(val.Data))
		for _, c := range val.Data {
			items = append(items, ir.Lit(ir.Int(int64(c))))
		}
		return u.padTo(items, u.arrayLen(t, a)), true
	}
	b := make([]byte, 0, len(val.Data))
	for _, c := range val.Data[:len(val.Data)-1] {
		b = append(b, byte(c))
	}
	n := u.arrayLen(t, a)
	if int64(len(b)+1) > n {
		b = b[:n]
	}
	return ir.Str(string(b)), true
}

// arrayLen is how many elements the declared object holds — the stated
// length, or the one the initializer settled.
func (u *unit) arrayLen(t types.Type, a *types.Array) int64 {
	if a.Form == types.FixedArray {
		return a.Len
	}
	esz, _ := u.sizeAlign(a.Elem)
	total, _ := u.sizeAlign(t)
	if esz == 0 {
		return 0
	}
	return int64(total / esz)
}

// padTo makes a positional initializer exactly as long as the object,
// because §19.10 compares them element for element.
func (u *unit) padTo(items []ir.Init, n int64) ir.Init {
	for int64(len(items)) < n {
		items = append(items, ir.ZeroInit)
	}
	if int64(len(items)) > n {
		items = items[:n]
	}
	return ir.List(items...)
}

// constFill is fill's constant twin: it consumes initializers from the
// cursor and produces the value of one object.
func (u *unit) constFill(t types.Type, c *initCursor) (ir.Init, bool) {
	if c.done() {
		return ir.ZeroInit, true
	}
	if !isAggregate(t) {
		it := c.items[c.i]
		c.i++
		return u.constOne(t, it.Value)
	}

	if it := c.peek(); it != nil && len(it.Designators) == 0 {
		if _, braced := it.Value.(*ast.InitList); braced {
			c.i++
			return u.constInit(it.Value, t)
		}
		if s, ok := stripParens(it.Value).(*ast.StringLit); ok && !s.Object && types.IsArray(t) {
			c.i++
			return u.constStringArray(s, t)
		}
		if !types.IsArray(t) && types.IsRecord(u.typeOf(it.Value)) {
			c.i++
			return ir.Init{}, false // a struct-valued constant has no spelling
		}
	}

	switch {
	case types.IsArray(t):
		return u.constArray(types.AsArray(t), t, c)
	case types.IsRecord(t):
		return u.constRecord(types.AsRecord(t), c)
	}
	return ir.Init{}, false
}

// constOne folds one initializer into one subobject.
func (u *unit) constOne(t types.Type, v ast.Expr) (ir.Init, bool) {
	if _, ok := v.(*ast.InitList); ok {
		return u.constInit(v, t)
	}
	return u.constScalar(v, t)
}

func (u *unit) constArray(a *types.Array, t types.Type, c *initCursor) (ir.Init, bool) {
	n := u.arrayLen(t, a)
	items := make([]ir.Init, n)
	for i := range items {
		items[i] = ir.ZeroInit
	}
	idx := int64(0)
	for !c.done() {
		it := c.peek()
		if d, ok := designator(it); ok {
			ix, isIndex := d.(*ast.IndexDesignator)
			if !isIndex {
				break
			}
			v, ok := u.foldInt(ix.Index)
			if !ok {
				return ir.Init{}, false
			}
			idx = v
			consumeDesignator(it)
		}
		if idx >= n {
			break
		}
		v, ok := u.constFill(a.Elem, c)
		if !ok {
			return ir.Init{}, false
		}
		items[idx] = v
		idx++
	}
	return ir.List(items...), true
}

func (u *unit) constRecord(r *types.Record, c *initCursor) (ir.Init, bool) {
	st, ok := u.recordType(r)
	if !ok {
		return ir.Init{}, false
	}
	// The VIR type may carry a tail field the C record does not, which the
	// initializer has to account for.
	items := make([]ir.Init, len(st.Fields()))
	for i := range items {
		items[i] = ir.ZeroInit
	}
	i := 0
	for !c.done() && i < len(r.Fields) {
		it := c.peek()
		if d, ok := designator(it); ok {
			fd, isField := d.(*ast.FieldDesignator)
			if !isField {
				break
			}
			name := u.name(fd.Name)
			found := -1
			for j := range r.Fields {
				if r.Fields[j].Name == name {
					found = j
					break
				}
			}
			if found < 0 {
				return ir.Init{}, false
			}
			i = found
			consumeDesignator(it)
		}
		v, ok := u.constFill(r.Fields[i].Type, c)
		if !ok {
			return ir.Init{}, false
		}
		items[i] = v
		if r.Union {
			// A union's value is its first member's, and VIR writes only
			// that one: the rest of the object is what the member does not
			// reach.
			return ir.List(items[:1]...), true
		}
		i++
	}
	return ir.List(items...), true
}

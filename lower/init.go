package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/analyzer"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// Initializers, which are the one place C's syntax and its object model
// disagree about shape.
//
// `struct { int a; struct { int b, c; } d; } s = { 1, 2, 3 };` has three
// values and two levels, and §6.7.9p17 says how to reconcile them: the
// initializers are consumed in order and the *object* is descended into,
// brace by brace, until a scalar wants one. That is what fill below does,
// and everything else here serves it.
//
// A braced initializer also zeroes what it does not mention (§6.7.9p21), so
// an object with one is cleared before anything is written. That is one
// memset rather than a store per hole, and it is also what makes a partly
// designated initializer correct without tracking which slots were filled.

// initLocal writes a declaration's initializer into an automatic object.
func (u *unit) initLocal(addr ir.Ptr, t types.Type, init ast.Expr) {
	if list, ok := init.(*ast.InitList); ok {
		u.zeroObject(addr, t)
		c := &initCursor{items: list.Items}
		// descend, not fill: these are the object's *own* braces, already
		// opened. fill's first question is whether the next item's braces
		// belong to the object in front of it, and here the answer is no —
		// they belong to its first subobject.
		u.descend(addr, t, c, list)
		return
	}
	u.initScalarOrCopy(addr, t, init)
}

// initScalarOrCopy handles an initializer that is not a braced list: a
// string literal into a character array, a whole-aggregate assignment, or an
// ordinary scalar.
func (u *unit) initScalarOrCopy(addr ir.Ptr, t types.Type, init ast.Expr) {
	if s, ok := stripParens(init).(*ast.StringLit); ok && !s.Object && types.IsArray(t) {
		u.initFromString(addr, t, s)
		return
	}
	if isAggregate(t) {
		if p, ok := u.rvalue(init).(ir.Ptr); ok {
			u.copyAggregate(addr, p, t)
		}
		return
	}
	v := u.rvalue(init)
	if v == nil {
		return
	}
	u.storeTo(addr, u.convert(v, u.typeOf(init), t), t)
}

// initFromString is `char s[] = "abc"` — the one initializer that is neither
// a list nor an assignment.
//
// The characters are copied into the object. A declared length shorter than
// the literal drops the rest, terminator included (§6.7.9p14); a longer one
// zeroes what is left over, like any other partial initializer.
func (u *unit) initFromString(addr ir.Ptr, t types.Type, s *ast.StringLit) {
	val := analyzer.DecodeString(u.src, s, u.model, func(string) {})
	var elem types.Type = types.Typ(types.Char)
	if a := types.AsArray(t); a != nil {
		elem = a.Elem
	}
	esz, _ := u.sizeAlign(elem)
	total, _ := u.sizeAlign(t)
	b := u.fn.cur

	n := int64(len(val.Data)) * int64(esz)
	if n > int64(total) {
		n = int64(total)
	}
	if n < int64(total) {
		// The tail, which §6.7.9p21 zeroes. Clearing it first is one
		// instruction; clearing it after would have to know where the
		// copy stopped.
		b.MemSet(addr, b.I32.Const(0), b.I64.Const(int64(total)))
	}
	if int64(esz) == 1 {
		// The characters already exist as a read-only object — that is
		// what a string literal is — so this is a copy and not a store
		// per byte.
		bs := make([]byte, 0, len(val.Data))
		for _, c := range val.Data {
			bs = append(bs, byte(c))
		}
		src, ok := u.cstring(string(bs[:len(bs)-1])).(ir.Ptr)
		if !ok {
			return
		}
		b.MemCpy(addr, src, b.I64.Const(n))
		return
	}
	for i, c := range val.Data {
		off := int64(i) * int64(esz)
		if off+int64(esz) > n {
			break
		}
		u.storeTo(b.Ptr.Add(addr, b.I64.Const(off)), u.constOf(int64(c), elem), elem)
	}
}

// zeroObject clears an object a braced initializer is about to fill.
func (u *unit) zeroObject(addr ir.Ptr, t types.Type) {
	size, _ := u.sizeAlign(t)
	if size == 0 {
		return
	}
	b := u.fn.cur
	b.MemSet(addr, b.I32.Const(0), b.I64.Const(int64(size)))
}

// initCursor is a position in one brace's list of initializers. fill takes
// items from it and does not put them back, which is what makes brace
// elision work: a nested object simply keeps consuming.
type initCursor struct {
	items []*ast.InitItem
	i     int
}

func (c *initCursor) done() bool { return c.i >= len(c.items) }

func (c *initCursor) peek() *ast.InitItem {
	if c.done() {
		return nil
	}
	return c.items[c.i]
}

// fill writes as much of one object as the cursor's items describe.
//
// It is §6.7.9's "current object" walk. A scalar takes one initializer; an
// aggregate takes a braced list whole, or — when the next item is not braced
// — descends into its subobjects and lets each take what it needs.
func (u *unit) fill(addr ir.Ptr, t types.Type, c *initCursor, at ast.Node) {
	if c.done() {
		return
	}
	if !isAggregate(t) {
		it := c.items[c.i]
		c.i++
		u.fillOne(addr, t, it.Value)
		return
	}

	// A braced list for this object initializes it whole, and so does an
	// expression of the object's own type — `struct S a[2] = { s0, s1 }`
	// assigns rather than descends.
	if it := c.peek(); it != nil && len(it.Designators) == 0 {
		if _, braced := it.Value.(*ast.InitList); braced {
			c.i++
			u.initLocal(addr, t, it.Value)
			return
		}
		// A character array takes a string literal whole, wherever it
		// stands — including as one member of an enclosing brace.
		if s, ok := stripParens(it.Value).(*ast.StringLit); ok && !s.Object && types.IsArray(t) {
			c.i++
			u.initFromString(addr, t, s)
			return
		}
		if !types.IsArray(t) && types.IsRecord(u.typeOf(it.Value)) {
			c.i++
			u.initScalarOrCopy(addr, t, it.Value)
			return
		}
	}

	u.descend(addr, t, c, at)
}

// descend writes an object from the initializers that follow, without asking
// whether the next one's braces are for the object itself.
//
// The distinction is §6.7.9p17's, and it is the whole of how nested braces
// work. `int m[2][2] = { {1,2}, {3,4} }` opens the array's own braces; the
// items inside them are for the *elements*, and treating the first as
// another initializer for the array itself leaves every element after m[0][0]
// at zero — which compiles, runs, and is wrong.
func (u *unit) descend(addr ir.Ptr, t types.Type, c *initCursor, at ast.Node) {
	switch {
	case types.IsArray(t):
		u.fillArray(addr, types.AsArray(t), c, at)
	case types.IsRecord(t):
		u.fillRecord(addr, types.AsRecord(t), c, at)
	case c.done():
	default:
		// A scalar in its own braces: `int x = { 5 }`.
		it := c.peek()
		c.i++
		u.fillOne(addr, t, it.Value)
	}
}

// fillOne writes one initializer into one subobject.
func (u *unit) fillOne(addr ir.Ptr, t types.Type, v ast.Expr) {
	if list, ok := v.(*ast.InitList); ok {
		u.zeroObject(addr, t)
		sub := &initCursor{items: list.Items}
		u.fill(addr, t, sub, list)
		return
	}
	u.initScalarOrCopy(addr, t, v)
}

// fillArray walks an array's elements, honouring [n] designators.
func (u *unit) fillArray(addr ir.Ptr, a *types.Array, c *initCursor, at ast.Node) {
	esz, _ := u.sizeAlign(a.Elem)
	b := u.fn.cur
	idx := int64(0)
	for !c.done() {
		it := c.peek()
		if d, ok := designator(it); ok {
			ix, isIndex := d.(*ast.IndexDesignator)
			if !isIndex {
				return // a .field designator belongs to an enclosing record
			}
			n, ok := u.foldInt(ix.Index)
			if !ok {
				u.errorf(ix, "an array designator must be a constant expression")
				return
			}
			idx = n
			consumeDesignator(it)
		}
		if a.Form == types.FixedArray && idx >= a.Len {
			return
		}
		elem := b.Ptr.Add(addr, b.I64.Const(idx*int64(esz)))
		u.fill(elem, a.Elem, c, at)
		// An array whose length the initializer decides has room for
		// whatever the list holds: the slot was sized from the same
		// count, so nothing here can overrun it.
		idx++
	}
}

// fillRecord walks a struct's members, honouring .name designators. A union
// takes one member: the designated one, or the first.
func (u *unit) fillRecord(addr ir.Ptr, r *types.Record, c *initCursor, at ast.Node) {
	offs, ok := u.model.FieldOffsets(r)
	if !ok {
		u.unsupported(at, "an initializer for an incomplete record")
		return
	}
	b := u.fn.cur
	i := 0
	for !c.done() {
		it := c.peek()
		// The bound is checked after the designator, not before it: a
		// designator names where to write, and it may name a member the
		// positional walk has already passed. `{ .y = 7, .x = 3 }` sets y,
		// runs off the end, and then goes back for x.
		if _, ok := designator(it); !ok && i >= len(r.Fields) {
			return
		}
		if d, ok := designator(it); ok {
			fd, isField := d.(*ast.FieldDesignator)
			if !isField {
				return // an [n] designator belongs to an enclosing array
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
				// A designator naming no member of *this* record belongs
				// to an enclosing one, and the walk inside a member has to
				// stop rather than report. One that names nothing anywhere
				// is the analyzer's to report.
				return
			}
			i = found
			consumeDesignator(it)
		}
		if i >= len(r.Fields) {
			return
		}
		f := r.Fields[i]
		if f.BitField {
			// A bit-field is written into its allocation unit rather than
			// to an address of its own. A zero-width one initializes
			// nothing: it is padding with a declaration.
			if f.Width > 0 {
				u.fillBitField(addr, r, i, c, at)
			}
			if r.Union {
				return
			}
			i++
			continue
		}
		u.fill(b.Ptr.Add(addr, b.I64.Const(offs[i])), f.Type, c, at)
		if r.Union {
			return
		}
		i++
	}
}

// designator is an item's next designator, if it still has one.
func designator(it *ast.InitItem) (ast.Node, bool) {
	if len(it.Designators) == 0 {
		return nil, false
	}
	return it.Designators[0], true
}

// consumeDesignator drops the designator just applied, so that a nested one
// — `.a[2].b = x` — is seen by the object it names.
func consumeDesignator(it *ast.InitItem) { it.Designators = it.Designators[1:] }

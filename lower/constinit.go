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
		// constDescend, not constFill: these are the object's own braces,
		// already opened, so the items inside belong to its subobjects.
		// See init.go's descend for what goes wrong otherwise.
		return u.constDescend(t, c)
	}
	return u.constScalar(e, t)
}

// constScalar folds one initializer that is not a braced list.
func (u *unit) constScalar(e ast.Expr, t types.Type) (ir.Init, bool) {
	if b, ok := stripParens(e).(*ast.BlockLit); ok {
		return u.blockConst(b, u.typeOf(b))
	}
	if cl, ok := stripParens(e).(*ast.CompoundLit); ok && !isAggregate(t) {
		// `&(struct S){…}` folds to the address of the object; the address
		// is taken by constAddress below, which descends to this.
		return u.compoundConst(cl)
	}
	if s, ok := stripParens(e).(*ast.StringLit); ok {
		if s.Object {
			// @"…" is a constant object in the image, so its address is a
			// link-time one like any other global's.
			if sym := u.constantStringSymbol(s); sym != nil {
				return ir.RelocInit(sym), true
			}
			return ir.Init{}, false
		}
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
		sym := u.symOf(st)
		if sym == nil {
			return ir.Init{}, false
		}
		return ir.RelocInit(sym), true
	case *ast.StringLit:
		if sym := u.stringSymbol(e); sym != nil {
			return ir.RelocInit(sym), true
		}
	case *ast.CastExpr:
		return u.constAddress(e.X)
	case *ast.CompoundLit:
		return u.compoundConst(e)
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

	return u.constDescend(t, c)
}

// constDescend builds an object from the initializers that follow, without
// asking whether the next one's braces are for the object itself.
func (u *unit) constDescend(t types.Type, c *initCursor) (ir.Init, bool) {
	switch {
	case types.IsArray(t):
		return u.constArray(types.AsArray(t), t, c)
	case types.IsRecord(t):
		return u.constRecord(types.AsRecord(t), c)
	case c.done():
		return ir.ZeroInit, true
	}
	// A scalar in its own braces: `int x = { 5 }`.
	it := c.items[c.i]
	c.i++
	return u.constOne(t, it.Value)
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
	offs, ok := u.model.FieldOffsets(r)
	if !ok {
		return ir.Init{}, false
	}
	lay, ok := u.virLayout(r, offs)
	if !ok {
		return ir.Init{}, false
	}
	bits := newBitImage(lay)
	i := 0
	for !c.done() {
		it := c.peek()
		// The bound is checked after the designator, not before it: see
		// init.go's fillRecord.
		if _, ok := designator(it); !ok && i >= len(r.Fields) {
			break
		}
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
				// A designator naming no member of *this* record belongs
				// to an enclosing one: `{ .a = 1, .d.c = 7, .name = "ok" }`
				// returns to the outer struct after .d.c, and the walk
				// inside .d has to stop rather than fail. A designator that
				// names nothing anywhere is the analyzer's to report.
				break
			}
			i = found
			consumeDesignator(it)
		}
		if i >= len(r.Fields) {
			break
		}
		if r.Fields[i].BitField {
			// A bit-field's constant is packed into the bytes of its
			// allocation unit rather than given a field of its own.
			if r.Fields[i].Width > 0 && !bits.pack(u, r, i, c) {
				return ir.Init{}, false
			}
			if r.Union {
				break
			}
			i++
			continue
		}
		v, ok := u.constFill(r.Fields[i].Type, c)
		if !ok {
			return ir.Init{}, false
		}
		items[lay.slot[i]] = v
		if r.Union {
			// One member of a union is initialized and the rest of the
			// object is whatever that member does not reach. Which member
			// is the point: `{.d = 2.5}` names the second, and a positional
			// list could only ever say the first. §19.10 takes a named-field
			// initializer for exactly this.
			return ir.Fields(ir.Val(st.Fields()[lay.slot[i]].Name, v)), true
		}
		i++
	}
	bits.emit(items)
	return ir.List(items...), true
}

// bitImage is the bytes a record's bit-field constants pack into.
//
// A file-scope initializer is a value rather than code, so there is no
// read-modify-write to emit: the bits are or'd into a buffer here and the
// buffer becomes the array field recordType appended for that range.
//
// The packing is little-endian, which is what the load in bitfield.go reads
// back: the allocation unit is loaded as an integer, so bit k of it is bit
// k%8 of byte k/8. Every target objv emits for says `endian little`.
type bitImage struct {
	lay  *virLayout
	bufs [][]byte // one per range, nil until something writes to it
}

func newBitImage(lay *virLayout) *bitImage {
	return &bitImage{lay: lay, bufs: make([][]byte, len(lay.ranges))}
}

// pack folds the next initializer into the bit-field at index i.
func (b *bitImage) pack(u *unit, r *types.Record, i int, c *initCursor) bool {
	places, ok := u.model.BitPlaces(r)
	if !ok {
		return false
	}
	it := c.peek()
	if it == nil {
		return true
	}
	c.i++
	if it.Value == nil {
		return true
	}
	v, ok := u.foldInt(it.Value)
	if !ok {
		return false
	}
	p := places[i]
	bit := p.Off*8 + p.BitOff
	k := b.rangeOf(bit / 8)
	if k < 0 {
		return false
	}
	if b.bufs[k] == nil {
		b.bufs[k] = make([]byte, b.lay.ranges[k][1]-b.lay.ranges[k][0])
	}
	buf := b.bufs[k]
	start := bit - b.lay.ranges[k][0]*8
	for n := int64(0); n < r.Fields[i].Width; n++ {
		if v&(1<<uint(n)) == 0 {
			continue
		}
		bit := start + n
		if bit/8 >= int64(len(buf)) {
			return false
		}
		buf[bit/8] |= 1 << uint(bit%8)
	}
	return true
}

func (b *bitImage) rangeOf(off int64) int {
	for k, rg := range b.lay.ranges {
		if off >= rg[0] && off < rg[1] {
			return k
		}
	}
	return -1
}

// emit writes each range's bytes into the initializer list.
func (b *bitImage) emit(items []ir.Init) {
	for k, buf := range b.bufs {
		if buf == nil {
			continue
		}
		// A byte per element rather than one literal for the run: the
		// field is an array, and §19.10 checks an initializer's shape
		// against the type it is for.
		es := make([]ir.Init, len(buf))
		for i, c := range buf {
			es[i] = ir.Lit(ir.Int(int64(c)))
		}
		items[b.lay.rslot[k]] = ir.List(es...)
	}
}

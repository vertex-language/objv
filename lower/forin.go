package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/types"
)

// Fast enumeration (§7.1), which is a protocol and not a loop form.
//
// `for (id x in c)` sends c -countByEnumeratingWithState:objects:count: and
// walks what comes back: the collection fills in a state structure, points
// itemsPtr at a batch of elements — its own storage, or the buffer the loop
// supplied — and says how many. When the batch runs out the loop asks again,
// and a zero answer ends it. That is why there are two nested loops here and
// only one in the source.
//
// The mutation check is not optional. The collection publishes a counter
// through the state, and the loop compares it against what it read at the
// start: a collection modified mid-walk would otherwise be enumerated
// through a buffer that no longer describes it.
func (u *unit) forIn(s *ast.ForInStmt) {
	if !u.at() {
		return
	}
	u.push()
	defer u.pop()
	u.pushARCScope()
	defer u.popARCScope()

	cp, ok := u.heldForStatement(s.Coll)
	if !ok {
		return
	}

	// The loop variable: declared here in `for (id x in c)`, and an
	// existing lvalue in `for (x in c)`.
	var slot ir.Ptr
	var elemType types.Type
	if s.Decl != nil {
		name := u.name(declName(s.Decl))
		t := u.typeOf(s)
		if t == nil {
			t = types.NewObject(nil)
		}
		elemType = t
		slot = u.slot(t, name)
		u.bind(name, &storage{kind: stLocal, typ: t, addr: slot})
	} else {
		addr, t := u.lvalue(s.X)
		if addr == nil {
			return
		}
		slot, elemType = *addr, t
	}

	b := u.fn.entry
	state := b.Ptr.Alloc(uint64(u.abi.SizeOf(runtime.FastEnumerationState)),
		uint64(u.abi.PtrBytes))
	b.Name(state, "enum_state")
	buf := b.Ptr.Alloc(uint64(runtime.FastEnumerationBatch*u.abi.PtrBytes),
		uint64(u.abi.PtrBytes))
	b.Name(buf, "enum_items")
	mutations := b.Ptr.Alloc(8, 8)
	b.Name(mutations, "enum_mutations")

	cur := u.fn.cur
	cur.MemSet(state, cur.I32.Const(0),
		cur.I64.Const(u.abi.SizeOf(runtime.FastEnumerationState)))

	batch := u.block("forin.batch")   // ask for the next batch
	body := u.block("forin.body")     // one element
	next := u.block("forin.next")     // i++
	done := u.block("forin.done")     // the loop is over
	inner := u.block("forin.inner")   // is there another element in the batch?
	check := u.block("forin.check")   // did the batch come back empty?
	mutate := u.block("forin.mutate") // the collection changed under us

	// The index into the current batch, and how many the batch holds.
	i := b.Ptr.Alloc(8, 8)
	b.Name(i, "forin_i")
	count := b.Ptr.Alloc(8, 8)
	b.Name(count, "forin_count")

	cur.Br(batch.To())

	// Ask the collection for a batch.
	u.fn.cur = batch
	n := u.send(cp, false, runtime.FastEnumerationSelector,
		[]ir.Value{state, buf, u.fn.cur.I64.Const(runtime.FastEnumerationBatch)},
		types.Typ(types.ULong), s)
	nv, ok := n.(ir.I64)
	if !ok {
		u.errorf(s, "internal: the enumeration count is not an i64")
		return
	}
	u.fn.cur.I64.Store(nv, count)
	u.fn.cur.I64.Store(u.fn.cur.I64.Const(0), i)
	u.fn.cur.BrIf(u.fn.cur.I64.Eq(nv, u.fn.cur.I64.Const(0)), done.To(), check.To())

	// A fresh batch: read the mutation counter the collection published, so
	// that the elements of this batch can be checked against it.
	u.fn.cur = check
	mp := u.fn.cur.Ptr.Load(u.fn.cur.Ptr.Add(state,
		u.fn.cur.I64.Const(u.fieldOffset(runtime.FastEnumerationState, "mutationsPtr"))))
	u.fn.cur.I64.Store(u.fn.cur.I64.Load(mp), mutations)
	u.fn.cur.Br(inner.To())

	// Another element in this batch?
	u.fn.cur = inner
	iv := u.fn.cur.I64.Load(i)
	u.fn.cur.BrIf(u.fn.cur.I64.ULt(iv, u.fn.cur.I64.Load(count)),
		body.To(), batch.To())

	// One element. The mutation check comes first, because a collection
	// that changed has already invalidated the pointer about to be read.
	u.fn.cur = body
	mp2 := u.fn.cur.Ptr.Load(u.fn.cur.Ptr.Add(state,
		u.fn.cur.I64.Const(u.fieldOffset(runtime.FastEnumerationState, "mutationsPtr"))))
	same := u.fn.cur.I64.Eq(u.fn.cur.I64.Load(mp2), u.fn.cur.I64.Load(mutations))
	live := u.block("forin.live")
	u.fn.cur.BrIf(same, live.To(), mutate.To())

	u.fn.cur = mutate
	mut := u.extern(runtime.EnumerationMutation, ir.NewSig().Param(ir.TypePtr))
	u.fn.cur.Call(mut, cp)
	u.fn.cur.Br(live.To())

	u.fn.cur = live
	items := u.fn.cur.Ptr.Load(u.fn.cur.Ptr.Add(state,
		u.fn.cur.I64.Const(u.fieldOffset(runtime.FastEnumerationState, "itemsPtr"))))
	idx := u.fn.cur.I64.Load(i)
	elem := u.fn.cur.Ptr.Load(u.fn.cur.Ptr.Add(items,
		u.fn.cur.I64.Mul(idx, u.fn.cur.I64.Const(u.abi.PtrBytes))))
	u.storeTo(slot, elem, elemType)

	// `continue` goes to the next element and `break` leaves the whole
	// loop, which is what makes the two-level structure invisible in the
	// source.
	u.loop(done, next, s.Body)

	u.fn.cur = next
	u.fn.cur.I64.Store(u.fn.cur.I64.Add(u.fn.cur.I64.Load(i), u.fn.cur.I64.Const(1)), i)
	u.fn.cur.Br(inner.To())

	u.fn.cur = done
}

// fieldOffset is one metadata field's byte offset, which the caller needs as
// a constant to add to an address.
func (u *unit) fieldOffset(fields []runtime.Field, name string) int64 {
	off, ok := u.abi.OffsetOf(fields, name)
	if !ok {
		return 0
	}
	return off
}
